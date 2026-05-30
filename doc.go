package js

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"apigo.cc/go/jsmod"
)

// Doc generates a TypeScript definition (.d.ts) for all registered Go modules.
// This is designed to be fed into an AI (LLM) to provide context for low-code development.
func Doc() string {
	var sb strings.Builder
	sb.WriteString("// TypeScript Definitions for Go/JS Low-Code Environment\n\n")

	modules := jsmod.GetModules()
	keys := make([]string, 0, len(modules))
	for k := range modules {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	sb.WriteString("declare namespace go {\n")
	for _, modName := range keys {
		mod := modules[modName]
		sb.WriteString(fmt.Sprintf("  namespace %s {\n", modName))

		expKeys := make([]string, 0, len(mod.Exports))
		for k := range mod.Exports {
			expKeys = append(expKeys, k)
		}
		sort.Strings(expKeys)

		for _, name := range expKeys {
			val := mod.Exports[name]
			isUnsafe := mod.UnsafeList[name]
			if isUnsafe {
				sb.WriteString("    /** @unsafe */\n")
			}
			sb.WriteString(fmt.Sprintf("    %s\n", formatExport(name, val)))
		}
		sb.WriteString("  }\n")
	}
	sb.WriteString("}\n")

	return sb.String()
}

func formatExport(name string, val any) string {
	t := reflect.TypeOf(val)
	if t == nil {
		return fmt.Sprintf("const %s: any;", name)
	}

	if t.Kind() == reflect.Func {
		return fmt.Sprintf("function %s%s;", name, formatFunc(t))
	}

	return fmt.Sprintf("const %s: %s;", name, goTypeToTS(t))
}

func formatFunc(t reflect.Type) string {
	var params []string
	numIn := t.NumIn()
	jsArgIdx := 0

	for i := 0; i < numIn; i++ {
		argType := t.In(i)
		// Skip Context and Logger in TS doc
		typeName := argType.String()
		if typeName == "context.Context" || typeName == "*log.Logger" {
			continue
		}

		params = append(params, fmt.Sprintf("arg%d: %s", jsArgIdx, goTypeToTS(argType)))
		jsArgIdx++
	}

	// Handle return values
	numOut := t.NumOut()
	var retType string
	if numOut == 0 {
		retType = "void"
	} else {
		// If last return is error, we only care about the first part for TS doc
		realOut := numOut
		if numOut > 0 && t.Out(numOut-1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			realOut--
		}

		if realOut <= 0 {
			retType = "void"
		} else if realOut == 1 {
			retType = goTypeToTS(t.Out(0))
		} else {
			retType = "any[]"
		}
	}

	return fmt.Sprintf("(%s): %s", strings.Join(params, ", "), retType)
}

func goTypeToTS(t reflect.Type) string {
	if t == nil {
		return "any"
	}

	// Handle known standard library types to avoid deep recursion and provide clear naming
	typeName := t.String()
	switch typeName {
	case "time.Time", "*time.Time":
		return "time.Time"
	case "time.Duration", "*time.Duration":
		return "time.Duration"
	}

	// Handle pointers
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice, reflect.Array:
		return goTypeToTS(t.Elem()) + "[]"
	case reflect.Map:
		return "Record<string, any>"
	case reflect.Struct:
		return "{ [key: string]: any }"
	case reflect.Interface:
		return "any"
	default:
		return "any"
	}
}
