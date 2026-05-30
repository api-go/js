package gojs_test

import (
	"sync"
	"testing"

	"apigo.cc/go/js/gojs"
)

func TestRegisterAndGet(t *testing.T) {
	gojs.Register("db", map[string]any{
		"query": func() string { return "ok" },
	})

	// Test merging
	gojs.Register("db", map[string]any{
		"exec": func() int { return 1 },
	})

	mods := gojs.GetModules()
	dbMod, ok := mods["db"]
	if !ok {
		t.Fatal("expected module 'db' to be registered")
	}

	if _, ok := dbMod["query"]; !ok {
		t.Error("expected 'query' in 'db' module")
	}
	if _, ok := dbMod["exec"]; !ok {
		t.Error("expected 'exec' in 'db' module")
	}
}

func TestConcurrentRegister(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			gojs.Register("concurrent_mod", map[string]any{
				"func": i,
			})
		}(i)
	}
	wg.Wait()

	mods := gojs.GetModules()
	if _, ok := mods["concurrent_mod"]; !ok {
		t.Fatal("expected 'concurrent_mod' to be registered")
	}
}

func BenchmarkGetModules(b *testing.B) {
	gojs.Register("bench_mod", map[string]any{"a": 1, "b": 2})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = gojs.GetModules()
	}
}
