package js

import (
	"context"
	"errors"
	"testing"

	"apigo.cc/go/jsmod"
	"github.com/dop251/goja"
)

type User struct {
	ID   int
	Name string
}

func (u *User) GetInfo() string {
	return u.Name
}

func TestBridgeDataFidelity(t *testing.T) {
	vm := goja.New()

	// 1. Setup Go functions
	originalUser := &User{ID: 1, Name: "Star"}

	getUser := func() *User {
		return originalUser
	}

	verifyUser := func(u *User) bool {
		// Verify pointer address remains the same (Host Object fidelity)
		return u == originalUser
	}

	// Register functions manually for testing bridge
	vm.Set("getUser", wrapGoFunc(vm, getUser))
	vm.Set("verifyUser", wrapGoFunc(vm, verifyUser))

	// 2. JS Execution
	script := `
		let u = getUser();
		if (u.Name !== "Star") throw "Name mismatch: " + u.Name;
		if (u.ID !== 1) throw "ID mismatch: " + u.ID;
		
		// Host Object method call (if exported)
		// Note: goja requires methods to be exported and usually works better with struct pointers
		
		let isSame = verifyUser(u);
		if (!isSame) throw "Pointer mismatch in Go side";
		
		"ok"
	`
	val, err := vm.RunString(script)
	if err != nil {
		t.Fatalf("JS execution failed: %v", err)
	}
	if val.Export() != "ok" {
		t.Errorf("expected 'ok', got %v", val.Export())
	}
}

func TestBridgeCasting(t *testing.T) {
	vm := goja.New()

	sum := func(a, b int) int {
		return a + b
	}

	vm.Set("sum", wrapGoFunc(vm, sum))

	// Test passing string as number (frictionless casting via go/cast)
	script := `sum("10", 20)`
	val, err := vm.RunString(script)
	if err != nil {
		t.Fatal(err)
	}
	if val.Export().(int64) != 30 {
		t.Errorf("expected 30, got %v", val.Export())
	}
}

func TestBridgeErrorHandling(t *testing.T) {
	vm := goja.New()

	failFunc := func() (string, error) {
		return "", errors.New("go_error")
	}

	vm.Set("failFunc", wrapGoFunc(vm, failFunc))

	script := `
		try {
			failFunc();
		} catch (e) {
			e.message;
		}
	`
	val, err := vm.RunString(script)
	if err != nil {
		t.Fatal(err)
	}
	if val.Export() != "go_error" {
		t.Errorf("expected 'go_error', got %v", val.Export())
	}
}

func TestBridgeContextInjection(t *testing.T) {
	vm := goja.New()
	ctx := context.WithValue(context.Background(), "key", "value")
	
	// Inject context into VM
	vm.Set("__ctx__", vm.ToValue(ctx))

	checkCtx := func(c context.Context) string {
		return c.Value("key").(string)
	}

	vm.Set("checkCtx", wrapGoFunc(vm, checkCtx))

	val, err := vm.RunString(`checkCtx()`)
	if err != nil {
		t.Fatal(err)
	}
	if val.Export() != "value" {
		t.Errorf("expected 'value', got %v", val.Export())
	}
}

func TestBridgeComplexStruct(t *testing.T) {
	vm := goja.New()

	type Complex struct {
		Data map[string]any
		Tags []string
	}

	process := func(c Complex) int {
		return len(c.Tags) + len(c.Data)
	}

	vm.Set("process", wrapGoFunc(vm, process))

	script := `
		process({
			Tags: ["a", "b"],
			Data: { "x": 1, "y": 2, "z": 3 }
		})
	`
	val, err := vm.RunString(script)
	if err != nil {
		t.Fatal(err)
	}
	if val.Export().(int64) != 5 {
		t.Errorf("expected 5, got %v", val.Export())
	}
}

// Ensure jsmod is used to avoid unused import if needed
func init() {
	_ = jsmod.GetModules()
}
