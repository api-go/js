package js_test

import (
	"github.com/api-go/js"
	"testing"
	"time"
)

func TestDefinitionDoesNotConfuseTextWithNamedFunctions(t *testing.T) {
	p := js.NewPool()
	for _, code := range []string{`() => "function formatStoreTime(value)"`, "() => `function example() {}`", `() => { /* function example() {} */ return "ok" }`, `() => /function named/.source`} {
		if err := p.Define("literal", code, 1); err != nil {
			t.Fatalf("valid function text rejected: %v", err)
		}
		if _, err := p.Call("literal", time.Second, nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, code := range []string{`function named(){return 1}`, `() => { function nested(){return 1};return nested() }`, `() => (function named(){return 1})()`} {
		if err := p.Define("named", code, 1); err == nil {
			t.Fatal("named function restriction bypassed", code)
		}
	}
	if p.CheckVersion("named", 1) {
		t.Fatal("rejected definition entered registry")
	}
}
