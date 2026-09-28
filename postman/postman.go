// Package postman 定义 Postman Collection v2.x 的**中立表示**与解析/生成规则。
//
// 中立 = 不依赖任何一端的业务实体：服务端把中立树映射为 model.Api + 分组后落库，
// 客户端把中立树映射为本地集合文件；两端共享的只有「Postman JSON ↔ 中立树」这一步。
package postman

import (
	"encoding/json"
	"strings"
)

// Schema Postman Collection v2.1 的 schema 标识（生成文档时写入 info.schema）。
const Schema = "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"

// Collection 中立集合：目录与请求组成的树。
type Collection struct {
	Name        string
	Description string
	Items       []Item
}

// Item 树节点：Folder 为真时读 Items，否则读 Req。
type Item struct {
	Name   string
	Folder bool
	Items  []Item
	Req    *Request
}

// Request 中立请求。Method 与 URL 保持文件原样，大小写、占位符处理由调用方决定。
type Request struct {
	Name           string
	Method         string
	URL            string
	Headers        []Header
	BodyRaw        string // body.mode=raw 时的原文
	Description    string
	ResponseSample string // response[0].body，作为响应样例
}

// Header 请求头（已过滤 disabled 与空名）。
type Header struct {
	Name  string
	Value string
}

// Decode 解析 Postman Collection JSON。
func Decode(data []byte) (*Collection, error) {
	var raw rawCollection
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return &Collection{
		Name:        raw.Info.Name,
		Description: textOf(raw.Info.Description),
		Items:       decodeItems(raw.Item),
	}, nil
}

// Encode 生成 Postman Collection（可直接 json.Marshal 后导出文件）。
func (c *Collection) Encode() map[string]any {
	items := make([]map[string]any, 0, len(c.Items))
	for _, it := range c.Items {
		items = append(items, it.encode())
	}
	// description 无条件写入，与历史导出结果保持一致。
	return map[string]any{
		"info": map[string]any{
			"name":        c.Name,
			"description": c.Description,
			"schema":      Schema,
		},
		"item": items,
	}
}

// QueryPairs 从 URL 的查询串提取键值对（按出现顺序，值保留原文）。
func QueryPairs(raw string) [][2]string {
	i := strings.Index(raw, "?")
	if i < 0 {
		return nil
	}
	query := raw[i+1:]
	if j := strings.IndexAny(query, "#"); j >= 0 {
		query = query[:j]
	}
	out := make([][2]string, 0, 4)
	for _, pair := range strings.Split(query, "&") {
		if pair == "" {
			continue
		}
		key, value := pair, ""
		if k := strings.Index(pair, "="); k >= 0 {
			key, value = pair[:k], pair[k+1:]
		}
		if key == "" {
			continue
		}
		out = append(out, [2]string{key, value})
	}
	return out
}

// ---- 编码 ----

func (it Item) encode() map[string]any {
	if !it.Folder {
		if it.Req == nil {
			return map[string]any{"name": it.Name}
		}
		return it.Req.encode(it.Name)
	}
	children := make([]map[string]any, 0, len(it.Items))
	for _, ch := range it.Items {
		children = append(children, ch.encode())
	}
	return map[string]any{"name": it.Name, "item": children}
}

func (r *Request) encode(name string) map[string]any {
	headers := make([]map[string]any, 0, len(r.Headers))
	for _, h := range r.Headers {
		headers = append(headers, map[string]any{"key": h.Name, "value": h.Value, "type": "text"})
	}
	req := map[string]any{"method": r.Method, "header": headers, "url": r.URL}
	if r.BodyRaw != "" {
		req["body"] = map[string]any{
			"mode":    "raw",
			"raw":     r.BodyRaw,
			"options": map[string]any{"raw": map[string]any{"language": "json"}},
		}
	}
	if r.Description != "" {
		req["description"] = r.Description
	}
	item := map[string]any{"name": name, "request": req}
	if r.ResponseSample != "" {
		item["response"] = []map[string]any{{
			"name":                     "成功响应",
			"status":                   "OK",
			"code":                     200,
			"_postman_previewlanguage": "json",
			"body":                     r.ResponseSample,
		}}
	}
	return item
}

// ---- 解码 ----

type rawCollection struct {
	Info rawInfo   `json:"info"`
	Item []rawItem `json:"item"`
}

type rawInfo struct {
	Name        string          `json:"name"`
	Description json.RawMessage `json:"description"`
}

type rawItem struct {
	Name     string        `json:"name"`
	Item     []rawItem     `json:"item"`
	Request  *rawRequest   `json:"request"`
	Response []rawResponse `json:"response"`
}

type rawRequest struct {
	Method      string          `json:"method"`
	URL         json.RawMessage `json:"url"`
	Header      []rawKV         `json:"header"`
	Body        *rawBody        `json:"body"`
	Description json.RawMessage `json:"description"`
}

type rawKV struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled"`
}

type rawBody struct {
	Mode string `json:"mode"`
	Raw  string `json:"raw"`
}

type rawResponse struct {
	Body string `json:"body"`
}

func decodeItems(raw []rawItem) []Item {
	out := make([]Item, 0, len(raw))
	for _, it := range raw {
		switch {
		case len(it.Item) > 0:
			out = append(out, Item{Name: it.Name, Folder: true, Items: decodeItems(it.Item)})
		case it.Request != nil:
			req := decodeRequest(it.Request, it.Response)
			req.Name = it.Name
			out = append(out, Item{Name: it.Name, Req: req})
		}
	}
	return out
}

func decodeRequest(raw *rawRequest, resps []rawResponse) *Request {
	r := &Request{
		Method:      raw.Method,
		URL:         urlOf(raw.URL),
		BodyRaw:     bodyOf(raw.Body),
		Description: textOf(raw.Description),
	}
	for _, h := range raw.Header {
		if h.Disabled || strings.TrimSpace(h.Key) == "" {
			continue
		}
		r.Headers = append(r.Headers, Header{Name: h.Key, Value: h.Value})
	}
	if len(resps) > 0 {
		r.ResponseSample = resps[0].Body
	}
	return r
}

// urlOf 兼容 url 的字符串与 `{ raw }` 两种写法。
func urlOf(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var obj struct {
		Raw string `json:"raw"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return strings.TrimSpace(obj.Raw)
	}
	return ""
}

// bodyOf 只取 raw 模式；其它模式（formdata / urlencoded）暂不映射。
func bodyOf(raw *rawBody) string {
	if raw == nil || !strings.EqualFold(strings.TrimSpace(raw.Mode), "raw") {
		return ""
	}
	return strings.TrimSpace(raw.Raw)
}

// textOf 兼容 description 的字符串与 `{ content }` 两种写法。
func textOf(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var obj struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return strings.TrimSpace(obj.Content)
	}
	return ""
}
