package openapi

import (
	"encoding/json"
	"strings"
	"testing"
)

// sample 覆盖：tags、summary/operationId、parameters（含 example）与 responses 示例。
const sample = `{
  "openapi": "3.0.0",
  "info": { "title": "演示项目", "version": "1.2" },
  "paths": {
    "/api/user/list": {
      "get": {
        "summary": "用户-列表",
        "operationId": "listUser",
        "tags": ["用户"],
        "parameters": [
          { "name": "page", "in": "query", "description": "页码", "required": true, "example": 1 }
        ],
        "responses": {
          "200": {
            "description": "成功",
            "content": { "application/json": { "example": { "code": 200, "data": [] } } }
          }
        }
      }
    },
    "/api/login": {
      "post": { "summary": "登录" }
    }
  }
}`

func TestDecode(t *testing.T) {
	doc, err := Decode([]byte(sample))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if doc.Title != "演示项目" || doc.Version != "1.2" {
		t.Fatalf("文档信息异常: %+v", doc)
	}
	if len(doc.Ops) != 2 {
		t.Fatalf("操作数量异常: %+v", doc.Ops)
	}
	// 路径与方法都按字典序展开（保证导入顺序稳定）。
	if doc.Ops[0].Path != "/api/login" || doc.Ops[0].Method != "post" {
		t.Fatalf("首个操作应为 /api/login post，got %+v", doc.Ops[0])
	}
	op := doc.Ops[1]
	if op.Path != "/api/user/list" || op.Method != "get" || op.Summary != "用户-列表" || op.OperationID != "listUser" {
		t.Fatalf("操作解析异常: %+v", op)
	}
	if len(op.Tags) != 1 || op.Tags[0] != "用户" {
		t.Fatalf("tags 解析异常: %+v", op.Tags)
	}
	if len(op.Params) != 1 || op.Params[0].Name != "page" || !op.Params[0].Required || op.Params[0].Example != "1" {
		t.Fatalf("参数解析异常: %+v", op.Params)
	}
	if len(op.Responses) != 1 || op.Responses[0].Code != "200" {
		t.Fatalf("响应解析异常: %+v", op.Responses)
	}
	var example map[string]any
	if err := json.Unmarshal([]byte(op.Responses[0].ExampleJSON), &example); err != nil || example["code"] != float64(200) {
		t.Fatalf("响应示例解析异常: %q (%v)", op.Responses[0].ExampleJSON, err)
	}
}

func TestDecodeInvalid(t *testing.T) {
	if _, err := Decode([]byte("{oops")); err == nil {
		t.Fatal("非法 JSON 应返回错误")
	}
}

func TestEncode(t *testing.T) {
	doc := &Document{Title: "导出项目", Version: "2.0", Ops: []Operation{
		{
			Path: "{{host}}/api/user/list", Method: "GET", Summary: "用户-列表",
			Tags:      []string{"用户"},
			Params:    []Param{{Name: "page", In: "query", Description: "页码", Required: true}},
			Responses: []Response{{Code: "200", Description: "成功", ExampleJSON: `{"code":200}`}},
		},
		{Path: "/api/login", Method: "post", Summary: "登录"},
	}}
	data, err := json.Marshal(doc.Encode())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		`"openapi":"3.0.0"`, `"title":"导出项目"`, `"version":"2.0"`,
		`"get":`, `"post":`, `"summary":"用户-列表"`, `"tags":["用户"]`,
		`"name":"page"`, `"in":"query"`, `"required":true`,
		`"example":{"code":200}`, `"description":"成功"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("导出结果缺少 %q:\n%s", want, text)
		}
	}
	// 无响应的操作补 200 兜底，避免生成的文档缺 responses 段。
	if !strings.Contains(text, `"responses":{"200":{"description":"成功"}}`) {
		t.Fatalf("缺少默认响应:\n%s", text)
	}
}

// TestRoundTrip Encode → Decode 后关键字段不丢。
func TestRoundTrip(t *testing.T) {
	doc := &Document{Title: "往返", Version: "1.0", Ops: []Operation{{
		Path: "/api/user/list", Method: "GET", Summary: "用户-列表", Description: "说明",
		Tags:      []string{"用户"},
		Params:    []Param{{Name: "page", In: "query", Example: "1"}},
		Responses: []Response{{Code: "200", Description: "成功", ExampleJSON: `{"code":200,"data":[]}`}},
	}}}
	data, err := json.Marshal(doc.Encode())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	back, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	op := back.Ops[0]
	if op.Path != "/api/user/list" || op.Method != "get" || op.Summary != "用户-列表" || op.Description != "说明" {
		t.Fatalf("往返后操作字段异常: %+v", op)
	}
	if len(op.Params) != 1 || op.Params[0].Example != "1" {
		t.Fatalf("往返后参数异常: %+v", op.Params)
	}
	if op.Responses[0].ExampleJSON != `{"code":200,"data":[]}` {
		t.Fatalf("往返后示例异常: %q", op.Responses[0].ExampleJSON)
	}
}
