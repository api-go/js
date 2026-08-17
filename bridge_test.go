package js

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"apigo.cc/go/cast"
	"apigo.cc/go/jsmod"
	"apigo.cc/go/log"
	"github.com/dop251/goja"
)

type bridgeFacadeFilter struct {
	Field    string
	Operator string
	Value    any
}

type bridgeFacadeRequest struct {
	Filter []bridgeFacadeFilter
	Limit  int
}

type bridgeCallbackGroup struct {
	OnData func([]byte) error
}

type bridgeCallbackOptions struct {
	Token  string
	Stream *bridgeCallbackGroup
}

// bridgeFacade intentionally follows the shape used by Go APIs exposed to JS:
// private state and exported methods.
type bridgeFacade struct {
	received bridgeFacadeRequest
}

func (f *bridgeFacade) Query(request bridgeFacadeRequest) {
	f.received = request
}

func TestBridgeSafeMode(t *testing.T) {
	vm := goja.New()

	// Set up safe context
	injects := map[string]any{"SafeMode": true}
	ctx := jsmod.NewContext(context.Background(), injects)
	vm.Set("__ctx__", vm.ToValue(ctx))

	unsafeFn := func() error { return nil }
	vm.Set("unsafe", wrapGoFunc(vm, unsafeFn, true))

	_, err := vm.RunString(`unsafe()`)
	if err == nil {
		t.Error("SafeMode failed to block unsafe function")
	} else if !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("Expected unauthorized error, got %v", err)
	}
}

func TestBridgeLoggerInjection(t *testing.T) {
	vm := goja.New()
	var buf bytes.Buffer
	logger := log.New("test")
	log.SetStdLogOutput(&buf) // Capture through std log for simplicity in test

	// Inject logger via context
	injects := map[string]any{"Logger": logger}
	ctx := jsmod.NewContext(context.Background(), injects)
	vm.Set("__ctx__", vm.ToValue(ctx))

	logFn := func(l *log.Logger) {
		l.Info("hello from js")
	}

	vm.Set("log", wrapGoFunc(vm, logFn, false))
	_, err := vm.RunString(`log()`)
	if err != nil {
		t.Fatalf("JS execution failed: %v", err)
	}
}

func TestBridgeMixedInjection(t *testing.T) {
	vm := goja.New()

	// Create context with multiple values
	injects := map[string]any{
		"UserID": "user123",
		"Base":   "some-base",
	}
	ctx := jsmod.NewContext(context.Background(), injects)
	vm.Set("__ctx__", vm.ToValue(ctx))

	mixedFn := func(c context.Context, a int) string {
		uid := cast.String(jsmod.Get(c, "UserID"))
		return fmt.Sprintf("%s:%d", uid, a)
	}

	vm.Set("mixed", wrapGoFunc(vm, mixedFn, false))

	val, err := vm.RunString(`mixed(42)`)
	if err != nil {
		t.Fatalf("JS execution failed: %v", err)
	}

	if val.Export() != "user123:42" {
		t.Errorf("Mixed injection failed, got %v", val.Export())
	}
}

func TestBridgeOptionalParams(t *testing.T) {
	vm := goja.New()

	optionalFn := func(a int, b *string) string {
		if b == nil {
			return fmt.Sprintf("%d:nil", a)
		}
		return fmt.Sprintf("%d:%s", a, *b)
	}

	vm.Set("opt", wrapGoFunc(vm, optionalFn, false))

	// Test without optional param
	val, _ := vm.RunString(`opt(1)`)
	if val.Export() != "1:nil" {
		t.Errorf("Optional param failed (nil), got %v", val.Export())
	}

	// Test with optional param
	val, _ = vm.RunString(`opt(2, "hello")`)
	if val.Export() != "2:hello" {
		t.Errorf("Optional param failed (val), got %v", val.Export())
	}
}

func TestBridgeVariadic(t *testing.T) {
	vm := goja.New()

	variadicFn := func(cmd string, args ...any) string {
		t.Logf("DEBUG variadicFn received: cmd=%q len(args)=%d args=%v argsType=%T", cmd, len(args), args, args)
		for i, a := range args {
			t.Logf("DEBUG args[%d] = %v (type=%T)", i, a, a)
		}
		return fmt.Sprintf("%s:%d:%v", cmd, len(args), args)
	}

	vm.Set("vfn", wrapGoFunc(vm, variadicFn, false))

	// 无额外参数
	val, _ := vm.RunString(`vfn("PING")`)
	t.Logf("DEBUG vfn(PING) result: %v", val.Export())
	if val.Export() != "PING:0:[]" {
		t.Errorf("Variadic zero args failed, got %v", val.Export())
	}

	// 一个额外参数
	val, _ = vm.RunString(`vfn("GET", "key1")`)
	t.Logf("DEBUG vfn(GET, key1) result: %v", val.Export())
	if val.Export() != "GET:1:[key1]" {
		t.Errorf("Variadic one arg failed, got %v", val.Export())
	}

	// 多个额外参数
	val, _ = vm.RunString(`vfn("HSET", "key1", "field1", "val1")`)
	result := val.Export().(string)
	t.Logf("DEBUG vfn(HSET, key1, field1, val1) result: %v", result)
	if !strings.Contains(result, "HSET:3:") {
		t.Errorf("Variadic multi args failed, got %v", result)
	}
	if !strings.Contains(result, "key1") || !strings.Contains(result, "field1") || !strings.Contains(result, "val1") {
		t.Errorf("Variadic args content wrong, got %v", result)
	}
}

func TestBridgeVariadicNestedCallback(t *testing.T) {
	vm := goja.New()
	call := func(options ...*bridgeCallbackOptions) error {
		if len(options) != 1 || options[0] == nil || options[0].Token != "internal" || options[0].Stream == nil || options[0].Stream.OnData == nil {
			return fmt.Errorf("options were not converted: %#v", options)
		}
		return options[0].Stream.OnData([]byte("chunk"))
	}
	if err := vm.Set("call", wrapGoFunc(vm, call, false)); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`let received = ''; call({token: 'internal', stream: {onData(chunk) { received = String.fromCharCode(...chunk); return null }}}); received`)
	if err != nil {
		t.Fatal(err)
	}
	if value.String() != "chunk" {
		t.Fatalf("callback received %q", value.String())
	}
}

func TestBridgeMapsLowerCamelCaseForReturnedObjectMethods(t *testing.T) {
	vm := goja.New()
	facade := &bridgeFacade{}
	if err := vm.Set("table", wrapGoFunc(vm, func() *bridgeFacade { return facade }, false)); err != nil {
		t.Fatal(err)
	}

	_, err := vm.RunString(`table().Query({filter: [{field: "Name", operator: "=", value: "Admin"}], limit: 1000})`)
	if err != nil {
		t.Fatalf("JS execution failed: %v", err)
	}
	if facade.received.Limit != 1000 || len(facade.received.Filter) != 1 {
		t.Fatalf("lower-camel request was not converted: %#v", facade.received)
	}
	filter := facade.received.Filter[0]
	if filter.Field != "Name" || filter.Operator != "=" || filter.Value != "Admin" {
		t.Errorf("unexpected converted filter: %#v", filter)
	}
}
