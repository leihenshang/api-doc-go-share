# 变更记录

本库遵循[语义化版本](https://semver.org/lang/zh-CN/)：破坏性变更必须两个消费方（服务端 / 客户端）同步升级，库内不保留兼容分支。

## [v0.14.0] - 2026-10-23

### 修复 + 新增（向后兼容）

- **`openapi`（Q-02 / 审核 M-10）**：`Decode` 现在要求文档显式带 `openapi: 3.x`；Swagger 2.0（`swagger: "2.0"`）与缺失版本字段会返回可读错误，不再把任意 JSON 当成空文档“导入成功”。
- **`openapi.Param` 新增 `Type` / `Format`**：从参数 `schema.type/format` 解析（如 `integer`/`int64`），供服务端契约校验判断类型变化；`Encode` 会回写 `schema`。旧调用方不读这两个字段时行为不变。
- 服务端 `ImportOpenAPI` 同步把参数类型写入接口参数表；契约校验（RM-08）使用该字段。

## [v0.13.0] - 2026-10-22

### 新增（向后兼容）

- **`syncapi`**：新增同步实体类型 `field_mapping`（项目级字段字典，服务端 `field_mapping` 表）。`Payload` 增加 `field` / `field_type` / `description` 三键（均 `omitempty`），一条字段字典变更只带这三个键。
  - 协议键集不变（`Proto` 仍为 1）：旧客户端不识别该类型时按未知类型跳过，新客户端对未开启该能力的服务端也只会收到空集。
  - 消费方：发布前把服务端与客户端的 `go.mod` require 升到 `v0.13.0`（打 tag 后 `go get`）。
- **`syncapi.Payload.Redacted`**（审核 H-11）：服务端对**无明文权限**的调用方隐藏明文 secret 的 `value`，置 `redacted=true`；客户端应保留本地已有值。`enc:v1:` 密文始终原样回传（客户端可用口令解密后使用）。
- **`collection`**：`EnvFile` / `SecretsFile` / `Manifest` 补 `Extra` 未知顶层字段 round-trip（审核 M-1），新增 `DecodeSecretsFile`/`SecretsFile.Encode`；`marshalWithExtra` 跳过已知键，避免 Extra 注入已知键时产出重复 YAML 键（审核 M-2）。

## [v0.12.0] - 2026-10-20

### 安全修复（代码审核 2026-10-19 系列，行为收紧但不改类型形状）

- `codegen`：python 片段 payload 改为字符串字面量（不再裸拼源码：`hello`/JSON 布尔注入都不再成立）；HTTP method 入口白名单收敛，非法方法五种语言统一报错；`quote()` 补转义 `\n`/`\r`/`\t` 与 U+2028/U+2029（多行 JSON 不再产生跨行字面量）。
- `varx`：占位符名字原生支持 `-`/`.`（两端语义统一，客户端 alias shim 同步删除）；嵌套展开加记忆化 + 步数预算（2 万）+ 单值 256KB 输出熔断（~1KB 恶意集合文件不再能指数放大打挂两端）；`Builtins` 半填（Now/UUID 为 nil）回退默认实现不再 panic；`$uuid` 生成失败保留占位符不写空串缓存；环展开保留原文并记缺失。
- `openapi`：多 content type 的响应示例选择改确定（优先 application/json，余按字典序），两端导入结果不再分叉。
- `mock`：`Path` 支持无 scheme 的 `host[:port]/path` 写法剥地址段；模板变量地址需调用方先渲染（契约写入注释）。

## [v0.11.0] - 2026-10-17

### 新增（向后兼容）

- **`syncapi`**：服务端与桌面客户端之间的同步**线协议唯一真源**（P2-1）。此前 `Meta`/`State`/`Item`/`Changes`/`Snapshot`/`Op`/`OpResult`/`PushReq`/`PushResp`/`Payload` 在两端各写一份，字段与 `json` tag 靠人工对齐——一端加字段另一端没加的表现是「某字段同步丢失且不报错」。
  - 现在两端都 `type` alias 到本包，字段不一致在**编译期**就不可能发生；`syncapi` 同时导出类型/操作/状态的字符串常量与 `Proto`。
  - 附带按 JSON 键写的线格式守卫测试（`TestWireKeys`）：改 `json` tag 会先让测试失败，等于强制一次有意识的协议变更。
  - 消费方：服务端与客户端删除各自的本地定义、改为 alias，**行为与线上格式逐字节不变**；两端需把 `go.mod` 升到 `v0.11.0`。

## [v0.10.3] - 2026-10-17

### 修复（向后兼容，行为修正）

- `codegen`：`fetch` 与 `axios` 片段的请求体改为字符串字面量（走 `quote`）。此前 `fetch` 输出 `body: JSON.stringify(<原文>)`，对已经是 JSON 文本的 body 会**二次序列化**，而纯文本 body（如 `hello`）会生成 `JSON.stringify(hello)` ⇒ 粘贴运行即 `ReferenceError`；`axios` 的 `data: <原文>` 同样把纯文本当成裸标识符（S-05）。
- `collection`：`ValidEntryName` 拒绝 `.` / `..` / 点开头与点结尾的名称。条目名会直接当文件名或目录名用，此前 `..` 能指向上级目录、`.git` 会与工具目录撞名，与 `ValidEnvName` 的口径也不一致（S-09）。
- `collection`：`HTTPBlock.IsZero` 把 `Body.Descs` 计入判空。此前只填了字段备注的 `http` 段被判为空，被 `omitempty` 整段丢弃，用户的备注凭空消失（S-11）。
  - 消费方：**代码无需改动**（`quote`/`ValidEntryName` 签名未变），服务端与客户端各自把 `go.mod` 升到 `v0.10.3` 即可。`codegen_test` 的 curl golden 不变，新增 fetch/axios body 用例。

## [v0.10.2] - 2026-10-17

### 修复（向后兼容，行为修正）

- `openapi`：`Decode` 只把 Path Item 下的 8 个合法方法键（`get/put/post/delete/options/head/patch/trace`）当成操作。此前把整个 Path Item 的键集合都当方法展开，`summary` / `description` / `servers` / `parameters` / `$ref` / `x-*` 全被解析成操作，导入后生成一批方法名为 `SUMMARY`、`PARAMETERS` 的**幽灵接口**（S-02）。
  - 同时补上规范要求但此前被丢掉的 **Path Item 级共享参数**：先按 path 级声明顺序继承，同名同 `in` 由操作级参数原位覆盖。
  - 方法体不是对象（例如手写 `"get": "TODO"`）现在返回带 `paths[路径].方法` 的定位错误，而不是静默产出一个空操作。
- `mock`：`Path` 先切 `?` / `#` 再识别 scheme。此前先找 `://`，像 `/v1/jump?redirect=://evil` 这类地址会被截成 `/`；现在只有出现在第一个 `/` 之前的 `://` 才算 scheme 分隔符（S-04）。
  - 消费方：**代码无需改动**，服务端与客户端各自把 `go.mod` 升到 `v0.10.2` 即可。`openapi_test` / `mock_test` 已补守卫用例，原有 `TestDecode` / `TestRoundTrip` / `TestPath` 输出逐字节不变。

## [v0.10.1] - 2026-10-17

### 修复（向后兼容，行为修正）

- `codegen`：`curl` 片段对 URL / 请求头 / 请求体一律做 POSIX shell 引用（`shellQuote`：单引号包裹 + 内嵌 `'` 拆成 `'\''`），HTTP 方法仅在含非字母数字字符时才加引号（`shellToken`，`-X POST` 等常见形态逐字节不变）。
  - 原实现直接把用户数据拼进单引号：请求头值写成 `v'; rm -rf ~; echo '` 就能闭合引号、让**粘贴执行即运行任意命令**（S-01）。`fetch`/`axios`/`go`/`python` 片段走各自语言的字符串字面量，不在 shell 中执行，本来就安全，本次未改。
  - 消费方：**代码均无需改动**，但服务端与客户端必须各自把 `go.mod` 的 require 升到 `v0.10.1` 才会生效。`codegen_test` 的 `TestSnippetCurlGolden` 输出逐字节不变，`TestSnippetQuote` 已同步改为断言转义后的形态。

## [v0.10.0] - 2026-10-16

### 新增（向后兼容）

- `varx`：占位符支持**可选默认值段** `{{name:默认值}}`——名字（含未知 `$内置名`）解析不到时取默认值且不算缺失；已定义则忽略默认；默认值不允许含花括号。无默认段语法行为与旧版逐字节一致。

## [v0.9.0] - 2026-10-15

### 新增（向后兼容）

- `collection`：`EnvFile` 与 `EnvVar` 增加 `UID`（yaml/json `uid,omitempty`）——环境/变量的**同步稳定身份**（ext_uid）。变量类同步（S16）用它跨端对齐条目；旧文件缺省由客户端首次推送补齐，`omitempty` 对既有集合文件与 Bruno 互操作零影响。

## [v0.8.1] - 2026-10-15

### 新增（向后兼容）

- `collection`：`Body` 增加 `Descs map[string]string`（yaml/json `descs,omitempty`，`字段路径 → 备注`）。供客户端「请求体表格视图」维护 JSON 体各字段的备注，并作为同步 `bodyDescs` 键的真值源往返服务端。可选字段、`omitempty`，旧集合文件与读写方完全不受影响（无自定义 (un)marshal，随 `http.body` 自动落盘）。
  - 消费方：客户端 `SyncExtra`/`ApplySyncExtra` 已接入该键；服务端以 `http_request_body_descs` 文本列存储、原样回传（不解释语义）。服务端不读取该字段，无需改代码。

## [v0.8.0] - 2026-10-15

### 变更（破坏性）

- `collection`：`RequestSettings.EncodeURL` 由 `bool` 改为 `*bool`，与同结构体的 `FollowRedirects` / `InsecureSSL` 三态口径统一——`nil` = 未设置（消费方回落默认「编码」），`true` = 对 URL 查询串/路径做百分号编码，`false` = 原样发送不编码。此前零值 `false` 会被误当成「显式关闭编码」，无法区分「没配」与「配了关」。`collection_test` 已同步。
  - 消费方：客户端 `runner` 按 `*bool` 读取（`MergeOptions` 仅在非 nil 时覆盖，默认 `EncodeURL=true`，未配置行为与历史逐字节一致）；服务端不读取该字段。**客户端与服务端均需把 go.mod 升到 v0.8.0。**

## [v0.7.0] - 2026-10-10 【补记，审核 H-8】

### 变更

- `collection`：环境名校验规则重构（`cb3e989`），支持更多合法字符。属两端共用的**校验行为变更**，当时未记录CHANGELOG，此处追溯。

## [v0.6.0] - 2026-10-08 【补记，审核 H-8】

### 变更（破坏性）

- `collection`：导出类型 `Var` 重命名为 `EnvVar`（`9f7e8b4`，避开 JS 保留字）。当时按 minor 发布且无记录，按惯例应为 major/标注破坏性，此处追溯。

## [v0.5.0] - 2026-10-02

### 新增

- `codegen`：`GrpcurlSnippet`（grpcurl 命令生成，客户端 G11.5）。中立入参 `GRPCRequest`（target / service / method / message / metadata / proto / imports / plaintext / insecure）；定义路径拆成 `-import-path <目录> -proto <文件名>`（定义所在目录自动进 import 路径且去重）；所有参数按 POSIX shell 单引号转义（`'` → `'\''`），路径统一正斜杠输出，`-plaintext` / `-insecure` 按连接安全设置二选一。
- `collection`：`GRPCBlock.Compress`（`gzip` = 压缩请求体；空 / `identity` = 不压缩），供客户端「Options 分段」的压缩开关落盘（G7.5）。
- `collection`：清单 `Manifest.GRPC`（`GRPCDefault{Proto, Imports}`）—— **集合级默认 gRPC 定义**（客户端 P8）：多个请求共享同一份定义时只配一次，请求自身没写 `proto` 就回落到它。

### 变更

- 无破坏性变更：`compress` 与清单 `grpc` 段都是**新增可选字段**，旧请求文件与未配置的清单行为完全不变（单测锁定「未配置默认定义时不写出 `grpc` 段」）。

## [v0.4.0] - 2026-10-02

### 新增

- `collection`：请求文件新增 `grpc` 段（`GRPCBlock`：`target` / `service` / `method` / `proto` / `imports` / `metadata` / `message` / `stream` / `tls`，另有 `GRPCTLS`：`mode` / `ca` / `cert` / `key` / `insecureSkipVerify`）。`RequestFile.GRPC` 以指针存在（非 nil 即 gRPC 请求），`RequestFile.Info.Type` 取值扩展为 `http` / `grpc`；`knownTopLevel` 同步加入 `grpc`，该段不再被收进 `Extra`。定义（`proto`）与 import 路径按约定相对集合目录，值支持 `{{变量}}`。

### 变更

- `collection`：`RequestFile.HTTP` 标签改为 `http,omitempty` 并新增 `HTTPBlock.IsZero()`（yaml.v3 认 `IsZeroer`）—— gRPC 请求文件不再写出空的 `http: {}` 段；`HTTP` 保持值类型，HTTP 请求文件的编码结果与既有读取方都不受影响。
- 工程：移除 GitHub Actions 工作流（`.github/workflows/ci.yml`），校验改由各仓库门禁脚本承担。

## [v0.3.0] - 2026-10-01 【补记，审核 H-8】

### 变更

- 模块路径改名为 `github.com/leihenshang/api-doc-go-share`（`9b2a14c`）；消费方需改 import。无 CHANGELOG 记录，此处追溯。

## [v0.2.0] - 2026-09-28（未打 tag，不可 `go get`，仅文档记录）

### 新增

- `postman`：Postman Collection v2.x ↔ 中立树。解析兼容 url 的字符串与 `{ raw }` 两种写法、过滤 disabled / 空名请求头、取 raw 请求体与首个响应样例；`Encode` 生成集合 JSON（含 schema、body options、响应样例）；`QueryPairs` 提取 URL 查询键值对。
- `openapi`：OpenAPI 3.0 ↔ 中立文档。解析 info（title/version）与 paths（按路径与方法字典序展开的 `Operation`，含 summary / operationId / description / tags / parameters / responses 示例）；`Encode` 生成文档（方法转小写、响应缺省补 `200`）。

### 变更

- 两端改为直接引用共享内核，业务映射仍留各自仓库：
  - 服务端：Postman / OpenAPI 的导入导出改调 `postman` / `openapi`，落库、分组复用、去重键、权限与操作日志仍留在 `service`；导入顺序由内核排序保证确定。
  - 客户端：新增导入入口 `App.ImportCollection`（format：`postman` / `openapi`）与 `Collection.ImportPostman` / `Collection.ImportOpenAPI`，把中立结构映射为本地请求并落盘，按「方法 + 地址」跳过重复。

## [v0.1.0] - 2026-09-28（未打 tag，不可 `go get`，仅文档记录）

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
