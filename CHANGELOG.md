# 变更记录

本库遵循[语义化版本](https://semver.org/lang/zh-CN/)：破坏性变更必须两个消费方（服务端 / 客户端）同步升级，库内不保留兼容分支。

## [v0.4.0] - 2026-10-02

### 新增

- `collection`：请求文件新增 `grpc` 段（`GRPCBlock`：`target` / `service` / `method` / `proto` / `imports` / `metadata` / `message` / `stream` / `tls`，另有 `GRPCTLS`：`mode` / `ca` / `cert` / `key` / `insecureSkipVerify`）。`RequestFile.GRPC` 以指针存在（非 nil 即 gRPC 请求），`RequestFile.Info.Type` 取值扩展为 `http` / `grpc`；`knownTopLevel` 同步加入 `grpc`，该段不再被收进 `Extra`。定义（`proto`）与 import 路径按约定相对集合目录，值支持 `{{变量}}`。

### 变更

- `collection`：`RequestFile.HTTP` 标签改为 `http,omitempty` 并新增 `HTTPBlock.IsZero()`（yaml.v3 认 `IsZeroer`）—— gRPC 请求文件不再写出空的 `http: {}` 段；`HTTP` 保持值类型，HTTP 请求文件的编码结果与既有读取方都不受影响。
- 工程：移除 GitHub Actions 工作流（`.github/workflows/ci.yml`），校验改由各仓库门禁脚本承担。

## [v0.2.0] - 2026-09-28

### 新增

- `postman`：Postman Collection v2.x ↔ 中立树。解析兼容 url 的字符串与 `{ raw }` 两种写法、过滤 disabled / 空名请求头、取 raw 请求体与首个响应样例；`Encode` 生成集合 JSON（含 schema、body options、响应样例）；`QueryPairs` 提取 URL 查询键值对。
- `openapi`：OpenAPI 3.0 ↔ 中立文档。解析 info（title/version）与 paths（按路径与方法字典序展开的 `Operation`，含 summary / operationId / description / tags / parameters / responses 示例）；`Encode` 生成文档（方法转小写、响应缺省补 `200`）。

### 变更

- 两端改为直接引用共享内核，业务映射仍留各自仓库：
  - 服务端：Postman / OpenAPI 的导入导出改调 `postman` / `openapi`，落库、分组复用、去重键、权限与操作日志仍留在 `service`；导入顺序由内核排序保证确定。
  - 客户端：新增导入入口 `App.ImportCollection`（format：`postman` / `openapi`）与 `Collection.ImportPostman` / `Collection.ImportOpenAPI`，把中立结构映射为本地请求并落盘，按「方法 + 地址」跳过重复。

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
