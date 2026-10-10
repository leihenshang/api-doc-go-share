package collection

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

// invalidChars 文件名 / 条目名里的非法字符。
var invalidChars = regexp.MustCompile(`[\\/:*?"<>|\x00-\x1f]`)

// envNameExtra 环境名允许的非字母数字字符：- _ ( ) .（含全角括号）
const envNameExtra = "-_().（）"

// envNameMaxLen 环境名长度上限（按 rune 计）
const envNameMaxLen = 60

// ValidEnvName 判断环境名是否合法。
//
// 环境名同时用作文件名（environments/<name>.yml），所以按白名单限制：
// Unicode 字母 / 数字（中文、英文等皆可）+ `-_().（）`；空格、路径分隔符、
// Windows 非法字符、控制字符一律拒绝。以 `.` 开头的名字也拒绝，
// 避免生成隐藏文件或 `..` 这类指向上级目录的名字。
func ValidEnvName(s string) bool {
	if s == "" || len([]rune(s)) > envNameMaxLen || strings.HasPrefix(s, ".") {
		return false
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		if !strings.ContainsRune(envNameExtra, r) {
			return false
		}
	}
	return true
}

// ValidEntryName 判断请求 / 分组名称是否可作为文件名（不含路径分隔符且非空）。
func ValidEntryName(s string) bool {
	return s != "" && len([]rune(s)) <= 120 && !invalidChars.MatchString(s)
}

// SanitizeFileName 把名称转成安全的文件名（不含扩展名）。
func SanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	name = invalidChars.ReplaceAllString(name, "-")
	name = strings.Trim(name, ". ")
	if name == "" {
		name = "untitled"
	}
	if len([]rune(name)) > 100 {
		name = string([]rune(name)[:100])
	}
	return name
}

// TrashName 生成回收站里的文件名：20260928-150405.<name>，便于按时间找回。
func TrashName(base string, at time.Time) string {
	return fmt.Sprintf("%s.%s", at.Format("20060102-150405"), base)
}

// SortNodes 排序树节点：分组在前（按名称），请求在后（按 seq 升序，同 seq 再按名称）。
func SortNodes(nodes []*Node) {
	sort.SliceStable(nodes, func(i, j int) bool {
		fi, fj := nodes[i].Type == "folder", nodes[j].Type == "folder"
		if fi != fj {
			return fi
		}
		if nodes[i].Seq != nodes[j].Seq {
			return nodes[i].Seq < nodes[j].Seq
		}
		return nodes[i].Name < nodes[j].Name
	})
}
