package collection

import (
	"sort"

	"gopkg.in/yaml.v3"
)

// FileInfo 请求/分组文件的 info 段。
type FileInfo struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
	Seq  int    `yaml:"seq"`
}

// FileMeta 请求/分组文件的 meta 段。
type FileMeta struct {
	UID     string `yaml:"uid"`
	BaseRev int64  `yaml:"base_rev"`
}

// HTTPBlock 请求文件的 http 段。
type HTTPBlock struct {
	Method  string `yaml:"method"`
	URL     string `yaml:"url"`
	Params  []KV   `yaml:"params"`
	Headers []KV   `yaml:"headers"`
	Body    Body   `yaml:"body"`
	Auth    *Auth  `yaml:"auth,omitempty"`
}

// IsZero 报告 http 段是否为空。配合 `yaml:"http,omitempty"` 让 gRPC 请求文件不带空的
// `http: {}` 段（yaml.v3 对实现了 IsZeroer 的值类型字段同样支持 omitempty）。
func (h HTTPBlock) IsZero() bool {
	return h.Method == "" && h.URL == "" && len(h.Params) == 0 && len(h.Headers) == 0 &&
		h.Body.Type == "" && h.Body.Raw == "" && h.Body.Data == "" && len(h.Body.Form) == 0 &&
		h.Auth == nil
}

// GRPCBlock 请求文件的 grpc 段（gRPC 请求；字段设计见
// api-doc-go-client/doc/客户端gRPC测试功能设计.md §2.1）。
// 定义与 import 路径都相对集合目录；message 为 protojson 原文；值支持 {{变量}}。
type GRPCBlock struct {
	Target   string   `yaml:"target" json:"target"`                       // 服务地址 host:port
	Service  string   `yaml:"service" json:"service"`                     // 完整服务名，如 demo.Greeter
	Method   string   `yaml:"method" json:"method"`                       // 方法名，如 SayHello
	Proto    string   `yaml:"proto,omitempty" json:"proto,omitempty"`     // 定义文件（相对集合目录）
	Imports  []string `yaml:"imports,omitempty" json:"imports,omitempty"` // import 搜索目录（相对集合目录）
	Metadata []KV     `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	Message  string   `yaml:"message,omitempty" json:"message,omitempty"`
	Stream   string   `yaml:"stream,omitempty" json:"stream,omitempty"` // unary | server | client | bidi
	TLS      *GRPCTLS `yaml:"tls,omitempty" json:"tls,omitempty"`
}

// GRPCTLS grpc 段的连接安全设置；Mode 为空/none 表示明文。
type GRPCTLS struct {
	Mode               string `yaml:"mode,omitempty" json:"mode,omitempty"`
	CA                 string `yaml:"ca,omitempty" json:"ca,omitempty"`
	Cert               string `yaml:"cert,omitempty" json:"cert,omitempty"`
	Key                string `yaml:"key,omitempty" json:"key,omitempty"`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify,omitempty" json:"insecureSkipVerify,omitempty"`
}

// RequestFile 一条接口在磁盘上的形态（对应集合里的一个 .yml 文件）。
// Extra 保存未被识别的顶层字段，写回时必须带回去（Bruno 兼容）。
// Info.Type 为 http / grpc：http 用 HTTP 段，grpc 用 GRPC 段，另一段为空并整体省略。
type RequestFile struct {
	Info     FileInfo         `yaml:"info"`
	Meta     FileMeta         `yaml:"meta"`
	HTTP     HTTPBlock        `yaml:"http,omitempty"`
	GRPC     *GRPCBlock       `yaml:"grpc,omitempty" json:"grpc,omitempty"`
	Settings *RequestSettings `yaml:"settings,omitempty"`
	Docs     string           `yaml:"docs"`
	Extra    map[string]any   `yaml:"-"`
}

// FolderFile 分组描述文件（folder.yml）。
type FolderFile struct {
	Info  FileInfo       `yaml:"info"`
	Meta  FileMeta       `yaml:"meta"`
	Docs  string         `yaml:"docs,omitempty"`
	Extra map[string]any `yaml:"-"`
}

// knownTopLevel 已知的顶层字段；其余键按「未知字段」原样保留。
var knownTopLevel = map[string]bool{"info": true, "meta": true, "http": true, "grpc": true, "docs": true, "settings": true}

// UnmarshalYAML 解码请求文件并把未知顶层字段收进 Extra。
func (f *RequestFile) UnmarshalYAML(node *yaml.Node) error {
	type plain RequestFile
	var tmp plain
	if err := node.Decode(&tmp); err != nil {
		return err
	}
	*f = RequestFile(tmp)
	f.Extra = unknownTopLevel(node)
	return nil
}

// MarshalYAML 编码请求文件并追加 Extra 中的未知字段（按字段名排序，输出稳定）。
func (f RequestFile) MarshalYAML() (any, error) {
	// plain 剥掉 MarshalYAML 方法，否则 node.Encode 会递归调用回来。
	type plain RequestFile
	return marshalWithExtra(plain(f), f.Extra)
}

// MergeExtra 把另一份文件的未知字段并入（已存在的键以本文件为准）。
func (f *RequestFile) MergeExtra(extra map[string]any) {
	if len(extra) == 0 {
		return
	}
	if f.Extra == nil {
		f.Extra = map[string]any{}
	}
	for k, v := range extra {
		if _, exists := f.Extra[k]; exists {
			continue
		}
		f.Extra[k] = v
	}
}

// UnmarshalYAML 解码分组文件（同样保留未知字段）。
func (f *FolderFile) UnmarshalYAML(node *yaml.Node) error {
	type plain FolderFile
	var tmp plain
	if err := node.Decode(&tmp); err != nil {
		return err
	}
	*f = FolderFile(tmp)
	f.Extra = unknownTopLevel(node)
	return nil
}

// MarshalYAML 编码分组文件并追加未知字段。
func (f FolderFile) MarshalYAML() (any, error) {
	type plain FolderFile
	return marshalWithExtra(plain(f), f.Extra)
}

// DecodeRequest 解析请求文件内容。
func DecodeRequest(data []byte) (*RequestFile, error) {
	var f RequestFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// Encode 序列化请求文件（含未知字段）。
func (f *RequestFile) Encode() ([]byte, error) {
	return yaml.Marshal(f)
}

// DecodeFolder 解析分组文件内容。
func DecodeFolder(data []byte) (*FolderFile, error) {
	var f FolderFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// Encode 序列化分组文件（含未知字段）。
func (f *FolderFile) Encode() ([]byte, error) {
	return yaml.Marshal(f)
}

// unknownTopLevel 取节点里不属于已知字段的顶层键值。
func unknownTopLevel(node *yaml.Node) map[string]any {
	var extra map[string]any
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i].Value
		if knownTopLevel[key] {
			continue
		}
		var v any
		if node.Content[i+1].Decode(&v) != nil {
			continue
		}
		if extra == nil {
			extra = map[string]any{}
		}
		extra[key] = v
	}
	return extra
}

// marshalWithExtra 先把主体编成节点，再按字段名字序追加未知字段。
func marshalWithExtra(v any, extra map[string]any) (any, error) {
	var node yaml.Node
	if err := node.Encode(v); err != nil {
		return nil, err
	}
	if len(extra) == 0 {
		return &node, nil
	}
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		var value yaml.Node
		if err := value.Encode(extra[k]); err != nil {
			continue
		}
		node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: k}, &value)
	}
	return &node, nil
}
