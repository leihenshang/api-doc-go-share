package collection

import "gopkg.in/yaml.v3"

// Var 环境变量；Secret 为真时值只存在 *.secrets.yml（本地明文，不进可提交文件）。
type Var struct {
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
	Vars []Var `yaml:"vars"`
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
