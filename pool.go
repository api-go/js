package js

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
		_ = vm.Set(modName, modObj) // Also inject into global
	}

	return vm
}

// --- Error ---

// Error wraps a JS execution error with merged call stacks from JS and Go,
// ordered from innermost (JS error site) to outermost (Go cause chain).
type Error struct {
	Message    string
	CallStacks []string
}

func (e *Error) Error() string { return e.Message }

// --- Validation ---

var namedFuncRe = regexp.MustCompile(`\bfunction\s+[a-zA-Z$_]`)

func validateCode(name, code string) error {
	if strings.TrimSpace(code) == "" {
		return fmt.Errorf("js.Define [%s]: code must not be empty", name)
	}
	if namedFuncRe.MatchString(code) {
		return fmt.Errorf("js.Define [%s]: named function declarations are not allowed, use an anonymous function expression", name)
	}
	return nil
}

// --- Define ---

// Define registers a JS function identified by name. code must be an anonymous
// function expression (arrow or function literal). version is used by
// CheckVersion for cache invalidation.
func (p *Pool) Define(name string, code string, version int64) error {
	if name == "" {
		return fmt.Errorf("js.Define: name must not be empty")
	}
	if err := validateCode(name, code); err != nil {
		return err
	}

	wrapped := fmt.Sprintf("globalThis[%q] = (%s);", name, code)
	program, err := goja.Compile(name, wrapped, false)
	if err != nil {
		return fmt.Errorf("js.Define [%s]: invalid JavaScript: %w", name, err)
	}
	validationVM := goja.New()
	validationTimer := time.AfterFunc(time.Second, func() { validationVM.Interrupt("definition validation timed out") })
	if _, err = validationVM.RunProgram(program); err != nil {
		validationTimer.Stop()
		return fmt.Errorf("js.Define [%s]: definition failed: %w", name, err)
	}
	validationTimer.Stop()
	if _, ok := goja.AssertFunction(validationVM.Get(name)); !ok {
		return fmt.Errorf("js.Define [%s]: code must evaluate to a function", name)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	entry := &scriptEntry{name: name, code: wrapped, version: version}
	p.scripts = append(p.scripts, entry)
	p.scriptMap[name] = entry
	p.functions[name] = struct{}{}
	atomic.AddInt32(&p.version, 1)
	return nil
}

// Define is the global convenience wrapper.
func Define(name string, code string, version int64) error {
	return DefaultPool.Define(name, code, version)
}

// --- CheckVersion ---

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

// --- Error formatting ---

// parseJSFrame parses a single stack trace line from Goja.
// Format is:
//
//	named:   \tat funcName (src:line:col) (optionalPC)
//	anon:    \tat src:line:col (optionalPC)
func parseJSFrame(line string) (src, lineNum, col string, ok bool) {
	if !strings.HasPrefix(line, "\t") {
		return "", "", "", false
	}
	// Strip leading tab
	line = line[1:]
	if !strings.HasPrefix(line, "at ") {
		return "", "", "", false
	}
	// Strip "at "
	line = line[3:]

	// Strip PC suffix like " (12)" or "(12)" at the end of the line
	if idx := strings.LastIndexByte(line, '('); idx != -1 && strings.HasSuffix(line, ")") {
		inner := line[idx+1 : len(line)-1]
		isPC := true
		for i := 0; i < len(inner); i++ {
			if inner[i] < '0' || inner[i] > '9' {
				isPC = false
				break
			}
		}
		if isPC && len(inner) > 0 {
			if idx > 0 && line[idx-1] == ' ' {
				line = line[:idx-1]
			} else {
				line = line[:idx]
			}
		}
	}

	// Try named format first: "funcName (src:line:col)"
	if idx := strings.Index(line, " ("); idx != -1 && strings.HasSuffix(line, ")") {
		srcLineCol := line[idx+2 : len(line)-1]
		return parseSrcLineCol(srcLineCol)
	}

	// Try anonymous format: "src:line:col"
	return parseSrcLineCol(line)
}

func parseSrcLineCol(s string) (src, lineNum, col string, ok bool) {
	colIdx := strings.LastIndexByte(s, ':')
	if colIdx == -1 {
		return "", "", "", false
	}
	lineIdx := strings.LastIndexByte(s[:colIdx], ':')
	if lineIdx == -1 {
		return "", "", "", false
	}
	return s[:lineIdx], s[lineIdx+1 : colIdx], s[colIdx+1:], true
}

// buildJSError constructs an *Error from a raw goja error, extracting
// JS stack frames and Go cause chain into CallStacks (inner to outer).
func buildJSError(funcName string, err error) *Error {
	if err == nil {
		return nil
	}

	if exc, ok := err.(*goja.Exception); ok {
		return buildExceptionError(funcName, exc)
	}
	if intErr, ok := err.(*goja.InterruptedError); ok {
		return &Error{Message: intErr.String()}
	}

	return &Error{Message: err.Error()}
}

func buildExceptionError(funcName string, exc *goja.Exception) *Error {
	// 1. Extract the pure error description for Message.
	//    Goja produces "Error: msg" or "GoError: [func at file:line] msg"
	errorSummary := exc.Value().String()
	errorSummary = strings.TrimPrefix(errorSummary, "GoError: ")

	goFileLine := ""
	if strings.HasPrefix(errorSummary, "[") {
		if idx := strings.Index(errorSummary, "] "); idx != -1 {
			goTag := errorSummary[1:idx] // "funcName at file:line"
			errorSummary = errorSummary[idx+2:]
			if pos := strings.LastIndex(goTag, " at "); pos != -1 {
				goFileLine = goTag[pos+4:] // just "file:line"
			}
		}
	}

	// 2. Parse JS stack frames: keep only source:line:col, skip bridge internals.
	s := exc.String()
	var jsFrames []string
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		src, lineno, col, ok := parseJSFrame(line)
		if !ok {
			continue
		}
		if src == "native" || strings.Contains(src, "apigo.cc/go/js.") || strings.Contains(src, "apigo.cc/go/js/") {
			continue
		}
		jsFrames = append(jsFrames, fmt.Sprintf("%s:%s:%s", src, lineno, col))
	}

	// Find if there is a *jsmod.Error in the error chain
	var jsmodErr *jsmod.Error
	if unwrapped := exc.Unwrap(); unwrapped != nil {
		for curr := unwrapped; curr != nil; {
			if je, ok := curr.(*jsmod.Error); ok {
				jsmodErr = je
				break
			}
			if u, ok := curr.(interface{ Unwrap() error }); ok {
				curr = u.Unwrap()
			} else {
				break
			}
		}
	}

	// 3. Build CallStacks: Go cause first (root), then JS frames (effect)
	var callStacks []string
	// Only prepend the static goFileLine if we do not have a dynamic jsmodErr.
	// This removes redundant or inaccurate entry-point lines from the stack.
	if jsmodErr == nil && goFileLine != "" {
		callStacks = append(callStacks, goFileLine)
	}
	callStacks = append(callStacks, jsFrames...)

	// 4. For errors/panics: append Go caller stacks if available
	if unwrapped := exc.Unwrap(); unwrapped != nil {
		if jsmodErr != nil {
			callStacks = append(callStacks, jsmodErr.CallStacks...)
		} else if stackErr, ok := unwrapped.(interface{ Stack() string }); ok {
			for _, frame := range strings.Split(stackErr.Stack(), "\n") {
				frame = strings.TrimSpace(frame)
				if frame == "" || strings.HasPrefix(frame, "panic(") || strings.HasPrefix(frame, ">") {
					continue
				}
				// Extract file:line from Go trace format "/path/to/file.go:123"
				if idx := strings.Index(frame, ".go:"); idx != -1 {
					end := idx + len(".go:")
					for end < len(frame) && frame[end] >= '0' && frame[end] <= '9' {
						end++
					}
					callStacks = append(callStacks, frame[:end])
				}
			}
		}
	}

	return &Error{
		Message:    errorSummary,
		CallStacks: callStacks,
	}
}

// --- Call ---

// Call executes a JS function from the pool.
// On error, returns *Error with merged JS + Go call stacks.
func (p *Pool) Call(funcName string, timeout time.Duration, injects map[string]any, args ...any) (any, *Error) {
	if atomic.LoadInt32(&p.closed) == 1 {
		return nil, &Error{Message: "js.Pool: pool is closed"}
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
			_, err := vm.RunScript(p.scripts[i].name, p.scripts[i].code)
			if err != nil {
				p.mu.RUnlock()
				syncErr := buildJSError("", err)
				return nil, &Error{
					Message:    fmt.Sprintf("js.sync error [%s]", p.scripts[i].name),
					CallStacks: syncErr.CallStacks,
				}
			}
		}
		instance.version = currentVersion
		p.mu.RUnlock()
	}

	// 2. Prepare Context
	execCtx := jsmod.NewContext(p.ctx, injects)
	var cancel context.CancelFunc
	if timeout > 0 {
		execCtx, cancel = context.WithTimeout(execCtx, timeout)
		defer cancel()
	}

	// 3. Set VM environment
	_ = vm.Set("__ctx__", vm.ToValue(execCtx))

	// 4. Set up interruption
	stopInterrupter := make(chan struct{})
	defer close(stopInterrupter)
	go func() {
		select {
		case <-execCtx.Done():
			reason := "execution timeout/canceled"
			if p.ctx.Err() != nil {
				reason = "application stopping"
			}
			vm.Interrupt(reason)
		case <-stopInterrupter:
		}
	}()

	// 5. Get and Call JS Function
	fnVal := vm.Get(funcName)
	if fnVal == nil || goja.IsUndefined(fnVal) {
		return nil, &Error{Message: fmt.Sprintf("js.Call: function '%s' not found", funcName)}
	}

	callable, ok := goja.AssertFunction(fnVal)
	if !ok {
		return nil, &Error{Message: fmt.Sprintf("js.Call: '%s' is not a function", funcName)}
	}

	jsArgs := make([]goja.Value, len(args))
	for i, arg := range args {
		jsArgs[i] = vm.ToValue(arg)
	}

	// 6. Execution with error capture
	var result goja.Value
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				if exc, ok := r.(*goja.Exception); ok {
					err = exc
				} else if intErr, ok := r.(*goja.InterruptedError); ok {
					err = intErr
				} else {
					err = fmt.Errorf("js panic: %v", r)
				}
			}
		}()
		result, err = callable(goja.Undefined(), jsArgs...)
	}()

	if err != nil {
		return nil, buildJSError(funcName, err)
	}

	return result.Export(), nil
}

// Call is the global convenience wrapper.
func Call(funcName string, timeout time.Duration, injects map[string]any, args ...any) (any, *Error) {
	return DefaultPool.Call(funcName, timeout, injects, args...)
}

// --- Starter Interface Implementation ---

func (p *Pool) Start(ctx context.Context, logger *log.Logger) error {
	if p.ctx.Err() != nil {
		p.ctx, p.cancel = context.WithCancel(context.Background())
	}
	atomic.StoreInt32(&p.closed, 0)
	return nil
}

func (p *Pool) Stop(ctx context.Context) error {
	atomic.StoreInt32(&p.closed, 1)
	p.cancel()

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

// FuncList returns the list of all registered function names.
func (p *Pool) FuncList() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	list := make([]string, 0, len(p.scriptMap))
	for name := range p.scriptMap {
		list = append(list, name)
	}
	sort.Strings(list)
	return list
}

func FuncList() []string {
	return DefaultPool.FuncList()
}
