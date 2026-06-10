package js

import (
	"testing"
)

func BenchmarkCall(b *testing.B) {
	p := NewPool()
	p.Define(`function add(a, b) { return a + b; }`)
	args := []any{1, 2}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Call("add", 0, nil, args...)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSync(b *testing.B) {
	p := NewPool()
	code := `function f() { return 1; }`
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Define(code)
		_, err := p.Call("f", 0, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}
