package ui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/0xBB2B/gitpane/internal/i18n"
)

// ---------------------------------------------------------------- 几何布局
//
// 上下布局（窄屏，或 --layout stack）：
//
//	0  头部：分支信息
//	1  标签页
//	2… 列表区（listH 行）
//	   详情标题栏
//	   详情区（detailH 行）
//	   按钮栏
//	   状态栏 / 提交输入框
//
// 左右布局（宽度 ≥ wideMin，或 --layout wide）：
//
//	0  头部
//	1  标签页
//	2… 列表区 │ 详情标题栏 + 详情区
//	   按钮栏
//	   状态栏
//
// 渲染和鼠标命中判断都用同一个 geom，保证点哪儿就是哪儿。
type geom struct {
	wide                                  bool
	headerY, tabsY                        int
	listX, listY, listW, listH            int
	detailX, detailHdrY, detailY, detailW int
	detailH                               int
	buttonsY, statusY                     int
}

const wideMin = 100

// Layout 决定列表和详情的排布方式。
type Layout int

const (
	LayoutAuto  Layout = iota // 宽度够就左右，否则上下
	LayoutWide                // 总是左右
	LayoutStack               // 总是上下
)

func (m Model) isWide() bool {
	switch m.opts.Layout {
	case LayoutWide:
		return m.w >= 60
	case LayoutStack:
		return false
	}
	return m.w >= wideMin
}

func (m Model) geom() geom {
	g := geom{headerY: 0, tabsY: 1, listY: 2}
	body := max(2, m.h-4) // 去掉头部、标签、按钮、状态四行
	if m.isWide() {
		g.wide = true
		g.listW = max(30, min(m.w*2/5, 64))
		if m.tab == tabGraph {
			// 图谱最该看清的是提交标题，列表给宽一点
			g.listW = max(40, min(m.w*11/20, 100))
		}
		g.listH = body
		g.detailX = g.listW + 1 // 中间一列分隔线
		g.detailW = m.w - g.detailX
		g.detailHdrY = g.listY
		g.detailY = g.listY + 1
		g.detailH = body - 1
		g.buttonsY = g.listY + body
		g.statusY = g.buttonsY + 1
		return g
	}

	avail := max(2, body-1) // 上下布局还要一行详情标题栏
	var listH int
	switch {
	case m.repo == nil:
		listH = 4
	case m.tab == tabChanges:
		listH = min(max(m.itemCount(), 4), avail*3/5)
	default:
		listH = min(max(m.itemCount(), 4), avail/2)
	}
	listH = max(1, min(listH, avail-1))
	g.listW, g.detailW = m.w, m.w
	g.listH = listH
	g.detailHdrY = g.listY + listH
	g.detailY = g.detailHdrY + 1
	g.detailH = avail - listH
	g.buttonsY = g.detailY + g.detailH
	g.statusY = g.buttonsY + 1
	return g
}

func (g geom) inList(x, y int) bool {
	return y >= g.listY && y < g.listY+g.listH && x >= g.listX && x < g.listX+g.listW
}

func (g geom) inDetail(x, y int) bool {
	return y >= g.detailHdrY && y < g.detailY+g.detailH && x >= g.detailX && x < g.detailX+g.detailW
}

// ---------------------------------------------------------------- 按钮

type btnKind int

const (
	btnNormal btnKind = iota
	btnPrimary
	btnDanger
)

type button struct {
	id, label string
	kind      btnKind
	x0, x1    int
}

func (b button) hit(x int) bool { return x >= b.x0 && x < b.x1 }

// place 从 x 开始依次排布按钮，放不下的丢弃。
func place(x, maxW int, bs []button) []button {
	var out []button
	for _, b := range bs {
		w := lipgloss.Width(b.label) + 2
		if x+w > maxW {
			break
		}
		b.x0, b.x1 = x, x+w
		out = append(out, b)
		x += w + 1
	}
	return out
}

func (m Model) tabButtons() []button {
	changes := i18n.T("变更")
	if n := m.changeCount(); n > 0 {
		changes = fmt.Sprintf(i18n.T("变更 %d"), n)
	}
	return place(1, m.w, []button{{id: "0", label: changes}, {id: "1", label: i18n.T("图谱")}})
}

func (m Model) bottomButtons() []button {
	if m.committing {
		return place(1, m.w, []button{
			{id: "commitConfirm", label: fmt.Sprintf(i18n.T("%s 提交 %d 个文件"), icons.check, m.stagedCount()), kind: btnPrimary},
			{id: "commitCancel", label: i18n.T("取消")},
		})
	}
	if m.repo == nil {
		return place(1, m.w, []button{{id: "init", label: "git init", kind: btnPrimary}, {id: "repoRetry", label: i18n.T("↻ 刷新")}})
	}
	pull, push := icons.down+i18n.T(" 拉取"), icons.up+i18n.T(" 推送")
	if st := m.status; st != nil {
		if st.Behind > 0 {
			pull += fmt.Sprintf(" %d", st.Behind)
		}
		if st.Ahead > 0 {
			push += fmt.Sprintf(" %d", st.Ahead)
		}
	}
	var bs []button
	switch m.tab {
	case tabChanges:
		bs = []button{{id: "commit", label: icons.check + i18n.T(" 提交"), kind: btnPrimary}, {id: "pull", label: pull}, {id: "push", label: push}, {id: "stageAll", label: i18n.T("+ 全部暂存")}}
		if r, ok := m.currentRow(); ok && r.kind == rowFile && r.sec == secChanges {
			bs = append(bs, button{id: "discard", label: i18n.T("✕ 放弃"), kind: btnDanger})
		}
		bs = append(bs, button{id: "refresh", label: "↻"})
	case tabGraph:
		bs = []button{{id: "picker", label: icons.branch + i18n.T(" 分支"), kind: btnPrimary}, {id: "pull", label: pull}, {id: "push", label: push}, {id: "fetch", label: i18n.T("⇣ 抓取")}, {id: "refresh", label: "↻"}}
	}
	return place(1, m.w, bs)
}

func btnStyle(k btnKind) lipgloss.Style {
	switch k {
	case btnPrimary:
		return lipgloss.NewStyle().Background(cAccent).Foreground(cOnPill).Bold(true)
	case btnDanger:
		return lipgloss.NewStyle().Background(cSurface).Foreground(cRed)
	}
	return lipgloss.NewStyle().Background(cSurface).Foreground(cText)
}

func renderButtons(w int, bs []button) string {
	var segs []seg
	x := 0
	for _, b := range bs {
		if b.x0 > x {
			segs = append(segs, sg(strings.Repeat(" ", b.x0-x), lipgloss.NewStyle()))
		}
		segs = append(segs, sg(" "+b.label+" ", btnStyle(b.kind)))
		x = b.x1
	}
	return paint(w, nil, segs...)
}

// rowButtonX 返回变更列表行右侧操作按钮的起始列（相对列表区）；-1 表示没有按钮。
func rowButtonX(r row, w int) int {
	switch r.kind {
	case rowSection:
		if r.count == 0 {
			return -1
		}
		return w - lipgloss.Width(sectionAction(r.sec)) - 3
	case rowDir, rowFile:
		return w - 4
	}
	return -1
}

func sectionAction(s section) string {
	switch s {
	case secStaged:
		return i18n.T("全部取消")
	case secConflict:
		return i18n.T("标记解决")
	}
	return i18n.T("全部暂存")
}

func sectionColor(s section) ac {
	switch s {
	case secConflict:
		return cRed
	case secStaged:
		return cGreen
	}
	return cYellow
}

// ---------------------------------------------------------------- 渲染

func (m Model) View() string {
	if m.w < 24 || m.h < 8 {
		return i18n.T("窗口太小")
	}
	if m.showHelp {
		return m.viewHelp()
	}
	if m.picking {
		return m.viewPicker()
	}
	g := m.geom()
	lines := make([]string, 0, m.h)
	lines = append(lines, m.viewHeader(), m.viewTabs())
	list := m.viewList(g)
	right := append([]string{m.viewDetailHeader(g)}, m.viewDetail(g)...)
	if g.wide {
		sep := fg(cGuide).Render("│")
		for i := 0; i < g.listH; i++ {
			lines = append(lines, list[i]+sep+right[i])
		}
	} else {
		lines = append(lines, list...)
		lines = append(lines, right...)
	}
	lines = append(lines, renderButtons(m.w, m.bottomButtons()), m.viewStatus())
	return strings.Join(lines, "\n")
}

func (m Model) viewHeader() string {
	pill := lipgloss.NewStyle().Background(cAccent).Foreground(cOnPill).Bold(true)
	var left, right []seg
	left = append(left, sg(" ", lipgloss.NewStyle()))
	switch {
	case m.repo == nil:
		left = append(left, sg(i18n.T("未检测到仓库"), fg(cMuted)))
	case m.status == nil:
		left = append(left, sg(i18n.T("读取中…"), fg(cMuted)))
	default:
		st := m.status
		if st.Detached {
			left = append(left, sg(" "+icons.branch+i18n.T(" HEAD 游离 "), pill.Background(cYellow)))
		} else {
			left = append(left, sg(" "+icons.branch+" "+st.Branch+" ", pill))
		}
		switch {
		case st.NoCommits:
			left = append(left, sg(i18n.T("  尚无提交"), fg(cMuted)))
		case st.Upstream != "":
			left = append(left, sg(" → "+st.Upstream, fg(cMuted)))
			if st.Gone {
				left = append(left, sg(i18n.T(" 上游已删除"), fg(cRed)))
			}
		case !st.Detached:
			left = append(left, sg(i18n.T("  未设置上游"), fg(cFaint)))
		}
		if st.Ahead > 0 {
			left = append(left, sg(fmt.Sprintf(" %s%d", icons.up, st.Ahead), fg(cGreen).Bold(true)))
		}
		if st.Behind > 0 {
			left = append(left, sg(fmt.Sprintf(" %s%d", icons.down, st.Behind), fg(cYellow).Bold(true)))
		}
	}
	if m.repo != nil {
		name := filepath.Base(m.repo.Root)
		if icons.repo != "" {
			name = icons.repo + " " + name
		}
		right = append(right, sg(name+" ", fg(cMuted)))
	}
	return rightAlign(m.w, cBar, left, right)
}

func (m Model) viewTabs() string {
	var segs []seg
	x := 0
	for _, t := range m.tabButtons() {
		if t.x0 > x {
			segs = append(segs, sg(strings.Repeat(" ", t.x0-x), lipgloss.NewStyle()))
		}
		active := int(t.id[0]-'0') == int(m.tab)
		name, count, _ := strings.Cut(t.label, " ")
		nameSt, cntSt := fg(cMuted), fg(cFaint)
		if active {
			nameSt = lipgloss.NewStyle().Background(cSurface).Foreground(cAccent).Bold(true)
			cntSt = lipgloss.NewStyle().Background(cSurface).Foreground(cYellow).Bold(true)
		} else if count != "" {
			cntSt = fg(cYellow)
		}
		if count == "" {
			segs = append(segs, sg(" "+name+" ", nameSt))
		} else {
			segs = append(segs, sg(" "+name+" ", nameSt), sg(count+" ", cntSt))
		}
		x = t.x1
	}
	rb := m.tabRightButtons()
	end := m.w - 1
	if len(rb) > 0 {
		end = rb[0].x0 - 1
	}
	if rest := end - x; rest > 0 {
		segs = append(segs, sg(" "+strings.Repeat("─", rest-1)+" ", fg(cGuide)))
		x = end
	}
	for _, b := range rb {
		if b.x0 > x {
			segs = append(segs, sg(strings.Repeat(" ", b.x0-x), lipgloss.NewStyle()))
		}
		st := lipgloss.NewStyle().Background(cSurface).Foreground(cSub)
		if b.kind == btnPrimary {
			st = lipgloss.NewStyle().Background(cSurface).Foreground(cAccent).Bold(true)
		}
		segs = append(segs, sg(" "+b.label+" ", st))
		x = b.x1
	}
	return paint(m.w, nil, segs...)
}

// tabRightButtons 是标签栏右端的按钮：变更页是「全部折叠/展开」，图谱页是分支筛选。
func (m Model) tabRightButtons() []button {
	var bs []button
	switch m.tab {
	case tabChanges:
		if b, ok := m.foldButton(); ok {
			bs = append(bs, b)
		}
	case tabGraph:
		if m.repo == nil {
			return nil
		}
		if len(m.graphRefs) > 0 {
			bs = append(bs, button{id: "filterClear", label: i18n.T("✕ 全部")})
		}
		bs = append(bs, button{id: "filter", label: m.filterLabel(), kind: btnPrimary})
	}
	// 从右往左排
	x := m.w - 1
	for i := len(bs) - 1; i >= 0; i-- {
		w := lipgloss.Width(bs[i].label) + 2
		bs[i].x1, bs[i].x0 = x, x-w
		x -= w + 1
	}
	if len(bs) > 0 && bs[0].x0 < 28 {
		return nil // 太窄就不显示，用键盘（z / b）
	}
	return bs
}

// foldButton 是标签栏右端的「全部折叠 / 全部展开」按钮，只在变更页且有文件夹时出现。
func (m Model) foldButton() (button, bool) {
	if m.tab != tabChanges || m.repo == nil {
		return button{}, false
	}
	hasDir, anyOpen := false, false
	for _, r := range m.rows {
		if r.kind == rowDir {
			hasDir = true
			if !r.collapsed {
				anyOpen = true
			}
		}
	}
	if !hasDir {
		return button{}, false
	}
	label := icons.dirClosed + i18n.T(" 全部折叠")
	if !anyOpen {
		label = icons.dirOpen + i18n.T(" 全部展开")
	}
	w := lipgloss.Width(label) + 2
	b := button{id: "foldAll", label: label, x0: m.w - w - 1, x1: m.w - 1}
	if b.x0 < 30 {
		return button{}, false // 太窄就不显示，用 z 键
	}
	return b, true
}

func (m Model) viewList(g geom) []string {
	w := g.listW
	out := make([]string, 0, g.listH)
	if m.repo == nil {
		out = append(out,
			paint(w, nil, sg(i18n.T("  这里还不是 Git 仓库"), fg(cText).Bold(true))),
			paint(w, nil, sg(i18n.T("  初始化后就能在这里暂存、提交、推送"), fg(cMuted))),
			paint(w, nil, sg("  ", lipgloss.NewStyle()), sg(" git init ", btnStyle(btnPrimary))))
	} else {
		off := m.offset[m.tab]
		for i := off; i < off+g.listH && i < m.itemCount(); i++ {
			sel := i == m.cursor[m.tab]
			switch m.tab {
			case tabChanges:
				out = append(out, m.renderRow(m.rows[i], sel, w))
			case tabGraph:
				out = append(out, renderGraphLine(m.graph[i], w, sel, ""))
			}
		}
		if m.itemCount() == 0 {
			msg := i18n.T("（空）")
			if m.tab == tabGraph {
				msg = i18n.T("还没有提交")
				if len(m.graphRefs) > 0 && m.graph == nil {
					msg = i18n.T("加载中…")
				}
			}
			if m.statusErr != "" {
				msg = m.statusErr
			}
			out = append(out, paint(w, nil, sg("  "+msg, fg(cMuted))))
		}
	}
	for len(out) < g.listH {
		out = append(out, strings.Repeat(" ", w))
	}
	return out[:g.listH]
}

func (m Model) renderRow(r row, sel bool, w int) string {
	var bg lipgloss.TerminalColor
	if sel {
		bg = cSel
	}
	arrow := icons.dirOpen
	if r.collapsed {
		arrow = icons.dirClosed
	}
	switch r.kind {
	case rowClean:
		return paint(w, nil, sg("  "+icons.clean+i18n.T(" 工作区很干净"), fg(cGreen).Bold(true)), sg(i18n.T("  没有未提交的改动"), fg(cMuted)))

	case rowSection:
		col := sectionColor(r.sec)
		left := []seg{sg("▍", fg(col)), sg(arrow+" ", fg(cMuted)), sg(r.name, fg(cText).Bold(true)), sg(fmt.Sprintf("  %d", r.count), fg(col))}
		var right []seg
		if r.count > 0 {
			right = []seg{sg(" "+sectionAction(r.sec)+" ", lipgloss.NewStyle().Background(cSurface).Foreground(cSub)), sg(" ", lipgloss.NewStyle())}
		}
		return rightAlign(w, bg, left, right)

	case rowDir:
		nameSt := fg(cAccent)
		if sel {
			nameSt = nameSt.Bold(true)
		}
		left := []seg{sg(" "+r.guide, fg(cGuide)), sg(arrow+" ", fg(cMuted)), sg(r.name+"/", nameSt)}
		var right []seg
		// 目录汇总增删行数，折起来也知道里面改了多少
		if w >= 46 {
			add, del := 0, 0
			for _, p := range r.files {
				if st, ok := m.stats[r.sec][p]; ok && st.Add >= 0 {
					add += st.Add
					del += st.Del
				}
			}
			if add > 0 {
				right = append(right, sg(fmt.Sprintf("+%d ", add), fg(cGreen).Faint(!r.collapsed)))
			}
			if del > 0 {
				right = append(right, sg(fmt.Sprintf("-%d ", del), fg(cRed).Faint(!r.collapsed)))
			}
		}
		cnt := fmt.Sprintf("%d ", r.count)
		if r.collapsed {
			cnt = fmt.Sprintf(i18n.T("%d 个文件 "), r.count)
		}
		right = append(right, sg(" "+cnt, fg(cFaint)), m.marker(r), sg(" ", lipgloss.NewStyle()))
		return rightAlign(w, bg, left, right)

	case rowFile:
		code := statusCode(r.file, r.sec)
		nameSt := fg(cText)
		switch {
		case code == 'D':
			nameSt = fg(cMuted).Strikethrough(true)
		case r.file.Untracked():
			nameSt = fg(cGreen)
		}
		if sel {
			nameSt = nameSt.Bold(true)
		}
		left := []seg{sg(" "+r.guide, fg(cGuide)), sg(fileIcon(r.name), fg(cMuted)), sg(r.name, nameSt)}
		var right []seg
		if w >= 46 {
			if st, ok := m.stats[r.sec][r.path]; ok {
				switch {
				case st.Add < 0:
					right = append(right, sg("bin ", fg(cFaint)))
				default:
					if st.Add > 0 {
						right = append(right, sg(fmt.Sprintf("+%d ", st.Add), fg(cGreen)))
					}
					if st.Del > 0 {
						right = append(right, sg(fmt.Sprintf("-%d ", st.Del), fg(cRed)))
					}
				}
			}
		}
		right = append(right, sg(" "+string(code)+" ", fg(statusColor(code)).Bold(true)), m.marker(r), sg(" ", lipgloss.NewStyle()))
		return rightAlign(w, bg, left, right)

	case rowEmpty:
		return paint(w, bg, sg(" └ ", fg(cGuide)), sg(i18n.T("无"), fg(cFaint)))
	}
	return strings.Repeat(" ", w)
}

func (m Model) marker(r row) seg {
	if r.sec == secStaged {
		return sg(" - ", lipgloss.NewStyle().Background(cSurface).Foreground(cYellow).Bold(true))
	}
	return sg(" + ", lipgloss.NewStyle().Background(cSurface).Foreground(cGreen).Bold(true))
}

var trackRe = regexp.MustCompile(`(ahead|behind) (\d+)`)

func (m Model) viewDetailHeader(g geom) string {
	d := m.detail.head
	if m.detailKey == "" {
		hint := i18n.T("选中文件查看差异")
		if m.tab == tabGraph {
			hint = i18n.T("选中提交查看详情")
		}
		return paint(g.detailW, cBar, sg(" ▌", fg(cGuide)), sg(" "+hint, fg(cMuted)))
	}
	if m.detail.key != m.detailKey {
		return paint(g.detailW, cBar, sg(" ▌", fg(cAccent)), sg(i18n.T(" 加载中…"), fg(cMuted)))
	}
	left := []seg{sg(" ▌", fg(cAccent)), sg(" "+d.icon+d.name, fg(cText).Bold(true))}
	if d.dir != "" {
		left = append(left, sg("  "+d.dir, fg(cMuted)))
	}
	var right []seg
	if d.add > 0 {
		right = append(right, sg(fmt.Sprintf("+%d ", d.add), fg(cGreen).Bold(true)))
	}
	if d.del > 0 {
		right = append(right, sg(fmt.Sprintf("-%d ", d.del), fg(cRed).Bold(true)))
	}
	if d.badge != "" {
		right = append(right, sg(" "+d.badge+" ", lipgloss.NewStyle().Background(d.badgeCol).Foreground(cOnPill).Bold(true)), sg(" ", lipgloss.NewStyle()))
	}
	if n := len(m.visibleDetail()); n > g.detailH {
		pct := 100 * (m.detailOff + g.detailH) / n
		right = append(right, sg(fmt.Sprintf("%d%% ", min(100, pct)), fg(cFaint)))
	}
	if b, ok := m.detailFoldButton(g); ok {
		right = append(right, sg(" "+b.label+" ", lipgloss.NewStyle().Background(cSurface).Foreground(cSub)), sg(" ", lipgloss.NewStyle()))
	}
	return rightAlign(g.detailW, cBar, left, right)
}

// detailFoldButton 是提交详情标题栏右端的「全部展开 / 全部折叠」按钮（坐标为整屏绝对列）。
func (m Model) detailFoldButton(g geom) (button, bool) {
	if m.detail.files < 2 || m.detail.key != m.detailKey {
		return button{}, false
	}
	label := icons.dirOpen + i18n.T(" 全部展开")
	for _, l := range m.detail.lines {
		if l.kind == dFile && !m.fileFolded(l.path) {
			label = icons.dirClosed + i18n.T(" 全部折叠")
			break
		}
	}
	w := lipgloss.Width(label) + 2
	x1 := g.detailX + g.detailW - 1
	return button{id: "filesFold", label: label, x0: x1 - w, x1: x1}, true
}

func (m Model) viewDetail(g geom) []string {
	out := make([]string, 0, g.detailH)
	if m.detail.key == m.detailKey {
		vis := m.visibleDetail()
		for i := m.detailOff; i < len(vis) && len(out) < g.detailH; i++ {
			out = append(out, renderDLine(vis[i], g.detailW, m.detail.digits))
		}
	}
	for len(out) < g.detailH {
		out = append(out, strings.Repeat(" ", g.detailW))
	}
	return out
}

func (m Model) viewStatus() string {
	switch {
	case m.committing:
		return paint(m.w, nil, sg(" ✎ ", fg(cAccent).Bold(true)), sgRaw(m.input.View(), lipgloss.NewStyle()))
	case m.busy != "":
		return paint(m.w, nil, sgRaw(" "+m.spin.View()+" ", lipgloss.NewStyle()), sg(m.busy, fg(cAccent)))
	case m.msg != "":
		icon, col := "ℹ", cMuted
		switch m.msgLevel {
		case lvOK:
			icon, col = icons.check, cGreen
		case lvErr:
			icon, col = icons.cross, cRed
		}
		return paint(m.w, nil, sg(" "+icon+" ", fg(col).Bold(true)), sg(m.msg, fg(col)))
	}
	keys := [][2]string{{i18n.T("空格"), i18n.T("暂存")}, {"←→", i18n.T("折叠")}, {"c", i18n.T("提交")}, {"a", i18n.T("全暂存")}, {"x", i18n.T("放弃")}, {"Tab", i18n.T("切换")}}
	switch m.tab {
	case tabGraph:
		keys = [][2]string{{"↑↓", i18n.T("选择")}, {"b", i18n.T("选分支")}, {"z", i18n.T("展开文件")}, {"J/K", i18n.T("翻详情")}, {"Tab", i18n.T("切换")}}
	}
	tail := [][2]string{{"?", i18n.T("帮助")}}
	if !m.opts.Embed {
		tail = append(tail, [2]string{"q", i18n.T("退出")})
	}
	kbd := lipgloss.NewStyle().Background(cSurface).Foreground(cSub)
	render := func(ks [][2]string) []seg {
		var s []seg
		for _, k := range ks {
			s = append(s, sg(" "+k[0]+" ", kbd), sg(" "+k[1]+"  ", fg(cFaint)))
		}
		return s
	}
	// 帮助/退出贴右边；放不下时 rightAlign 会先截左边的提示
	return rightAlign(m.w, nil, append([]seg{sg(" ", lipgloss.NewStyle())}, render(keys)...), render(tail))
}

// viewHelp 是按 ? 打开的全屏帮助。
func (m Model) viewHelp() string {
	type item struct{ k, v string }
	sections := []struct {
		title string
		items []item
	}{
		{i18n.T("通用"), []item{
			{"Tab / 1 2", i18n.T("切换 变更 / 图谱")},
			{"j k ↑ ↓", i18n.T("上下移动")},
			{"g G", i18n.T("到顶 / 到底")},
			{"J K", i18n.T("详情区翻页（也可以滚轮）")},
			{"r", i18n.T("立即刷新（平时每 2 秒自动刷新）")},
			{"?", i18n.T("打开 / 关闭帮助")},
		}},
		{i18n.T("变更"), []item{
			{i18n.T("空格"), i18n.T("暂存 / 取消暂存当前文件、目录或分组")},
			{i18n.T("回车 / ← →"), i18n.T("折叠 / 展开目录；在文件上按 ← 回到所在目录")},
			{"z", i18n.T("全部折叠 / 全部展开")},
			{"a  u", i18n.T("全部暂存 / 全部取消暂存")},
			{"c", i18n.T("提交（回车确认，Esc 取消）")},
			{"x", i18n.T("放弃当前文件的改动（按两次确认）")},
		}},
		{i18n.T("图谱与分支"), []item{
			{"b", i18n.T("选择要显示的分支（可多选，都不选 = 全部）")},
			{"z", i18n.T("提交详情里全部展开 / 折叠文件")},
			{"p  P  f", "pull / push / fetch"},
			{i18n.T("s / 双击"), i18n.T("在分支选择面板里切换到该分支")},
		}},
		{i18n.T("鼠标"), []item{
			{i18n.T("点文件"), i18n.T("查看 diff；点右侧 + / - 暂存或取消")},
			{i18n.T("点目录 / 分组"), i18n.T("折叠；分组右侧按钮批量操作")},
			{i18n.T("点「全部折叠」"), i18n.T("标签栏右侧，一键折叠 / 展开所有目录")},
			{i18n.T("点顶部分支名"), i18n.T("打开分支选择面板")},
			{i18n.T("点提交详情里的文件名"), i18n.T("展开 / 折叠该文件的改动")},
			{i18n.T("滚轮"), i18n.T("列表上滚列表，详情上滚详情")},
		}},
	}
	quit := i18n.T("Ctrl+C 两次")
	if !m.opts.Embed {
		quit = "q / Ctrl+C"
	}
	sections[0].items = append(sections[0].items, item{quit, i18n.T("退出")})

	kbd := lipgloss.NewStyle().Background(cSurface).Foreground(cAccent).Bold(true)
	var lines []string
	lines = append(lines, paint(m.w, cBar, sg(" gitpane ", lipgloss.NewStyle().Background(cAccent).Foreground(cOnPill).Bold(true)), sg(i18n.T("  快捷键与鼠标操作"), fg(cMuted))), "")
	for _, s := range sections {
		lines = append(lines, paint(m.w, nil, sg("  "+s.title, fg(cYellow).Bold(true))))
		for _, it := range s.items {
			k := " " + it.k + " "
			pad := strings.Repeat(" ", max(1, 16-lipgloss.Width(k)))
			lines = append(lines, paint(m.w, nil, sg("    ", lipgloss.NewStyle()), sg(k, kbd), sg(pad+it.v, fg(cSub))))
		}
		lines = append(lines, "")
	}
	for len(lines) < m.h-1 {
		lines = append(lines, "")
	}
	lines = lines[:m.h-1]
	for i, l := range lines {
		if l == "" {
			lines[i] = strings.Repeat(" ", m.w)
		}
	}
	lines = append(lines, paint(m.w, nil, sg(i18n.T("  按任意键或点击返回"), fg(cFaint))))
	return strings.Join(lines, "\n")
}
