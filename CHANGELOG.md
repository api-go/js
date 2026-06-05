# CHANGELOG - go/js

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
