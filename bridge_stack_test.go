package js

import (
	"fmt"
	"strings"
	"testing"

	"github.com/api-go/jsmod"
)

func dummyGoFunc() error {
	return fmt.Errorf("knowbase: context UserID not found")
}

func dummyGoFunc2() error {
	panic("something went terribly wrong")
}

func dummyGoFuncWithMakeError() error {
	return jsmod.MakeError(fmt.Errorf("knowbase: context UserID not found"))
}

func init() {
	jsmod.Register("testmod", map[string]any{
		"query":            dummyGoFunc,
		"crash":            dummyGoFunc2,
		"query_make_error": dummyGoFuncWithMakeError,
	})
}

func TestGoStackErrorInterface(t *testing.T) {
	err := &goStackError{
		cause: fmt.Errorf("test error"),
		stack: "test stack",
	}
	var e error = err
	if s, ok := e.(interface{ Stack() string }); ok {
		t.Logf("Stack: %s", s.Stack())
	} else {
		t.Errorf("goStackError does NOT satisfy interface{ Stack() string }")
	}
}

func TestBridgeGoErrorWithFuncName(t *testing.T) {
	p := NewPool()

	err := p.Define("callQuery", `() => { return go.testmod.query(); }`, 0)
	if err != nil {
		t.Fatal(err)
	}

	_, callErr := p.Call("callQuery", 0, nil)
	if callErr == nil {
		t.Fatal("expected error")
	}

	t.Logf("Message: %s", callErr.Message)
	t.Logf("CallStacks: %v", callErr.CallStacks)

	// Message: just the error description
	if !strings.Contains(callErr.Message, "knowbase") {
		t.Errorf("message should contain 'knowbase', got: %q", callErr.Message)
	}
	if strings.Contains(callErr.Message, "GoError") || strings.Contains(callErr.Message, "[") {
		t.Errorf("message should have no GoError/function prefix, got: %q", callErr.Message)
	}
	// CallStacks: just file:line entries, Go first
	if len(callErr.CallStacks) < 1 {
		t.Fatal("expected at least one CallStacks entry")
	}
	if !strings.Contains(callErr.CallStacks[0], "bridge_stack_test.go:") {
		t.Errorf("first CallStack should be Go location, got: %q", callErr.CallStacks[0])
	}
	foundCallQuery := false
	for _, s := range callErr.CallStacks {
		if strings.Contains(s, "callQuery:") {
			foundCallQuery = true
		}
	}
	if !foundCallQuery {
		t.Errorf("CallStacks should contain JS call site 'callQuery:...', got: %v", callErr.CallStacks)
	}
	// No bridge internal frames
	for _, s := range callErr.CallStacks {
		if strings.Contains(s, "wrapGoFunc") || strings.Contains(s, "pool.go") {
			t.Errorf("CallStacks should NOT contain bridge internals, got: %s", s)
		}
	}
}

func TestBridgeGoPanicWithStack(t *testing.T) {
	p := NewPool()

	err := p.Define("callCrash", `() => { return go.testmod.crash(); }`, 0)
	if err != nil {
		t.Fatal(err)
	}

	_, callErr := p.Call("callCrash", 0, nil)
	if callErr == nil {
		t.Fatal("expected error")
	}

	t.Logf("Message: %s", callErr.Message)
	t.Logf("CallStacks: %v", callErr.CallStacks)

	// Message: just the panic description
	if !strings.Contains(callErr.Message, "panic") {
		t.Errorf("message should contain 'panic', got: %q", callErr.Message)
	}
	// CallStacks: should contain Go trace file:line frames
	hasGoTrace := false
	for _, s := range callErr.CallStacks {
		if strings.Contains(s, "bridge_stack_test.go:") {
			hasGoTrace = true
		}
	}
	if !hasGoTrace {
		t.Errorf("CallStacks should contain Go trace frames, got: %v", callErr.CallStacks)
	}
}

func TestBridgeGoErrorWithMakeError(t *testing.T) {
	p := NewPool()

	err := p.Define("callQueryMakeError", `() => { return go.testmod.query_make_error(); }`, 0)
	if err != nil {
		t.Fatal(err)
	}

	_, callErr := p.Call("callQueryMakeError", 0, nil)
	if callErr == nil {
		t.Fatal("expected error")
	}

	t.Logf("Message: %s", callErr.Message)
	t.Logf("CallStacks: %v", callErr.CallStacks)

	// Message: just the error description
	if !strings.Contains(callErr.Message, "knowbase") {
		t.Errorf("message should contain 'knowbase', got: %q", callErr.Message)
	}
	// CallStacks: should contain the dynamic caller location!
	if len(callErr.CallStacks) < 1 {
		t.Fatal("expected at least one CallStacks entry")
	}
	foundDynamicFunc := false
	for _, s := range callErr.CallStacks {
		if strings.Contains(s, "dummyGoFuncWithMakeError") && strings.Contains(s, "bridge_stack_test.go:") {
			foundDynamicFunc = true
			break
		}
	}
	if !foundDynamicFunc {
		t.Errorf("CallStacks should contain dynamic caller 'dummyGoFuncWithMakeError' at 'bridge_stack_test.go:', got: %v", callErr.CallStacks)
	}
}
