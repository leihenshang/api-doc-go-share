// Package syncapi 是服务端与桌面客户端之间的**同步线协议**（wire contract）唯一真源。
//
// 为什么单独成包（P2-1）：同步的请求/响应结构与 SyncPayload 原来在两端各写一份
// （服务端 service/sync.go、客户端 syncengine/client.go），字段名/`json` tag/可选性
// 全靠人工同步——一端加了字段、另一端没加，表现是「某个字段同步丢失」且不报错。
// 现在两端都 type alias 到这里，字段不一致在编译期就不可能发生。
//
// 约定：本包只放**能在网络上对得上**的东西（结构 + 常量），不放业务规则、不做 IO。
// 改动这里等于改协议，必须：改 CHANGELOG → 打 tag → 两端升 require，
// 且破坏性改动要同步升 Proto。
package syncapi

import "encoding/json"

// Proto 当前同步协议版本（能力协商用，服务端 /meta 回传，客户端校验下限）。
const Proto = 1

// 条目类型（entity_type / push 的 type 字段取值）。
const (
	TypeGroup    = "group"
	TypeAPI      = "api"
	TypeDoc      = "doc"
	TypeExample  = "example"
	TypeVariable = "variable"
	TypeVarEnv   = "variable_env"
	// TypeFieldMapping 项目级字段字典（field_mapping 表）：field/type/description。
	TypeFieldMapping = "field_mapping"
)

// 操作类型。
const (
	OpUpsert = "upsert"
	OpDelete = "delete"
)

// push 逐条结果的状态取值。
const (
	StatusOK       = "ok"
	StatusConflict = "conflict"
	StatusRejected = "rejected"
)

// Meta 能力协商（GET /meta）。
type Meta struct {
	ServerVersion    string   `json:"server_version"`
	APIVersion       string   `json:"api_version,omitempty"`
	SyncProto        int      `json:"sync_proto"`
	Features         []string `json:"features"`
	MinClientVersion string   `json:"min_client_version,omitempty"`
}

// State 游标探测（GET /sync/state）。
type State struct {
	Cursor          int64 `json:"cursor"`
	OldestAvailable int64 `json:"oldest_available"`
}

// Item 增量/快照里的一条。
type Item struct {
	Type    string          `json:"type"`
	UID     string          `json:"uid"`
	Op      string          `json:"op"`
	Payload json.RawMessage `json:"payload,omitempty"`
	ItemRev int64           `json:"item_rev"`
	Deleted bool            `json:"deleted,omitempty"`
}

// Changes 增量响应（GET /sync/changes）。
type Changes struct {
	Cursor  int64  `json:"cursor"`
	Items   []Item `json:"items"`
	HasMore bool   `json:"has_more"`
	Gap     bool   `json:"gap"`
}

// Snapshot 全量响应（GET /sync/snapshot）。
type Snapshot struct {
	Cursor  int64  `json:"cursor"`
	Items   []Item `json:"items"`
	HasMore bool   `json:"has_more"`
	Offset  int    `json:"offset"`
}

// Op push 的一条（POST /sync/push 的 ops 元素）。
type Op struct {
	OpID    string          `json:"op_id"`
	Op      string          `json:"op"`
	Type    string          `json:"type"`
	UID     string          `json:"uid"`
	BaseRev int64           `json:"base_rev"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// PushReq push 请求体。
type PushReq struct {
	ProjectID  uint64 `json:"project_id"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	Ops        []Op   `json:"ops"`
}

// OpResult push 逐条结果。
type OpResult struct {
	OpID    string          `json:"op_id"`
	Status  string          `json:"status"` // ok | conflict | rejected
	ItemRev int64           `json:"item_rev,omitempty"`
	Reason  string          `json:"reason,omitempty"`
	Server  json.RawMessage `json:"server,omitempty"` // conflict 时的服务端版本
}

// PushResp push 响应体。
type PushResp struct {
	Results []OpResult `json:"results"`
	Cursor  int64      `json:"cursor"`
}

// Payload 条目序列化载荷（客户端按 Item.Type 解析）。
//
// 字段按条目类型分组，全部 omitempty：一条变更只会带上自己那组字段。
type Payload struct {
	// group
	Title    string `json:"title,omitempty"`
	PID      string `json:"parent_uid,omitempty"`
	Priority int    `json:"priority,omitempty"`
	// api
	Name     string `json:"name,omitempty"`
	Method   string `json:"method,omitempty"`
	URL      string `json:"url,omitempty"`
	GroupUID string `json:"group_uid,omitempty"`
	Extra    string `json:"extra,omitempty"` // 客户端全量原文 JSON（S11）
	// doc
	Content string `json:"content,omitempty"`
	Icon    string `json:"icon,omitempty"`
	// example（响应示例）：Name 复用为示例名
	APIUID      string `json:"api_uid,omitempty"`
	Sample      string `json:"sample,omitempty"`
	Status      int    `json:"status,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Seq         int    `json:"seq,omitempty"`
	// variable / variable_env（S16）：Name 复用为变量名/环境名；secret 变量的 Value
	// 为客户端密文（enc:v1: 前缀）——服务端只存储、不解析。
	EnvUID  string `json:"env_uid,omitempty"`
	Value   string `json:"value,omitempty"`
	Secret  bool   `json:"secret,omitempty"`
	Enabled *bool  `json:"enabled,omitempty"`
	// Redacted 为真表示服务端没有下发明文敏感值（调用方无明文权限，审核 H-11）：
	// 客户端必须保留本地已有值，不得用空值/掩码值覆盖。密文（enc:v1:）始终原样回传。
	Redacted bool `json:"redacted,omitempty"`
	// field_mapping：项目字段字典（Field 为字段路径如 data.list[].id，FieldType 为类型名）。
	Field       string `json:"field,omitempty"`
	FieldType   string `json:"field_type,omitempty"`
	Description string `json:"description,omitempty"`
	// 通用
	Deleted bool `json:"deleted,omitempty"`
}
