package i18n

import "golang.org/x/sys/windows"

// systemLocales 在 Windows 上读用户界面语言（Windows 一般不设 LANG）。
func systemLocales() []string {
	ls, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil {
		return nil
	}
	return ls
}
