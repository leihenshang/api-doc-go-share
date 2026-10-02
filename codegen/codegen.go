// Package codegen 依据请求要素生成各语言的请求代码片段（curl / fetch / axios / go / python）。
// 入参是中立的 Request 结构，不依赖 model、HTTP 框架或网络 IO：同一份模板在服务端导出与客户端预览都可用。
package codegen

import (
	"fmt"
	"strconv"
	"strings"
)

// Lang 生成目标语言。
type Lang string

// 支持的语言取值。
const (
	LangCurl   Lang = "curl"
	LangFetch  Lang = "fetch"
	LangAxios  Lang = "axios"
	LangGo     Lang = "go"
	LangPython Lang = "python"
)

// KV 一行请求头。
type KV struct {
	Name  string
	Value string
}

// Request 代码生成的中立入参：URL、请求头与请求体均已按调用方规则渲染完毕。
type Request struct {
	Method  string
	URL     string
	Headers []KV
	Body    string
}

// ParseLang 把外部传入的语言串解析为 Lang（小写、去空白）；不支持时返回 false。
func ParseLang(s string) (Lang, bool) {
	switch l := Lang(strings.ToLower(strings.TrimSpace(s))); l {
	case LangCurl, LangFetch, LangAxios, LangGo, LangPython:
		return l, true
	}
	return "", false
}

// Snippet 生成指定语言的代码片段；l 不支持时返回错误。
func Snippet(l Lang, r Request) (string, error) {
	switch l {
	case LangCurl, LangFetch, LangAxios, LangGo, LangPython:
	default:
		return "", fmt.Errorf("codegen: 不支持的代码语言 %q", l)
	}
	if r.Method == "" {
		r.Method = "GET"
	}
	switch l {
	case LangFetch:
		return fetchSnippet(r.Method, r.URL, r.Headers, r.Body), nil
	case LangAxios:
		return axiosSnippet(r.Method, r.URL, r.Headers, r.Body), nil
	case LangGo:
		return goSnippet(r.Method, r.URL, r.Headers, r.Body), nil
	case LangPython:
		return pythonSnippet(r.Method, r.URL, r.Headers, r.Body), nil
	}
	return curlSnippet(r.Method, r.URL, r.Headers, r.Body), nil
}

// curlSnippet 生成可执行 cURL；输出格式与历史实现保持一致（回归点）。
func curlSnippet(method, url string, headers []KV, body string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "curl -X %s '%s'", method, url)
	for _, h := range headers {
		fmt.Fprintf(&b, " \\\n  -H '%s: %s'", h.Name, h.Value)
	}
	if body != "" {
		fmt.Fprintf(&b, " \\\n  -d '%s'", body)
	}
	return b.String()
}

// fetchSnippet 生成浏览器/Node 的 fetch 示例。
func fetchSnippet(method, url string, headers []KV, body string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "const res = await fetch('%s', {\n  method: '%s',", quote(url), method)
	if len(headers) > 0 {
		b.WriteString("\n  headers: {")
		for _, h := range headers {
			fmt.Fprintf(&b, "\n    '%s': '%s',", quote(h.Name), quote(h.Value))
		}
		b.WriteString("\n  },")
	}
	if body != "" {
		fmt.Fprintf(&b, "\n  body: JSON.stringify(%s),", body)
	}
	b.WriteString("\n});\nconst data = await res.json();\nconsole.log(data);")
	return b.String()
}

// axiosSnippet 生成 axios（TS）示例。
func axiosSnippet(method, url string, headers []KV, body string) string {
	var b strings.Builder
	b.WriteString("import axios from 'axios';\n\n")
	b.WriteString("const { data } = await axios.request({\n")
	fmt.Fprintf(&b, "  method: '%s',\n  url: '%s',", method, quote(url))
	if len(headers) > 0 {
		b.WriteString("\n  headers: {")
		for _, h := range headers {
			fmt.Fprintf(&b, "\n    '%s': '%s',", quote(h.Name), quote(h.Value))
		}
		b.WriteString("\n  },")
	}
	if body != "" {
		fmt.Fprintf(&b, "\n  data: %s,", body)
	}
	b.WriteString("\n});\nconsole.log(data);")
	return b.String()
}

// goSnippet 生成 Go net/http 示例。
func goSnippet(method, url string, headers []KV, body string) string {
	reader := "nil"
	if body != "" {
		reader = "strings.NewReader(" + strconv.Quote(body) + ")"
	}
	var b strings.Builder
	b.WriteString("package main\n\nimport (\n\t\"fmt\"\n\t\"io\"\n\t\"net/http\"\n")
	if body != "" {
		b.WriteString("\t\"strings\"\n")
	}
	b.WriteString(")\n\nfunc main() {\n")
	fmt.Fprintf(&b, "\treq, err := http.NewRequest(%s, %s, %s)\n", strconv.Quote(method), strconv.Quote(url), reader)
	b.WriteString("\tif err != nil {\n\t\tpanic(err)\n\t}\n")
	for _, h := range headers {
		fmt.Fprintf(&b, "\treq.Header.Set(%s, %s)\n", strconv.Quote(h.Name), strconv.Quote(h.Value))
	}
	b.WriteString("\tres, err := http.DefaultClient.Do(req)\n\tif err != nil {\n\t\tpanic(err)\n\t}\n")
	b.WriteString("\tdefer res.Body.Close()\n\tdata, _ := io.ReadAll(res.Body)\n\tfmt.Println(res.Status, string(data))\n}")
	return b.String()
}

// pythonSnippet 生成 Python requests 示例。
func pythonSnippet(method, url string, headers []KV, body string) string {
	var b strings.Builder
	b.WriteString("import requests\n\n")
	fmt.Fprintf(&b, "url = '%s'\n", quote(url))
	if len(headers) > 0 {
		b.WriteString("headers = {\n")
		for _, h := range headers {
			fmt.Fprintf(&b, "    '%s': '%s',\n", quote(h.Name), quote(h.Value))
		}
		b.WriteString("}\n")
	} else {
		b.WriteString("headers = {}\n")
	}
	if body != "" {
		fmt.Fprintf(&b, "payload = %s\n", body)
	}
	// 无请求体时不带 data 参数。
	if body != "" {
		fmt.Fprintf(&b, "res = requests.request('%s', url, headers=headers, data=payload)\n", method)
	} else {
		fmt.Fprintf(&b, "res = requests.request('%s', url, headers=headers)\n", method)
	}
	b.WriteString("print(res.status_code, res.text)")
	return b.String()
}

// quote 转义单引号字符串（JS/Python 通用）。
func quote(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	return strings.ReplaceAll(s, "'", "\\'")
}

// GRPCRequest grpcurl 生成的中立入参（gRPC 专用，字段均已按调用方规则渲染完毕）。
type GRPCRequest struct {
	Target  string // 服务地址 host:port
	Service string // 完整服务名，如 demo.Greeter
	Method  string // 方法名，如 SayHello
	Message string // 请求消息（protojson 原文；空 = 不带 -d）
	// Metadata 元数据行（顺序即生成顺序）
	Metadata []KV
	// Proto 入口定义（相对集合目录，如 protos/greeter.proto）；空 = 不带 -proto
	Proto string
	// Imports import 搜索目录（相对集合目录或绝对路径）
	Imports []string
	// Plaintext 明文连接（-plaintext）；TLS 下为 false
	Plaintext bool
	// Insecure TLS 且跳过证书校验（-insecure）
	Insecure bool
}

// GrpcurlSnippet 生成可直接执行的 grpcurl 命令。
//
// 约定：定义路径拆成「-import-path <目录> -proto <文件名>」（grpcurl 的 -proto 相对 import 路径），
// Proto 所在目录也会自动进 import 路径；所有参数一律单引号包裹，单引号按 shell 规则转义。
func GrpcurlSnippet(r GRPCRequest) string {
	parts := make([]string, 0, 8)
	switch {
	case r.Plaintext:
		parts = append(parts, "-plaintext")
	case r.Insecure:
		parts = append(parts, "-insecure")
	}
	paths := cleanPaths(r.Imports)
	if dir := protoDir(r.Proto); dir != "" && !containsPath(paths, dir) {
		paths = append(paths, dir)
	}
	for _, p := range paths {
		parts = append(parts, "-import-path "+shellQuote(p))
	}
	if name := protoBase(r.Proto); name != "" {
		parts = append(parts, "-proto "+shellQuote(name))
	}
	for _, kv := range r.Metadata {
		if strings.TrimSpace(kv.Name) == "" {
			continue
		}
		parts = append(parts, "-H "+shellQuote(kv.Name+": "+kv.Value))
	}
	if strings.TrimSpace(r.Message) != "" {
		parts = append(parts, "-d "+shellQuote(r.Message))
	}
	// 位置参数：目标地址 + 服务/方法（同一行的两个参数，便于整行替换）
	positional := shellQuote(strings.TrimSpace(r.Target))
	if r.Service != "" {
		positional += " " + shellQuote(r.Service+"/"+r.Method)
	}
	parts = append(parts, positional)
	return "grpcurl " + strings.Join(parts, " \\\n  ")
}

// protoDir 取定义文件的目录（无目录时返回空）。
func protoDir(rel string) string {
	rel = strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	if i := strings.LastIndex(rel, "/"); i > 0 {
		return rel[:i]
	}
	return ""
}

// protoBase 取定义文件名。
func protoBase(rel string) string {
	rel = strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		return rel[i+1:]
	}
	return rel
}

// shellQuote 单引号包裹（POSIX shell）：内部单引号按 '\” 拆分。
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// cleanPaths 去空白与重复（保持顺序）。
func cleanPaths(list []string) []string {
	out := make([]string, 0, len(list))
	for _, p := range list {
		p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
		if p == "" || containsPath(out, p) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func containsPath(list []string, p string) bool {
	for _, item := range list {
		if item == p {
			return true
		}
	}
	return false
}
