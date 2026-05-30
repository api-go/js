package js

import (
	"context"
	"testing"
)

func TestPoolVersioning(t *testing.T) {
	// 1. Define initial function
	Define(`function hello(name) { return "Hello " + name; }`)

	res, err := Call(context.Background(), "hello", []any{"World"})
	if err != nil {
		t.Fatal(err)
	}
	if res != "Hello World" {
		t.Errorf("expected 'Hello World', got %v", res)
	}

	// 2. Define new function (incremental update)
	Define(`function add(a, b) { return a + b; }`)

	res, err = Call(context.Background(), "add", []any{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if res.(int64) != 3 {
		t.Errorf("expected 3, got %v", res)
	}

	// 3. Ensure old function still works
	res, err = Call(context.Background(), "hello", []any{"Again"})
	if err != nil {
		t.Fatal(err)
	}
	if res != "Hello Again" {
		t.Errorf("expected 'Hello Again', got %v", res)
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
