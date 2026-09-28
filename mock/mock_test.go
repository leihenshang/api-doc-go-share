package mock

import "testing"

// NormalizePath 只处理路径本身：带 scheme/host 的输入先走 Path。
func TestNormalizePath(t *testing.T) {
	cases := map[string]string{
		"":              "/",
		"  ":            "/",
		"/":             "/",
		"users/1/":      "/users/1",
		"/users//1":     "/users/1",
		"/users?page=1": "/users",
		"/users#top":    "/users",
	}
	for in, want := range cases {
		if got := NormalizePath(in); got != want {
			t.Fatalf("NormalizePath(%q) = %q, want %q", in, got, want)
		}
	}
}

// NormalizeURL 是接口地址与请求路径的统一比较口径。
func TestNormalizeURL(t *testing.T) {
	cases := map[string]string{
		"http://127.0.0.1/api/users": "/api/users",
		"https://a.com":              "/",
		"{{host}}/api/user/list?p=1": "/{{host}}/api/user/list",
		"/api/user/list":             "/api/user/list",
	}
	for in, want := range cases {
		if got := NormalizeURL(in); got != want {
			t.Fatalf("NormalizeURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPath(t *testing.T) {
	cases := map[string]string{
		"https://api.example.com/v1/user/list?page=1#top": "/v1/user/list",
		"http://a.com":  "/",
		"/v1/user/list": "/v1/user/list",
	}
	for in, want := range cases {
		if got := Path(in); got != want {
			t.Fatalf("Path(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatch(t *testing.T) {
	apis := []Api{
		{ID: 1, Method: "GET", URL: "https://a.com/api/user/list"},
		{ID: 2, Method: "POST", URL: "/api/user/list"},
		{ID: 3, Method: "GET", URL: "{{host}}/api/order/detail?x=1"},
	}
	// 路径 + 方法都命中。
	if i := Match("get", "/api/user/list", apis); i != 0 {
		t.Fatalf("应命中 GET 接口，got %d", i)
	}
	// 同路径不同方法：方法优先，不再回退。
	if i := Match("POST", "/api/user/list?utm=1", apis); i != 1 {
		t.Fatalf("应命中 POST 接口，got %d", i)
	}
	// query 不影响路径归一；模板 host 与请求路径不同段时视为不同路径。
	if i := Match("GET", "/api/order/detail?x=2", apis); i != -1 {
		t.Fatalf("「{{host}}」段与请求路径不同，应视为不同路径，got %d", i)
	}
	if i := Match("GET", "/{{host}}/api/order/detail", apis); i != 2 {
		t.Fatalf("同段模板地址应命中，got %d", i)
	}
	// 方法不匹配时回退到该路径的第一条。
	if i := Match("DELETE", "/api/user/list", apis); i != 0 {
		t.Fatalf("方法不匹配应回退到同路径首条，got %d", i)
	}
	// 路径不匹配 → 未命中。
	if i := Match("GET", "/api/nope", apis); i != -1 {
		t.Fatalf("未命中应返回 -1，got %d", i)
	}
	if i := Match("GET", "/api/nope", nil); i != -1 {
		t.Fatalf("空列表应返回 -1，got %d", i)
	}
}
