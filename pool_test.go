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
	// 1. Define initial function
	p.Define(`function hello(name) { return "Hello " + name; }`, "hello.js", int64(100))

	if !p.CheckVersion("hello.js", 100) {
		t.Error("expected CheckVersion to be true for v100")
	}
	if p.CheckVersion("hello.js", 101) {
		t.Error("expected CheckVersion to be false for v101")
	}

	res, err := p.Call("hello", 0, nil, "World")
	if err != nil {
		t.Fatal(err)
	}
	if res != "Hello World" {
		t.Errorf("expected 'Hello World', got %v", res)
	}

	// 2. Define new function (incremental update)
	p.Define(`function add(a, b) { return a + b; }`)

	res, err = p.Call("add", 0, nil, 1, 2)
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
				_, _ = Call("heavy", 0, nil, 1000)
			}()
		}
	})
}

func TestPoolGracefulShutdown(t *testing.T) {
	p := NewPool()
	p.Define(`function sleep(ms) { 
		let start = Date.now();
		while(Date.now() - start < ms);
		return "done";
	}`)

	// 1. Test Timeout
	_, err := p.Call("sleep", 100*time.Millisecond, nil, 1000)
	if err == nil || !strings.Contains(err.Error(), "execution timeout/canceled") {
		t.Errorf("expected timeout error, got %v", err)
	}

	// 2. Test Graceful Stop
	go func() {
		time.Sleep(100 * time.Millisecond)
		p.Stop(context.Background())
	}()

	_, err = p.Call("sleep", 10*time.Second, nil, 5000)
	if err == nil || !strings.Contains(err.Error(), "application stopping") {
		t.Errorf("expected app stopping error, got %v", err)
	}
}

func TestGlobalInjection(t *testing.T) {
	p := NewPool()
	// Test if 'cast' module is available globally without 'go.' prefix
	p.Define(`function testGlobal() { return cast.ToJSON({a:1}); }`)
	res, err := p.Call("testGlobal", 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res != `{"a":1}` {
		t.Errorf("expected '{\"a\":1}', got %v", res)
	}
}
