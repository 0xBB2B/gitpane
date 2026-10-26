package ui

import (
	"os"
	"path"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/0xBB2B/gitpane/internal/i18n"
)

// 配色：深色是 One Monokai（VS Code 主题 azemoh.one-monokai：One Dark 底色 +
// Monokai 风格的语法色），浅色是同一体系的 One Light。根据终端背景自动切换，
// 也可以用 GITPANE_THEME=dark|light 指定。
type ac = lipgloss.AdaptiveColor

var (
	cText    = ac{Dark: "#d7dae0", Light: "#383a42"}
	cSub     = ac{Dark: "#abb2bf", Light: "#4f525e"}
	cMuted   = ac{Dark: "#7f848e", Light: "#696c77"}
	cFaint   = ac{Dark: "#5c6370", Light: "#a0a1a7"}
	cGuide   = ac{Dark: "#3b4048", Light: "#d4d4d6"}
	cAccent  = ac{Dark: "#61afef", Light: "#4078f2"}
	cGreen   = ac{Dark: "#98c379", Light: "#50a14f"}
	cYellow  = ac{Dark: "#e5c07b", Light: "#c18401"}
	cOrange  = ac{Dark: "#d19a66", Light: "#986801"}
	cRed     = ac{Dark: "#e06c75", Light: "#e45649"}
	cPurple  = ac{Dark: "#c678dd", Light: "#a626a4"}
	cCyan    = ac{Dark: "#56b6c2", Light: "#0184bc"}
	cOnPill  = ac{Dark: "#282c34", Light: "#ffffff"}
	cBar     = ac{Dark: "#21252b", Light: "#eaeaeb"}
	cSurface = ac{Dark: "#2f333d", Light: "#e0e0e2"}
	cSel     = ac{Dark: "#3e4451", Light: "#d7dae0"}
	cAddBg   = ac{Dark: "#2b3830", Light: "#e2f0e0"}
	cDelBg   = ac{Dark: "#3d2b31", Light: "#fbe1dd"}
	cAddHi   = ac{Dark: "#3a5040", Light: "#c6e2c3"}
	cDelHi   = ac{Dark: "#5a343b", Light: "#f5c3bb"}
	cHunkBg  = ac{Dark: "#21252b", Light: "#f0f0f1"}
)

// 提交图谱每一列轮流使用的颜色。
var graphColors = []ac{cAccent, cPurple, cCyan, cGreen, cYellow, cOrange, cRed}

func fg(c lipgloss.TerminalColor) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }

func statusColor(c byte) ac {
	switch c {
	case 'M', 'T':
		return cYellow
	case 'A', 'U':
		return cGreen
	case 'D', '!':
		return cRed
	case 'R', 'C':
		return cPurple
	}
	return cMuted
}

func statusWord(c byte) string {
	switch c {
	case 'M':
		return i18n.T("修改")
	case 'A':
		return i18n.T("新增")
	case 'D':
		return i18n.T("删除")
	case 'R':
		return i18n.T("重命名")
	case 'C':
		return i18n.T("复制")
	case 'U':
		return i18n.T("未跟踪")
	case 'T':
		return i18n.T("类型变更")
	case '!':
		return i18n.T("冲突")
	}
	return ""
}

// ---------------------------------------------------------------- 图标

// nerd 为 true 时使用 Nerd Font 图标（GITPANE_NERD_FONT=1）。
var nerd = func() bool {
	v := strings.ToLower(os.Getenv("GITPANE_NERD_FONT"))
	return v == "1" || v == "true" || v == "yes"
}()

type iconSet struct {
	branch, dirOpen, dirClosed, commit, check, cross, busy, up, down, clean, repo string
}

var icons = func() iconSet {
	if nerd {
		return iconSet{
			branch: "", dirOpen: "", dirClosed: "", commit: "",
			check: "", cross: "", busy: "", up: "", down: "",
			clean: "", repo: "",
		}
	}
	return iconSet{
		branch: "⎇", dirOpen: "▾", dirClosed: "▸", commit: "●",
		check: "✓", cross: "✗", busy: "", up: "↑", down: "↓",
		clean: "✓", repo: "",
	}
}()

var extIcons = map[string]string{
	".go": "", ".md": "", ".js": "", ".mjs": "", ".ts": "", ".tsx": "",
	".jsx": "", ".py": "", ".rs": "", ".java": "", ".json": "",
	".sh": "", ".zsh": "", ".html": "", ".css": "", ".yml": "",
	".yaml": "", ".toml": "", ".lua": "", ".rb": "", ".php": "",
	".c": "", ".h": "", ".cpp": "", ".swift": "", ".kt": "",
	".sql": "", ".vue": "", ".dart": "", ".proto": "",
}

// fileIcon 返回文件图标（含尾随空格）；未开启 Nerd Font 时为空。
func fileIcon(name string) string {
	if !nerd {
		return ""
	}
	switch strings.ToLower(name) {
	case "dockerfile":
		return " "
	case "makefile":
		return " "
	case "go.mod", "go.sum":
		return " "
	}
	if ic, ok := extIcons[strings.ToLower(path.Ext(name))]; ok {
		return ic + " "
	}
	return " "
}
