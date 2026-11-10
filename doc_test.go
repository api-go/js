package js

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/api-go/jsmod"
)

func TestDocGeneration(t *testing.T) {
	jsmod.Register("db", map[string]any{
		"query": func(ctx context.Context, sql string, args []any) ([]map[string]any, error) {
			return nil, nil
		},
		"now":              func() time.Time { return time.Time{} },
		"version":          "1.0.0",
		"__exportInternal": func() *struct{ Name string } { return nil },
	})

	doc := Doc()
	if !strings.Contains(doc, "interface GoContext {") {
		t.Error("doc should contain GoContext interface")
	}
	if !strings.Contains(doc, "interface Logger {") {
		t.Error("doc should contain Logger interface")
	}
	if !strings.Contains(doc, "Info(message: string, ...extra: any[]): void;") {
		t.Error("doc should contain Logger methods")
	}
	if strings.Contains(doc, "GetTraceId(): string;") || strings.Contains(doc, "New(traceId: string): Logger;") || strings.Contains(doc, "As<T = any>(value: T, err: any): T;") {
		t.Error("doc should not contain Logger internal methods")
	}
	if !strings.Contains(doc, "interface GoTime {") {
		t.Error("doc should contain GoTime interface")
	}
	if !strings.Contains(doc, "interface Db_Module") {
		t.Error("doc should contain Db_Module interface")
	}
	if !strings.Contains(doc, "declare const db: Db_Module;") {
		t.Error("doc should contain top-level module declaration")
	}
	if strings.Contains(doc, "declare const go:") {
		t.Error("doc should not contain legacy go namespace declaration")
	}
	if strings.Contains(doc, "__exportInternal") {
		t.Error("doc should NOT contain __exportInternal")
	}
	if !strings.Contains(doc, "interface Db_") {
		t.Error("doc should contain supporting interface for internal struct")
	}
}
