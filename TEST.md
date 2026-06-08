# Test Report - go/js

## Performance (Benchmark)
Date: 2026-06-08
OS: darwin
Arch: amd64
CPU: Intel(R) Core(TM) i9-9980HK CPU @ 2.40GHz

| Benchmark | Iterations | Time/op |
|-----------|------------|---------|
| BenchmarkCall | 990318 | 1109 ns/op |
| BenchmarkSync | 57362 | 78846 ns/op |

*Note: BenchmarkCall covers the hot path of executing a JS function from the pool. BenchmarkSync covers the cost of defining new code (including regex parsing) and syncing a VM.*

## Coverage
```
=== RUN   TestBridgeSafeMode
--- PASS: TestBridgeSafeMode (0.00s)
=== RUN   TestBridgeLoggerInjection
--- PASS: TestBridgeLoggerInjection (0.00s)
=== RUN   TestBridgeMixedInjection
--- PASS: TestBridgeMixedInjection (0.00s)
=== RUN   TestDocGeneration
--- PASS: TestDocGeneration (0.00s)
=== RUN   TestPoolVersioning
--- PASS: TestPoolVersioning (0.00s)
=== RUN   TestPoolConcurrent
--- PASS: TestPoolConcurrent (0.00s)
=== RUN   TestPoolGracefulShutdown
--- PASS: TestPoolGracefulShutdown (0.50s)
PASS
ok      apigo.cc/go/js  0.932s
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
