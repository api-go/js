package js

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"sync"
	"sync/atomic"

	"apigo.cc/go/jsmod"
	"apigo.cc/go/log"
	"github.com/dop251/goja"
)

type scriptEntry struct {
	name    string
	code    string
	version int64
}

type vmInstance struct {
	runtime *goja.Runtime
	version int32
}

// Pool represents an isolated JS execution environment with its own script registry and VM pool.
type Pool struct {
	version   int32
	scripts   []*scriptEntry
	scriptMap map[string]*scriptEntry
	functions map[string]struct{}
	mu        sync.RWMutex
	pool      sync.Pool

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	closed int32
}

// NewPool creates a new isolated JS execution environment.
func NewPool() *Pool {
	p := &Pool{
		scriptMap: make(map[string]*scriptEntry),
		functions: make(map[string]struct{}),
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.pool = sync.Pool{
		New: func() any {
			return &vmInstance{
				runtime: createNewRuntime(),
				version: 0,
			}
		},
	}
	return p
}

// DefaultPool is the global singleton pool.
var DefaultPool = NewPool()

func createNewRuntime() *goja.Runtime {
	vm := goja.New()

	// 1. Inject Native Modules from jsmod
	goObj := vm.NewObject()
	_ = vm.Set("go", goObj)

	modules := jsmod.GetModules()
	for modName, mod := range modules {
		modObj := vm.NewObject()
		for name, val := range mod.Exports {
			isUnsafe := mod.UnsafeList[name]
			if val != nil && reflect.TypeOf(val).Kind() == reflect.Func {
				_ = modObj.Set(name, wrapGoFunc(vm, val, isUnsafe))
			} else {
				_ = modObj.Set(name, vm.ToValue(val))
			}
		}
		_ = goObj.Set(modName, modObj)
	}

	return vm
}

var funcRegex = regexp.MustCompile(`function\s+([a-zA-Z0-9_]+)\s*\(`)
var constFuncRegex = regexp.MustCompile(`(?:const|let|var)\s+([a-zA-Z0-9_]+)\s*=\s*(?:function|\([^)]*\)\s*=>)`)

// Define adds JS code to the pool's registry.
// name and version are optional and used for CheckVersion.
func (p *Pool) Define(code string, args ...any) {
	p.mu.Lock()
	defer p.mu.Unlock()

	name := ""
	version := int64(0)
	if len(args) > 0 {
		if s, ok := args[0].(string); ok {
			name = s
		}
	}
	if len(args) > 1 {
		if v, ok := args[1].(int64); ok {
			version = v
		}
	}

	entry := &scriptEntry{name: name, code: code, version: version}
	p.scripts = append(p.scripts, entry)
	if name != "" {
		p.scriptMap[name] = entry
	}

	// Extract functions for FuncList
	matches := funcRegex.FindAllStringSubmatch(code, -1)
	for _, m := range matches {
		if len(m) > 1 {
			p.functions[m[1]] = struct{}{}
		}
	}
	matches = constFuncRegex.FindAllStringSubmatch(code, -1)
	for _, m := range matches {
		if len(m) > 1 {
			p.functions[m[1]] = struct{}{}
		}
	}

	atomic.AddInt32(&p.version, 1)
}

// CheckVersion returns true if a script with the given name exists and its version is >= the provided version.
func (p *Pool) CheckVersion(name string, version int64) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if entry, ok := p.scriptMap[name]; ok {
		return entry.version >= version
	}
	return false
}

func CheckVersion(name string, version int64) bool {
	return DefaultPool.CheckVersion(name, version)
}

// Call executes a JS function from the pool.
func (p *Pool) Call(ctx context.Context, funcName string, args []any, opts ...CallOption) (any, error) {
	if atomic.LoadInt32(&p.closed) == 1 {
		return nil, fmt.Errorf("js.Pool: pool is closed")
	}

	instance := p.pool.Get().(*vmInstance)
	defer p.pool.Put(instance)

	// Tracking active calls for graceful shutdown
	p.wg.Add(1)
	defer p.wg.Done()

	vm := instance.runtime
	vm.ClearInterrupt()

	// 1. Synchronize scripts if version is behind
	currentVersion := atomic.LoadInt32(&p.version)
	if instance.version < currentVersion {
		p.mu.RLock()
		for i := int(instance.version); i < len(p.scripts); i++ {
			_, err := vm.RunString(p.scripts[i].code)
			if err != nil {
				p.mu.RUnlock()
				return nil, fmt.Errorf("js.sync error at script %d: %w", i, err)
			}
		}
		instance.version = currentVersion
		p.mu.RUnlock()
	}

	// 2. Set Context and default state
	_ = vm.Set("__ctx__", vm.ToValue(ctx))
	_ = vm.Set("__safeMode__", true) // Default is safe
	_ = vm.Set("__logger__", goja.Undefined())

	// Set up context interruption
	if ctx != nil && ctx.Done() != nil {
		stop := make(chan struct{})
		defer close(stop)
		go func() {
			select {
			case <-ctx.Done():
				vm.Interrupt("context canceled")
			case <-stop:
			}
		}()
	}

	// Apply Options
	for _, opt := range opts {
		opt(vm)
	}

	// 3. Get and Call JS Function
	fnVal := vm.Get(funcName)
	if fnVal == nil || goja.IsUndefined(fnVal) {
		return nil, fmt.Errorf("js.Call: function '%s' not found", funcName)
	}

	callable, ok := goja.AssertFunction(fnVal)
	if !ok {
		return nil, fmt.Errorf("js.Call: '%s' is not a function", funcName)
	}

	jsArgs := make([]goja.Value, len(args))
	for i, arg := range args {
		jsArgs[i] = vm.ToValue(arg)
	}

	// 4. Execution with error capture
	var result goja.Value
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("js panic: %v", r)
			}
		}()
		result, err = callable(goja.Undefined(), jsArgs...)
	}()

	if err != nil {
		return nil, err
	}

	return result.Export(), nil
}

// --- Global Proxy Functions ---

func Define(code string, args ...any) {
	DefaultPool.Define(code, args...)
}

func Call(ctx context.Context, funcName string, args []any, opts ...CallOption) (any, error) {
	return DefaultPool.Call(ctx, funcName, args, opts...)
}

// --- Starter Interface Implementation ---

func (p *Pool) Start(ctx context.Context, logger *log.Logger) error {
	// For JS engine, start is mostly for pre-warming or registry checking.
	// We ensure the context is not canceled.
	if p.ctx.Err() != nil {
		p.ctx, p.cancel = context.WithCancel(context.Background())
	}
	atomic.StoreInt32(&p.closed, 0)
	return nil
}

func (p *Pool) Stop(ctx context.Context) error {
	atomic.StoreInt32(&p.closed, 1)
	p.cancel() // Notify any long-running JS that are context-aware

	// Wait for active Call() to finish or context timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("js.Pool: shutdown timeout, active tasks may still be running")
	}
}

func (p *Pool) Status() (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return fmt.Sprintf("scripts: %d, functions: %d, version: %d, closed: %v", len(p.scripts), len(p.functions), p.version, atomic.LoadInt32(&p.closed) == 1), nil
}

// --- Helper types from original file ---

// CallOption allows configuring the JS execution environment.
type CallOption func(vm *goja.Runtime)

// WithSafeMode enables or disables safe mode for the call.
func WithSafeMode(enabled bool) CallOption {
	return func(vm *goja.Runtime) {
		_ = vm.Set("__safeMode__", enabled)
	}
}

// WithLogger injects a custom logger for the call.
func WithLogger(logger *log.Logger) CallOption {
	return func(vm *goja.Runtime) {
		_ = vm.Set("__logger__", vm.ToValue(logger))
	}
}

// FuncList returns the list of all defined JS function names.
func (p *Pool) FuncList() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	list := make([]string, 0, len(p.functions))
	for name := range p.functions {
		list = append(list, name)
	}
	sort.Strings(list)
	return list
}

func FuncList() []string {
	return DefaultPool.FuncList()
}
