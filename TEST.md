# Test Report - go/js

## Performance (Benchmark)
Date: 2026-06-21
OS: darwin
Arch: amd64
CPU: Intel(R) Core(TM) i9-9980HK CPU @ 2.40GHz

| Benchmark | Iterations | Time/op |
|-----------|------------|---------|
| BenchmarkCall | 661954 | 1566 ns/op |
| BenchmarkSync | 51656 | 50748 ns/op |

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
ok      apigo.cc/go/js  0.572s
```

## Features Verified
- [x] JS calling Go with automatic type conversion.
- [x] Go pointers preservation in JS.
- [x] Context and Logger injection.
- [x] Concurrent execution and script versioning.
- [x] Script version checking (`CheckVersion`).
- [x] Function discovery (`FuncList`).
- [x] Context cancellation interruption.
- [x] Graceful shutdown.
- [x] TypeScript definition generation.
- [x] JS VM call stack parsing & Go dynamic call stack restoration with `jsmod.MakeError`.
