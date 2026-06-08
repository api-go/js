package js

import (
	"context"
	"testing"

	"apigo.cc/go/cast"
)

func TestPoolVersioning(t *testing.T) {
	p := NewPool()
	// 1. Define initial function
	p.Define(`function hello(name) { return "Hello " + name; }`, "hello.js", int64(100))

	if !p.CheckVersion("hello.js", 100) {
		t.Error("expected CheckVersion to be true for v100")
	}
	if p.CheckVersion("hello.js", 101) {
		t.Error("expected CheckVersion to be false for v101")
	}

	res, err := p.Call(context.Background(), "hello", []any{"World"})
	if err != nil {
		t.Fatal(err)
	}
	if res != "Hello World" {
		t.Errorf("expected 'Hello World', got %v", res)
	}

	// 2. Define new function (incremental update)
	p.Define(`function add(a, b) { return a + b; }`)

	res, err = p.Call(context.Background(), "add", []any{1, 2})
	if err != nil {
		t.Fatal(err)
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
	Define(`function heavy(n) { 
		let s = 0; 
		for(let i=0; i<n; i++) s += i; 
		return s; 
	}`)

	t.Run("Parallel", func(t *testing.T) {
		t.Parallel()
		for i := 0; i < 10; i++ {
			go func() {
				_, _ = Call(context.Background(), "heavy", []any{1000})
			}()
		}
	})
}
