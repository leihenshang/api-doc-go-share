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
	// 审核 C-2：method 也是不可信输入（Postman/OpenAPI 导入原样透传），
	// 在入口收敛成安全的 HTTP token 标识符，非法直接报错，杜绝片段的注入。
	method, err := normalizeMethod(r.Method)
	if err != nil {
		return "", err
	}
	r.Method = method
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

// normalizeMethod 把 method 收敛为代码安全的 token：空→GET；
// 合法形态为字母开头的 [A-Za-z0-9_-]（RFC 自定义方法允许），否则报错。
func normalizeMethod(m string) (string, error) {
	m = strings.ToUpper(strings.TrimSpace(m))
	if m == "" {
		return "GET", nil
	}
	for i, r := range m {
		alnum := (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-'
		if r >= 'a' && r <= 'z' {
			alnum = true // ToUpper 后不应出现，防御兼报错路径友好
		}
		if !alnum || (i == 0 && r >= '0' && r <= '9') {
			return "", fmt.Errorf("codegen: 非法 HTTP 方法 %q", m)
		}
	}
	return m, nil
}

// curlSnippet 生成可执行 cURL；输出格式与历史实现保持一致（回归点）。
// 所有插值一律走 shell 引用：URL / 请求头 / 请求体都来自用户数据，
// 直接拼进单引号会在遇到内嵌单引号时破坏引号配对，用户粘贴执行即被注入任意命令。
func curlSnippet(method, url string, headers []KV, body string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "curl -X %s %s", shellToken(method), shellQuote(url))
	for _, h := range headers {
		fmt.Fprintf(&b, " \\\n  -H %s", shellQuote(h.Name+": "+h.Value))
	}
	if body != "" {
		fmt.Fprintf(&b, " \\\n  -d %s", shellQuote(body))
	}
	return b.String()
}

// shellToken 生成一个 shell 词：仅由字母/数字/下划线/短横线组成时原样输出（保持 `-X POST` 的常见形态），
// 出现其它字符（含 ';' '|' '&' '$(' 空格等）则单引号包裹。
func shellToken(s string) string {
	if s == "" {
		return "''"
	}
	for _, r := range s {
		safe := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '-'
		if !safe {
			return shellQuote(s)
		}
	}
	return s
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
	// body 原样作为字符串字面量传：HTTP 请求体本来就是文本，拼成
	// JSON.stringify(<原文>) 会把已经序列化好的 JSON 再序列化一次，
	// 而非 JSON 文本（如 hello）会生成 JSON.stringify(hello) ⇒ 运行时 ReferenceError。
	if body != "" {
		fmt.Fprintf(&b, "\n  body: '%s',", quote(body))
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
	// 与 fetch 同理：data 用字符串字面量，避免非 JSON 文本被当成标识符而 ReferenceError。
	if body != "" {
		fmt.Fprintf(&b, "\n  data: '%s',", quote(body))
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
	// 审核 C-1：body 必须作为字符串字面量写入，不能裸拼进 Python 源码——
	// 裸拼时 `hello` 是 NameError、`{"ok":true}` 的 true 未定义，而多行/恶意 body
	// 直接变成代码注入面（复制运行即执行）。quote() 后交给 data= 的是 str，requests 接受。
	if body != "" {
		fmt.Fprintf(&b, "payload = '%s'\n", quote(body))
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
// 审核 H-1：除反斜杠/单引号外还必须转义裸换行（CR/LF/Tab）与 U+2028/U+2029——
// JS/Python 的单引号字面量不允许内嵌裸换行，美化过的多行 JSON（绝对主流）
// 过去会生成跨行字面量 → SyntaxError；两个行终止符在 JS 里同样断行。
func quote(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	s = strings.ReplaceAll(s, "\u2028", "\\u2028")
	s = strings.ReplaceAll(s, "\u2029", "\\u2029")
	return s
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
