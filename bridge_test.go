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
