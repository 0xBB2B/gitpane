package i18n

import (
	"os/exec"
	"strings"
)

// systemLocales 在 macOS 上读系统偏好的语言（终端没设 LANG 时用）。
func systemLocales() []string {
	out, err := exec.Command("defaults", "read", "-g", "AppleLanguages").Output()
	if err != nil {
		return nil
	}
	// 输出形如 ( "zh-Hans-CN", "en-US" )
	var ls []string
	for _, f := range strings.FieldsFunc(string(out), func(r rune) bool {
		return r == '(' || r == ')' || r == ',' || r == '"' || r == '\n' || r == ' '
	}) {
		f = strings.Replace(f, "zh-Hans", "zh-CN", 1)
		ls = append(ls, f)
	}
	return ls
}
