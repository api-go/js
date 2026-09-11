# Test Report - go/js

## v1.5.15 验证
- 验证 Context 取消、Pool 停止与复用 VM 中断清理。
- 验证 `null`/`undefined` 参数安全转换为 Go 零值。
- `go test -v ./...` 与 `go test -bench=. ./...` 通过。

## v1.5.13 验证
- 独立模块解析、全量测试与基准测试均通过。
- `TestDefineValidation` 覆盖语法错误、定义阶段异常、非函数结果、定义超时，以及失败定义不改变共享注册表。

## v1.5.10 验证
- 独立模块解析、全量测试与基准测试均通过。
- `TestBridgeVariadicNestedCallback` 验证可变 options 中 lower-camel 数据与嵌套 JavaScript 回调同时正确转换。

## Performance (Benchmark)
Date: 2026-08-24
OS: darwin
Arch: arm64
CPU: Apple M3 Max

| Benchmark | Iterations | Time/op |
|-----------|------------|---------|
| BenchmarkCall | 1578303 | 742.8 ns/op |
| BenchmarkSync | 13456 | 136636 ns/op |

*Note: BenchmarkCall covers the hot path of executing a JS function from the pool. BenchmarkSync covers the cost of defining new code (including VM sync).*

## Coverage
```
=== RUN   TestGoStackErrorInterface
--- PASS: TestGoStackErrorInterface (0.00s)
=== RUN   TestBridgeGoErrorWithFuncName
--- PASS: TestBridgeGoErrorWithFuncName (0.00s)
=== RUN   TestBridgeGoPanicWithStack
--- PASS: TestBridgeGoPanicWithStack (0.00s)
=== RUN   TestBridgeGoErrorWithMakeError
--- PASS: TestBridgeGoErrorWithMakeError (0.00s)
=== RUN   TestBridgeSafeMode
--- PASS: TestBridgeSafeMode (0.00s)
=== RUN   TestBridgeLoggerInjection
--- PASS: TestBridgeLoggerInjection (0.00s)
=== RUN   TestBridgeMixedInjection
--- PASS: TestBridgeMixedInjection (0.00s)
=== RUN   TestBridgeOptionalParams
--- PASS: TestBridgeOptionalParams (0.00s)
=== RUN   TestBridgeVariadic
--- PASS: TestBridgeVariadic (0.00s)
=== RUN   TestBridgeMapsLowerCamelCaseForReturnedObjectMethods
--- PASS: TestBridgeMapsLowerCamelCaseForReturnedObjectMethods (0.00s)
=== RUN   TestDocGeneration
--- PASS: TestDocGeneration (0.00s)
=== RUN   TestPoolVersioning
--- PASS: TestPoolVersioning (0.00s)
=== RUN   TestPoolConcurrent
--- PASS: TestPoolConcurrent (0.00s)
=== RUN   TestPoolGracefulShutdown
--- PASS: TestPoolGracefulShutdown (0.20s)
=== RUN   TestGlobalInjection
--- PASS: TestGlobalInjection (0.00s)
=== RUN   TestDefineValidation
--- PASS: TestDefineValidation (0.00s)
=== RUN   TestJSErrorStackTrace
--- PASS: TestJSErrorStackTrace (0.00s)
=== RUN   TestDefineRedefine
--- PASS: TestDefineRedefine (0.00s)
PASS
ok      apigo.cc/go/js  0.800s
```

## Features Verified
- [x] JS calling Go with automatic type conversion.
- [x] Returned Go API object methods use the same lower-camel struct conversion.
- [x] Go pointers preservation in JS.
- [x] Context and Logger injection.
- [x] Concurrent execution and script versioning.
- [x] Script version checking (`CheckVersion`).
- [x] Transactional definition validation and shared Pool fault isolation.
- [x] Function discovery (`FuncList`).
- [x] Context cancellation interruption.
- [x] Graceful shutdown.
- [x] TypeScript definition generation.
- [x] Top-level global module declarations for low-code hints.
- [x] Opaque type naming stability (`GoContext`, `GoTime`, `GoURL`).
- [x] Runtime-injected `Logger` type hints with JS-safe methods only.
- [x] JS VM call stack parsing & Go dynamic call stack restoration with `jsmod.MakeError`.
