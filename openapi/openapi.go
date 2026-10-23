// Package openapi 定义 OpenAPI 3.0 文档的**中立表示**与解析/生成规则。
//
// 中立 = 不依赖任何一端的业务实体：服务端把 Operation 映射为 model.Api 后落库，
// 客户端把 Operation 映射为本地集合请求；两端共享的只有「OpenAPI JSON ↔ 中立文档」这一步。
// 当前只处理 JSON（与两端现有导入行为一致），不解析 $ref 与 schema。
package openapi

import (
	"encoding/json"
	"fmt"
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
// Type/Format 来自 OpenAPI 的 schema（如 integer/int64、string/date-time），
// 供契约 diff 判断类型变化；缺失时为空串。
type Param struct {
	Name        string
	In          string // query / path / header / cookie
	Type        string
	Format      string
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
//
// Path Item 下并非所有键都是操作：summary / description / servers / parameters /
// $ref 以及 x-* 扩展也是合法键。只有下面这 8 个 HTTP 方法键才产出 Operation，
// 否则一份带 path 级 summary 的文档会导出一堆方法名为 "summary" 的鬼接口。
func Decode(data []byte) (*Document, error) {
	var raw rawDocument
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	// 审核 M-10 / Q-02：此前任意 JSON 都会被当成空文档导入成功；
	// 这里要求显式 openapi 3.x，并对 Swagger 2.0 给出可操作提示。
	if strings.TrimSpace(raw.Swagger) != "" {
		return nil, fmt.Errorf("检测到 Swagger 2.0（swagger: %s）；请先转换为 OpenAPI 3.x", raw.Swagger)
	}
	if strings.TrimSpace(raw.OpenAPI) == "" {
		return nil, fmt.Errorf("缺少 openapi 版本字段（需为 OpenAPI 3.x JSON）")
	}
	if !strings.HasPrefix(strings.TrimSpace(raw.OpenAPI), "3.") {
		return nil, fmt.Errorf("不支持的 OpenAPI 版本 %q（仅支持 3.x）", raw.OpenAPI)
	}
	doc := &Document{Title: raw.Info.Title, Version: raw.Info.Version}
	paths := make([]string, 0, len(raw.Paths))
	for p := range raw.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		item := raw.Paths[p]
		shared := pathItemParams(item)
		for _, m := range pathItemMethods(item) {
			var op rawOperation
			if err := json.Unmarshal(item[m], &op); err != nil {
				// 方法体不是对象（例如手写的 "get": "TODO"）时给出可定位的错误，
				// 而不是静默跳过整份文档里的这一个接口。
				return nil, fmt.Errorf("paths[%s].%s: %w", p, m, err)
			}
			doc.Ops = append(doc.Ops, op.toOperation(p, m, shared))
		}
	}
	return doc, nil
}

// httpMethods Path Item 下代表操作的合法键（OpenAPI 3 规范全集）。
var httpMethods = []string{"delete", "get", "head", "options", "patch", "post", "put", "trace"}

// pathItemMethods 返回该 Path Item 里实际存在的操作键，按字典序。
func pathItemMethods(item map[string]json.RawMessage) []string {
	out := make([]string, 0, len(httpMethods))
	for _, m := range httpMethods {
		if _, ok := item[m]; ok {
			out = append(out, m)
		}
	}
	return out
}

// pathItemParams 取 Path Item 级共享参数（可被同 name+in 的操作级参数覆盖）。
// 该段写坏时按「无共享参数」处理：宁可少几条参数，也不该让整次导入失败。
func pathItemParams(item map[string]json.RawMessage) []rawParam {
	rawParams, ok := item["parameters"]
	if !ok {
		return nil
	}
	var out []rawParam
	if err := json.Unmarshal(rawParams, &out); err != nil {
		return nil
	}
	return out
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
	if p.Type != "" {
		schema := map[string]any{"type": p.Type}
		if p.Format != "" {
			schema["format"] = p.Format
		}
		out["schema"] = schema
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
	OpenAPI string `json:"openapi"`
	Swagger string `json:"swagger"`
	Info    struct {
		Title   string `json:"title"`
		Version string `json:"version"`
	} `json:"info"`
	// Paths 的值保留为原始 JSON：Path Item 的键集合混杂着方法、共享参数与扩展，
	// 必须先按键名挑出真正的操作再解析（见 Decode）。
	Paths map[string]map[string]json.RawMessage `json:"paths"`
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
	Schema      struct {
		Type   string `json:"type"`
		Format string `json:"format"`
	} `json:"schema"`
}

type rawResponseBody struct {
	Description string `json:"description"`
	Content     map[string]struct {
		Example json.RawMessage `json:"example"`
	} `json:"content"`
}

// toOperation 合并 Path Item 级共享参数后生成中立操作。
// 同名同 in 的参数以操作级为准（OpenAPI 规定操作级覆盖 path 级）。
func (r rawOperation) toOperation(path, method string, shared []rawParam) Operation {
	op := Operation{
		Path:        path,
		Method:      method,
		Summary:     r.Summary,
		OperationID: r.OperationID,
		Description: r.Description,
		Tags:        r.Tags,
	}
	for _, p := range mergeParams(shared, r.Parameters) {
		op.Params = append(op.Params, Param{
			Name: p.Name, In: p.In, Type: p.Schema.Type, Format: p.Schema.Format,
			Description: p.Description, Example: scalarText(p.Example), Required: p.Required,
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

// exampleOf 取响应示例的 JSON 文本（无则空）。
// 审核 H-2：多 content type 时选择必须确定——优先 application/json，
// 否则按 content type 字典序取第一个带 example 的（map 遍历随机序会让两端导入结果分叉）。
func exampleOf(body rawResponseBody) string {
	if c, ok := body.Content["application/json"]; ok && len(c.Example) > 0 {
		return string(c.Example)
	}
	keys := make([]string, 0, len(body.Content))
	for k := range body.Content {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if ex := body.Content[k].Example; len(ex) > 0 {
			return string(ex)
		}
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

// mergeParams 合并 path 级与操作级参数：同名同 in 以操作级为准。
// 顺序上保持 path 级的声明位置做「原位替换」，操作级独有的参数追加在后，
// 这样导入后的参数表顺序与文档里的书写顺序一致。
func mergeParams(shared, own []rawParam) []rawParam {
	if len(shared) == 0 {
		return own
	}
	key := func(p rawParam) string { return p.In + "\x00" + p.Name }
	ownByKey := make(map[string]rawParam, len(own))
	for _, p := range own {
		ownByKey[key(p)] = p
	}
	out := make([]rawParam, 0, len(shared)+len(own))
	used := make(map[string]struct{}, len(own))
	for _, p := range shared {
		k := key(p)
		if replacement, ok := ownByKey[k]; ok {
			out = append(out, replacement)
			used[k] = struct{}{}
			continue
		}
		out = append(out, p)
	}
	for _, p := range own {
		k := key(p)
		if _, done := used[k]; done {
			continue
		}
		used[k] = struct{}{}
		out = append(out, p)
	}
	return out
}
