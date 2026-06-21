package js

import (
	"context"
	"strings"
	"testing"
	"time"

	"apigo.cc/go/cast"
)

func TestPoolVersioning(t *testing.T) {
	p := NewPool()

	// 1. Define with anonymous function expression
	err := p.Define("hello", `(name) => { return "Hello " + name; }`, 100)
	if err != nil {
		t.Fatal(err)
	}

	if !p.CheckVersion("hello", 100) {
		t.Error("expected CheckVersion to be true for v100")
	}
	if p.CheckVersion("hello", 101) {
		t.Error("expected CheckVersion to be false for v101")
	}

	res, callErr := p.Call("hello", 0, nil, "World")
	if callErr != nil {
		t.Fatal(callErr)
	}
	if res != "Hello World" {
		t.Errorf("expected 'Hello World', got %v", res)
	}

	// 2. Define second function (incremental update)
	err = p.Define("add", `(a, b) => { return a + b; }`, 0)
	if err != nil {
		t.Fatal(err)
	}

	res, callErr = p.Call("add", 0, nil, 1, 2)
	if callErr != nil {
		t.Fatal(callErr)
	}
	if cast.To[int64](res) != 3 {
		t.Errorf("expected 3, got %v", res)
	}

	// 3. Check FuncList
	funcs := p.FuncList()
	foundHello := false
	foundAdd := false
	for _, f := range funcs {
		if f == "hello" {
			foundHello = true
		}
		if f == "add" {
			foundAdd = true
		}
	}
	if !foundHello || !foundAdd {
		t.Errorf("FuncList missing functions: %v", funcs)
	}
}

func TestPoolConcurrent(t *testing.T) {
	err := Define("heavy", `(n) => {
		let s = 0;
		for (let i = 0; i < n; i++) s += i;
		return s;
	}`, 0)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Parallel", func(t *testing.T) {
		t.Parallel()
		for i := 0; i < 10; i++ {
			go func() {
				_, _ = Call("heavy", 0, nil, 1000)
			}()
		}
	})
}

func TestPoolGracefulShutdown(t *testing.T) {
	p := NewPool()
	err := p.Define("sleep", `(ms) => {
		let start = Date.now();
		while (Date.now() - start < ms);
		return "done";
	}`, 0)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Test Timeout
	_, callErr := p.Call("sleep", 100*time.Millisecond, nil, 1000)
	if callErr == nil || !strings.Contains(callErr.Error(), "execution timeout/canceled") {
		t.Errorf("expected timeout error, got %v", callErr)
	}

	// 2. Test Graceful Stop
	go func() {
		time.Sleep(100 * time.Millisecond)
		p.Stop(context.Background())
	}()

	_, callErr = p.Call("sleep", 10*time.Second, nil, 5000)
	if callErr == nil || !strings.Contains(callErr.Error(), "application stopping") {
		t.Errorf("expected app stopping error, got %v", callErr)
	}
}

func TestGlobalInjection(t *testing.T) {
	p := NewPool()
	// Test if 'cast' module is available globally without 'go.' prefix
	err := p.Define("testGlobal", `() => { return cast.ToJSON({a:1}); }`, 0)
	if err != nil {
		t.Fatal(err)
	}
	res, callErr := p.Call("testGlobal", 0, nil)
	if callErr != nil {
		t.Fatal(callErr)
	}
	if res != `{"a":1}` {
		t.Errorf("expected '{\"a\":1}', got %v", res)
	}
}

func TestDefineValidation(t *testing.T) {
	p := NewPool()

	// Named function declaration should be rejected
	err := p.Define("bad1", `function foo() { return 1; }`, 0)
	if err == nil {
		t.Error("expected error for named function declaration")
	}

	// Empty code should be rejected
	err = p.Define("bad2", ``, 0)
	if err == nil {
		t.Error("expected error for empty code")
	}

	// Empty name should be rejected
	err = p.Define("", `() => { return 1; }`, 0)
	if err == nil {
		t.Error("expected error for empty name")
	}

	// Anonymous function expression should be accepted
	err = p.Define("good", `() => { return 1; }`, 0)
	if err != nil {
		t.Errorf("unexpected error for anonymous function: %v", err)
	}

	// Arrow function should be accepted
	err = p.Define("good2", `(a, b) => a + b`, 0)
	if err != nil {
		t.Errorf("unexpected error for arrow function: %v", err)
	}
}

func TestJSErrorStackTrace(t *testing.T) {
	p := NewPool()

	// Register two functions where one calls the other
	err := p.Define("validateInput", `(args) => {
		if (!args.name) throw new Error("name is required");
		return true;
	}`, 0)
	if err != nil {
		t.Fatal(err)
	}

	err = p.Define("processOrder", `(args) => {
		validateInput(args);
		return "order processed: " + args.name;
	}`, 0)
	if err != nil {
		t.Fatal(err)
	}

	_, callErr := p.Call("processOrder", 0, nil, map[string]any{})
	if callErr == nil {
		t.Fatal("expected error")
	}

	// Message should contain the JS error info
	if !strings.Contains(callErr.Message, "name is required") {
		t.Errorf("message should contain JS error 'name is required', got: %q", callErr.Message)
	}
	// CallStacks should contain function names and line numbers
	stacks := strings.Join(callErr.CallStacks, "\n")
	t.Logf("CallStacks:\n%s", stacks)
	if !strings.Contains(stacks, "validateInput") {
		t.Errorf("CallStacks should contain 'validateInput', got:\n%s", stacks)
	}
	if !strings.Contains(stacks, "processOrder") {
		t.Errorf("CallStacks should contain 'processOrder', got:\n%s", stacks)
	}
}

func TestDefineRedefine(t *testing.T) {
	p := NewPool()

	// Define the same name twice should not error (globalThis handles it)
	err := p.Define("test", `() => "v1"`, 1)
	if err != nil {
		t.Fatal(err)
	}
	err = p.Define("test", `() => "v2"`, 2)
	if err != nil {
		t.Fatal(err)
	}

	// After re-define, the latest version wins
	if !p.CheckVersion("test", 2) {
		t.Error("expected CheckVersion to be true for v2")
	}

	res, callErr := p.Call("test", 0, nil)
	if callErr != nil {
		t.Fatal(callErr)
	}
	if res != "v2" {
		t.Errorf("expected 'v2', got %v", res)
	}
}
