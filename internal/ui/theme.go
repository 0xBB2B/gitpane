package ui

import (
	"os"
	"path"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// 配色基于 Tokyo Night（深色）/ Tokyo Night Day（浅色），根据终端背景自动切换。
type ac = lipgloss.AdaptiveColor

var (
	cText    = ac{Dark: "#c0caf5", Light: "#3760bf"}
	cSub     = ac{Dark: "#a9b1d6", Light: "#4c5a8f"}
	cMuted   = ac{Dark: "#737aa2", Light: "#7c84a8"}
	cFaint   = ac{Dark: "#545c7e", Light: "#a1a6c5"}
	cGuide   = ac{Dark: "#3b4261", Light: "#c4c8da"}
	cAccent  = ac{Dark: "#7aa2f7", Light: "#2e7de9"}
	cGreen   = ac{Dark: "#9ece6a", Light: "#587539"}
	cYellow  = ac{Dark: "#e0af68", Light: "#8c6c3e"}
	cOrange  = ac{Dark: "#ff9e64", Light: "#b15c00"}
	cRed     = ac{Dark: "#f7768e", Light: "#f52a65"}
	cPurple  = ac{Dark: "#bb9af7", Light: "#9854f1"}
	cCyan    = ac{Dark: "#7dcfff", Light: "#007197"}
	cOnPill  = ac{Dark: "#1a1b26", Light: "#ffffff"}
	cBar     = ac{Dark: "#1f2335", Light: "#d6d9e6"}
	cSurface = ac{Dark: "#292e42", Light: "#c8ccdd"}
	cSel     = ac{Dark: "#2d3f76", Light: "#b6c3ea"}
	cAddBg   = ac{Dark: "#1d3330", Light: "#d6e8cf"}
	cDelBg   = ac{Dark: "#3a2232", Light: "#f3d6dc"}
	cAddHi   = ac{Dark: "#2b5248", Light: "#bcdcae"}
	cDelHi   = ac{Dark: "#5c2c42", Light: "#ebb6c1"}
	cHunkBg  = ac{Dark: "#1f2335", Light: "#dfe2ee"}
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
		return "修改"
	case 'A':
		return "新增"
	case 'D':
		return "删除"
	case 'R':
		return "重命名"
	case 'C':
		return "复制"
	case 'U':
		return "未跟踪"
	case 'T':
		return "类型变更"
	case '!':
		return "冲突"
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
