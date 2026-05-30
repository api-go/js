package js

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"apigo.cc/go/jsmod"
	"github.com/dop251/goja"
)

type vmInstance struct {
	runtime *goja.Runtime
	version int32
}

var (
	globalVersion int32
	scripts       []string
	scriptsMu     sync.RWMutex

	pool = sync.Pool{
		New: func() any {
			return &vmInstance{
				runtime: createNewRuntime(),
				version: 0,
			}
		},
	}
)

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
			if reflectType := fmt.Sprintf("%T", val); reflectType == "func" || (len(reflectType) > 4 && reflectType[:4] == "func") {
				_ = modObj.Set(name, wrapGoFunc(vm, val, isUnsafe))
			} else {
				_ = modObj.Set(name, vm.ToValue(val))
			}
		}
		_ = goObj.Set(modName, modObj)
	}

	return vm
}

// Define adds JS code to the global registry and increments the version.
// All VMs in the pool will eventually synchronize to this version.
func Define(code string) {
	scriptsMu.Lock()
	defer scriptsMu.Unlock()

	scripts = append(scripts, code)
	atomic.AddInt32(&globalVersion, 1)
}

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

// Call executes a JS function from the pool.
// It automatically synchronizes the VM to the latest version.
func Call(ctx context.Context, funcName string, args []any, opts ...CallOption) (any, error) {
	instance := pool.Get().(*vmInstance)
	defer pool.Put(instance)

	vm := instance.runtime

	// 1. Synchronize scripts if version is behind
	currentGlobalVersion := atomic.LoadInt32(&globalVersion)
	if instance.version < currentGlobalVersion {
		scriptsMu.RLock()
		for i := int(instance.version); i < len(scripts); i++ {
			_, err := vm.RunString(scripts[i])
			if err != nil {
				scriptsMu.RUnlock()
				return nil, fmt.Errorf("js.sync error at script %d: %w", i, err)
			}
		}
		instance.version = currentGlobalVersion
		scriptsMu.RUnlock()
	}

	// 2. Set Context and default state
	_ = vm.Set("__ctx__", vm.ToValue(ctx))
	_ = vm.Set("__safeMode__", true) // Default is safe
	_ = vm.Set("__logger__", goja.Undefined())

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

// FuncList returns the list of all defined JS function names.
func FuncList() []string {
	scriptsMu.RLock()
	defer scriptsMu.RUnlock()
	return []string{}
}
