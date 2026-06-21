# CHANGELOG - go/js

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
