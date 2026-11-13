package ui

import (
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/0xBB2B/gitpane/internal/gitx"
	"github.com/0xBB2B/gitpane/internal/i18n"
)

// 右键菜单：在变更页的文件 / 目录、图谱页的提交上点右键（或选中后按 m）弹出。
// 参考 vscode-git-graph：按用途分组、组间用分隔线；会改动仓库的项用和按钮一样的颜色
// （切换类橙色，放弃红色），放弃改动要在菜单里再点一次确认。

type menuStyle int

const (
	menuNormal menuStyle = iota
	menuSwitch           // 会执行 git switch：橙色
	menuView             // 只换工作区目录：蓝色
	menuDanger           // 会丢改动：红色，需要再点一次
)

type menuItem struct {
	id, label string
	arg       string // 路径 / 分支名 / 提交哈希
	style     menuStyle
	sep       bool   // 分隔线
	confirm   string // 非空时第一次点只「上膛」，显示这段文字，再点一次才执行
}

type menuState struct {
	open   bool
	x, y   int // 菜单框左上角（屏幕格子）
	items  []menuItem
	cursor int
	armed  int    // 等待确认的那一项；-1 表示没有
	row    int    // 为列表里的哪一行打开的
	key    string // 那一行是谁（文件的 key / 提交哈希）：菜单开着时自动刷新可能让行号变了
}

func (s menuState) width() int {
	w := 0
	for _, it := range s.items {
		w = max(w, textWidth(it.label), textWidth(it.confirm))
	}
	return w + 4 // 左右边框 + 各一格留白
}

func (s menuState) height() int { return len(s.items) + 2 }

// itemAt 返回屏幕坐标对应的菜单项下标；不在菜单项上时返回 -1。
func (s menuState) itemAt(x, y int) int {
	if x <= s.x || x >= s.x+s.width()-1 {
		return -1
	}
	i := y - s.y - 1
	if i < 0 || i >= len(s.items) || s.items[i].sep {
		return -1
	}
	return i
}

func (s menuState) inside(x, y int) bool {
	return x >= s.x && x < s.x+s.width() && y >= s.y && y < s.y+s.height()
}

// openMenu 为列表第 idx 行打开菜单，左上角尽量放在 (x, y)，放不下就往左 / 往上挪。
func (m *Model) openMenu(idx, x, y int) tea.Cmd {
	items := m.menuItems(idx)
	if len(items) == 0 {
		return nil
	}
	m.sel = selState{}
	s := menuState{open: true, items: items, armed: -1, row: idx, key: m.rowKey(idx)}
	if w := s.width(); x+w > m.w {
		x = max(0, m.w-w)
	}
	if h := s.height(); y+h > m.h {
		y = max(0, m.h-h)
	}
	s.x, s.y = x, y
	m.menu = s
	return nil
}

// openMenuAtCursor 在当前选中的行旁边打开菜单（按 m 或点底部「⋯ 更多」）。
func (m *Model) openMenuAtCursor() tea.Cmd {
	idx := m.cursor[m.tab]
	if idx < 0 || idx >= m.itemCount() || len(m.menuItems(idx)) == 0 {
		m.setMsg(lvInfo, i18n.T("先选中一个文件或提交"))
		return nil
	}
	g := m.geom()
	return m.openMenu(idx, g.listX+4, g.listY+idx-m.offset[m.tab]+1)
}

func (m Model) menuItems(idx int) []menuItem {
	if m.repo == nil {
		return nil
	}
	sep := menuItem{sep: true}
	switch m.tab {
	case tabChanges:
		if idx < 0 || idx >= len(m.rows) {
			return nil
		}
		r := m.rows[idx]
		if r.kind == rowEmpty || len(r.files) == 0 && r.kind != rowFile {
			return nil
		}
		stage := menuItem{id: "stage", label: i18n.T("暂存")}
		if r.sec == secStaged {
			stage.label = i18n.T("取消暂存")
		}
		items := []menuItem{stage}
		if r.kind == rowFile && r.sec == secChanges {
			lbl := i18n.T("放弃改动")
			if r.file.Untracked() {
				lbl = i18n.T("删除这个新文件")
			}
			items = append(items, menuItem{id: "discard", label: lbl, style: menuDanger, confirm: i18n.T("再点一次确认（无法撤销）")})
		}
		if r.kind == rowSection {
			return items
		}
		return append(items, sep,
			menuItem{id: "copy", label: i18n.T("复制相对路径"), arg: r.path},
			menuItem{id: "copy", label: i18n.T("复制绝对路径"), arg: filepath.Join(m.repo.Root, filepath.FromSlash(r.path))})

	case tabGraph:
		if idx < 0 || idx >= len(m.graph) || m.graph[idx].Hash == "" {
			return nil
		}
		c := m.graph[idx]
		var items []menuItem
		for _, b := range m.commitBranches(c) {
			if wi := m.branchWorktree(b); wi >= 0 {
				// 已在其他工作区检出：git 不允许检出两次，选它会切到那个工作区（不执行 git switch）
				items = append(items, menuItem{id: "branch", arg: b.Name, style: menuView,
					label: icons.wt + " " + i18n.Tf("切换到工作区 %s（%s）", filepath.Base(m.worktrees[wi].Path), b.Name)})
				continue
			}
			items = append(items, menuItem{id: "branch", arg: b.Name, style: menuSwitch,
				label: icons.swap + " " + i18n.Tf("切换到分支 %s", b.Name)})
		}
		if !(c.Head && m.status != nil && m.status.Detached) {
			items = append(items, menuItem{id: "detach", arg: c.Full, style: menuSwitch,
				label: icons.swap + " " + i18n.Tf("切换到此提交 %s（游离 HEAD）", c.Hash)})
		}
		return append(items, sep,
			menuItem{id: "tag", label: "◆ " + i18n.T("打标签…")},
			sep,
			menuItem{id: "copy", label: i18n.T("复制哈希"), arg: c.Full},
			menuItem{id: "copyMsg", label: i18n.T("复制提交信息"), arg: c.Full})
	}
	return nil
}

// commitBranches 返回提交上可以切换过去的分支：本地分支在前；远程分支只在没有同名本地分支时列出；
// 当前分支、标签、origin/HEAD 不列。
func (m Model) commitBranches(c gitx.GraphLine) []gitx.Branch {
	byName := map[string]gitx.Branch{}
	for _, b := range m.branches {
		byName[b.Name] = b
	}
	var local, remote []gitx.Branch
	have := map[string]bool{}
	for _, ref := range strings.Split(c.Refs, ", ") {
		ref = strings.TrimPrefix(strings.TrimSpace(ref), "HEAD -> ")
		if ref == "" || ref == "HEAD" || strings.HasPrefix(ref, "tag: ") || strings.HasSuffix(ref, "/HEAD") {
			continue
		}
		b, ok := byName[ref]
		if !ok {
			continue
		}
		if b.Remote {
			remote = append(remote, b)
		} else {
			have[b.Name] = true
			if !b.Current {
				local = append(local, b)
			}
		}
	}
	for _, b := range remote {
		short := b.Name
		if i := strings.IndexByte(short, '/'); i >= 0 {
			short = short[i+1:]
		}
		if have[short] {
			continue // 有同名本地分支，切远程分支其实就是切它
		}
		if cur, ok := byName[short]; ok && cur.Current {
			continue
		}
		local = append(local, b)
	}
	return local
}

func (m *Model) closeMenu() { m.menu = menuState{} }

func (m Model) rowKey(idx int) string {
	switch {
	case m.tab == tabChanges && idx >= 0 && idx < len(m.rows):
		return m.rows[idx].key()
	case m.tab == tabGraph && idx >= 0 && idx < len(m.graph):
		return m.graph[idx].Full
	}
	return ""
}

// findRow 按 key 找回菜单对应的行（先看原来的位置）；找不到返回 -1。
func (m Model) findRow(key string, hint int) int {
	if m.rowKey(hint) == key {
		return hint
	}
	n := len(m.rows)
	if m.tab == tabGraph {
		n = len(m.graph)
	}
	for i := 0; i < n; i++ {
		if m.rowKey(i) == key {
			return i
		}
	}
	return -1
}

func (m *Model) menuMove(d int) {
	n := len(m.menu.items)
	for i := 1; i <= n; i++ {
		j := ((m.menu.cursor+d*i)%n + n) % n
		if !m.menu.items[j].sep {
			m.menu.cursor = j
			return
		}
	}
}

// runMenu 执行第 i 项。需要确认的项第一次只上膛。
func (m *Model) runMenu(i int) tea.Cmd {
	if i < 0 || i >= len(m.menu.items) || m.menu.items[i].sep {
		return nil
	}
	it := m.menu.items[i]
	if it.confirm != "" && m.menu.armed != i {
		m.menu.armed, m.menu.cursor = i, i
		return nil
	}
	row := m.findRow(m.menu.key, m.menu.row)
	m.closeMenu()
	if row < 0 {
		m.setMsg(lvInfo, i18n.T("这一行已经变了，请重新右键"))
		return nil
	}
	m.cursor[m.tab] = row
	repo := m.repo
	switch it.id {
	case "stage":
		return m.toggleStage(row)
	case "discard":
		if row < len(m.rows) && m.rows[row].kind == rowFile {
			return m.discardFile(m.rows[row].file)
		}
	case "copy":
		return copyAs(it.arg, it.arg)
	case "copyMsg":
		return func() tea.Msg {
			msg, err := repo.CommitMessage(it.arg)
			if err != nil {
				return copiedMsg{err: err}
			}
			return writeClipboard(msg, i18n.T("提交信息"))
		}
	case "tag":
		return m.beginTag()
	case "branch":
		for _, b := range m.branches {
			if b.Name == it.arg {
				return m.switchBranch(b)
			}
		}
	case "detach":
		short := it.arg
		if len(short) > 7 {
			short = short[:7]
		}
		return m.startOp(i18n.T("切换到提交…"), func() (string, error) {
			return i18n.Tf("已切换到提交 %s（游离 HEAD）", short), repo.SwitchDetach(it.arg)
		})
	}
	return nil
}

func (m Model) handleMenuKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc", "q", "m":
		m.closeMenu()
	case "up", "k", "shift+tab":
		m.menuMove(-1)
		m.menu.armed = -1
	case "down", "j", "tab":
		m.menuMove(1)
		m.menu.armed = -1
	case "enter", " ":
		return m, m.runMenu(m.menu.cursor)
	case "ctrl+c":
		m.closeMenu()
		return m.handleKey(k)
	}
	return m, nil
}

func (m Model) handleMenuMouse(ev tea.MouseMsg) (tea.Model, tea.Cmd) {
	if ev.Action != tea.MouseActionPress {
		return m, nil
	}
	switch ev.Button {
	case tea.MouseButtonLeft:
		if i := m.menu.itemAt(ev.X, ev.Y); i >= 0 {
			return m, m.runMenu(i)
		}
		if !m.menu.inside(ev.X, ev.Y) {
			m.closeMenu() // 点菜单外面只关菜单，不触发底下的东西
		}
		return m, nil
	case tea.MouseButtonRight:
		if m.menu.inside(ev.X, ev.Y) {
			return m, nil
		}
		m.closeMenu() // 在别处右键：换成那里的菜单
		return m.handleMouse(ev)
	}
	m.closeMenu() // 滚轮等
	return m, nil
}

// overlayMenu 把菜单框画在主界面上。
func (m Model) overlayMenu(lines []string) {
	s := m.menu
	w := s.width()
	box := lipgloss.NewStyle().Background(cSurface)
	edge := styleOpen(box.Foreground(cGuide))
	put := func(y int, content string) {
		if y < 0 || y >= len(lines) {
			return
		}
		lines[y] = spliceCells(lines[y], s.x, w, m.w, content)
	}
	inner := w - 2
	put(s.y, edge+"╭"+strings.Repeat("─", inner)+"╮")
	for i, it := range s.items {
		if it.sep {
			put(s.y+1+i, edge+"├"+strings.Repeat("─", inner)+"┤")
			continue
		}
		label := it.label
		st := box.Foreground(cText)
		switch it.style {
		case menuSwitch:
			st = box.Foreground(cOrange)
		case menuView:
			st = box.Foreground(cAccent)
		case menuDanger:
			st = box.Foreground(cRed)
		}
		if i == s.armed {
			label, st = it.confirm, box.Foreground(cRed).Bold(true)
		}
		if i == s.cursor {
			st = st.Background(cSel).Bold(true)
		}
		pad := inner - 2 - textWidth(label)
		put(s.y+1+i, edge+"│"+styleOpen(st)+" "+label+strings.Repeat(" ", max(0, pad))+" "+sgrReset+edge+"│")
	}
	put(s.y+s.height()-1, edge+"╰"+strings.Repeat("─", inner)+"╯")
}

// spliceCells 把一行里第 [x, x+w) 格换成 content（content 正好 w 格宽），整行保持 total 格宽。
// 边界切在两格宽的字（中文等）中间时，被切开的字换成空格，不能让整行变宽，否则后面的内容会错位。
func spliceCells(line string, x, w, total int, content string) string {
	left := ansi.Truncate(line, x, "") // Truncate 放不下的宽字会整个丢掉，不会超出 x 格
	if lw := textWidth(ansi.Strip(left)); lw < x {
		left += sgrReset + strings.Repeat(" ", x-lw)
	}
	want := max(0, total-x-w)
	right := ansi.TruncateLeft(line, x+w, "")
	if textWidth(ansi.Strip(right)) > want {
		// 切在宽字中间时 TruncateLeft 会把整个字留下：从下一格开始取，把这个字去掉，后面补空格
		right = ansi.TruncateLeft(line, x+w+1, "")
	}
	if rw := textWidth(ansi.Strip(right)); rw < want {
		right = strings.Repeat(" ", want-rw) + right
	}
	return left + sgrReset + content + sgrReset + right
}
