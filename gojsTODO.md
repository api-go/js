# Go/JS 底层低代码引擎架构与开发计划

**目标受众**: 负责实现该项目的 AI 开发助手。
**项目背景**: 这是一个用于 Go 应用的低代码框架，其核心目标是提供一个 **极低摩擦、无状态、对 AI 极度友好** 的 Go/JS 桥接层。允许业务系统在不重启、不重新编译的情况下，通过动态执行 JS 代码来扩展能力。
**核心原则**: 性能优先、快速失败 (Fail-Fast)、严格的测试覆盖、彻底解耦底层依赖。

---

## 1. 架构总览

项目分为两个完全解耦的模块：

### 1.1 `go/jsmod` (注册与标准层)
- **定位**: 轻量级注册中心，**零第三方依赖**。其他 Go 业务模块（如 `go/db`, `go/http`）仅引入此包进行能力暴露，避免污染 `goja` 依赖。
- **核心 API**:
  - `func Register(name string, exports map[string]any)`: 注册全局模块。`exports` 的 value 可以是函数、基本类型或复杂的 Go Struct/Pointer。
  - `func GetModules() map[string]map[string]any`: 获取所有已注册的模块，供引擎层调用。

### 1.2 `go/js` (执行与引擎层)
- **定位**: 核心执行环境，依赖 `github.com/dop251/goja`、`go/cast` 和 `go/jsmod`。
- **核心职责**: 维护虚拟机对象池 (Pool)、实现 Go-JS 双向数据桥接 (Bridge)、处理无状态调用 (Call)、以及生成 AI 友好的文档 (TS Definition)。

---

## 2. 核心技术规范与难点攻克

### 2.1 模块引入机制 (Global Object)
- **规范**: JS 侧通过全局 `go.<moduleName>` 对象访问注册的模块。
- **实现方案**: 在 VM 初始化时，将所有从 `jsmod` 获取的模块注入到全局 `go` 对象中。

### 2.2 双向桥接与数据保真 (The Bridge)
- **JS 调用 Go (入参)**:
  - 拦截 JS 传入的参数，如果 Go 函数的第一个参数是 `context.Context`，则自动从 VM 的 `__ctx__` 注入。
  - 优先尝试直接赋值以保持 Host Object 指针一致性；若类型不匹配，则使用 `go/cast.Convert` 进行强类型转换。
  - **快速失败**: 如果 `cast` 失败，立刻 `panic` 抛出 JS 异常。
- **Go 返回 JS (出参 & Host Object)**:
  - **已验证**: 利用 `goja` 的 Host Object 机制，Go 返回的指针在 JS 传递后返回 Go 侧，地址完全一致。

### 2.3 无状态与全局池 (Versioned Pool)
- 虚拟机池化复用 (`sync.Pool`)。
- **`js.Define(code string)`**: 增加全局版本号。
- **`js.Call(ctx context.Context, funcName string, args ...any) (any, error)`**:
  - 自动增量同步落后的 VM 版本。
  - 确保 Call 之间无状态残留（每次 Call 注入新的 Context）。

### 2.4 智能文档生成 (TypeScript D.TS)
- **定位**: 为 AI 生成精准上下文。
- **`js.Doc() string`**: 自动反射 Go 模块，生成标准的 `.d.ts` 文件。

---

## 3. 开发执行步骤 (Status: ALL COMPLETED)

### Phase 1: 基础设施建设 (Registry)
- [x] 初始化 `go/jsmod` 独立仓库。
- [x] 实现 `jsmod.Register(name, map[string]any)`。
- [x] 编写基础单元测试。

### Phase 2: 核心桥接器与数据保真 (Bridge & Test)
- [x] 实现 `wrapGoFunc`，处理 `context` 自动注入，利用 `cast` 兼容转换。
- [x] **严苛测试验证**: `Go指针 -> JS变量 -> Go函数` 指针一致性通过。
- [x] 测试参数自动转换（如 "10" -> 10）通过。

### Phase 3: 对象池与生命周期管理 (Pool)
- [x] 实现 `js.Define(code)` 与增量版本同步。
- [x] 实现 `js.Call(ctx, name, args...)`。
- [x] 并发与版本同步测试通过。

### Phase 4: AI 智能文档导出 (Doc)
- [x] 实现反射解析 Go Struct 和 Func。
- [x] 生成 TypeScript `.d.ts` 字符串。
- [x] 测试验证生成内容准确性通过。

---

## 4. 关键提示 (Hints)
- 已经通过 `expV.Type().AssignableTo(argType)` 解决了指针丢失问题。
- `go/cast` 仅作为兜底转换，保证了性能与灵活性。
- 整个系统保持了无 CGO、纯 Go 的特性。
