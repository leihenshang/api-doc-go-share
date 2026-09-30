# api-doc-go-share

API-DOC（Go 服务端 + Wails 桌面客户端）的**共享底层库**：只放两端必须保持一致的「规则 / 算法 / 定义」，不放 IO、数据库、HTTP 框架与 UI。

- 模块路径：`github.com/leihenshang/api-doc-go-share`
- Go 版本：`1.25`（取两个消费方的较低者；库内不使用更高版本的新特性）
- 当前状态：本地联调（父目录 `go.work`），尚未发布远端

## 包含什么

| 包 | 内容 | 消费方 |
|---|---|---|
| `codegen` | 代码片段生成（curl / fetch / axios / go / python）、`ParseLang`、中立入参 `Request/KV` | 服务端接口导出；客户端代码生成 |
| `varx` | 占位符语法、四级优先级（请求 > 项目当前环境 > 项目通用 > 全局）、敏感值掩码、内置动态变量（可注入取值器便于测试） | 服务端变量解析与导出；客户端环境变量替换 |
| `collection` | 集合文件格式定义（Bruno OpenCollection 风格 yml：请求 / 分组 / 环境 / secret / 清单）+ YAML codec（未知字段 round-trip）+ 命名与排序规则 | 客户端集合读写；服务端 Bruno 导入 |
| `mock` | Mock 匹配规则：URL 路径归一 + 方法优先 + 同路径首条兜底 | 服务端内置 Mock 服务；客户端本地 Mock |
| `postman` | Postman Collection v2.x ↔ 中立树（目录 / 请求 / 请求头 / raw 请求体 / 响应样例）、`QueryPairs` | 服务端 Postman 导入导出；客户端 Postman 导入 |
| `openapi` | OpenAPI 3.0 ↔ 中立文档（`Operation`：summary / operationId / tags / parameters / responses 示例） | 服务端 OpenAPI 导入导出；客户端 OpenAPI 导入 |

**不包含**（按原则留在各自仓库）：文件 IO / 目录扫描、数据库与 ORM、gin / wails、真实网络发送、错误码契约（后续按需）。

## 设计原则

1. **只搬「规则 / 格式」，不搬 IO 驱动**。服务端是「拿到文件内容再解析」，客户端是「用 os 读目录」→ 库只提供结构体、编解码与规则。
2. **两端必须一致的行为才进来**；单端专用的逻辑留在各自仓库，不提前抽象。
3. **不在库里保留兼容分支或开关**来掩盖语义分叉：破坏性变更必须两端同步改。
4. 每个包只依赖标准库（`collection` 另加 `gopkg.in/yaml.v3`）。

## 依赖接入

### 本地联调（当前方式）

在三个仓库的**父目录**放 `go.work`（不入任何仓库，各仓库 `.gitignore` 已忽略）：

```go
go 1.25

use (
	./api-doc-go/server
	./api-doc-go-client
	./api-doc-go-share
)
```

两端 `go.mod` 零改动，直接 `go build ./...` 即可解析到本模块。

### 接入远端（后期）

1. 给本仓库打语义化 tag（如 `v0.1.0`）。
2. 消费方 `go.mod` 增加依赖，并配置私有仓库：

```bash
go env -w GOPRIVATE=github.com/leihenshang/*
go get github.com/leihenshang/api-doc-go-share@v0.1.0
```

不要提交相对路径 `replace`（远程 CI 会因路径不存在失败）。

## 开发与校验

```bash
gofmt -l .            # 应为空
go build ./...
go vet ./...
go test ./...         # 纯单测：无 DB、无网络、无外部服务
```

CI（`.github/workflows/ci.yml`）与上面命令同源，并额外校验 `go.mod` 的 `go` 指令 ≤ `1.25`。

## 目录说明

```
codegen/     代码片段模板
varx/        变量规则（占位符 / 优先级 / 掩码 / 内置）
collection/  集合文件格式定义与 codec、命名与排序规则
mock/        Mock 路径归一与匹配规则
postman/     Postman Collection 中立树与解析/生成
openapi/     OpenAPI 文档中立表示与解析/生成
doc/         设计与方案文档（共享模块方案.md 为完整方案与实施批次）；
             .doc/ 只放本地草稿，已在 .gitignore 中忽略
```

## 版本与变更

- 语义化版本，破坏性变更必须两端同步升级。
- 变更记录见 [CHANGELOG.md](./CHANGELOG.md)。

## 许可证

[MIT](./LICENSE)
