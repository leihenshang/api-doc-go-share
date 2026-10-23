package collection

import "gopkg.in/yaml.v3"

// EnvVar 环境变量；Secret 为真时值只存在 *.secrets.yml（本地明文，不进可提交文件）。
//
// 命名避开 JS 保留字 var：Wails 反射本类型生成前端绑定时，名为 Var 的返回类型会与
// JS 保留字冲突而被跳过（"Usage of reserved keyword found and not supported"）。
type EnvVar struct {
	// UID 变量稳定身份（同步用 ext_uid）；旧文件缺省时由客户端首次同步补齐。
	UID     string `yaml:"uid,omitempty" json:"uid,omitempty"`
	Name    string `yaml:"name" json:"name"`
	Value   string `yaml:"value" json:"value"`
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Secret  bool   `yaml:"secret" json:"secret"`
}

// EnvFile 环境主文件（文件名去掉扩展名即环境名）。
type EnvFile struct {
	Info struct {
		Name string `yaml:"name"`
	} `yaml:"info"`
	// UID 环境稳定身份（同步用 ext_uid）；旧文件缺省时由客户端首次同步补齐。
	UID  string   `yaml:"uid,omitempty" json:"uid,omitempty"`
	Vars []EnvVar `yaml:"vars"`
	// Extra 未被识别的顶层字段：整份重写时原样带回（审核 M-1）。
	Extra map[string]any `yaml:"-"`
}

// envKnownTopLevel 环境主文件已知的顶层字段。
var envKnownTopLevel = map[string]bool{"info": true, "uid": true, "vars": true}

// UnmarshalYAML 解码环境主文件并把未知顶层字段收进 Extra。
func (f *EnvFile) UnmarshalYAML(node *yaml.Node) error {
	type plain EnvFile
	var tmp plain
	if err := node.Decode(&tmp); err != nil {
		return err
	}
	*f = EnvFile(tmp)
	f.Extra = unknownTopLevel(node, envKnownTopLevel)
	return nil
}

// MarshalYAML 编码环境主文件并追加 Extra 中的未知字段（按字段名排序，输出稳定）。
func (f EnvFile) MarshalYAML() (any, error) {
	type plain EnvFile
	return marshalWithExtra(plain(f), f.Extra, envKnownTopLevel)
}

// SecretsFile 环境的 secret 值文件（ *.secrets.yml ）。
type SecretsFile struct {
	Secrets map[string]string `yaml:"secrets"`
	// Extra 未被识别的顶层字段：整份重写时原样带回（审核 M-1）。
	Extra map[string]any `yaml:"-"`
}

// secretsKnownTopLevel secret 文件已知的顶层字段。
var secretsKnownTopLevel = map[string]bool{"secrets": true}

// UnmarshalYAML 解码 secret 文件并把未知顶层字段收进 Extra。
func (f *SecretsFile) UnmarshalYAML(node *yaml.Node) error {
	type plain SecretsFile
	var tmp plain
	if err := node.Decode(&tmp); err != nil {
		return err
	}
	*f = SecretsFile(tmp)
	f.Extra = unknownTopLevel(node, secretsKnownTopLevel)
	return nil
}

// MarshalYAML 编码 secret 文件并追加 Extra 中的未知字段。
func (f SecretsFile) MarshalYAML() (any, error) {
	type plain SecretsFile
	return marshalWithExtra(plain(f), f.Extra, secretsKnownTopLevel)
}

// DecodeEnv 解析环境主文件内容。
func DecodeEnv(data []byte) (*EnvFile, error) {
	var f EnvFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// Encode 序列化环境主文件。
func (f *EnvFile) Encode() ([]byte, error) {
	return yaml.Marshal(f)
}

// DecodeSecretsFile 解析 secret 文件内容（含未知顶层字段，审核 M-1）。
func DecodeSecretsFile(data []byte) (*SecretsFile, error) {
	var f SecretsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// DecodeSecrets 解析 secret 文件里的值映射（主文件里 secret 变量只留占位）。
// 需要保留未知字段时用 DecodeSecretsFile。
func DecodeSecrets(data []byte) (map[string]string, error) {
	f, err := DecodeSecretsFile(data)
	if err != nil {
		return nil, err
	}
	return f.Secrets, nil
}

// Encode 序列化 secret 文件（含未知字段，审核 M-1）。
func (f *SecretsFile) Encode() ([]byte, error) {
	return yaml.Marshal(f)
}

// EncodeSecrets 序列化 secret 文件（无未知字段的便捷入口）。
func EncodeSecrets(secrets map[string]string) ([]byte, error) {
	return yaml.Marshal(&SecretsFile{Secrets: secrets})
}

// Manifest 集合清单 opencollection.yml。
type Manifest struct {
	OpenCollection string `yaml:"opencollection"`
	Info           struct {
		Name string `yaml:"name"`
	} `yaml:"info"`
	Meta struct {
		UID string `yaml:"uid"`
	} `yaml:"meta"`
	// GRPC 集合级默认 gRPC 定义：请求自身没写 proto 时回落到它（一次配置，多个请求共享）
	GRPC *GRPCDefault `yaml:"grpc,omitempty"`
	// Defaults 集合级公共请求参数：请求未覆盖时生效（请求头同名以请求为准，认证按 inherit 继承）。
	Defaults *Defaults `yaml:"defaults,omitempty"`
	// Extra 未被识别的顶层字段：整份重写时原样带回（审核 M-1）。
	Extra map[string]any `yaml:"-"`
}

// manifestKnownTopLevel 集合清单已知的顶层字段。
var manifestKnownTopLevel = map[string]bool{
	"opencollection": true, "info": true, "meta": true, "grpc": true, "defaults": true,
}

// UnmarshalYAML 解码集合清单并把未知顶层字段收进 Extra。
func (m *Manifest) UnmarshalYAML(node *yaml.Node) error {
	type plain Manifest
	var tmp plain
	if err := node.Decode(&tmp); err != nil {
		return err
	}
	*m = Manifest(tmp)
	m.Extra = unknownTopLevel(node, manifestKnownTopLevel)
	return nil
}

// MarshalYAML 编码集合清单并追加 Extra 中的未知字段。
func (m Manifest) MarshalYAML() (any, error) {
	type plain Manifest
	return marshalWithExtra(plain(m), m.Extra, manifestKnownTopLevel)
}

// Defaults 集合级公共请求参数（清单里的 defaults 段）。
// 一次性配置所有请求共享的请求头与认证；值支持 {{变量}}。
// 语义：请求头同名（大小写不敏感）由请求覆盖，enabled=false 的行不发送；
// 认证仅在请求 auth 为 inherit（或未设置）时生效，none/basic/bearer/apikey 均以请求为准。
type Defaults struct {
	Headers []KV  `yaml:"headers,omitempty"`
	Auth    *Auth `yaml:"auth,omitempty"`
}

// GRPCDefault 集合级默认 gRPC 定义（清单里的 grpc 段）。
// 只放「可被请求继承」的两项：定义文件与 import 搜索目录；服务与方法仍由每个请求自己写。
type GRPCDefault struct {
	Proto   string   `yaml:"proto,omitempty"`
	Imports []string `yaml:"imports,omitempty"`
}

// DecodeManifest 解析集合清单。
func DecodeManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Encode 序列化集合清单；version 为 opencollection 版本号。
func (m *Manifest) Encode(version string) ([]byte, error) {
	m.OpenCollection = version
	return yaml.Marshal(m)
}
