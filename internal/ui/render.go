package ui

import (
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// 快速着色。
//
// lipgloss 的 Style.Render 要处理边框、内外边距、对齐、制表符、换行等一整套逻辑，
// 一屏两三百个片段每个都走一遍，滚动时大部分时间都花在这里。界面里的片段只用到
// 前景色、底色、粗体、斜体、淡色、删除线，所以按样式把对应的 SGR 转义序列算一次缓存下来，
// 之后直接拼接字符串。输出和 Style.Render 一致（同样按终端的颜色能力降级）。

type sgrKey struct {
	fg, bg                      lipgloss.TerminalColor
	bold, italic, faint, strike bool
	dark                        bool
}

var (
	sgrMu    sync.Mutex
	sgrCache = map[sgrKey]string{}
)

// styleOpen 返回样式对应的开头转义序列；空串表示没有任何样式（或终端不支持颜色）。
func styleOpen(st lipgloss.Style) string {
	k := sgrKey{
		fg: st.GetForeground(), bg: st.GetBackground(),
		bold: st.GetBold(), italic: st.GetItalic(), faint: st.GetFaint(), strike: st.GetStrikethrough(),
		dark: lipgloss.HasDarkBackground(),
	}
	sgrMu.Lock()
	defer sgrMu.Unlock()
	if s, ok := sgrCache[k]; ok {
		return s
	}
	s := buildSGR(k)
	sgrCache[k] = s
	return s
}

func buildSGR(k sgrKey) string {
	p := lipgloss.ColorProfile()
	if p == termenv.Ascii {
		return "" // 和 lipgloss 一致：不支持颜色的终端连粗体、斜体也不输出
	}
	var seq []string
	if c := colorSeq(p, k.fg, k.dark, false); c != "" {
		seq = append(seq, c)
	}
	if c := colorSeq(p, k.bg, k.dark, true); c != "" {
		seq = append(seq, c)
	}
	if k.bold {
		seq = append(seq, termenv.BoldSeq)
	}
	if k.faint {
		seq = append(seq, termenv.FaintSeq)
	}
	if k.italic {
		seq = append(seq, termenv.ItalicSeq)
	}
	if k.strike {
		seq = append(seq, termenv.CrossOutSeq)
	}
	if len(seq) == 0 {
		return ""
	}
	return termenv.CSI + strings.Join(seq, ";") + "m"
}

func colorSeq(p termenv.Profile, c lipgloss.TerminalColor, dark, bg bool) string {
	var s string
	switch v := c.(type) {
	case nil, lipgloss.NoColor:
		return ""
	case lipgloss.Color:
		s = string(v)
	case lipgloss.AdaptiveColor:
		s = v.Light
		if dark {
			s = v.Dark
		}
	case lipgloss.ANSIColor:
		s = strconv.Itoa(int(v))
	default:
		// 其他颜色类型界面里没用到；交给 lipgloss 处理，取它生成的序列
		st := lipgloss.NewStyle()
		if bg {
			st = st.Background(c)
		} else {
			st = st.Foreground(c)
		}
		out := st.Render("x")
		if i := strings.Index(out, "x"); i > 0 {
			return strings.TrimSuffix(strings.TrimPrefix(out[:i], termenv.CSI), "m")
		}
		return ""
	}
	tc := p.Color(s)
	if tc == nil {
		return ""
	}
	return tc.Sequence(bg)
}

const sgrReset = termenv.CSI + termenv.ResetSeq + "m"

// writeStyled 把带样式的文字写进 b，等价于 b.WriteString(st.Render(txt))。
func writeStyled(b *strings.Builder, st lipgloss.Style, txt string) {
	open := styleOpen(st)
	if open == "" {
		b.WriteString(txt)
		return
	}
	b.WriteString(open)
	b.WriteString(txt)
	b.WriteString(sgrReset)
}

// textWidth 是 lipgloss.Width 的快速版本：纯 ASCII 可打印字符直接用长度。
func textWidth(s string) int {
	for i := 0; i < len(s); i++ {
		if c := s[i]; c < 0x20 || c >= 0x7f {
			return lipgloss.Width(s)
		}
	}
	return len(s)
}
