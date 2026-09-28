// Package mock 定义 Mock 匹配的规则：URL 路径归一化 +「路径相同再看方法」的匹配优先级。
// 服务端（内置 Mock 服务）与桌面客户端（本地 Mock）共用这套判定，避免两端命中结果不一致；
// 查库/取样例/渲染变量等实现各留各的。
package mock

import "strings"

// Api 参与匹配的最小接口信息（调用方从自己的实体映射过来）。
type Api struct {
	ID     uint64
	Method string
	URL    string
}

// Path 取 URL 的路径部分：去掉 scheme、host、query 与 fragment。
func Path(raw string) string {
	u := strings.TrimSpace(raw)
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
		if j := strings.Index(u, "/"); j >= 0 {
			u = u[j:]
		} else {
			u = "/"
		}
	}
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	return u
}

// NormalizePath 归一化路径：去 query/fragment、剔除空段，统一为以 "/" 开头且无尾斜杠。
func NormalizePath(p string) string {
	p = strings.TrimSpace(p)
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	parts := make([]string, 0, 8)
	for _, part := range strings.Split(p, "/") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return "/" + strings.Join(parts, "/")
}

// NormalizeURL 归一化接口地址 / 请求路径：先取路径部分，再去 query 与空段。
// 接口地址（可能带 scheme/host/模板变量）与实际请求路径都用它归一，比较口径才一致。
func NormalizeURL(u string) string {
	return NormalizePath(Path(u))
}

// Match 返回命中的接口下标；未命中返回 -1。
// 先按「路径归一化后相同」筛出候选，其中方法相同者优先；方法都对不上时用该路径的第一条兜底
// （路径已由文档定义，方法写错也给出样例，比 404 更有用）。
func Match(method, reqPath string, apis []Api) int {
	want := NormalizeURL(reqPath)
	fallback := -1
	for i := range apis {
		if NormalizeURL(apis[i].URL) != want {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(apis[i].Method), strings.TrimSpace(method)) {
			return i
		}
		if fallback < 0 {
			fallback = i
		}
	}
	return fallback
}
