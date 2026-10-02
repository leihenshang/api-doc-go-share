package codegen

import (
	"strings"
	"testing"
)

func sample() Request {
	return Request{
		Method:  "POST",
		URL:     "https://dev.example.com/api/user/list?page=1",
		Headers: []KV{{Name: "Authorization", Value: "Bearer t-123"}, {Name: "X-Empty", Value: ""}},
		Body:    `{"name":"alice"}`,
	}
}

func TestParseLang(t *testing.T) {
	cases := map[string]bool{
		"curl": true, "CURL": true, " go ": true, "  Fetch": true, "axios": true, "python": true,
		"ruby": false, "": false,
	}
	for in, want := range cases {
		_, ok := ParseLang(in)
		if ok != want {
			t.Fatalf("ParseLang(%q) ok = %v, want %v", in, ok, want)
		}
	}
}

func TestSnippetCurlGolden(t *testing.T) {
	want := "curl -X POST 'https://dev.example.com/api/user/list?page=1' \\\n" +
		"  -H 'Authorization: Bearer t-123' \\\n" +
		"  -H 'X-Empty: ' \\\n" +
		"  -d '{\"name\":\"alice\"}'"
	got, err := Snippet(LangCurl, sample())
	if err != nil {
		t.Fatalf("Snippet(curl): %v", err)
	}
	if got != want {
		t.Fatalf("curl 片段与回归基线不一致:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestSnippetWithoutBody(t *testing.T) {
	r := sample()
	r.Body = ""
	got, err := Snippet(LangCurl, r)
	if err != nil {
		t.Fatalf("Snippet(curl): %v", err)
	}
	if strings.Contains(got, "-d '") {
		t.Fatalf("无请求体时不应输出 -d，got:\n%s", got)
	}
	if _, err := Snippet(LangPython, r); err != nil {
		t.Fatalf("Snippet(python): %v", err)
	}
	if _, err := Snippet(LangGo, r); err != nil {
		t.Fatalf("Snippet(go): %v", err)
	}
}

func TestSnippetDefaultMethod(t *testing.T) {
	r := sample()
	r.Method = ""
	got, err := Snippet(LangCurl, r)
	if err != nil {
		t.Fatalf("Snippet(curl): %v", err)
	}
	if !strings.HasPrefix(got, "curl -X GET ") {
		t.Fatalf("空方法应补全为 GET，got:\n%s", got)
	}
}

func TestSnippetAllLangs(t *testing.T) {
	// 各语言片段都应带上渲染后的 URL 与请求头。
	for _, l := range []Lang{LangCurl, LangFetch, LangAxios, LangGo, LangPython} {
		code, err := Snippet(l, sample())
		if err != nil {
			t.Fatalf("Snippet(%s): %v", l, err)
		}
		for _, want := range []string{"https://dev.example.com/api/user/list?page=1", "Bearer t-123", "POST"} {
			if !strings.Contains(code, want) {
				t.Fatalf("%s 片段应包含 %q，got:\n%s", l, want, code)
			}
		}
	}
}

func TestSnippetQuote(t *testing.T) {
	r := Request{Method: "GET", URL: "http://h/p"}
	r.Headers = []KV{{Name: "X-Q", Value: "a'b\\c"}}
	got, err := Snippet(LangFetch, r)
	if err != nil {
		t.Fatalf("Snippet(fetch): %v", err)
	}
	if !strings.Contains(got, `'X-Q': 'a\'b\\c'`) {
		t.Fatalf("fetch 片段应转义引号: %q", got)
	}
	// cURL 历史输出不做转义（回归基线），迁移时保持原样。
	if curled, err := Snippet(LangCurl, r); err != nil || !strings.Contains(curled, `'X-Q: a'b\c'`) {
		t.Fatalf("curl 片段保持不转义，got: %q err: %v", curled, err)
	}
}

func TestSnippetUnknownLang(t *testing.T) {
	if _, err := Snippet(Lang("ruby"), sample()); err == nil {
		t.Fatal("不支持的语言应返回错误")
	}
}

// grpcurl 片段（G11.5）：明文、import 路径拆分、元数据、消息转义与位置参数。
func TestGrpcurlSnippet(t *testing.T) {
	got := GrpcurlSnippet(GRPCRequest{
		Target:    "localhost:50051",
		Service:   "demo.Greeter",
		Method:    "SayHello",
		Message:   `{"name":"alice"}`,
		Metadata:  []KV{{Name: "x-token", Value: "t-123"}, {Name: "", Value: "ignored"}},
		Proto:     "protos/greeter.proto",
		Imports:   []string{"vendor/proto", "protos"},
		Plaintext: true,
	})
	for _, want := range []string{
		"grpcurl ",
		"-plaintext",
		"-import-path 'vendor/proto'",
		"-import-path 'protos'",  // 定义所在目录去重后仍只在 import 路径里出现一次
		"-proto 'greeter.proto'", // -proto 用文件名（相对 import 路径）
		"-H 'x-token: t-123'",
		`-d '{"name":"alice"}'`,
		"'localhost:50051' 'demo.Greeter/SayHello'",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("grpcurl 片段缺少 %q：\n%s", want, got)
		}
	}
	if strings.Count(got, "-import-path 'protos'") != 1 {
		t.Errorf("定义目录应去重：\n%s", got)
	}
	if strings.Contains(got, "ignored") {
		t.Errorf("空名元数据不应生成 -H：\n%s", got)
	}
}

func TestGrpcurlSnippetTLSAndEscaping(t *testing.T) {
	// TLS + 跳过校验 → -insecure（且不带 -plaintext）
	got := GrpcurlSnippet(GRPCRequest{Target: "h:443", Service: "s.M", Method: "F", Proto: "greeter.proto", Insecure: true})
	if strings.Contains(got, "-plaintext") || !strings.Contains(got, "-insecure") {
		t.Errorf("TLS 片段应带 -insecure：\n%s", got)
	}
	// 无目录的定义：不生成 -import-path，-proto 直接用文件名
	if strings.Contains(got, "-import-path") {
		t.Errorf("无目录定义不应生成 -import-path：\n%s", got)
	}

	// 单引号按 shell 规则转义（' → '\''）
	quoted := GrpcurlSnippet(GRPCRequest{Target: "h:1", Service: "s.M", Method: "F", Message: `{"a":"it's"}`, Plaintext: true})
	if !strings.Contains(quoted, `it'\''s`) {
		t.Errorf("消息里的单引号应按 shell 规则转义：\n%s", quoted)
	}
	// Windows 风格路径统一成正斜杠
	win := GrpcurlSnippet(GRPCRequest{Target: "h:1", Service: "s.M", Method: "F", Proto: `protos\demo\greeter.proto`})
	if !strings.Contains(win, "-import-path 'protos/demo'") || !strings.Contains(win, "-proto 'greeter.proto'") {
		t.Errorf("路径应统一成斜杠：\n%s", win)
	}
}
