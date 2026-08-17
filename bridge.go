package js

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"apigo.cc/go/cast"
	"apigo.cc/go/jsmod"
	"apigo.cc/go/log"
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

	// Capture Go function metadata for error tracing
	goFunc := runtime.FuncForPC(v.Pointer())
	goFuncRef := "<unknown>"
	goFuncName := "<unknown>"
	if goFunc != nil {
		goFuncName = goFunc.Name()
		file, line := goFunc.FileLine(goFunc.Entry())
		goFuncRef = fmt.Sprintf("%s at %s:%d", goFuncName, trimGoPath(file), line)
	}

	return vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// 1. Safety Check
		safeMode := true // Default to safe mode
		ctxVal := vm.Get("__ctx__")
		var currentCtx context.Context
		if ctxVal != nil && !goja.IsUndefined(ctxVal) {
			if c, ok := ctxVal.Export().(context.Context); ok {
				currentCtx = c
				safeMode = jsmod.IsSafeMode(c)
			}
		}

		if isUnsafe && safeMode {
			panic(vm.NewGoError(fmt.Errorf("unauthorized: unsafe operation blocked by safeMode")))
		}

		// 2. Prepare Arguments
		numIn := t.NumIn()
		goArgs := make([]reflect.Value, 0, numIn+len(call.Arguments))
		jsArgs := call.Arguments
		jsArgIdx := 0
		isVariadic := t.IsVariadic()

		for i := 0; i < numIn; i++ {
			argType := t.In(i)

			// Magic Injection: context.Context
			if argType.Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
				ctx := currentCtx
				if ctx == nil {
					ctx = context.Background()
				}
				goArgs = append(goArgs, reflect.ValueOf(ctx))
				continue
			}

			// Magic Injection: *log.Logger
			if argType == reflect.TypeOf((*log.Logger)(nil)) {
				var logger *log.Logger
				if currentCtx != nil {
					if l, ok := jsmod.Get(currentCtx, "Logger").(*log.Logger); ok {
						logger = l
					}
				}
				if logger == nil {
					logger = log.DefaultLogger
				}
				goArgs = append(goArgs, reflect.ValueOf(logger))
				continue
			}

			// 可变参数：剩余 JS 参数直接追加到 goArgs，不由 Call 再包一层
			if isVariadic && i == numIn-1 {
				elemType := argType.Elem()
				for jsArgIdx < len(jsArgs) {
					jsValue := jsArgs[jsArgIdx]
					exported := jsValue.Export()
					expV := reflect.ValueOf(exported)
					if !expV.IsValid() || !expV.Type().AssignableTo(elemType) {
						elem := reflect.New(elemType).Elem()
						cast.Convert(elem.Addr().Interface(), exported)
						exportJSCallbacks(vm, jsValue, elem)
						expV = elem
					}
					goArgs = append(goArgs, expV)
					jsArgIdx++
				}
				continue
			}

			// Normal JS Argument with go/cast
			goArg := reflect.New(argType).Elem()
			if jsArgIdx < len(jsArgs) {
				jsVal := jsArgs[jsArgIdx]
				exported := jsVal.Export()

				expV := reflect.ValueOf(exported)
				if expV.IsValid() && expV.Type().AssignableTo(argType) {
					goArg.Set(expV)
				} else {
					cast.Convert(goArg.Addr().Interface(), exported)
				}
				jsArgIdx++
			}
			goArgs = append(goArgs, goArg)
		}

		// 3. Call the Go function
		var results []reflect.Value
		var recovered any
		var panicStack string
		func() {
			defer func() {
				recovered = recover()
				if recovered != nil {
					// Capture full goroutine stack while panicking frame is still on it
					buf := make([]byte, 8192)
					n := runtime.Stack(buf, false)
					panicStack = formatGoStack(string(buf[:n]), goFuncName)
				}
			}()
			results = v.Call(goArgs)
		}()

		if recovered != nil {
			err := &goStackError{
				cause: fmt.Errorf("[%s] panic: %v", goFuncRef, recovered),
				stack: panicStack,
			}
			panic(vm.NewGoError(err))
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
				err = fmt.Errorf("[%s] %w", goFuncRef, err)
				panic(vm.NewGoError(err))
			}
			if len(results) == 1 {
				return goja.Undefined()
			}
			results = results[:len(results)-1]
		}

		if len(results) == 1 {
			return toJSValue(vm, results[0].Interface())
		}

		resSlice := make([]any, len(results))
		for i, r := range results {
			resSlice[i] = toJSValue(vm, r.Interface())
		}
		return vm.ToValue(resSlice)
	})
}

// exportJSCallbacks overlays callable fields after cast.Convert has applied the
// project's lower-camel struct mapping. goja's native ExportTo preserves JS
// functions, while cast remains authoritative for ordinary data conversion.
func exportJSCallbacks(vm *goja.Runtime, source goja.Value, target reflect.Value) {
	if source == nil {
		return
	}
	for target.Kind() == reflect.Ptr {
		if target.IsNil() {
			if goja.IsUndefined(source) || goja.IsNull(source) {
				return
			}
			target.Set(reflect.New(target.Type().Elem()))
		}
		target = target.Elem()
	}
	if target.Kind() != reflect.Struct || goja.IsUndefined(source) || goja.IsNull(source) {
		return
	}
	object := source.ToObject(vm)
	targetType := target.Type()
	for i := 0; i < target.NumField(); i++ {
		field := target.Field(i)
		if !field.CanSet() {
			continue
		}
		name := cast.GetLowerName(targetType.Field(i).Name)
		value := object.Get(name)
		if goja.IsUndefined(value) {
			value = object.Get(targetType.Field(i).Name)
		}
		if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
			continue
		}
		if field.Kind() == reflect.Func {
			_ = vm.ExportTo(value, field.Addr().Interface())
			continue
		}
		if field.Kind() == reflect.Struct || field.Kind() == reflect.Ptr {
			exportJSCallbacks(vm, value, field)
		}
	}
}

// toJSValue exposes Go API objects through bridge-wrapped methods rather than
// goja's direct reflection. Directly reflected methods bypass cast.Convert, so
// nested lower-camel JavaScript objects would not be converted to Go structs.
// Plain data structs retain goja's normal value export behavior.
func toJSValue(vm *goja.Runtime, value any) goja.Value {
	reflectValue := reflect.ValueOf(value)
	if !reflectValue.IsValid() {
		return goja.Null()
	}
	reflectType := reflectValue.Type()
	structType := reflectType
	if structType.Kind() == reflect.Pointer {
		if reflectValue.IsNil() {
			return goja.Null()
		}
		structType = structType.Elem()
	}
	if structType.Kind() != reflect.Struct || reflectType.NumMethod() == 0 || hasExportedField(structType) {
		return vm.ToValue(value)
	}

	object := vm.NewObject()
	for index := 0; index < reflectType.NumMethod(); index++ {
		method := reflectType.Method(index)
		boundMethod := reflectValue.Method(index)
		if boundMethod.IsValid() {
			_ = object.Set(method.Name, wrapGoFunc(vm, boundMethod.Interface(), false))
		}
	}
	return object
}

func hasExportedField(structType reflect.Type) bool {
	for index := 0; index < structType.NumField(); index++ {
		if structType.Field(index).IsExported() {
			return true
		}
	}
	return false
}

// trimGoPath shortens a Go source file path for display:
// keeps only the last two path components (package/file.go).
func trimGoPath(fullPath string) string {
	dir, file := filepath.Split(fullPath)
	if dir == "" {
		return file
	}
	parent := filepath.Base(filepath.Clean(dir))
	if parent == "." || parent == "/" {
		return file
	}
	return filepath.Join(parent, file)
}

// goStackError wraps a Go error with a captured stack trace.
// formatException detects it to append the stack without duplication.
type goStackError struct {
	cause error
	stack string
}

func (e *goStackError) Error() string { return e.cause.Error() }
func (e *goStackError) Unwrap() error { return e.cause }
func (e *goStackError) Stack() string { return e.stack }

// formatGoStack filters a full goroutine stack trace, keeping only frames
// relevant to the user: dropping runtime, reflect, bridge, and goja internals.
func formatGoStack(fullStack, goFuncName string) string {
	lines := strings.Split(fullStack, "\n")
	var b strings.Builder
	inHeader := true

	for _, line := range lines {
		if inHeader {
			if strings.HasPrefix(line, "goroutine ") {
				inHeader = false
			}
			continue
		}
		if isNoiseFrame(line) {
			continue
		}
		// Highlight the user's function
		trimmed := strings.TrimSpace(line)
		if goFuncName != "" && strings.Contains(trimmed, goFuncName) {
			b.WriteString("  > " + trimmed + "\n")
		} else {
			b.WriteString("  " + trimmed + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func isNoiseFrame(line string) bool {
	trimmed := strings.TrimSpace(line)
	// Function-name level noise
	if strings.Contains(trimmed, "github.com/dop251/goja") {
		return true
	}
	if strings.Contains(trimmed, "apigo.cc/go/js.wrapGoFunc") {
		return true
	}
	if strings.Contains(trimmed, "apigo.cc/go/js.(*Pool)") {
		return true
	}
	if strings.Contains(trimmed, "reflect.Value") {
		return true
	}
	if strings.Contains(trimmed, "reflect.") && !strings.Contains(trimmed, "dummyGoFunc") {
		return true
	}
	// File-path level noise
	noisePaths := []string{
		"/js/bridge.go",
		"/js/pool.go",
		"/reflect/",
		"/goja@",
		"/goja/",
		"/src/runtime/",
	}
	for _, p := range noisePaths {
		if strings.Contains(trimmed, p) {
			return true
		}
	}
	return false
}
