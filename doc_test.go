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
		"version":          "1.0.0",
		"__exportInternal": func() *struct{ Name string } { return nil },
	})

	doc := Doc()
	fmt.Println(doc)

	if !strings.Contains(doc, "interface GoTime") {
		t.Error("doc should contain GoTime interface")
	}
	if !strings.Contains(doc, "interface Db_Module") {
		t.Error("doc should contain Db_Module interface")
	}
	if !strings.Contains(doc, "declare const go:") {
		t.Error("doc should contain global go declaration")
	}
	if strings.Contains(doc, "__exportInternal") {
		t.Error("doc should NOT contain __exportInternal")
	}
	if !strings.Contains(doc, "interface Db_") {
		t.Error("doc should contain supporting interface for internal struct")
	}
}
