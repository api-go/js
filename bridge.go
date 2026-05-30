package js

import (
	"context"
	"fmt"
	"reflect"

	"apigo.cc/go/cast"
	"github.com/dop251/goja"
)

// wrapGoFunc converts a standard Go function into a goja.Callable.
// It handles context injection and automatic type conversion via go/cast.
func wrapGoFunc(vm *goja.Runtime, fn any) goja.Value {
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func {
		panic(fmt.Sprintf("js.bridge: expected func, got %T", fn))
	}

	t := v.Type()

	return vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// 1. Prepare Arguments
		numIn := t.NumIn()
		goArgs := make([]reflect.Value, numIn)
		jsArgs := call.Arguments
		jsArgIdx := 0

		// Handle context.Context injection
		startIdx := 0
		if numIn > 0 && t.In(0).Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
			// Inject context from VM's current execution context if available, 
			// otherwise use Background. (We can improve this by storing ctx in VM's data)
			ctx := context.Background()
			if c, ok := vm.Get("__ctx__").Export().(context.Context); ok {
				ctx = c
			}
			goArgs[0] = reflect.ValueOf(ctx)
			startIdx = 1
		}

		for i := startIdx; i < numIn; i++ {
			argType := t.In(i)
			goArgs[i] = reflect.New(argType).Elem()

			if jsArgIdx < len(jsArgs) {
				jsVal := jsArgs[jsArgIdx]
				// Use goja's Export() to get a Go-compatible value
				exported := jsVal.Export()

				// First, try direct assignment to preserve pointer identity (Host Object fidelity)
				expV := reflect.ValueOf(exported)
				if expV.IsValid() && expV.Type().AssignableTo(argType) {
					goArgs[i].Set(expV)
				} else {
					// Otherwise, use go/cast to convert to the target Go type (frictionless)
					cast.Convert(goArgs[i].Addr().Interface(), exported)
				}
				jsArgIdx++
			} else {
				// If JS args are missing, cast will keep it as zero value (frictionless)
			}
		}

		// 2. Call the Go function
		// We use recover to catch Go panics and turn them into JS errors
		var results []reflect.Value
		var recovered any
		func() {
			defer func() { recovered = recover() }()
			results = v.Call(goArgs)
		}()

		if recovered != nil {
			panic(vm.NewGoError(fmt.Errorf("go panic: %v", recovered)))
		}

		// 3. Process Results
		if len(results) == 0 {
			return goja.Undefined()
		}

		// If the last return value is an error, check it
		if len(results) > 0 {
			last := results[len(results)-1]
			if last.Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
				if !last.IsNil() {
					err := last.Interface().(error)
					panic(vm.NewGoError(err))
				}
				// If it's an error but nil, exclude it from normal results if it's the only result
				if len(results) == 1 {
					return goja.Undefined()
				}
				// Otherwise, we take results up to len-1
				results = results[:len(results)-1]
			}
		}

		if len(results) == 1 {
			return vm.ToValue(results[0].Interface())
		}

		// Multiple return values (other than the handled error) are returned as a JS array
		resSlice := make([]any, len(results))
		for i, r := range results {
			resSlice[i] = r.Interface()
		}
		return vm.ToValue(resSlice)
	})
}
