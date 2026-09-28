package postman

import (
	"encoding/json"
	"strings"
	"testing"
)

// sample 覆盖：目录嵌套、请求（字符串 url、对象 url、disabled 头、raw body、响应样例）与中文描述。
const sample = `{
  "info": { "name": "演示集合", "description": "来自 Postman", "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json" },
  "item": [
    {
      "name": "用户",
      "item": [
        {
          "name": "用户-列表",
          "request": {
            "method": "get",
            "url": "https://api.example.com/api/user/list?page=1&size=20",
            "header": [
              { "key": "Authorization", "value": "Bearer {{token}}" },
              { "key": "X-Skip", "value": "1", "disabled": true },
              { "key": "", "value": "空名丢弃" }
            ],
            "body": { "mode": "raw", "raw": "{\"name\":\"alice\"}" },
            "description": { "content": "查询用户" }
          },
          "response": [ { "body": "{\"code\":200}" } ]
        },
        {
          "name": "空请求",
          "request": { "method": "", "url": "" }
        }
      ]
    },
    {
      "name": "未分组请求",
      "request": { "method": "POST", "url": { "raw": "{{host}}/api/login" } }
    }
  ]
}`

func TestDecode(t *testing.T) {
	col, err := Decode([]byte(sample))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if col.Name != "演示集合" || col.Description != "来自 Postman" {
		t.Fatalf("集合信息解析异常: %+v", col)
	}
	if len(col.Items) != 2 || !col.Items[0].Folder || col.Items[0].Name != "用户" {
		t.Fatalf("顶层节点异常: %+v", col.Items)
	}
	folder := col.Items[0]
	if len(folder.Items) != 2 {
		t.Fatalf("子节点异常: %+v", folder.Items)
	}
	req := folder.Items[0].Req
	if req == nil {
		t.Fatal("请求节点缺少 Req")
	}
	if req.Method != "get" || req.URL != "https://api.example.com/api/user/list?page=1&size=20" {
		t.Fatalf("方法与地址解析异常: %+v", req)
	}
	if len(req.Headers) != 1 || req.Headers[0].Name != "Authorization" {
		t.Fatalf("disabled / 空名请求头应被丢弃: %+v", req.Headers)
	}
	if req.BodyRaw != `{"name":"alice"}` || req.Description != "查询用户" {
		t.Fatalf("请求体或描述解析异常: %+v", req)
	}
	if req.ResponseSample != `{"code":200}` {
		t.Fatalf("响应样例解析异常: %q", req.ResponseSample)
	}
	// url 为 `{ raw }` 对象时同样能解析；method/url 为空的请求仍保留原样，由调用方决定是否跳过。
	if got := col.Items[1].Req.URL; got != "{{host}}/api/login" {
		t.Fatalf("对象写法 url 解析异常: %q", got)
	}
	if col.Items[1].Req.Method != "POST" {
		t.Fatalf("顶层请求方法解析异常: %q", col.Items[1].Req.Method)
	}
}

func TestDecodeInvalid(t *testing.T) {
	if _, err := Decode([]byte("{oops")); err == nil {
		t.Fatal("非法 JSON 应返回错误")
	}
}

func TestEncode(t *testing.T) {
	col := &Collection{
		Name:        "导出集合",
		Description: "说明",
		Items: []Item{
			{Name: "用户", Folder: true, Items: []Item{
				{Name: "用户-列表", Req: &Request{
					Method: "GET", URL: "https://a.com/api/user/list",
					Headers: []Header{{Name: "Authorization", Value: "Bearer t"}},
					BodyRaw: `{"name":"alice"}`, ResponseSample: `{"code":200}`,
				}},
			}},
			{Name: "登录", Req: &Request{Method: "POST", URL: "{{host}}/api/login"}},
			{Name: "空目录", Folder: true},
		},
	}
	data, err := json.Marshal(col.Encode())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		`"schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"`,
		`"key":"Authorization"`, `"value":"Bearer t"`, `"type":"text"`,
		`"mode":"raw"`, `"language":"json"`, `"code":200`, `"_postman_previewlanguage":"json"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("导出结果缺少 %q:\n%s", want, text)
		}
	}
	// 空目录也要保留，否则导出的目录树会缺层。
	if !strings.Contains(text, `"name":"空目录"`) {
		t.Fatalf("空目录丢失:\n%s", text)
	}
}

// TestRoundTrip Encode → Decode 后关键字段不丢。
func TestRoundTrip(t *testing.T) {
	col := &Collection{Name: "往返", Items: []Item{{
		Name: "用户-列表",
		Req: &Request{
			Method: "GET", URL: "https://a.com/api/user/list?a=1",
			Headers: []Header{{Name: "X-Token", Value: "t"}}, BodyRaw: `{"a":1}`,
			Description: "描述", ResponseSample: `{"code":200}`,
		},
	}}}
	data, err := json.Marshal(col.Encode())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	back, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	req := back.Items[0].Req
	if req.Method != "GET" || req.URL != "https://a.com/api/user/list?a=1" || req.BodyRaw != `{"a":1}` {
		t.Fatalf("往返后请求字段异常: %+v", req)
	}
	if req.Description != "描述" || req.ResponseSample != `{"code":200}` {
		t.Fatalf("往返后描述/样例异常: %+v", req)
	}
	if len(req.Headers) != 1 || req.Headers[0].Value != "t" {
		t.Fatalf("往返后请求头异常: %+v", req.Headers)
	}
}

func TestQueryPairs(t *testing.T) {
	pairs := QueryPairs("https://a.com/api/user/list?page=1&size=20&empty=#frag")
	if len(pairs) != 3 {
		t.Fatalf("键值对数量不符: %+v", pairs)
	}
	if pairs[0] != [2]string{"page", "1"} || pairs[1] != [2]string{"size", "20"} || pairs[2] != [2]string{"empty", ""} {
		t.Fatalf("键值对内容不符: %+v", pairs)
	}
	if got := QueryPairs("https://a.com/api/user/list"); got != nil {
		t.Fatalf("无查询串应返回 nil，got %+v", got)
	}
	if got := QueryPairs("https://a.com/x?=1&&a=2"); len(got) != 1 || got[0] != [2]string{"a", "2"} {
		t.Fatalf("空键与空段应跳过，got %+v", got)
	}
}
