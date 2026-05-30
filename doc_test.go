package js

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"apigo.cc/go/jsmod"
)

func TestDocGeneration(t *testing.T) {
	jsmod.Register("db", map[string]any{
		"query": func(ctx context.Context, sql string, args []any) ([]map[string]any, error) {
			return nil, nil
		},
		"version": "1.0.0",
	})

	doc := Doc()
	fmt.Println(doc)

	if !strings.Contains(doc, "namespace db") {
		t.Error("doc should contain namespace db")
	}
	if !strings.Contains(doc, "function query(arg0: string, arg1: any[]): Record<string, any>[];") {
		t.Error("doc should contain query function with correct signature")
	}
	if !strings.Contains(doc, "const version: string;") {
		t.Error("doc should contain version constant")
	}
}
