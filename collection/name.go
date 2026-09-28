package collection

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// invalidChars 文件名 / 条目名里的非法字符。
var invalidChars = regexp.MustCompile(`[\\/:*?"<>|\x00-\x1f]`)

// envNamePattern 环境名只能包含字母、数字、- 与 _（同时用于文件名，跨平台安全）。
var envNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,60}$`)

// ValidEnvName 判断环境名是否合法。
func ValidEnvName(s string) bool {
	return envNamePattern.MatchString(s)
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
