# 变更记录

本库遵循[语义化版本](https://semver.org/lang/zh-CN/)：破坏性变更必须两个消费方（服务端 / 客户端）同步升级，库内不保留兼容分支。

## [v0.1.0] - 2026-09-28

首个版本，从服务端与客户端各自实现中抽出「两端必须一致」的规则与定义。

### 新增

- `codegen`：curl / fetch / axios / go / python 五种代码片段模板、`ParseLang`、中立入参 `Request/KV`（原服务端 `service/code.go` 的模板函数）。
- `varx`：占位符 `{{name}}` / `{{$builtin}}` 语法、四级优先级（`ScopeRequest` > `ScopeProjectEnv` > `ScopeProjectCommon` > `ScopeGlobal`）、敏感值掩码 `••••••`、内置动态变量（`$uuid` / `$timestamp` / `$isoTimestamp` / `$randomInt`，取值器可注入）、`Names` 收集引用名。
- `collection`：集合文件格式定义（请求 / 分组 / 环境 / secret / 清单）与 YAML codec；未知顶层字段经 `Extra` round-trip 不丢失；命名规则 `ValidEnvName` / `ValidEntryName` / `SanitizeFileName` / `TrashName`；树节点排序 `SortNodes`。
- `mock`：URL 路径归一（`Path` / `NormalizePath` / `NormalizeURL`）与 Mock 匹配 `Match`（路径归一相同后方法优先，方法不匹配回退同路径首条）。
- 工程配套：`go.mod`（go 1.25）、CI（gofmt / build / vet / test + go 指令校验）、`README`、本变更记录。

### 语义归一（有意变更，两端已同步）

- 渲染占位符放宽为允许 `$` 前缀且不限长度；变量名写入校验仍为 64 位（各端自行校验）。
- 同一次 `Resolve` 内同名内置变量取同一个值（如两处 `{{$uuid}}` 结果相同）。
- 客户端未选择环境时不再合并全部环境，只保留内置动态变量。
