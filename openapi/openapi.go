// Package openapi 定义 OpenAPI 3.0 文档的**中立表示**与解析/生成规则。
//
// 中立 = 不依赖任何一端的业务实体：服务端把 Operation 映射为 model.Api 后落库，
// 客户端把 Operation 映射为本地集合请求；两端共享的只有「OpenAPI JSON ↔ 中立文档」这一步。
// 当前只处理 JSON（与两端现有导入行为一致），不解析 $ref 与 schema。
package openapi

import (
	"encoding/json"
	"sort"
	"strings"
)

// Version 生成文档时写入的 openapi 字段。
const Version = "3.0.0"

// DefaultResponseDescription 响应缺少描述时的兜底文案。
const DefaultResponseDescription = "成功"

// Document 中立文档。
type Document struct {
	Title   string
	Version string
	Ops     []Operation
}

// Operation 一个「路径 + 方法」的操作。
type Operation struct {
	Path        string
	Method      string // 保持文件原样，大小写由调用方决定
	Summary     string
	OperationID string
	Description string
	Tags        []string
	Params      []Param
	Responses   []Response
}

// Param 请求参数。
type Param struct {
	Name        string
	In          string // query / path / header / cookie
	Description string
	Example     string
	Required    bool
}

// Response 响应：Code 为状态码，ExampleJSON 为 JSON 文本（原文保留）。
type Response struct {
	Code        string
	Description string
	ExampleJSON string
}

// Decode 解析 OpenAPI JSON；路径与方法按字典序展开，保证顺序确定。
func Decode(data []byte) (*Document, error) {
	var raw rawDocument
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	doc := &Document{Title: raw.Info.Title, Version: raw.Info.Version}
	paths := make([]string, 0, len(raw.Paths))
	for p := range raw.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		ops := raw.Paths[p]
		methods := make([]string, 0, len(ops))
		for m := range ops {
			methods = append(methods, m)
		}
		sort.Strings(methods)
		for _, m := range methods {
			doc.Ops = append(doc.Ops, ops[m].toOperation(p, m))
		}
	}
	return doc, nil
}

// Encode 生成 OpenAPI 文档（路径 key 即接口地址，方法转小写）。
func (d *Document) Encode() map[string]any {
	paths := map[string]any{}
	for _, op := range d.Ops {
		pathItem, ok := paths[op.Path].(map[string]any)
		if !ok {
			pathItem = map[string]any{}
			paths[op.Path] = pathItem
		}
		pathItem[strings.ToLower(strings.TrimSpace(op.Method))] = op.encode()
	}
	return map[string]any{
		"openapi": Version,
		"info":    map[string]any{"title": d.Title, "version": d.Version},
		"paths":   paths,
	}
}

func (o Operation) encode() map[string]any {
	op := map[string]any{"summary": o.Summary, "responses": encodeResponses(o.Responses)}
	if len(o.Tags) > 0 {
		op["tags"] = o.Tags
	}
	if o.Description != "" {
		op["description"] = o.Description
	}
	if len(o.Params) > 0 {
		params := make([]map[string]any, 0, len(o.Params))
		for _, p := range o.Params {
			params = append(params, p.encode())
		}
		op["parameters"] = params
	}
	return op
}

func (p Param) encode() map[string]any {
	in := p.In
	if in == "" {
		in = "query"
	}
	out := map[string]any{"name": p.Name, "in": in, "required": p.Required}
	if p.Description != "" {
		out["description"] = p.Description
	}
	if p.Example != "" {
		out["example"] = p.Example
	}
	return out
}

// encodeResponses 无响应时补一个 200 兜底，避免导出文档缺 responses 段。
func encodeResponses(resps []Response) map[string]any {
	out := map[string]any{}
	for _, r := range resps {
		desc := r.Description
		if desc == "" {
			desc = DefaultResponseDescription
		}
		item := map[string]any{"description": desc}
		if r.ExampleJSON != "" {
			var sample any
			if json.Unmarshal([]byte(r.ExampleJSON), &sample) == nil {
				item["content"] = map[string]any{"application/json": map[string]any{"example": sample}}
			}
		}
		code := r.Code
		if code == "" {
			code = "200"
		}
		out[code] = item
	}
	if len(out) == 0 {
		out["200"] = map[string]any{"description": DefaultResponseDescription}
	}
	return out
}

// ---- 解码用结构 ----

type rawDocument struct {
	Info struct {
		Title   string `json:"title"`
		Version string `json:"version"`
	} `json:"info"`
	Paths map[string]map[string]rawOperation `json:"paths"`
}

type rawOperation struct {
	Summary     string                     `json:"summary"`
	OperationID string                     `json:"operationId"`
	Description string                     `json:"description"`
	Tags        []string                   `json:"tags"`
	Parameters  []rawParam                 `json:"parameters"`
	Responses   map[string]rawResponseBody `json:"responses"`
}

type rawParam struct {
	Name        string          `json:"name"`
	In          string          `json:"in"`
	Description string          `json:"description"`
	Required    bool            `json:"required"`
	Example     json.RawMessage `json:"example"`
}

type rawResponseBody struct {
	Description string `json:"description"`
	Content     map[string]struct {
		Example json.RawMessage `json:"example"`
	} `json:"content"`
}

func (r rawOperation) toOperation(path, method string) Operation {
	op := Operation{
		Path:        path,
		Method:      method,
		Summary:     r.Summary,
		OperationID: r.OperationID,
		Description: r.Description,
		Tags:        r.Tags,
	}
	for _, p := range r.Parameters {
		op.Params = append(op.Params, Param{
			Name: p.Name, In: p.In, Description: p.Description,
			Example: scalarText(p.Example), Required: p.Required,
		})
	}
	codes := make([]string, 0, len(r.Responses))
	for code := range r.Responses {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		body := r.Responses[code]
		op.Responses = append(op.Responses, Response{
			Code: code, Description: body.Description, ExampleJSON: exampleOf(body),
		})
	}
	return op
}

// exampleOf 取 content.application/json.example 的 JSON 文本（无则空）。
func exampleOf(body rawResponseBody) string {
	for _, ct := range body.Content {
		if len(ct.Example) == 0 {
			continue
		}
		return string(ct.Example)
	}
	return ""
}

// scalarText 把 example 的标量转成文本；对象/数组保留 JSON 原文。
func scalarText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}
