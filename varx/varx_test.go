package varx

import (
	"strings"
	"testing"
	"time"
)

// fixedBuiltins 固定内置取值，用于确定化断言。
func fixedBuiltins() Builtins {
	now := time.Date(2026, 9, 28, 15, 4, 5, 0, time.UTC)
	return Builtins{UUID: func() string { return "uuid-1" }, Now: func() time.Time { return now }}
}

func TestResolveBasic(t *testing.T) {
	table := Build([]Var{
		{Name: "host", Value: "http://127.0.0.1:8000", Scope: ScopeProjectCommon, Enabled: true},
		{Name: "name", Value: "alice", Scope: ScopeProjectCommon, Enabled: true},
	}, 0)
	got, missing := table.Resolve("{{host}}/users?name={{name}}&x={{missing}}")
	if got != "http://127.0.0.1:8000/users?name=alice&x={{missing}}" {
		t.Fatalf("got %q", got)
	}
	if len(missing) != 1 || missing[0] != "missing" {
		t.Fatalf("missing = %v", missing)
	}
}

// TestResolveEmptyValueIsDefined 空值变量命中即替换为空串，不算未定义。
func TestResolveEmptyValueIsDefined(t *testing.T) {
	table := Build([]Var{{Name: "empty", Value: "", Scope: ScopeRequest, Enabled: true}}, 0)
	got, missing := table.Resolve("a{{empty}}b")
	if got != "ab" || missing != nil {
		t.Fatalf("got %q missing %v", got, missing)
	}
}

// TestBuildPrecedence 四层优先级：请求 > 项目当前环境 > 项目通用 > 全局；其它环境的变量不参与。
func TestBuildPrecedence(t *testing.T) {
	vars := []Var{
		{Name: "token", Value: "global", Scope: ScopeGlobal, Enabled: true},
		{Name: "token", Value: "common", Scope: ScopeProjectCommon, Enabled: true},
		{Name: "token", Value: "test-env", Scope: ScopeProjectEnv, EnvID: 2, Enabled: true},
		{Name: "token", Value: "dev-env", Scope: ScopeProjectEnv, EnvID: 1, Enabled: true},
		{Name: "token", Value: "request", Scope: ScopeRequest, Enabled: true},
		{Name: "off", Value: "x", Scope: ScopeRequest, Enabled: false},
		{Name: "off", Value: "global", Scope: ScopeGlobal, Enabled: true},
	}
	cases := map[uint64]string{1: "request", 2: "request", 0: "request"}
	for envID, want := range cases {
		v, ok := Build(vars, envID).get("token")
		if !ok || v.Value != want {
			t.Fatalf("envID=%d token = %q ok=%v, want %q", envID, v.Value, ok, want)
		}
	}
	// 禁用变量不参与（保留全局值）。
	if v, _ := Build(vars, 1).get("off"); v.Value != "global" {
		t.Fatalf("禁用的请求级变量不应参与解析，got %q", v.Value)
	}
	// 去掉请求级后，按环境取值 / 回落通用。
	lower := []Var{vars[0], vars[1], vars[2], vars[3], vars[6]}
	for envID, want := range map[uint64]string{1: "dev-env", 2: "test-env", 0: "common"} {
		if v, _ := Build(lower, envID).get("token"); v.Value != want {
			t.Fatalf("envID=%d token = %q, want %q", envID, v.Value, want)
		}
	}
}

// TestBuildProjectEnvOutOfScope 绑定其它环境的变量在不匹配的 envID 下不参与。
func TestBuildProjectEnvOutOfScope(t *testing.T) {
	vars := []Var{{Name: "host", Value: "dev", Scope: ScopeProjectEnv, EnvID: 7, Enabled: true}}
	if _, ok := Build(vars, 0).get("host"); ok {
		t.Fatal("envID=0 时绑定具体环境的变量不应参与解析")
	}
	if _, ok := Build(vars, 9).get("host"); ok {
		t.Fatal("绑定其它环境的变量不应参与解析")
	}
}

func TestBuiltins(t *testing.T) {
	got, missing := resolve(nil, "{{$uuid}} {{$timestamp}} {{$isoTimestamp}} {{$randomInt}} {{$nope}}")
	if missing != nil {
		t.Fatalf("内置变量不应报缺失: %v", missing)
	}
	parts := strings.Fields(got)
	if parts[0] != "uuid-1" {
		t.Fatalf("uuid = %q", parts[0])
	}
	if parts[1] != "1790607845" || !strings.Contains(parts[2], "2026-09-28T15:04:05") {
		t.Fatalf("时间与 ISO 时间不符: %q %q", parts[1], parts[2])
	}
	if parts[3] != "0" {
		t.Fatalf("randomInt = %q", parts[3])
	}
	if !strings.Contains(got, "{{$nope}}") {
		t.Fatalf("未知 $ 变量应保留原样: %q", got)
	}
}

// TestBuiltinsMemoized D2：一次 Resolve 内同名内置变量必须取同一个值。
func TestBuiltinsMemoized(t *testing.T) {
	got, _ := resolve(nil, "{{$uuid}}|{{$uuid}}|{{$timestamp}}|{{$timestamp}}")
	parts := strings.Split(got, "|")
	if parts[0] != parts[1] || parts[2] != parts[3] {
		t.Fatalf("同一次 Resolve 内内置变量应同值，got %q", got)
	}
}

func TestMaskedValueOf(t *testing.T) {
	if got := MaskedValueOf(Var{Value: "s3cret", Secret: true}); got != MaskedValue {
		t.Fatalf("敏感值应掩码，got %q", got)
	}
	if got := MaskedValueOf(Var{Value: "", Secret: true}); got != "" {
		t.Fatalf("空敏感值应保持空串，got %q", got)
	}
	if got := MaskedValueOf(Var{Value: "v"}); got != "v" {
		t.Fatalf("非敏感值不应掩码，got %q", got)
	}
}

func TestNames(t *testing.T) {
	names := Names("{{a}} {{ b }} {{c}} {{$uuid}} {{a}}")
	if len(names) != 3 || names[0] != "a" || names[1] != "b" || names[2] != "c" {
		t.Fatalf("names = %v", names)
	}
	if got := Names(""); got != nil {
		t.Fatalf("空文本应返回 nil，got %v", got)
	}
}

// resolve 用固定的内置取值渲染，避免测试结果依赖随机数。
func resolve(vars []Var, text string) (string, []string) {
	return ResolveWithBuiltins(text, Build(vars, 0), fixedBuiltins())
}

// 嵌套变量：{{api}} 的值里引用 {{host}}，应递归展开。
func TestNestedResolve(t *testing.T) {
	tbl := Build([]Var{
		{Name: "host", Value: "http://x.com", Scope: ScopeProjectCommon, Enabled: true},
		{Name: "api", Value: "{{host}}/v1", Scope: ScopeProjectCommon, Enabled: true},
		{Name: "deep", Value: "{{api}}/users", Scope: ScopeProjectCommon, Enabled: true},
	}, 0)
	got, missing := tbl.Resolve("{{deep}}")
	if got != "http://x.com/v1/users" {
		t.Fatalf("嵌套解析: got %q", got)
	}
	if len(missing) != 0 {
		t.Fatalf("不应有缺失: %v", missing)
	}
}

// 循环引用：a→b→a，有环时保留原文不 panic。
func TestNestedResolveCycle(t *testing.T) {
	tbl := Build([]Var{
		{Name: "a", Value: "{{b}}", Scope: ScopeProjectCommon, Enabled: true},
		{Name: "b", Value: "{{a}}", Scope: ScopeProjectCommon, Enabled: true},
	}, 0)
	got, _ := tbl.Resolve("{{a}}")
	if got == "" {
		t.Fatalf("循环引用不应返回空串")
	}
}
