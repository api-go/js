## 未发布 — 日志联动 2026-11-12
- 显式联动实时压缩日志及文件级字典依赖，兼容明文 opt-out；正式版本待验收发布。

## [v1.5.18] - 2026-11-10

- 迁移至 github.com/api-go/js，同步内部模块路径与依赖。

# CHANGELOG - go/js

## [v1.5.17] - 2026-10-15
- Validate named functions from parsed syntax rather than source-text matching, preserving valid strings/comments/templates/regex and the nested-function restriction.

## [v1.5.16] - 2026-10-10
- 对齐 Log v1.5.17 及当前已发布的上游依赖。

## v1.5.15 (2026-09-12)
- **调用取消**: 新增 `Pool.CallContext` 与包级 `CallContext`，调用方取消或 Pool 停止时可靠中断 JavaScript，并避免复用 VM 残留中断信号。
- **桥接修复**: JavaScript `null` 与 `undefined` 参数安全映射为 Go 零值。

## v1.5.13 (2026-09-05)
- **依赖对齐**: 升级 `config`、`file` 与 `log` 至当前已发布版本。

## v1.5.12 (2026-09-05)
- **依赖对齐**: 升级全部 `github.com/api-go/*` 依赖至当前已发布版本。

## v1.5.11 (2026-08-24)
- **定义隔离**: `Define` 在发布到共享 VM Pool 前编译并在一次性隔离 VM 中验证脚本，拒绝语法错误、定义阶段异常、非函数结果和超过 1 秒的定义执行。
- **故障边界**: 无效定义不再增加 Pool 版本或进入共享脚本列表，避免单个低代码脚本阻塞其他 VM 和服务。
- **依赖对齐**: 更新 `log`、`config`、`encoding`、`file`、`id`、`rand`、`safe`、`shell` 至 monorepo 当前版本。

## v1.5.10 (2026-08-17)
- **嵌套回调桥接**: 可变参数中的 lower-camel 配置对象现可包含 JavaScript 回调；普通字段继续使用 `cast` 转换，函数字段由 goja 原生导出，支持 API 流式回调。

## v1.5.9 (2026-07-18)
- **依赖校验修复**: 重新整理模块 checksum，并对齐 `x/crypto v0.52.0`、`x/sys v0.45.0`。

## v1.5.8 (2026-07-13)
- **桥接一致性修复**: Go 函数返回的 API 门面对象，其导出方法也统一经由桥接层调用。
  JavaScript 的 lower-camel 对象字段会与顶层 API 一样自动转换为 Go struct 字段。
- **回归测试**: 覆盖 `table().Query({filter: [...]})` 形式的返回对象方法调用。

## v1.5.7 (2026-06-28)
- **低代码 TS 文档对齐**:
  - `Doc()` 生成的模块入口改为顶层全局变量声明，如 `declare const api: Api_Module;`，不再使用 `declare const go: { ... }` 包裹。
  - 为运行时注入的日志对象补充 `Logger` 接口，仅暴露 `Debug`、`Info`、`Warning`、`Error` 四个 JS 侧需要的方法。
  - 统一非项目类型的不透明命名风格，默认声明改为 `GoContext`、`GoHTTPRequest`、`GoHTTPResponse`、`GoURL`，标准库结构体采用 `GoTime` 这类更稳定的命名。
- **测试增强**:
  - `TestDocGeneration` 新增对顶层全局模块声明、`Logger` 暴露面和 `GoTime` 命名的覆盖。

## v1.5.6 (2026-06-21)
- **可变参数桥接修复**: `wrapGoFunc` 修复对 Go 可变参数（`...any`）的桥接处理。之前将 JS 剩余参数错误打包为单一切片元素，导致 `Call` 二次嵌套；现在改为逐个追加到 `goArgs` 尾部，由 `reflect.Call` 自动构建可变切片，确保 JS 调用 `redis.Do('HSET', a, b, c)` 正确展开。
- **新增测试**: `TestBridgeVariadic` 覆盖可变参数 0/1/多参数场景。

## v1.5.5 (2026-06-21)
- **JS 对齐**: 重构 JS 运行时的报错堆栈提取逻辑，采用高效的字符解析替换正则表达式，并支持 `jsmod.MakeError` 错误包装在桥接层还原出真实的 Go 运行时调用堆栈。
- **依赖更新**: 升级依赖 `jsmod` 至 `v1.5.3`，`cast` 至 `v1.5.3`，`log` 至 `v1.5.8`。

## v1.5.2 (2026-06-08)
- **API 增强: 脚本版本管理与发现**:
  - `Define` 方法现在支持可选的 `name` 和 `version` (int64) 参数。
  - 增加 `CheckVersion(name string, version int64) bool` 用于检查脚本是否已加载且版本匹配，减少重复加载。
  - 实现 `FuncList() []string`，支持动态发现脚本中定义的函数名（基于正则提取）。
- **执行安全: Context 中断支持**:
  - `Call` 方法现在支持 `context.Context` 中断。如果 context 被取消或超时，JS 执行将被立即中断。
- **性能优化**:
  - 优化 `createNewRuntime` 中的反射检查逻辑。
  - 优化 `Call` 方法，在 Context 不可取消时避免创建额外的 goroutine。
- **稳定性**:
  - 完善 `Pool` 状态上报，包含已加载脚本和函数数量。

## v1.5.1 (2026-06-05)
- **架构重构: 多例支持与优雅停机**:
  - 引入 `Pool` 结构体，支持通过 `js.NewPool()` 创建相互隔离的执行环境，避免业务间脚本冲突。
  - 实现 `starter.Service` 接口（Start, Stop, Status），支持集成到基础设施生命周期管理中。
  - 优化优雅停机逻辑：`Stop` 会自动阻塞并等待所有活跃的 JS 调用完成，或直到 Context 超时。
- **文档增强: AI/IDE 丝滑对齐**:
  - 彻底重写 `js.Doc()`，采用 `declare const go` 全局声明，支持 VSCode 零配置代码提示。
  - **穿透防护**: 自动识别并拦截非项目路径下的结构体，统一映射为 `GoPkg_Name` 不透明句柄。
  - **方法提纯**: 自动过滤涉及 `io.Reader/Writer`、`reflect`、`sync` 等 JS 无法处理的底层方法。
  - **放行 time.Time**: 开放时间对象的方法反射，支持 JS 直接调用 `Unix()`, `Format()` 等业务方法。
  - **隐式类型导出**: 支持 `__export` 命名前缀，用于导出类型结构而不暴露工厂函数。
- **稳定性**: 修复了基于原始类型别名（如 `time.Month`）导致反射崩溃的问题。
