package js

import (
	"context"
	"fmt"
	"io"
	"log"
	"reflect"

	"apigo.cc/go/cast"
	"apigo.cc/go/jsmod"
	"github.com/dop251/goja"
)

// wrapGoFunc converts a standard Go function into a goja.Callable.
// It handles context/logger injection, safeMode enforcement, and automatic type conversion.
func wrapGoFunc(vm *goja.Runtime, fn any, isUnsafe bool) goja.Value {
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func {
		panic(fmt.Sprintf("js.bridge: expected func, got %T", fn))
	}

	t := v.Type()

	return vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// 1. Safety Check
		safeMode := true // Default to safe mode
		smVal := vm.Get("__safeMode__")
		if smVal != nil && !goja.IsUndefined(smVal) {
			if sm, ok := smVal.Export().(bool); ok {
				safeMode = sm
			}
		}

		if isUnsafe && safeMode {
			panic(vm.NewGoError(fmt.Errorf("unauthorized: unsafe operation blocked by safeMode")))
		}

		// 2. Prepare Arguments
		numIn := t.NumIn()
		goArgs := make([]reflect.Value, numIn)
		jsArgs := call.Arguments
		jsArgIdx := 0

		for i := 0; i < numIn; i++ {
			argType := t.In(i)

			// Magic Injection: context.Context
			if argType.Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
				ctx := context.Background()
				ctxVal := vm.Get("__ctx__")
				if ctxVal != nil && !goja.IsUndefined(ctxVal) {
					if c, ok := ctxVal.Export().(context.Context); ok {
						ctx = c
					}
				}
				// Inject SafeMode status into context
				ctx = context.WithValue(ctx, jsmod.SafeModeKey, safeMode)
				goArgs[i] = reflect.ValueOf(ctx)
				continue
			}

			// Magic Injection: *log.Logger
			if argType == reflect.TypeOf((*log.Logger)(nil)) {
				var logger *log.Logger
				logVal := vm.Get("__logger__")
				if logVal != nil && !goja.IsUndefined(logVal) {
					if l, ok := logVal.Export().(*log.Logger); ok {
						logger = l
					}
				}
				if logger == nil {
					// Fallback to a discard logger if none provided to avoid nil panic in Go side
					logger = log.New(io.Discard, "", 0)
				}
				goArgs[i] = reflect.ValueOf(logger)
				continue
			}

			// Normal JS Argument with go/cast
			goArgs[i] = reflect.New(argType).Elem()
			if jsArgIdx < len(jsArgs) {
				jsVal := jsArgs[jsArgIdx]
				exported := jsVal.Export()

				expV := reflect.ValueOf(exported)
				if expV.IsValid() && expV.Type().AssignableTo(argType) {
					goArgs[i].Set(expV)
				} else {
					cast.Convert(goArgs[i].Addr().Interface(), exported)
				}
				jsArgIdx++
			}
		}

		// 3. Call the Go function
		var results []reflect.Value
		var recovered any
		func() {
			defer func() { recovered = recover() }()
			results = v.Call(goArgs)
		}()

		if recovered != nil {
			panic(vm.NewGoError(fmt.Errorf("go panic: %v", recovered)))
		}

		// 4. Process Results
		if len(results) == 0 {
			return goja.Undefined()
		}

		// Check for error return
		last := results[len(results)-1]
		if last.Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			if !last.IsNil() {
				err := last.Interface().(error)
				panic(vm.NewGoError(err))
			}
			if len(results) == 1 {
				return goja.Undefined()
			}
			results = results[:len(results)-1]
		}

		if len(results) == 1 {
			return vm.ToValue(results[0].Interface())
		}

		resSlice := make([]any, len(results))
		for i, r := range results {
			resSlice[i] = r.Interface()
		}
		return vm.ToValue(resSlice)
	})
}
