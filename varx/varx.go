// Package varx 定义变量的占位符语法、解析优先级、敏感值掩码与内置动态变量。
// 服务端与桌面客户端共用同一份规则，任何一处的语义变化都必须改这里（而不是各自加兼容分支）。
// 规则摘要：
//   - 占位符 {{name}}（允许花括号内两侧空白），$ 前缀表示内置动态变量；
//   - 未定义的普通变量保留原文并回传名字（去重保序），未知的内置变量只保留原文、不算缺失；
//   - 命中即替换：值为空串时替换成空串，不视为未定义；
//   - 一次 Resolve 内同名的内置变量取同一个值。
package varx

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Scope 变量作用域，同时决定解析顺序（Rank 越小越优先）。
type Scope int

// 四层优先级：请求级 > 项目当前环境 > 项目通用（EnvID=0）> 全局。
const (
	ScopeRequest Scope = iota + 1
	ScopeProjectEnv
	ScopeProjectCommon
	ScopeGlobal
)

// MaskedValue 敏感值对外展示的掩码。
const MaskedValue = "••••••"

// placeholderPattern 匹配 {{name}} / {{$builtin}}；渲染阶段不对名字长度设限，
// 长度规则属于写入校验，由各端自己的变量名规则负责。
var placeholderPattern = regexp.MustCompile(`\{\{\s*(\$?[A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

// Var 一条变量的解析要素。
type Var struct {
	Name    string
	Value   string
	Scope   Scope
	EnvID   uint64 // 仅 ScopeProjectEnv 有意义，指向所属环境
	Enabled bool
	Secret  bool
}

// Table 解析表：同名变量只保留优先级最高的一条。
type Table struct {
	vars map[string]Var
}

// Build 按优先级去重构建解析表；未启用、不属于 envID 的变量不参与解析。
func Build(vars []Var, envID uint64) *Table {
	t := &Table{vars: make(map[string]Var, len(vars))}
	ranks := make(map[string]int, len(vars))
	for _, v := range vars {
		rank, ok := Rank(v, envID)
		if !ok {
			continue
		}
		if cur, exists := ranks[v.Name]; exists && cur <= rank {
			continue
		}
		ranks[v.Name] = rank
		t.vars[v.Name] = v
	}
	return t
}

// Rank 返回变量在 envID 下的优先级；ok 为 false 表示该变量不参与本次解析。
func Rank(v Var, envID uint64) (int, bool) {
	if !v.Enabled {
		return 0, false
	}
	switch v.Scope {
	case ScopeRequest:
		return 1, true
	case ScopeProjectEnv:
		if envID > 0 && v.EnvID == envID {
			return 2, true
		}
		return 0, false
	case ScopeProjectCommon:
		return 3, true
	case ScopeGlobal:
		return 4, true
	}
	return 0, false
}

// get 取表中的变量；不存在返回 false。
func (t *Table) get(name string) (Var, bool) {
	if t == nil {
		return Var{}, false
	}
	v, ok := t.vars[name]
	return v, ok
}

// Resolve 渲染文本，返回渲染结果与未定义的变量名（去重、按出现顺序）。
func (t *Table) Resolve(text string) (string, []string) {
	return ResolveWithBuiltins(text, t, DefaultBuiltins())
}

// MaskedValueOf 敏感变量的对外值：有值回掩码，空值回空串。
func MaskedValueOf(v Var) string {
	if v.Secret && v.Value != "" {
		return MaskedValue
	}
	return v.Value
}

// Names 收集文本中引用的普通变量名（不含内置变量），去重并按出现顺序返回。
func Names(text string) []string {
	if text == "" {
		return nil
	}
	var out []string
	seen := map[string]struct{}{}
	for _, m := range placeholderPattern.FindAllStringSubmatch(text, -1) {
		name := m[1]
		if name == "" || name[0] == '$' {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

// Builtins 内置动态变量的取值器，可注入以便单测确定化。
type Builtins struct {
	UUID func() string
	Now  func() time.Time
}

// DefaultBuiltins 默认取值器：UUID 取随机数，时间取当前时刻。
func DefaultBuiltins() Builtins {
	return Builtins{UUID: randomUUID, Now: time.Now}
}

// Values 返回四个内置变量的取值（同一次调用共用同一个时刻）。
func (b Builtins) Values() map[string]string {
	now := b.Now()
	return map[string]string{
		"$uuid":         b.UUID(),
		"$timestamp":    strconv.FormatInt(now.Unix(), 10),
		"$isoTimestamp": now.UTC().Format(time.RFC3339Nano),
		"$randomInt":    strconv.FormatInt(now.UnixNano()%1000, 10),
	}
}

// ResolveWithBuiltins 用给定的内置取值器渲染文本。
// 支持嵌套变量（`{{a}}` 的值里可以再引用 `{{b}}`），有环时保留原文并记缺失。
func ResolveWithBuiltins(text string, t *Table, b Builtins) (string, []string) {
	if text == "" {
		return "", nil
	}
	r := &renderer{table: t, b: b, cache: map[string]string{}}
	var missing []string
	seen := map[string]struct{}{}
	out := r.expand(text, 0, &missing, seen)
	return out, missing
}

// maxNestDepth 嵌套解析最大深度（防环）。
const maxNestDepth = 8

// expand 递归展开文本中的占位符；depth 用于防环，seen 用于 missing 去重。
func (r *renderer) expand(text string, depth int, missing *[]string, seen map[string]struct{}) string {
	if text == "" || depth > maxNestDepth {
		return text
	}
	return placeholderPattern.ReplaceAllStringFunc(text, func(m string) string {
		name := placeholderPattern.FindStringSubmatch(m)[1]
		if name != "" && name[0] == '$' {
			// 未知的内置名既不是变量也不算缺失（保留原文，UI 不做告警）。
			if v, ok := r.builtin(name); ok {
				return v
			}
			return m
		}
		v, ok := r.table.get(name)
		if !ok {
			if _, dup := seen[name]; !dup {
				seen[name] = struct{}{}
				*missing = append(*missing, name)
			}
			return m
		}
		// 嵌套展开：值里的 {{...}} 递归解析
		if strings.Contains(v.Value, "{{") {
			return r.expand(v.Value, depth+1, missing, seen)
		}
		return v.Value
	})
}

// renderer 渲染上下文：缓存内置变量值，保证同一 Resolve 内同名的内置变量取同一个值。
type renderer struct {
	table *Table
	b     Builtins
	cache map[string]string
	now   time.Time
}

func (r *renderer) builtin(name string) (string, bool) {
	if name == "" || name[0] != '$' {
		return "", false
	}
	if v, cached := r.cache[name]; cached {
		return v, true
	}
	var (
		v   string
		ok  bool
		gen func() string
	)
	switch name {
	case "$uuid":
		gen = r.b.UUID
	case "$timestamp":
		gen = func() string { return strconv.FormatInt(r.at().Unix(), 10) }
	case "$isoTimestamp":
		gen = func() string { return r.at().UTC().Format(time.RFC3339Nano) }
	case "$randomInt":
		gen = func() string { return strconv.FormatInt(r.at().UnixNano()%1000, 10) }
	}
	if gen == nil {
		return "", false
	}
	v, ok = gen(), true
	r.cache[name] = v
	return v, ok
}

// at 本次 Resolve 的统一时刻。
func (r *renderer) at() time.Time {
	if r.now.IsZero() {
		r.now = r.b.Now()
	}
	return r.now
}

// randomUUID 生成带版本位的随机 UUID v4。
func randomUUID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}
