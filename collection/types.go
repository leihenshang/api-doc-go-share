// Package collection 定义集合文件（Bruno OpenCollection 风格 yml）的格式与纯编解码规则。
// 只搬「格式」不搬 IO：目录扫描、读写文件与 trash 移动由各端自己做，磁盘后半句一致靠这里。
package collection

import "gopkg.in/yaml.v3"

// RequestSettings 请求级覆盖项：未设置（零值）时沿用全局设置。
type RequestSettings struct {
	TimeoutSec      int   `yaml:"timeout,omitempty" json:"timeoutSec,omitempty"`
	FollowRedirects *bool `yaml:"followRedirects,omitempty" json:"followRedirects,omitempty"`
	MaxRedirects    int   `yaml:"maxRedirects,omitempty" json:"maxRedirects,omitempty"`
	InsecureSSL     *bool `yaml:"insecureSsl,omitempty" json:"insecureSsl,omitempty"`
	EncodeURL       bool  `yaml:"encodeUrl,omitempty" json:"encodeUrl,omitempty"`
}

// KV 请求参数 / 请求头 / 表单的一行。
// Description 为参数表「说明」列，空值不落盘（omitempty），不影响既有文件与 Bruno 互操作。
type KV struct {
	Name        string `yaml:"name" json:"name"`
	Value       string `yaml:"value" json:"value"`
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// Body 请求体：none / json / text / form（urlencoded）/ multipart。
// Raw 为客户端的 json / text 原文，Data 为 Bruno 的 data 字段，两者都保留以免读写第三方集合时丢内容。
type Body struct {
	Type string `yaml:"type" json:"type"`
	Data string `yaml:"data,omitempty" json:"data,omitempty"`
	Raw  string `yaml:"raw,omitempty" json:"raw"`
	Form []KV   `yaml:"form,omitempty" json:"form"`
}

// Auth 认证配置：none | basic | bearer | apikey；值支持 {{变量}}。
type Auth struct {
	Type     string `yaml:"type,omitempty" json:"type,omitempty"`
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
	Token    string `yaml:"token,omitempty" json:"token,omitempty"`
	Key      string `yaml:"key,omitempty" json:"key,omitempty"`
	Value    string `yaml:"value,omitempty" json:"value,omitempty"`
	In       string `yaml:"in,omitempty" json:"in,omitempty"` // apikey 位置：header | query
}

// UnmarshalYAML 兼容 Bruno 的标量写法（如 auth: inherit）。
func (a *Auth) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		a.Type = node.Value
		return nil
	}
	type plain Auth
	var tmp plain
	if err := node.Decode(&tmp); err != nil {
		return err
	}
	*a = Auth(tmp)
	return nil
}

// MarshalYAML 只有类型时回写标量，保持 Bruno 原样（如 auth: inherit）。
func (a Auth) MarshalYAML() (any, error) {
	if a.Username == "" && a.Password == "" && a.Token == "" && a.Key == "" && a.Value == "" && a.In == "" {
		return a.Type, nil
	}
	type plain Auth
	return plain(a), nil
}

// Node 集合树的节点。
type Node struct {
	Type     string  `json:"type"` // folder | request
	UID      string  `json:"uid"`
	Name     string  `json:"name"`
	Path     string  `json:"path"`
	Method   string  `json:"method,omitempty"`
	Seq      int     `json:"seq,omitempty"`
	Children []*Node `json:"children,omitempty"`
}
