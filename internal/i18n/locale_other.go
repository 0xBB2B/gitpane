//go:build !darwin && !windows

package i18n

// systemLocales 在 Linux 等系统上只看环境变量，这里没有额外来源。
func systemLocales() []string { return nil }
