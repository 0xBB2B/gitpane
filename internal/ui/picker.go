package ui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xBB2B/gitpane/internal/gitx"
	"github.com/0xBB2B/gitpane/internal/i18n"
)

// 查看分支面板：只能点图谱页标签栏右侧的「⊚ 查看：全部分支 ▾」打开（蓝色）。
// 勾选一个或多个分支后，图谱只显示这些分支；一个都不勾 = 显示全部。
// 这个面板只负责筛选，不切换分支。
//
// 切换模式（任意页按 b，或点底部橙色的「⇄ 切换分支」）：不显示勾选框，单击就切换；
// 按 / 输入关键字筛选；已在其他工作树检出的分支，选它就切换到那个工作树。

type pickRow struct {
	header string // 非空表示分组标题行
	bi     int    // m.branches 下标
}

func (m Model) pickRows() []pickRow {
	var rows []pickRow
	lastRemote := -1
	filter := strings.ToLower(m.pickFilter)
	for i, b := range m.branches {
		if m.pickSwitchMode && filter != "" && !strings.Contains(strings.ToLower(b.Name), filter) {
			continue
		}
		r := 0
		if b.Remote {
			r = 1
		}
		if r != lastRemote {
			rows = append(rows, pickRow{header: map[int]string{0: i18n.T("本地分支"), 1: i18n.T("远程分支")}[r]})
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
	m.pickSwitchMode, m.pickFilter, m.pickFiltering = false, "", false
	m.pickCursorToCurrent()
	m.pickClamp()
	return cmd
}

// openSwitchPicker 打开「切换分支」面板（任意页都能用）。
func (m *Model) openSwitchPicker() tea.Cmd {
	if m.repo == nil {
		return nil
	}
	m.picking = true
	m.pickSwitchMode, m.pickFilter, m.pickFiltering = true, "", false
	m.pickCursor, m.pickOffset = 0, 0
	m.pickAtCurrent = true // 分支列表可能还没加载（变更页不加载），到了再定位
	m.pickCursorToCurrent()
	m.pickClamp()
	return branchesCmd(m.repo)
}

func (m *Model) pickCursorToCurrent() {
	for i, r := range m.pickRows() {
		if r.header == "" && r.bi < len(m.branches) && m.branches[r.bi].Current {
			m.pickCursor = i
		}
	}
}

// branchWorktree 返回检出了这个分支的「其他」工作树的下标；没有时返回 -1。
// 远程分支按同名的本地分支算（切换远程分支其实是切到同名本地分支）。
func (m Model) branchWorktree(b gitx.Branch) int {
	name := b.Name
	if b.Remote {
		if i := strings.IndexByte(name, '/'); i >= 0 {
			name = name[i+1:]
		}
	}
	cur := m.currentWorktree()
	for i, w := range m.worktrees {
		if i != cur && w.Branch == name {
			return i
		}
	}
	return -1
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
		m.setMsg(lvInfo, i18n.Tf("已经在 %s 上", b.Name))
		return nil
	}
	// 这个分支已经在另一个工作区里检出了（git 不允许同时检出两次）：直接切到那个工作区，不执行 git switch
	if wi := m.branchWorktree(b); wi >= 0 {
		m.picking = false
		w := m.worktrees[wi]
		return m.openWorktree(w, i18n.Tf("%s 已在工作区 %s 检出，已切到那个工作区（没有执行 git switch）", b.Name, filepath.Base(w.Path)))
	}
	repo := m.repo
	if m.pickSwitchMode {
		m.picking = false // 切换模式：选完就回到原来的页面
	}
	return m.startOp(i18n.T("切换分支…"), func() (string, error) { return i18n.Tf("已切换到 %s", b.Name), repo.Switch(b) })
}

func (m Model) handlePickKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.pickSwitchMode {
		return m.handleSwitchPickKey(k)
	}
	rows := m.pickRows()
	switch k.String() {
	case "esc", "enter", "q":
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
	}
	return m, nil
}

// handleSwitchPickKey 是切换模式下的按键：回车切换，/ 筛选。
func (m Model) handleSwitchPickKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.pickFiltering {
		switch k.Type {
		case tea.KeyEsc:
			m.pickFiltering, m.pickFilter = false, ""
		case tea.KeyEnter:
			m.pickFiltering = false
			m.pickFirstBranch()
			return m, m.pickSwitch()
		case tea.KeyBackspace:
			if r := []rune(m.pickFilter); len(r) > 0 {
				m.pickFilter = string(r[:len(r)-1])
			}
		case tea.KeyDown:
			m.pickMove(1)
			return m, nil
		case tea.KeyUp:
			m.pickMove(-1)
			return m, nil
		case tea.KeyRunes, tea.KeySpace:
			m.pickFilter += string(k.Runes)
			if k.Type == tea.KeySpace {
				m.pickFilter += " "
			}
		}
		m.pickFirstBranch()
		return m, nil
	}
	rows := m.pickRows()
	switch k.String() {
	case "esc", "q", "b":
		m.picking = false
	case "ctrl+c":
		m.picking = false
		if !m.opts.Embed {
			return m, tea.Quit
		}
	case "/":
		m.pickFiltering = true
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
	case "enter", " ", "s", "l":
		return m, m.pickSwitch()
	}
	return m, nil
}

// pickFirstBranch 把光标放到筛选结果的第一个分支上（跳过分组标题）。
func (m *Model) pickFirstBranch() {
	m.pickCursor, m.pickOffset = 0, 0
	for i, r := range m.pickRows() {
		if r.header == "" {
			m.pickCursor = i
			break
		}
	}
	m.pickClamp()
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
				case "pickDone", "pickBack":
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
	if m.pickSwitchMode { // 切换模式：单击就切换
		m.pickCursor = i
		m.pickClamp()
		return m, m.pickSwitch()
	}
	m.pickCursor = i
	m.pickClamp()
	return m, m.togglePick(m.branches[rows[i].bi].Name)
}

func (m Model) pickButtons() []button {
	if m.pickSwitchMode {
		return place(1, m.w, []button{
			{id: "pickSwitch", label: icons.swap + " " + i18n.T("切换到此分支"), kind: btnSwitch},
			{id: "pickBack", label: i18n.T("返回")},
		})
	}
	return place(1, m.w, []button{
		{id: "pickDone", label: icons.check + i18n.T(" 完成"), kind: btnPrimary},
		{id: "pickAll", label: i18n.T("显示全部")},
	})
}

// filterLabel 是标签栏右侧筛选按钮上的文字。
func (m Model) filterLabel() string {
	switch len(m.graphRefs) {
	case 0:
		return icons.view + " " + i18n.Tf("查看：%s", i18n.T("全部分支")) + " ▾"
	case 1:
		return icons.view + " " + i18n.Tf("查看：%s", m.graphRefs[0]) + " ▾"
	}
	return icons.view + " " + i18n.Tf("查看：%s", fmt.Sprintf("%s +%d", m.graphRefs[0], len(m.graphRefs)-1)) + " ▾"
}

func (m Model) viewPicker() string {
	rows := m.pickRows()
	lines := make([]string, 0, m.h)

	if m.pickSwitchMode {
		cur := ""
		if m.status != nil {
			cur = i18n.Tf("当前：%s", m.status.Branch)
		}
		hint := sg(i18n.T("  点击或回车切换 · / 筛选 · Esc 返回"), fg(cFaint))
		if m.pickFiltering || m.pickFilter != "" {
			hint = sg("  / "+m.pickFilter, fg(cText))
			if m.pickFiltering {
				hint = sg("  / "+m.pickFilter+"▏", fg(cAccent))
			}
		}
		lines = append(lines,
			rightAlign(m.w, cBar,
				[]seg{sg(" "+icons.swap+" "+i18n.T("切换分支")+" ", btnStyle(btnSwitch)),
					sg("  "+i18n.T("会执行 git switch，工作区会换成所选分支"), fg(cOrange))},
				[]seg{sg(cur+" ", fg(cGreen))}),
			paint(m.w, nil, hint))
	} else {
		sel := i18n.T("显示全部分支")
		if n := len(m.graphRefs); n > 0 {
			sel = i18n.Tf("已选 %d 个分支", n)
		}
		lines = append(lines,
			rightAlign(m.w, cBar,
				[]seg{sg(" "+icons.view+" "+i18n.T("查看分支")+" ", btnStyle(btnPrimary)),
					sg("  "+i18n.T("只改变图谱显示哪些分支，不会切换分支"), fg(cAccent))},
				[]seg{sg(sel+" ", fg(cYellow))}),
			paint(m.w, nil, sg(i18n.T("  点击或空格勾选，可多选；都不勾 = 全部"), fg(cFaint))))
	}

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
		// ☐ ☑ 在不少字体里只有半个字大，用方括号更清楚
		box, boxSt := " [ ] ", fg(cFaint)
		if m.isPicked(b.Name) {
			box, boxSt = " ["+icons.check+"] ", fg(cAccent).Bold(true)
		}
		dot, nameSt := "  ", fg(cText)
		if b.Current {
			dot, nameSt = "● ", fg(cGreen).Bold(true)
		}
		if b.Remote {
			nameSt = fg(cPurple)
		}
		left := []seg{sg(" ", lipgloss.NewStyle()), sg(box, boxSt), sg(dot, fg(cGreen)), sg(b.Name, nameSt)}
		if m.pickSwitchMode {
			left = []seg{sg("  ", lipgloss.NewStyle()), sg(dot, fg(cGreen)), sg(b.Name, nameSt)}
		}
		for _, mm := range trackRe.FindAllStringSubmatch(b.Track, -1) {
			n, _ := strconv.Atoi(mm[2])
			if mm[1] == "ahead" {
				left = append(left, sg(fmt.Sprintf(" %s%d", icons.up, n), fg(cGreen)))
			} else {
				left = append(left, sg(fmt.Sprintf(" %s%d", icons.down, n), fg(cYellow)))
			}
		}
		if strings.Contains(b.Track, "gone") {
			left = append(left, sg(i18n.T(" 上游已删除"), fg(cRed)))
		}
		if b.Upstream != "" {
			left = append(left, sg("  → "+b.Upstream, fg(cFaint)))
		}
		right := []seg{sg(b.When+" ", fg(cFaint))}
		if wi := m.branchWorktree(b); wi >= 0 {
			// 已在其他工作树检出：选它会切到那个工作树
			right = append([]seg{sg(i18n.Tf("工作区 %s", filepath.Base(filepath.FromSlash(m.worktrees[wi].Path)))+"  ", fg(cCyan))}, right...)
		}
		lines = append(lines, rightAlign(m.w, bg, left, right))
	}
	if len(rows) == 0 {
		empty := i18n.T("  （没有分支）")
		if m.pickFilter != "" {
			empty = i18n.Tf("  没有包含「%s」的分支", m.pickFilter)
		}
		lines = append(lines, paint(m.w, nil, sg(empty, fg(cMuted))))
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
		tip := i18n.T("  Esc / 回车 返回图谱 · a 显示全部")
		if m.pickSwitchMode {
			tip = i18n.T("  有未提交的改动也可以切换（git 会带过去）；冲突时会提示先提交或暂存")
		}
		lines = append(lines, paint(m.w, nil, sg(tip, fg(cFaint))))
	}
	return strings.Join(lines, "\n")
}
