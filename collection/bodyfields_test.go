package collection

// 请求体字段表格（Body.Fields）与 API 概述（RequestFile.Summary）的编解码契约。
// 两组字段都是「用户手填、不能凭空消失」的数据：
//   - fields 只有表格没有原文时，http 段不能被 omitempty 整段丢掉（IsZero 必须覆盖 Fields）；
//   - summary 必须登记进 knownTopLevel，否则会被当未知字段收进 Extra 而重复/丢失。

import (
	"strings"
	"testing"
)

// bodyFieldsSample 含 summary + body.fields（且有 descs）的请求文件。
const bodyFieldsSample = `info:
  name: 创建订单
  type: http
  seq: 3
meta:
  uid: 6f1c2a9e
  base_rev: 0
summary: 下单接口，创建一笔待支付订单
http:
  method: POST
  url: "{{host}}/api/v1/orders"
  params: []
  headers:
    - name: Content-Type
      value: application/json
      enabled: true
  body:
    type: json
    raw: |-
      {
        "uid": "u-42",
        "items": [{ "sku": "A-1", "qty": 2 }]
      }
    descs:
      items[].sku: 商品编码
    fields:
      - name: uid
        type: string
        required: true
        example: u-42
        description: 用户ID
      - name: items[].sku
        type: string
        required: false
        example: A-1
        description: 商品编码
      - name: items[].qty
        type: number
        required: false
        example: "2"
docs: |-
  ## 简要描述
  - 下单
`

// TestRequestBodyFieldsRoundTrip summary 与 body.fields 经 解码 → 编码 → 再解码 后逐字段相等，
// 且 required=false / 空 example / 空 description 都不丢语义。
func TestRequestBodyFieldsRoundTrip(t *testing.T) {
	f, err := DecodeRequest([]byte(bodyFieldsSample))
	if err != nil {
		t.Fatalf("DecodeRequest: %v", err)
	}
	if f.Summary != "下单接口，创建一笔待支付订单" {
		t.Fatalf("summary 解析异常: %q", f.Summary)
	}
	if len(f.HTTP.Body.Fields) != 3 {
		t.Fatalf("fields 行数异常: %+v", f.HTTP.Body.Fields)
	}
	if got := f.HTTP.Body.Fields[0]; got.Name != "uid" || got.Type != "string" || !got.Required || got.Example != "u-42" || got.Description != "用户ID" {
		t.Fatalf("fields[0] 解析异常: %+v", got)
	}
	if got := f.HTTP.Body.Fields[1]; got.Name != "items[].sku" || got.Required || got.Example != "A-1" || got.Description != "商品编码" {
		t.Fatalf("fields[1] 解析异常: %+v", got)
	}

	data, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	back, err := DecodeRequest(data)
	if err != nil {
		t.Fatalf("DecodeRequest(round-trip): %v", err)
	}
	if back.Summary != f.Summary {
		t.Fatalf("summary 往返丢失: %q", back.Summary)
	}
	if len(back.HTTP.Body.Fields) != len(f.HTTP.Body.Fields) {
		t.Fatalf("fields 往返行数不一致: %d != %d", len(back.HTTP.Body.Fields), len(f.HTTP.Body.Fields))
	}
	for i := range f.HTTP.Body.Fields {
		if back.HTTP.Body.Fields[i] != f.HTTP.Body.Fields[i] {
			t.Fatalf("fields[%d] 往返不一致: %+v != %+v", i, back.HTTP.Body.Fields[i], f.HTTP.Body.Fields[i])
		}
	}
	if back.HTTP.Body.Descs["items[].sku"] != "商品编码" {
		t.Fatalf("descs 往返丢失: %+v", back.HTTP.Body.Descs)
	}
}

// TestFieldsOnlyBlockNotDropped 只有表格（无 type/raw/form/descs）的请求必须仍写出 http 段。
// 这是 IsZero 漏判 Fields 时的回归点：整段 http 被 omitempty 丢掉，表格与 summary 一起消失。
func TestFieldsOnlyBlockNotDropped(t *testing.T) {
	f := &RequestFile{
		Info:    FileInfo{Name: "只有表格", Type: "http", Seq: 1},
		Meta:    FileMeta{UID: "u-fields-only"},
		Summary: "只有表格的接口",
	}
	f.HTTP.Body.Fields = []BodyField{
		{Name: "uid", Type: "string", Required: true, Example: "u-42", Description: "用户ID"},
	}
	if f.HTTP.IsZero() {
		t.Fatalf("IsZero 误判：只有 body.fields 的 http 块被判为空")
	}
	data, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "http:") {
		t.Fatalf("只有 fields 时 http 段被整段丢弃:\n%s", text)
	}
	if !strings.Contains(text, "fields:") {
		t.Fatalf("fields 未落盘:\n%s", text)
	}
	if !strings.Contains(text, "summary: 只有表格的接口") {
		t.Fatalf("summary 未落盘:\n%s", text)
	}
	back, err := DecodeRequest(data)
	if err != nil {
		t.Fatalf("DecodeRequest: %v", err)
	}
	if len(back.HTTP.Body.Fields) != 1 || back.HTTP.Body.Fields[0].Name != "uid" {
		t.Fatalf("只有 fields 的请求往返丢失表格: %+v", back.HTTP.Body.Fields)
	}
}

// TestLegacyFileWithoutSummaryAndFields 老文件（无 summary / fields）解码后为零值，且 summary 不进 Extra。
func TestLegacyFileWithoutSummaryAndFields(t *testing.T) {
	f, err := DecodeRequest([]byte(brunoSample))
	if err != nil {
		t.Fatalf("DecodeRequest: %v", err)
	}
	if f.Summary != "" {
		t.Fatalf("老文件 summary 应为空: %q", f.Summary)
	}
	if len(f.HTTP.Body.Fields) != 0 {
		t.Fatalf("老文件 fields 应为空: %+v", f.HTTP.Body.Fields)
	}
	if _, ok := f.Extra["summary"]; ok {
		t.Fatalf("summary 被当成未知字段收进 Extra: %+v", f.Extra)
	}
	data, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if strings.Contains(string(data), "summary:") {
		t.Fatalf("空 summary 不应落盘（omitempty）:\n%s", data)
	}
}

// TestEmptyFieldsNotPersisted 空表格不落盘，老文件零改动。
func TestEmptyFieldsNotPersisted(t *testing.T) {
	f := &RequestFile{Info: FileInfo{Name: "空表", Type: "http", Seq: 1}, Meta: FileMeta{UID: "u1"}}
	f.HTTP.Body.Type = "json"
	f.HTTP.Body.Fields = []BodyField{}
	data, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if strings.Contains(string(data), "fields:") {
		t.Fatalf("空 fields 不应落盘（omitempty）:\n%s", data)
	}
}

// TestRequiredFalseExplicit 必填列不 omitempty：required: false 也显式落盘，便于阅读与 diff。
func TestRequiredFalseExplicit(t *testing.T) {
	f := &RequestFile{
		Info: FileInfo{Name: "必填列", Type: "http", Seq: 1},
		Meta: FileMeta{UID: "u2"},
	}
	f.HTTP.Body.Fields = []BodyField{{Name: "page", Type: "integer"}}
	data, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !strings.Contains(string(data), "required: false") {
		t.Fatalf("required: false 未显式落盘:\n%s", data)
	}
}
