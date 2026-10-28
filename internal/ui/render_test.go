package ui

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

var sgrRe = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

// sgrSets 把一段输出里每个转义序列的参数整理成排好序的集合，便于比较（参数顺序不影响效果）。
func sgrSets(s string) (sets []string, text string) {
	for _, m := range sgrRe.FindAllStringSubmatch(s, -1) {
		ps := strings.Split(m[1], ";")
		// 38;2;r;g;b 这类颜色参数要当成一组
		var groups []string
		for i := 0; i < len(ps); i++ {
			if (ps[i] == "38" || ps[i] == "48") && i+1 < len(ps) {
				n := 3
				if ps[i+1] == "2" {
					n = 5
				}
				groups = append(groups, strings.Join(ps[i:min(len(ps), i+n)], ";"))
				i += n - 1
				continue
			}
			groups = append(groups, ps[i])
		}
		sort.Strings(groups)
		sets = append(sets, strings.Join(groups, ","))
	}
	return sets, sgrRe.ReplaceAllString(s, "")
}

// perChar 返回每个字符和它生效的样式参数（参数排序后），用于比较终端里的实际效果。
func perChar(s string) []string {
	var out []string
	cur := ""
	for len(s) > 0 {
		if loc := sgrRe.FindStringSubmatchIndex(s); loc != nil && loc[0] == 0 {
			sets, _ := sgrSets(s[:loc[1]])
			if sets[0] == "" || sets[0] == "0" {
				cur = ""
			} else {
				cur = sets[0]
			}
			s = s[loc[1]:]
			continue
		}
		r := []rune(s)[0]
		out = append(out, string(r)+"="+cur)
		s = s[len(string(r)):]
	}
	return out
}

func TestFastStyleMatchesLipgloss(t *testing.T) {
	defer lipgloss.SetColorProfile(lipgloss.ColorProfile())
	defer lipgloss.SetHasDarkBackground(lipgloss.HasDarkBackground())
	styles := []lipgloss.Style{
		lipgloss.NewStyle(),
		fg(cText),
		fg(cMuted).Italic(true),
		fg(cGreen).Bold(true),
		fg(cRed).Strikethrough(true),
		fg(cSub).Faint(true),
		lipgloss.NewStyle().Background(cSel),
		lipgloss.NewStyle().Background(cAccent).Foreground(cOnPill).Bold(true),
		fg(cYellow).Background(cAddBg),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#abcdef")).Background(lipgloss.Color("203")),
	}
	for _, prof := range []termenv.Profile{termenv.TrueColor, termenv.ANSI256, termenv.ANSI, termenv.Ascii} {
		for _, dark := range []bool{true, false} {
			lipgloss.SetColorProfile(prof)
			lipgloss.SetHasDarkBackground(dark)
			sgrMu.Lock()
			sgrCache = map[sgrKey]string{}
			sgrMu.Unlock()
			for i, st := range styles {
				want := st.Render("hello 你好")
				var b strings.Builder
				writeStyled(&b, st, "hello 你好")
				if w, g := perChar(want), perChar(b.String()); strings.Join(w, "|") != strings.Join(g, "|") {
					t.Errorf("profile=%v dark=%v style#%d:\n want %q\n  got %q", prof, dark, i, want, b.String())
				}
			}
		}
	}
}

func TestTextWidth(t *testing.T) {
	for _, s := range []string{"", "abc", "a b\tc", "中文", "⎇ main", "é", "\x1b[1mx\x1b[0m"} {
		if textWidth(s) != lipgloss.Width(s) {
			t.Errorf("%q: %d vs %d", s, textWidth(s), lipgloss.Width(s))
		}
	}
}
