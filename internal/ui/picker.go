package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// 分支选择面板：图谱页里按 b、点标签栏右侧的「⎇ 全部分支 ▾」或点顶部分支名打开。
// 勾选一个或多个分支后，图谱只显示这些分支；一个都不勾 = 显示全部。
// 在这里也可以切换当前分支（双击，或选中后按 s / 点「切换到此分支」）。

type pickRow struct {
	header string // 非空表示分组标题行
	bi     int    // m.branches 下标
}

func (m Model) pickRows() []pickRow {
	var rows []pickRow
	lastRemote := -1
	for i, b := range m.branches {
		r := 0
		if b.Remote {
			r = 1
		}
		if r != lastRemote {
			rows = append(rows, pickRow{header: map[int]string{0: "本地分支", 1: "远程分支"}[r]})
			lastRemote = r
		}
		rows = append(rows, pickRow{bi: i})
	}
	return rows
}

func (m Model) isPicked(name string) bool {
	for _, r := range m.graphRefs {
		if r == name {
			return true
		}
	}
	return false
}

func (m *Model) openPicker() tea.Cmd {
	if m.repo == nil {
		return nil
	}
	var cmd tea.Cmd
	if m.tab != tabGraph {
		cmd = m.switchTab(tabGraph)
	}
	m.picking = true
	// 光标放在当前分支上
	for i, r := range m.pickRows() {
		if r.header == "" && r.bi < len(m.branches) && m.branches[r.bi].Current {
			m.pickCursor = i
		}
	}
	m.pickClamp()
	return cmd
}

func (m Model) pickListH() int { return max(1, m.h-4) }

func (m *Model) pickClamp() {
	rows := m.pickRows()
	if len(rows) == 0 {
		m.pickCursor, m.pickOffset = 0, 0
		return
	}
	m.pickCursor = max(0, min(m.pickCursor, len(rows)-1))
	if rows[m.pickCursor].header != "" && m.pickCursor+1 < len(rows) {
		m.pickCursor++
	}
	h := m.pickListH()
	if m.pickCursor < m.pickOffset {
		m.pickOffset = m.pickCursor
	}
	if m.pickCursor >= m.pickOffset+h {
		m.pickOffset = m.pickCursor - h + 1
	}
	m.pickOffset = max(0, min(m.pickOffset, len(rows)-h))
}

func (m *Model) pickMove(d int) {
	rows := m.pickRows()
	i := m.pickCursor + d
	for i >= 0 && i < len(rows) && rows[i].header != "" {
		i += d
	}
	if i >= 0 && i < len(rows) {
		m.pickCursor = i
	}
	m.pickClamp()
}

// setGraphRefs 改变图谱筛选并从头加载。
func (m *Model) setGraphRefs(refs []string) tea.Cmd {
	m.graphRefs = refs
	m.graphLimit = graphPage
	m.graph = nil
	m.cursor[tabGraph], m.offset[tabGraph] = 0, 0
	m.detailKey = ""
	m.detail = detailView{}
	return m.refreshCmd()
}

func (m *Model) togglePick(name string) tea.Cmd {
	var refs []string
	found := false
	for _, r := range m.graphRefs {
		if r == name {
			found = true
			continue
		}
		refs = append(refs, r)
	}
	if !found {
		// 按分支列表顺序排，标签上显示更稳定
		refs = nil
		for _, b := range m.branches {
			if b.Name == name || m.isPicked(b.Name) {
				refs = append(refs, b.Name)
			}
		}
	}
	return m.setGraphRefs(refs)
}

func (m *Model) pickSwitch() tea.Cmd {
	rows := m.pickRows()
	if m.pickCursor >= len(rows) || rows[m.pickCursor].header != "" {
		return nil
	}
	b := m.branches[rows[m.pickCursor].bi]
	if b.Current {
		m.setMsg(lvInfo, "已经在 "+b.Name+" 上")
		return nil
	}
	repo := m.repo
	return m.startOp("切换分支…", func() (string, error) { return "已切换到 " + b.Name, repo.Switch(b) })
}

func (m Model) handlePickKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := m.pickRows()
	switch k.String() {
	case "esc", "enter", "q", "b":
		m.picking = false
	case "ctrl+c":
		m.picking = false
		if !m.opts.Embed {
			return m, tea.Quit
		}
	case "j", "down":
		m.pickMove(1)
	case "k", "up":
		m.pickMove(-1)
	case "g", "home":
		m.pickCursor = 0
		m.pickClamp()
	case "G", "end":
		m.pickCursor = len(rows) - 1
		m.pickClamp()
	case " ", "x":
		if m.pickCursor < len(rows) && rows[m.pickCursor].header == "" {
			return m, m.togglePick(m.branches[rows[m.pickCursor].bi].Name)
		}
	case "a":
		return m, m.setGraphRefs(nil)
	case "s":
		return m, m.pickSwitch()
	}
	return m, nil
}

func (m Model) handlePickMouse(ev tea.MouseMsg) (tea.Model, tea.Cmd) {
	rows := m.pickRows()
	switch ev.Button {
	case tea.MouseButtonWheelUp:
		m.pickOffset = max(0, m.pickOffset-3)
		return m, nil
	case tea.MouseButtonWheelDown:
		m.pickOffset = max(0, min(m.pickOffset+3, len(rows)-m.pickListH()))
		return m, nil
	}
	if ev.Action != tea.MouseActionPress || ev.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if ev.Y == m.h-2 {
		for _, b := range m.pickButtons() {
			if b.hit(ev.X) {
				switch b.id {
				case "pickDone":
					m.picking = false
				case "pickAll":
					return m, m.setGraphRefs(nil)
				case "pickSwitch":
					return m, m.pickSwitch()
				}
			}
		}
		return m, nil
	}
	if ev.Y < 2 || ev.Y >= 2+m.pickListH() {
		return m, nil
	}
	i := m.pickOffset + ev.Y - 2
	if i >= len(rows) || rows[i].header != "" {
		return m, nil
	}
	double := i == m.lastClickRow && time.Since(m.lastClickAt) < 400*time.Millisecond
	m.lastClickRow, m.lastClickAt = i, time.Now()
	m.pickCursor = i
	m.pickClamp()
	if double {
		// 双击：第二下把勾选还原，然后切换分支
		return m, tea.Batch(m.togglePick(m.branches[rows[i].bi].Name), m.pickSwitch())
	}
	return m, m.togglePick(m.branches[rows[i].bi].Name)
}

func (m Model) pickButtons() []button {
	return place(1, m.w, []button{
		{id: "pickDone", label: icons.check + " 完成", kind: btnPrimary},
		{id: "pickAll", label: "显示全部"},
		{id: "pickSwitch", label: "⇄ 切换到此分支"},
	})
}

// filterLabel 是标签栏右侧筛选按钮上的文字。
func (m Model) filterLabel() string {
	switch len(m.graphRefs) {
	case 0:
		return icons.branch + " 全部分支 ▾"
	case 1:
		return icons.branch + " " + m.graphRefs[0] + " ▾"
	}
	return fmt.Sprintf("%s %s +%d ▾", icons.branch, m.graphRefs[0], len(m.graphRefs)-1)
}

func (m Model) viewPicker() string {
	rows := m.pickRows()
	lines := make([]string, 0, m.h)

	sel := "显示全部分支"
	if n := len(m.graphRefs); n > 0 {
		sel = "已选 " + strconv.Itoa(n) + " 个分支"
	}
	lines = append(lines,
		rightAlign(m.w, cBar,
			[]seg{sg(" "+icons.branch+" 选择要显示的分支 ", lipgloss.NewStyle().Background(cAccent).Foreground(cOnPill).Bold(true))},
			[]seg{sg(sel+" ", fg(cYellow))}),
		paint(m.w, nil, sg("  点击或空格勾选，可多选；都不勾 = 全部 · 双击或 s 切换到该分支", fg(cFaint))))

	h := m.pickListH()
	for i := m.pickOffset; i < m.pickOffset+h && i < len(rows); i++ {
		r := rows[i]
		if r.header != "" {
			lines = append(lines, paint(m.w, nil, sg("  "+r.header, fg(cYellow).Bold(true))))
			continue
		}
		b := m.branches[r.bi]
		var bg lipgloss.TerminalColor
		if i == m.pickCursor {
			bg = cSel
		}
		box, boxSt := " ☐ ", fg(cFaint)
		if m.isPicked(b.Name) {
			box, boxSt = " ☑ ", fg(cAccent).Bold(true)
		}
		dot, nameSt := "  ", fg(cText)
		if b.Current {
			dot, nameSt = "● ", fg(cGreen).Bold(true)
		}
		if b.Remote {
			nameSt = fg(cPurple)
		}
		left := []seg{sg(" ", lipgloss.NewStyle()), sg(box, boxSt), sg(dot, fg(cGreen)), sg(b.Name, nameSt)}
		for _, mm := range trackRe.FindAllStringSubmatch(b.Track, -1) {
			n, _ := strconv.Atoi(mm[2])
			if mm[1] == "ahead" {
				left = append(left, sg(fmt.Sprintf(" %s%d", icons.up, n), fg(cGreen)))
			} else {
				left = append(left, sg(fmt.Sprintf(" %s%d", icons.down, n), fg(cYellow)))
			}
		}
		if strings.Contains(b.Track, "gone") {
			left = append(left, sg(" 上游已删除", fg(cRed)))
		}
		if b.Upstream != "" {
			left = append(left, sg("  → "+b.Upstream, fg(cFaint)))
		}
		lines = append(lines, rightAlign(m.w, bg, left, []seg{sg(b.When+" ", fg(cFaint))}))
	}
	if len(rows) == 0 {
		lines = append(lines, paint(m.w, nil, sg("  （没有分支）", fg(cMuted))))
	}
	for len(lines) < m.h-2 {
		lines = append(lines, strings.Repeat(" ", m.w))
	}
	lines = lines[:m.h-2]
	lines = append(lines, renderButtons(m.w, m.pickButtons()))
	if m.busy != "" {
		lines = append(lines, paint(m.w, nil, sgRaw(" "+m.spin.View()+" ", lipgloss.NewStyle()), sg(m.busy, fg(cAccent))))
	} else if m.msg != "" {
		col := cMuted
		if m.msgLevel == lvErr {
			col = cRed
		} else if m.msgLevel == lvOK {
			col = cGreen
		}
		lines = append(lines, paint(m.w, nil, sg(" "+m.msg, fg(col))))
	} else {
		lines = append(lines, paint(m.w, nil, sg("  Esc / 回车 返回图谱 · a 显示全部", fg(cFaint))))
	}
	return strings.Join(lines, "\n")
}
