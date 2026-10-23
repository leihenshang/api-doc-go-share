package collection

// M-1：环境/清单/secret 文件与请求/分组一样保留未知顶层字段（round-trip）。
// M-2：Extra 里即使混入已知键，编码时也不能重复追加，否则会产出重复 YAML 键、文件再也读不回来。

import (
	"strings"
	"testing"
)

const envExtraSample = `info:
  name: dev
uid: env-1
vars:
  - { name: host, value: "http://x", enabled: true }
future:
  nested: [1, 2]
`

// TestEnvFileExtraRoundTrip 环境主文件未知顶层字段必须原样写回（审核 M-1）。
func TestEnvFileExtraRoundTrip(t *testing.T) {
	f, err := DecodeEnv([]byte(envExtraSample))
	if err != nil {
		t.Fatalf("DecodeEnv: %v", err)
	}
	if _, ok := f.Extra["future"]; !ok {
		t.Fatalf("未知字段 future 未收进 Extra: %+v", f.Extra)
	}
	f.Vars[0].Value = "http://y"
	data, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !strings.Contains(string(data), "future:") {
		t.Fatalf("写回丢失未知字段:\n%s", data)
	}
	back, err := DecodeEnv(data)
	if err != nil {
		t.Fatalf("DecodeEnv(round-trip): %v", err)
	}
	if back.Vars[0].Value != "http://y" || back.Extra["future"] == nil {
		t.Fatalf("round-trip 不一致: %+v (extra=%+v)", back, back.Extra)
	}
}

// TestSecretsFileExtraRoundTrip secret 文件的未知顶层字段同样要保留（审核 M-1）。
func TestSecretsFileExtraRoundTrip(t *testing.T) {
	src := `secrets:
  token: abc
vault:
  hint: keep
`
	f, err := DecodeSecretsFile([]byte(src))
	if err != nil {
		t.Fatalf("DecodeSecretsFile: %v", err)
	}
	if _, ok := f.Extra["vault"]; !ok {
		t.Fatalf("未知字段 vault 未收进 Extra: %+v", f.Extra)
	}
	f.Secrets["token"] = "def"
	data, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	back, err := DecodeSecretsFile(data)
	if err != nil {
		t.Fatalf("DecodeSecretsFile(round-trip): %v", err)
	}
	if back.Secrets["token"] != "def" || back.Extra["vault"] == nil {
		t.Fatalf("round-trip 不一致: %+v (extra=%+v)", back, back.Extra)
	}
}

// TestManifestExtraRoundTrip 集合清单的未知顶层字段同样要保留（审核 M-1）。
func TestManifestExtraRoundTrip(t *testing.T) {
	src := `opencollection: 1.0.0
info:
  name: demo
meta:
  uid: u1
grpc:
  proto: a.proto
defaults:
  headers:
    - { name: X-Trace, value: v, enabled: true }
tool:
  custom: yes
`
	m, err := DecodeManifest([]byte(src))
	if err != nil {
		t.Fatalf("DecodeManifest: %v", err)
	}
	if _, ok := m.Extra["tool"]; !ok {
		t.Fatalf("未知字段 tool 未收进 Extra: %+v", m.Extra)
	}
	data, err := m.Encode("1.0.0")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	back, err := DecodeManifest(data)
	if err != nil {
		t.Fatalf("DecodeManifest(round-trip): %v", err)
	}
	if back.Info.Name != "demo" || back.Meta.UID != "u1" || back.Extra["tool"] == nil {
		t.Fatalf("round-trip 不一致: %+v (extra=%+v)", back, back.Extra)
	}
	if !strings.Contains(string(data), "tool:") {
		t.Fatalf("写回丢失未知字段:\n%s", data)
	}
}

// TestMarshalWithExtraSkipsKnownKeys 已知键混入 Extra 时不能重复追加（审核 M-2）：
// 否则 yaml.v3 会因重复键拒绝解析，文件永久损坏。
func TestMarshalWithExtraSkipsKnownKeys(t *testing.T) {
	f, err := DecodeRequest([]byte(brunoSample))
	if err != nil {
		t.Fatalf("DecodeRequest: %v", err)
	}
	f.Extra["info"] = map[string]any{"name": "evil"}
	f.Extra["http"] = "evil"
	data, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	back, err := DecodeRequest(data)
	if err != nil {
		t.Fatalf("重复键导致文件不可读: %v\n%s", err, data)
	}
	if back.Info.Name != "用户-列表" || back.HTTP.Method != "GET" {
		t.Fatalf("已知键应只由结构体输出，got %+v / %+v", back.Info, back.HTTP)
	}
	if strings.Contains(string(data), "evil") {
		t.Fatalf("Extra 里的已知键不应被写出:\n%s", data)
	}
}
