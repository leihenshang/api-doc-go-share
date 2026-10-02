package collection

import (
	"strings"
	"testing"
	"time"
)

// brunoSample 设计文档 §3.3 的 Bruno 超集样例：读入 → 写回必须逐字段 round-trip，
// 且未知顶层字段（vars/script/assert）不能丢。
const brunoSample = `info:
  name: 用户-列表
  type: http
  seq: 1
meta:
  uid: 0f5c2d7a-9b31-4c8e-a2f1-7d0e5b6a1c33
  base_rev: 4211
http:
  method: GET
  url: "{{host}}/api/user/list"
  params:
    - { name: name, value: "{{userName}}", type: query }
  headers:
    - { name: Authorization, value: "Bearer {{token}}" }
  auth: inherit
settings: { encodeUrl: true, timeout: 0, followRedirects: true, maxRedirects: 5 }
vars:
  pre-request:
    - { name: traceId, value: "{{$uuid}}", enabled: true }
script:
  pre-request: bru.setVar('ts', Date.now())
assert:
  - { name: 状态码为 200, expr: "res.status === 200" }
docs: |-
  ## 简要描述
  - 用户查询接口
`

func TestDecodeRequest(t *testing.T) {
	f, err := DecodeRequest([]byte(brunoSample))
	if err != nil {
		t.Fatalf("DecodeRequest: %v", err)
	}
	if f.Info.Name != "用户-列表" || f.Info.Type != "http" || f.Info.Seq != 1 {
		t.Fatalf("info 解析异常: %+v", f.Info)
	}
	if f.Meta.UID != "0f5c2d7a-9b31-4c8e-a2f1-7d0e5b6a1c33" || f.Meta.BaseRev != 4211 {
		t.Fatalf("meta 解析异常: %+v", f.Meta)
	}
	if f.HTTP.Method != "GET" || f.HTTP.URL != "{{host}}/api/user/list" {
		t.Fatalf("http 基础字段异常: %+v", f.HTTP)
	}
	if f.HTTP.Auth == nil || f.HTTP.Auth.Type != "inherit" {
		t.Fatalf("auth: inherit 未解析为标量: %+v", f.HTTP.Auth)
	}
	if f.Settings == nil || !f.Settings.EncodeURL || f.Settings.MaxRedirects != 5 || f.Settings.FollowRedirects == nil {
		t.Fatalf("settings 解析异常: %+v", f.Settings)
	}
	if len(f.HTTP.Params) != 1 || f.HTTP.Params[0].Value != "{{userName}}" {
		t.Fatalf("params 解析异常: %+v", f.HTTP.Params)
	}
	for _, key := range []string{"vars", "script", "assert"} {
		if _, ok := f.Extra[key]; !ok {
			t.Fatalf("未知顶层字段 %q 未收进 Extra: %+v", key, f.Extra)
		}
	}
}

// TestEncodeKeepsUnknownFields 写回时必须带回未知字段，否则 Bruno 里的数据会被悄悄删掉。
func TestEncodeKeepsUnknownFields(t *testing.T) {
	f, err := DecodeRequest([]byte(brunoSample))
	if err != nil {
		t.Fatalf("DecodeRequest: %v", err)
	}
	next := *f
	next.HTTP.Method = "POST"
	next.MergeExtra(f.Extra)

	data, err := next.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	text := string(data)
	for _, want := range []string{"method: POST", "auth: inherit", "encodeUrl: true", "traceId", "bru.setVar", "状态码为 200", "用户查询接口"} {
		if !strings.Contains(text, want) {
			t.Fatalf("输出缺少 %q:\n%s", want, text)
		}
	}
}

// TestAuthMarshalShape 只有类型时保持标量写回，带字段时为映射（Bruno 兼容）。
func TestAuthMarshalShape(t *testing.T) {
	if _, err := (&RequestFile{}).Encode(); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	bare := RequestFile{}
	bare.HTTP.Auth = &Auth{Type: "inherit"}
	data, err := bare.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !strings.Contains(string(data), "auth: inherit") {
		t.Fatalf("标量写法丢失:\n%s", data)
	}
	full := RequestFile{}
	full.HTTP.Auth = &Auth{Type: "basic", Username: "u", Password: "p"}
	data, err = full.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !strings.Contains(string(data), "username: u") {
		t.Fatalf("映射写法异常:\n%s", data)
	}
}

func TestSortNodes(t *testing.T) {
	nodes := []*Node{
		{Type: "request", Name: "b", Seq: 1},
		{Type: "folder", Name: "z"},
		{Type: "request", Name: "a", Seq: 2},
		{Type: "folder", Name: "a"},
		{Type: "request", Name: "a", Seq: 1},
	}
	SortNodes(nodes)
	want := [][2]string{{"folder", "a"}, {"folder", "z"}, {"request", "a"}, {"request", "b"}, {"request", "a"}}
	for i, w := range want {
		if nodes[i].Type != w[0] || nodes[i].Name != w[1] {
			t.Fatalf("第 %d 位为 %s/%s，want %s/%s", i, nodes[i].Type, nodes[i].Name, w[0], w[1])
		}
	}
}

func TestNameRules(t *testing.T) {
	if !ValidEnvName("dev") || !ValidEnvName("prod_2-1") || ValidEnvName("prod/x") || ValidEnvName("") {
		t.Fatal("ValidEnvName 判定不符")
	}
	if !ValidEntryName("用户 列表") || ValidEntryName("a/b") || ValidEntryName("") {
		t.Fatal("ValidEntryName 判定不符")
	}
	if got := SanitizeFileName(" a/b:c? "); got != "a-b-c-" {
		t.Fatalf("SanitizeFileName = %q", got)
	}
	if got := SanitizeFileName("  "); got != "untitled" {
		t.Fatalf("空名应兜底为 untitled，got %q", got)
	}
	at := time.Date(2026, 9, 28, 15, 4, 5, 0, time.UTC)
	if got := TrashName("user-list.yml", at); got != "20260928-150405.user-list.yml" {
		t.Fatalf("TrashName = %q", got)
	}
}

func TestEnvAndSecretsCodec(t *testing.T) {
	f := &EnvFile{}
	f.Info.Name = "dev"
	f.Vars = []Var{{Name: "host", Value: "http://127.0.0.1", Enabled: true}, {Name: "token", Secret: true}}
	data, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode env: %v", err)
	}
	back, err := DecodeEnv(data)
	if err != nil {
		t.Fatalf("DecodeEnv: %v", err)
	}
	if back.Info.Name != "dev" || len(back.Vars) != 2 || !back.Vars[1].Secret {
		t.Fatalf("环境 round-trip 不符: %+v", back)
	}
	sec, err := EncodeSecrets(map[string]string{"token": "t-1"})
	if err != nil {
		t.Fatalf("EncodeSecrets: %v", err)
	}
	got, err := DecodeSecrets(sec)
	if err != nil || got["token"] != "t-1" {
		t.Fatalf("secret round-trip 不符: %+v %v", got, err)
	}
}

func TestManifestCodec(t *testing.T) {
	m := &Manifest{}
	m.Info.Name, m.Meta.UID = "demo", "uid-1"
	data, err := m.Encode("1.0.0")
	if err != nil {
		t.Fatalf("Encode manifest: %v", err)
	}
	back, err := DecodeManifest(data)
	if err != nil {
		t.Fatalf("DecodeManifest: %v", err)
	}
	if back.OpenCollection != "1.0.0" || back.Info.Name != "demo" || back.Meta.UID != "uid-1" {
		t.Fatalf("清单 round-trip 不符: %+v", back)
	}
}

// grpc 段的新字段与服务端 / 客户端共享的 round-trip：compress（G7.5）。
func TestGRPCBlockEncodeKeepsCompress(t *testing.T) {
	f := &RequestFile{
		Info: FileInfo{Name: "SayHello", Type: "grpc", Seq: 1},
		Meta: FileMeta{UID: "u-1"},
		GRPC: &GRPCBlock{
			Target:   "localhost:50051",
			Service:  "demo.Greeter",
			Method:   "SayHello",
			Proto:    "protos/greeter.proto",
			Message:  `{"name":"alice"}`,
			Stream:   "unary",
			Compress: "gzip",
			TLS:      &GRPCTLS{Mode: "tls", InsecureSkipVerify: true},
		},
		Docs: "",
	}
	data, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !strings.Contains(string(data), "compress: gzip") {
		t.Fatalf("grpc 段应写出 compress：\n%s", data)
	}
	if strings.Contains(string(data), "http:") {
		t.Fatalf("gRPC 文件不应写出 http 段：\n%s", data)
	}
	back, err := DecodeRequest(data)
	if err != nil {
		t.Fatalf("DecodeRequest: %v", err)
	}
	if back.GRPC == nil || back.GRPC.Compress != "gzip" || back.GRPC.TLS == nil || !back.GRPC.TLS.InsecureSkipVerify {
		t.Fatalf("grpc 段 round-trip 不符: %+v", back.GRPC)
	}
}

// 清单里的集合级默认 gRPC 定义（P8）：配置一次，请求可回落使用。
func TestManifestGRPCDefaultCodec(t *testing.T) {
	m := &Manifest{}
	m.Info.Name, m.Meta.UID = "demo", "uid-1"
	m.GRPC = &GRPCDefault{Proto: "protos/greeter.proto", Imports: []string{"vendor/proto"}}
	data, err := m.Encode("1.0.0")
	if err != nil {
		t.Fatalf("Encode manifest: %v", err)
	}
	back, err := DecodeManifest(data)
	if err != nil {
		t.Fatalf("DecodeManifest: %v", err)
	}
	if back.GRPC == nil || back.GRPC.Proto != "protos/greeter.proto" || len(back.GRPC.Imports) != 1 {
		t.Fatalf("默认定义 round-trip 不符: %+v", back.GRPC)
	}
	// 未配置时不写出该段（旧清单不受影响）
	plain := &Manifest{}
	plain.Info.Name, plain.Meta.UID = "demo", "uid-2"
	pdata, err := plain.Encode("1.0.0")
	if err != nil {
		t.Fatalf("Encode manifest: %v", err)
	}
	if strings.Contains(string(pdata), "grpc:") {
		t.Fatalf("未配置默认定义时不应写出 grpc 段：\n%s", pdata)
	}
}
