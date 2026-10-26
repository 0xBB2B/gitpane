// Package i18n 提供界面文字的多语言支持。
//
// 采用 gettext 的做法：代码里直接写简体中文原文，T("原文") 在运行时查表换成当前语言。
// 原文首尾的空格会原样保留，翻译表里只存中间的文字。
package i18n

import (
	"fmt"
	"os"
	"strings"
	"unicode"
)

// Lang 是界面语言。
type Lang string

const (
	ZhCN Lang = "zh-CN" // 简体中文（原文，默认）
	ZhTW Lang = "zh-TW" // 繁體中文
	En   Lang = "en"    // English
	Ja   Lang = "ja"    // 日本語
	Ko   Lang = "ko"    // 한국어
)

// All 是支持的语言，按显示顺序。
var All = []Lang{ZhCN, ZhTW, En, Ja, Ko}

// Name 返回语言自己的名字，比如 "日本語"。
func (l Lang) Name() string {
	switch l {
	case ZhTW:
		return "繁體中文"
	case En:
		return "English"
	case Ja:
		return "日本語"
	case Ko:
		return "한국어"
	}
	return "简体中文"
}

var current = ZhCN

// Set 设置当前语言。
func Set(l Lang) { current = l }

// Current 返回当前语言。
func Current() Lang { return current }

// catalogs 由各语言文件在 init 时注册。
var catalogs = map[Lang]map[string]string{}

func register(l Lang, m map[string]string) { catalogs[l] = m }

// T 把简体中文原文翻译成当前语言；找不到翻译时返回原文。
func T(s string) string {
	if current == ZhCN || s == "" {
		return s
	}
	core := strings.TrimFunc(s, unicode.IsSpace)
	if core == "" {
		return s
	}
	tr, ok := catalogs[current][core]
	if !ok {
		return s
	}
	i := strings.Index(s, core)
	return s[:i] + tr + s[i+len(core):]
}

// Tf 先翻译格式串再格式化。
func Tf(format string, a ...any) string { return fmt.Sprintf(T(format), a...) }

// Has 报告某条原文在某语言里是否有翻译（测试用）。
func Has(l Lang, s string) bool {
	_, ok := catalogs[l][strings.TrimFunc(s, unicode.IsSpace)]
	return ok
}

// Parse 识别语言名或 locale 字符串：zh-CN、zh_TW.UTF-8、ja_JP、ko、en_US.UTF-8、zh-Hant……
func Parse(s string) (Lang, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.IndexAny(s, ".@"); i >= 0 {
		s = s[:i] // 去掉 .UTF-8 / @modifier
	}
	s = strings.ReplaceAll(s, "_", "-")
	switch {
	case s == "", s == "c", s == "posix":
		return "", false
	case s == "zh-tw", s == "zh-hk", s == "zh-mo", strings.HasPrefix(s, "zh-hant"), s == "tw", s == "hk":
		return ZhTW, true
	case s == "zh", strings.HasPrefix(s, "zh-"), s == "cn":
		return ZhCN, true
	case s == "en", strings.HasPrefix(s, "en-"):
		return En, true
	case s == "ja", strings.HasPrefix(s, "ja-"), s == "jp":
		return Ja, true
	case s == "ko", strings.HasPrefix(s, "ko-"), s == "kr":
		return Ko, true
	}
	return "", false
}

// Detect 决定默认语言：GITPANE_LANG → LC_ALL → LC_MESSAGES → LANG → 系统设置 → 简体中文。
func Detect() Lang {
	for _, k := range []string{"GITPANE_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if l, ok := Parse(os.Getenv(k)); ok {
			return l
		}
	}
	for _, s := range systemLocales() {
		if l, ok := Parse(s); ok {
			return l
		}
	}
	return ZhCN
}
