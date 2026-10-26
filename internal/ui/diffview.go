package ui

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/0xBB2B/gitpane/internal/gitx"
	"github.com/0xBB2B/gitpane/internal/i18n"
)

// 详情区的一行。解析一次，按需渲染可见部分。
type dkind int

const (
	dCtx dkind = iota
	dAdd
	dDel
	dHunk
	dMeta
	dFile    // 多文件 diff 里的文件标题
	dStat    // --stat 的一行
	dStatSum // --stat 的汇总
	dBlank
	dTitle // 提交标题
	dInfo  // 提交作者/时间
	dBody  // 提交正文
	dGraph // 分支日志里的一条提交
	dText  // 普通文本
	dErr
)

type dline struct {
	kind      dkind
	old, new  int
	text      string
	path      string
	add, del  int
	g         gitx.GraphLine
	file      int  // 所属文件标题行的下标；-1 表示不属于任何文件（提交信息等）
	collapsed bool // 仅 dFile：该文件是否折叠（渲染时由 visibleDetail 填）
	cont      bool // 自动换行折出来的后续行：不画行号、符号和图标
	src       int  // 自动换行时，这一屏幕行属于 foldedDetail 里的第几行
}

// detailView 是详情区的内容：标题栏 + 若干行。
type detailView struct {
	key    string
	head   detailHead
	lines  []dline
	digits int // 行号宽度
	files  int // 多文件详情（提交）里的文件数；0 表示不分文件
}

// indexFiles 给每一行标上所属文件，返回文件数。文件前的空行算上一个文件的，
// 这样相邻的折叠文件会紧挨着排成一张清单。
func indexFiles(lines []dline) int {
	cur, n := -1, 0
	for i := range lines {
		if lines[i].kind == dFile {
			cur = i
			n++
		}
		lines[i].file = cur
	}
	return n
}

type detailHead struct {
	icon     string
	name     string
	dir      string
	add, del int
	badge    string
	badgeCol ac
	hint     string // 没有内容时的提示
}

var hunkRe = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@ ?(.*)$`)

// parseDiff 解析 git diff / git show 的输出（无颜色）。
func parseDiff(raw string, fileHeaders bool) ([]dline, int) {
	raw = strings.ReplaceAll(raw, "\r", "")
	src := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	var out []dline
	maxN := 0
	oldN, newN := 0, 0
	inDiff := false
	fileIdx := -1
	var oldMode string

	for _, l := range src {
		switch {
		case strings.HasPrefix(l, "diff --git "):
			inDiff = true
			if fileHeaders {
				if len(out) > 0 {
					out = append(out, dline{kind: dBlank})
				}
				p := l[len("diff --git "):]
				if i := strings.Index(p, " b/"); i >= 0 {
					p = p[i+3:]
				}
				out = append(out, dline{kind: dFile, path: p})
				fileIdx = len(out) - 1
			}
		case !inDiff:
			// --stat 部分（git show 时出现在补丁前）。逐文件的统计行不再单独显示：
			// 下面每个文件的标题行带了 +增 -删，并且可以折叠，本身就是一张清单。
			if strings.Contains(l, " | ") {
				continue
			} else if strings.Contains(l, "changed") {
				out = append(out, dline{kind: dStatSum, text: strings.TrimSpace(l)})
			}
		case strings.HasPrefix(l, "index "), strings.HasPrefix(l, "--- "), strings.HasPrefix(l, "+++ "),
			strings.HasPrefix(l, "similarity index"), strings.HasPrefix(l, "dissimilarity index"),
			strings.HasPrefix(l, "rename to "), strings.HasPrefix(l, "copy to "):
		case strings.HasPrefix(l, "new file mode"):
			out = append(out, dline{kind: dMeta, text: i18n.T("新文件")})
		case strings.HasPrefix(l, "deleted file mode"):
			out = append(out, dline{kind: dMeta, text: i18n.T("文件已删除")})
		case strings.HasPrefix(l, "rename from "):
			out = append(out, dline{kind: dMeta, text: i18n.Tf("重命名自 %s", strings.TrimPrefix(l, "rename from "))})
		case strings.HasPrefix(l, "copy from "):
			out = append(out, dline{kind: dMeta, text: i18n.Tf("复制自 %s", strings.TrimPrefix(l, "copy from "))})
		case strings.HasPrefix(l, "old mode "):
			oldMode = strings.TrimPrefix(l, "old mode ")
		case strings.HasPrefix(l, "new mode "):
			out = append(out, dline{kind: dMeta, text: i18n.Tf("权限 %s → %s", oldMode, strings.TrimPrefix(l, "new mode "))})
		case strings.HasPrefix(l, "Binary files"):
			out = append(out, dline{kind: dMeta, text: i18n.T("二进制文件，不显示差异")})
		case strings.HasPrefix(l, "@@"):
			m := hunkRe.FindStringSubmatch(l)
			if m == nil {
				out = append(out, dline{kind: dHunk, text: l})
				continue
			}
			oldN, _ = strconv.Atoi(m[1])
			newN, _ = strconv.Atoi(m[3])
			oc, nc := 1, 1
			if m[2] != "" {
				oc, _ = strconv.Atoi(m[2])
			}
			if m[4] != "" {
				nc, _ = strconv.Atoi(m[4])
			}
			maxN = max(maxN, oldN+oc, newN+nc)
			out = append(out, dline{kind: dHunk, old: oldN, new: newN, text: m[5]})
		case strings.HasPrefix(l, "+"):
			out = append(out, dline{kind: dAdd, new: newN, text: l[1:]})
			newN++
			if fileIdx >= 0 {
				out[fileIdx].add++
			}
		case strings.HasPrefix(l, "-"):
			out = append(out, dline{kind: dDel, old: oldN, text: l[1:]})
			oldN++
			if fileIdx >= 0 {
				out[fileIdx].del++
			}
		case strings.HasPrefix(l, "\\"):
			out = append(out, dline{kind: dMeta, text: i18n.T("文件末尾没有换行")})
		default:
			text := l
			if strings.HasPrefix(l, " ") {
				text = l[1:]
			}
			out = append(out, dline{kind: dCtx, old: oldN, new: newN, text: text})
			oldN++
			newN++
		}
	}
	return out, max(3, len(strconv.Itoa(maxN)))
}

// ---------------------------------------------------------------- 渲染

type seg struct {
	s  string
	st lipgloss.Style
}

// sg 生成一个文字片段。来自 git 的文字（提交信息、文件名、文件内容）可能带
// \r、ESC、退格等控制字符，直接输出会让终端光标乱跳、整屏错位，所以这里统一清洗。
func sg(s string, st lipgloss.Style) seg { return seg{clean(s), st} }

// sgRaw 用于本身就带 ANSI 样式的片段（输入框、加载动画），不做清洗。
func sgRaw(s string, st lipgloss.Style) seg { return seg{s, st} }

// clean 去掉 \r，把 Tab 展开成空格，其余控制字符（含换行、ESC、C1 控制符）
// 和会改变文字方向的 Unicode 控制符替换成 �，保证一个片段只占一行、宽度可算。
func clean(s string) string {
	ok := true
	for _, r := range s {
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) || isBidi(r) {
			ok = false
			break
		}
	}
	if ok {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\r':
		case r == '\t':
			b.WriteString("    ")
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) || isBidi(r):
			b.WriteRune('\uFFFD')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isBidi(r rune) bool {
	return (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069)
}

// paint 把若干片段拼成恰好 w 列的一行；bg 非空时整行铺底色。
func paint(w int, bg lipgloss.TerminalColor, segs ...seg) string {
	var b strings.Builder
	used := 0
	for _, sgm := range segs {
		if used >= w {
			break
		}
		st := sgm.st
		if bg != nil {
			// 自带底色的片段（标签、按钮）保留自己的底色
			if _, none := st.GetBackground().(lipgloss.NoColor); none {
				st = st.Background(bg)
			}
		}
		txt := sgm.s
		sw := lipgloss.Width(txt)
		if used+sw > w {
			txt = ansi.Truncate(txt, w-used, "…")
			sw = lipgloss.Width(txt)
		}
		b.WriteString(st.Render(txt))
		used += sw
	}
	if used < w {
		pad := lipgloss.NewStyle()
		if bg != nil {
			pad = pad.Background(bg)
		}
		b.WriteString(pad.Render(strings.Repeat(" ", w-used)))
	}
	return b.String()
}

// rightAlign 生成左右两组片段之间的空白，使右组贴右边。
func rightAlign(w int, bg lipgloss.TerminalColor, left, right []seg) string {
	lw, rw := 0, 0
	for _, s := range left {
		lw += lipgloss.Width(s.s)
	}
	for _, s := range right {
		rw += lipgloss.Width(s.s)
	}
	if lw+rw+1 > w {
		// 放不下时优先保留右侧，截断左侧
		return paint(w, bg, append(append(trimSegs(left, max(0, w-rw-1)), sg(" ", lipgloss.NewStyle())), right...)...)
	}
	segs := append(append(left, sg(strings.Repeat(" ", w-lw-rw), lipgloss.NewStyle())), right...)
	return paint(w, bg, segs...)
}

func trimSegs(segs []seg, w int) []seg {
	var out []seg
	used := 0
	for _, s := range segs {
		sw := lipgloss.Width(s.s)
		if used+sw > w {
			if rest := w - used; rest > 0 {
				out = append(out, seg{ansi.Truncate(s.s, rest, "…"), s.st})
				used = w
			}
			break
		}
		out = append(out, s)
		used += sw
	}
	if used < w {
		out = append(out, sg(strings.Repeat(" ", w-used), lipgloss.NewStyle()))
	}
	return out
}

func num(n, digits int) string {
	if n <= 0 {
		return strings.Repeat(" ", digits)
	}
	return fmt.Sprintf("%*d", digits, n)
}

func expandTabs(s string) string { return strings.ReplaceAll(s, "\t", "    ") }

// dlinePrefix 返回各类行在正文前面占的列数（行号栏、图标、缩进）；
// 不参与自动换行的行（文件标题、hunk 标题、图谱）返回 -1。
func dlinePrefix(k dkind, w, digits int) int {
	switch k {
	case dCtx, dAdd, dDel:
		if w >= 44 { // 和 renderDLine 的 twoCols 一致：旧行号 + 新行号
			return 2*digits + 5
		}
		return digits + 3
	case dBody, dInfo, dStatSum:
		return 3
	case dTitle:
		return lipgloss.Width(icons.commit) + 2
	case dMeta:
		return 5
	case dText:
		return 1
	case dErr:
		return lipgloss.Width(icons.cross) + 2
	}
	return -1
}

// wrapDLine 把超出宽度的一行按屏幕宽度折成多段；不需要折时返回 nil。
// 代码按字符硬折（保留缩进和空格），提交信息等文字优先在空格处折。
func wrapDLine(l dline, w, digits int) []string {
	pre := dlinePrefix(l.kind, w, digits)
	avail := w - pre
	if pre < 0 || avail < 8 {
		return nil
	}
	text := clean(l.text)
	if len(text) <= avail { // 字节数不小于显示宽度，放得下就不用再算
		return nil
	}
	if lipgloss.Width(text) <= avail {
		return nil
	}
	var wrapped string
	switch l.kind {
	case dCtx, dAdd, dDel:
		wrapped = ansi.Hardwrap(text, avail, true)
	default:
		wrapped = ansi.Wrap(text, avail, "")
	}
	return strings.Split(wrapped, "\n")
}

// renderDLine 渲染详情区的一行。
func renderDLine(l dline, w, digits int) string {
	gut := fg(cFaint)
	twoCols := w >= 44
	gutter := func(o, n int, sign string, signSt lipgloss.Style, bg lipgloss.TerminalColor) []seg {
		st := gut
		if bg != nil {
			st = st.Background(bg)
		}
		var s []seg
		if twoCols {
			s = append(s, sg(" "+num(o, digits)+" ", st))
		}
		s = append(s, sg(num(n, digits)+" ", st), sg(sign, signSt), sg(" ", lipgloss.NewStyle()))
		return s
	}
	if l.cont { // 换行折出来的后续行：行号栏留空，底色照旧
		l.old, l.new = 0, 0
	}
	lead := func(s string) string { // 后续行把行首的图标 / 符号换成等宽空白
		if l.cont {
			return strings.Repeat(" ", lipgloss.Width(s))
		}
		return s
	}
	switch l.kind {
	case dCtx:
		return paint(w, nil, append(gutter(l.old, l.new, " ", gut, nil), sg(expandTabs(l.text), fg(cSub)))...)
	case dAdd:
		return paint(w, cAddBg, append(gutter(0, l.new, lead("+"), fg(cGreen).Bold(true), cAddHi), sg(expandTabs(l.text), fg(cText)))...)
	case dDel:
		return paint(w, cDelBg, append(gutter(l.old, 0, lead("-"), fg(cRed).Bold(true), cDelHi), sg(expandTabs(l.text), fg(cText)))...)
	case dHunk:
		where := " " + i18n.Tf("第 %d 行", l.new) + " "
		return paint(w, cHunkBg, sg(" ⋯", fg(cCyan)), sg(where, fg(cMuted)), sg(l.text, fg(cSub).Italic(true)))
	case dMeta:
		return paint(w, nil, sg("   ", lipgloss.NewStyle()), sg(lead("· ")+l.text, fg(cMuted).Italic(true)))
	case dFile:
		dir, name := path.Split(l.path)
		right := []seg{}
		if l.add > 0 {
			right = append(right, sg(fmt.Sprintf("+%d ", l.add), fg(cGreen)))
		}
		if l.del > 0 {
			right = append(right, sg(fmt.Sprintf("-%d ", l.del), fg(cRed)))
		}
		arrow := icons.dirOpen
		if l.collapsed {
			arrow = icons.dirClosed
		}
		return rightAlign(w, cBar, []seg{sg(" ▌ ", fg(cAccent)), sg(arrow+" ", fg(cMuted)), sg(fileIcon(name)+name, fg(cText).Bold(true)), sg("  "+strings.TrimSuffix(dir, "/"), fg(cMuted))}, right)
	case dStat:
		bar := l.text
		var segs []seg
		segs = append(segs, sg("   ", lipgloss.NewStyle()), sg(l.path, fg(cSub)), sg("  ", lipgloss.NewStyle()))
		if i := strings.IndexAny(bar, "+-"); i >= 0 {
			segs = append(segs, sg(strings.TrimSpace(bar[:i])+" ", fg(cMuted)))
			plus := strings.Count(bar[i:], "+")
			minus := strings.Count(bar[i:], "-")
			segs = append(segs, sg(strings.Repeat("■", min(plus, 20)), fg(cGreen)), sg(strings.Repeat("■", min(minus, 20)), fg(cRed)))
		} else {
			segs = append(segs, sg(bar, fg(cMuted)))
		}
		return paint(w, nil, segs...)
	case dStatSum:
		return paint(w, nil, sg("   "+l.text, fg(cMuted)))
	case dTitle:
		return paint(w, nil, sg(lead(" "+icons.commit+" "), fg(cAccent)), sg(l.text, fg(cText).Bold(true)))
	case dInfo:
		return paint(w, nil, sg("   "+l.text, fg(cMuted)))
	case dBody:
		return paint(w, nil, sg("   "+expandTabs(l.text), fg(cSub)))
	case dGraph:
		return renderGraphLine(l.g, w, false, "")
	case dErr:
		return paint(w, nil, sg(lead(" "+icons.cross+" ")+l.text, fg(cRed)))
	case dText:
		return paint(w, nil, sg(" "+l.text, fg(cMuted)))
	}
	return strings.Repeat(" ", w)
}

// commitLines 把提交信息排成详情区开头的几行。
func commitLines(ci *gitx.CommitInfo) []dline {
	out := []dline{
		{kind: dBlank},
		{kind: dTitle, text: ci.Subject},
		{kind: dInfo, text: i18n.Tf("%s · %s · %s（%s）", ci.Short, ci.Author, ci.Date, ci.When)},
	}
	if ci.Refs != "" {
		out = append(out, dline{kind: dInfo, text: ci.Refs})
	}
	if ci.Body != "" {
		out = append(out, dline{kind: dBlank})
		body := strings.ReplaceAll(strings.ReplaceAll(ci.Body, "\r\n", "\n"), "\r", "\n")
		for _, b := range strings.Split(body, "\n") {
			out = append(out, dline{kind: dBody, text: b})
		}
	}
	return append(out, dline{kind: dBlank})
}

// ---------------------------------------------------------------- 提交图谱

// renderGraphLine 渲染一行提交图谱：彩色连线 + 哈希 + 引用标签 + 标题 + 时间。
func renderGraphLine(g gitx.GraphLine, w int, sel bool, headBranch string) string {
	var bg lipgloss.TerminalColor
	if sel {
		bg = cSel
	}
	var segs []seg
	segs = append(segs, sg(" ", lipgloss.NewStyle()))
	for _, c := range g.Cells {
		if c.Color < 0 {
			segs = append(segs, sg(c.Glyph, lipgloss.NewStyle()))
			continue
		}
		st := fg(graphColors[c.Color%len(graphColors)])
		if c.Glyph == "●" || c.Glyph == "◉" || c.Glyph == "○" {
			st = st.Bold(true)
		}
		segs = append(segs, sg(c.Glyph, st))
	}
	segs = append(segs, sg(" ", lipgloss.NewStyle()))
	if g.Hash == "" {
		return paint(w, bg, segs...)
	}
	hashSt := fg(cYellow)
	if g.Merge {
		hashSt = fg(cMuted)
	}
	segs = append(segs, sg(g.Hash+" ", hashSt))
	for _, ref := range splitRefs(g.Refs) {
		segs = append(segs, refPill(ref), sg(" ", lipgloss.NewStyle()))
	}
	subjSt := fg(cText)
	if g.Merge {
		subjSt = fg(cMuted) // 合并提交淡一点，突出真正的改动
	}
	if sel {
		subjSt = subjSt.Bold(true)
	}
	left := append(segs, sg(g.Subject, subjSt))
	right := []seg{sg(" "+g.When+" ", fg(cFaint))}
	if lipgloss.Width(g.When) > w/3 { // 列表太窄时宁可不显示时间，也要给标题留地方
		right = nil
	}
	return rightAlign(w, bg, left, right)
}

func splitRefs(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, r := range strings.Split(s, ", ") {
		if r = strings.TrimSpace(r); r != "" && r != "origin/HEAD" {
			out = append(out, r)
		}
	}
	return out
}

func refPill(ref string) seg {
	st := lipgloss.NewStyle().Background(cSurface).Bold(true)
	label := ref
	switch {
	case strings.HasPrefix(ref, "HEAD -> "):
		label = icons.branch + " " + strings.TrimPrefix(ref, "HEAD -> ")
		st = lipgloss.NewStyle().Background(cGreen).Foreground(cOnPill).Bold(true)
	case ref == "HEAD":
		st = st.Foreground(cCyan)
	case strings.HasPrefix(ref, "tag: "):
		label = "◆ " + strings.TrimPrefix(ref, "tag: ")
		st = st.Foreground(cYellow)
	case strings.Contains(ref, "/"):
		st = st.Foreground(cPurple)
	default:
		st = st.Foreground(cGreen)
	}
	return sg(" "+label+" ", st)
}
