# Go/JS 底层低代码引擎架构与开发计划

**目标受众**: 负责实现该项目的 AI 开发助手。
**项目背景**: 这是一个用于 Go 应用的低代码框架，其核心目标是提供一个 **极低摩擦、无状态、对 AI 极度友好** 的 Go/JS 桥接层。允许业务系统在不重启、不重新编译的情况下，通过动态执行 JS 代码来扩展能力。
**核心原则**: 性能优先、快速失败 (Fail-Fast)、严格的测试覆盖、彻底解耦底层依赖。

---

## 1. 架构总览

项目分为两个完全解耦的模块：

### 1.1 `go/js/gojs` (注册与标准层)
- **定位**: 轻量级注册中心，**零第三方依赖**。其他 Go 业务模块（如 `go/db`, `go/http`）仅引入此包进行能力暴露，避免污染 `goja` 依赖。
- **核心 API**:
  - `func Register(name string, exports map[string]any)`: 注册全局模块。`exports` 的 value 可以是函数、基本类型或复杂的 Go Struct/Pointer。
  - `func GetModules() map[string]map[string]any`: 获取所有已注册的模块，供引擎层调用。

### 1.2 `go/js` (执行与引擎层)
- **定位**: 核心执行环境，依赖 `github.com/dop251/goja`、`go/cast` 和 `go/js/gojs`。
- **核心职责**: 维护虚拟机对象池 (Pool)、实现 Go-JS 双向数据桥接 (Bridge)、处理无状态调用 (Call)、以及生成 AI 友好的文档 (TS Definition)。

---

## 2. 核心技术规范与难点攻克

### 2.1 模块引入机制 (Import/Require)
- **规范**: JS 侧应该能通过类似 `const db = require('db')` 或 `import db from 'db'` 的方式加载 `gojs.Register` 注册的模块。
- **实现方案**: 优先考虑利用 `goja/require` 扩展，或者在 VM 初始化时，将所有注册的模块作为全局只读对象注入（例如全局暴露 `go.db`, `go.http`，或者直接劫持 require）。**测试阶段需敲定一种对 AI 最直观的引入方式**。

### 2.2 双向桥接与数据保真 (The Bridge)
这是项目的绝对核心，必须用海量的测试用例覆盖。
- **JS 调用 Go (入参)**:
  - 拦截 JS 传入的参数，如果 Go 函数的第一个参数是 `context.Context`，则自动将 `js.Call` 传入的 context 注入。
  - 对于普通的 JS 对象，使用 `goja.Value.Export()` 转为 Go 的原生 `any`，再通过 `go/cast.Convert` 精确投射到 Go 函数要求的 `Struct/Slice/Map` 类型上。
  - **快速失败**: 如果 `cast` 失败或参数个数不匹配，立刻 `panic`，并在桥接层 `recover` 抛出 JS 异常。
- **Go 返回 JS (出参 & Host Object)**:
  - **难点**: Go 返回的复杂对象（特别是带有指针、方法的 Struct）在 JS 中必须保持原样（Host Object）。
  - **验证要求**: 当 JS 收到这个 Go 包装对象后，如果仅仅是做一些传递，再次调用另一个 Go 函数并把该对象传回去，Go 侧接收到时，**必须能够精准还原为原来的指针/类型**，不能发生失真（不能变成普通的 map）。
  - **参考**: 利用 `goja` 的 `Runtime.ToValue(ptr)` 机制，默认情况下 `goja` 会保留底层 Go 类型。需编写严格测试。

### 2.3 无状态与全局池 (Versioned Pool)
- 虚拟机是非线程安全的，必须池化。
- **`js.Define(code string)`**: 定义或覆盖全局业务函数。每次调用时，内部版本号 `version++`，并将新代码追加到全局 Registry。
- **`js.Call(ctx context.Context, funcName string, args ...any) (any, error)`**:
  - 从 Pool 获取一个 `goja.Runtime`。
  - 检查 `vm.version` 是否落后于全局 `version`，若落后，则增量 `RunString` 未同步的代码块，并更新 `vm.version`。
  - 将 Go 的 `args` 转换为 JS arguments。
  - 获取并执行对应的 `funcName`。
  - 执行完毕后，清空 VM 中的临时状态（如果需要），归还 Pool。**确保不同 Call 之间绝对无状态隔离**。

### 2.4 智能文档生成 (TypeScript D.TS)
- **定位**: 为大语言模型 (AI) 生成准确的上下文。
- **`js.Doc() string`**:
  - 遍历 `gojs.GetModules()` 和动态 `Define` 的函数。
  - 利用 Go 的 `reflect` 包解析函数的入参、返回值类型。
  - 输出标准的 TypeScript 声明文件格式 (`.d.ts`)。不需要 100% 完美的泛型支持，但结构体字段、参数名、基础类型必须准确。

---

## 3. 开发执行步骤 (Steps for AI)

### Phase 1: 基础设施建设 (Registry)
- [ ] 初始化 `go/js/gojs` 目录和 `go.mod` (如果需要独立 mod)。
- [ ] 实现 `gojs.Register(name, map[string]any)` 及其内部存储。
- [ ] 编写对应的基础单元测试。

### Phase 2: 核心桥接器与数据保真 (Bridge & Test)
- [ ] 初始化 `go/js` 目录及依赖 (`go get github.com/dop251/goja`)。
- [ ] 实现 `wrapGoFunc`，处理 `context` 自动注入，利用 `cast` 进行参数的强类型转换。
- [ ] **必须编写极其严苛的测试用例 (`bridge_test.go`)**:
  - 测试基础类型双向转换。
  - 测试复杂的 Go Struct (带指针、嵌套) 传入 JS 的读取。
  - **关键测试**: `Go返回对象 -> JS变量 -> 将该变量传给另一个Go函数`，断言 Go 侧拿到的指针地址与原始地址一致。
  - 测试参数不足、类型错乱时的 Panic/Error 捕获机制（验证 AI 容错能力）。

### Phase 3: 对象池与生命周期管理 (Pool)
- [ ] 实现 `js.Define(code)`，管理全局代码片段和版本号。
- [ ] 实现 `js.Call(ctx, name, args...)`。
- [ ] 实现 VM 的获取、版本比对同步、增量编译、执行及回收逻辑。
- [ ] 编写高并发下的 `Call` 测试，确保对象池无锁竞争问题和状态隔离正常。

### Phase 4: AI 智能文档导出 (Doc)
- [ ] 实现反射解析 Go Struct 和 Func 的逻辑。
- [ ] 实现 TypeScript `.d.ts` 字符串的生成。
- [ ] 编写测试，断言生成的 TS 定义字符串符合预期。

---

## 4. 关键提示 (Hints)
- 遇到类型转换阻碍时，优先相信 `go/cast` 的能力，而不是在 bridge 里写大量的反射 if-else。
- 保证 `goja.Value.Export()` 的正确使用。
- 不要尝试在 `go/js` 内实现 HTTP 或服务发现，保持纯粹的计算和桥接引擎定位。