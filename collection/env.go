package collection

import "gopkg.in/yaml.v3"

// EnvVar 环境变量；Secret 为真时值只存在 *.secrets.yml（本地明文，不进可提交文件）。
//
// 命名避开 JS 保留字 var：Wails 反射本类型生成前端绑定时，名为 Var 的返回类型会与
// JS 保留字冲突而被跳过（"Usage of reserved keyword found and not supported"）。
type EnvVar struct {
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
	Vars []EnvVar `yaml:"vars"`
}

// SecretsFile 环境的 secret 值文件（ *.secrets.yml ）。
type SecretsFile struct {
	Secrets map[string]string `yaml:"secrets"`
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

// DecodeSecrets 解析 secret 文件内容（主文件里 secret 变量只留占位）。
func DecodeSecrets(data []byte) (map[string]string, error) {
	var f SecretsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return f.Secrets, nil
}

// EncodeSecrets 序列化 secret 文件。
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
