# Go/JS Low-Code Engine

A lightweight, frictionless, and AI-friendly JavaScript engine for Go applications based on `goja`.

## Features

- **Decoupled Architecture**: Capability providers only need to depend on `apigo.cc/go/jsmod`.
- **Frictionless Bridging**: Automatic type conversion using `go/cast`.
- **Host Object Fidelity**: Go pointers and structs are preserved when passed back and forth between Go and JS.
- **Context Injection**: Automatic `context.Context` propagation from `js.Call`.
- **Versioned Pool**: Thread-safe VM pool with incremental code synchronization and version checking (`CheckVersion`).
- **Function Discovery**: List all defined functions via `FuncList()`.
- **Context Interruption**: Safe execution with `context.Context` cancellation support.
- **AI-Ready**: Generates TypeScript definitions (`.d.ts`) for AI to understand available capabilities.

## Usage

### 1. Register Go Capability (in any module)

```go
import "apigo.cc/go/jsmod"

func init() {
    jsmod.Register("db", map[string]any{
        "query": func(ctx context.Context, sql string) ([]map[string]any, error) {
            // ...
        },
    })
}
```

### 2. Execute JS with Version Checking

```go
import "apigo.cc/go/js"

func main() {
    // Check if script needs update (e.g., from file mtime)
    if !js.CheckVersion("myTask", mtime) {
        js.Define("myTask", code, mtime)
    }

    res, err := js.Call("myTask", time.Second, nil, "star")
}
```

### 3. Discover Functions

```go
funcs := js.FuncList()
// ["myTask", ...]
```

### 3. Generate AI Context

```go
dts := js.Doc()
// Feed d.ts to LLM to provide coding context
```

The generated declarations are optimized for low-code editors:
- bridged modules are exposed as top-level globals such as `api`, `cast`, `db`, `service`
- injected logger values are typed as `Logger`
- opaque runtime handles use stable names such as `GoContext`, `GoHTTPRequest`, `GoHTTPResponse`, `GoURL`
- exposed standard-library structs use stable names such as `GoTime`

Example:

```ts
declare const service: Service_Module

interface Logger {
    Debug(message: string, ...extra: any[]): void
    Info(message: string, ...extra: any[]): void
    Warning(message: string, ...extra: any[]): void
    Error(message: string, ...extra: any[]): void
}
```

## Internal Bridge Details

The engine preserves ordinary Go data values. API facade objects returned to JavaScript expose their exported methods through the same bridge as top-level module functions, ensuring identical context injection, safe-mode checking, error conversion, and struct conversion.

Types are automatically coerced:
- JS `string` -> Go `int` (via `go/cast`)
- JS `Object` -> Go `Struct`
- Go `error` -> JS `Exception`

Therefore both styles use normal JavaScript lower-camel fields:

```js
knowbase.Table("Account").Query({
  filter: [{ field: "Name", operator: "=", value: "Admin" }],
  limit: 1000
})
```
