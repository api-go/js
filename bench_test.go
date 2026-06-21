package js

import (
	"testing"
)

func BenchmarkCall(b *testing.B) {
	p := NewPool()
	p.Define("add", `(a, b) => { return a + b; }`, 0)
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

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Define("f", `() => { return 1; }`, 0)
		_, err := p.Call("f", 0, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}
