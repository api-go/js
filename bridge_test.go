package js

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"

	"github.com/dop251/goja"
)

func TestBridgeSafeMode(t *testing.T) {
	vm := goja.New()

	unsafeFn := func() string { return "danger" }

	// Register with isUnsafe = true
	vm.Set("danger", wrapGoFunc(vm, unsafeFn, true))

	// 1. Default (SafeMode = true)
	_, err := vm.RunString(`danger()`)
	if err == nil || !strings.Contains(err.Error(), "blocked by safeMode") {
		t.Fatalf("Expected safeMode block, got: %v", err)
	}

	// 2. Disable SafeMode
	vm.Set("__safeMode__", false)
	val, err := vm.RunString(`danger()`)
	if err != nil {
		t.Fatal(err)
	}
	if val.Export() != "danger" {
		t.Errorf("Expected 'danger', got %v", val.Export())
	}
}

func TestBridgeLoggerInjection(t *testing.T) {
	vm := goja.New()
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	vm.Set("__logger__", vm.ToValue(logger))

	logFn := func(l *log.Logger, msg string) {
		l.Print(msg)
	}

	vm.Set("logMsg", wrapGoFunc(vm, logFn, false))

	// JS only passes the 'msg' argument, logger is injected
	_, err := vm.RunString(`logMsg("hello from js")`)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "hello from js") {
		t.Errorf("Logger injection failed, buffer: %s", buf.String())
	}
}

func TestBridgeMixedInjection(t *testing.T) {
	vm := goja.New()
	ctx := context.WithValue(context.Background(), "k", "v")
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	vm.Set("__ctx__", vm.ToValue(ctx))
	vm.Set("__logger__", vm.ToValue(logger))

	mixedFn := func(c context.Context, l *log.Logger, a int) string {
		l.Printf("val: %d", a)
		return c.Value("k").(string)
	}

	vm.Set("mixed", wrapGoFunc(vm, mixedFn, false))

	val, err := vm.RunString(`mixed(42)`)
	if err != nil {
		t.Fatal(err)
	}

	if val.Export() != "v" {
		t.Errorf("Context injection failed")
	}
	if !strings.Contains(buf.String(), "val: 42") {
		t.Errorf("Logger injection failed")
	}
}
