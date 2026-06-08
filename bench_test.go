package js

import (
	"context"
	"testing"
)

func BenchmarkCall(b *testing.B) {
	p := NewPool()
	p.Define(`function add(a, b) { return a + b; }`)
	ctx := context.Background()
	args := []any{1, 2}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Call(ctx, "add", args)
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
		_, err := p.Call(context.Background(), "f", nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}
