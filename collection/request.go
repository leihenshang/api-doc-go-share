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

// RequestFile 一条接口在磁盘上的形态（对应集合里的一个 .yml 文件）。
// Extra 保存未被识别的顶层字段，写回时必须带回去（Bruno 兼容）。
type RequestFile struct {
	Info     FileInfo         `yaml:"info"`
	Meta     FileMeta         `yaml:"meta"`
	HTTP     HTTPBlock        `yaml:"http"`
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
var knownTopLevel = map[string]bool{"info": true, "meta": true, "http": true, "docs": true, "settings": true}

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
