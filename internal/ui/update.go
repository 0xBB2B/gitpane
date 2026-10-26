package ui

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xBB2B/gitpane/internal/gitx"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.input.Width = max(10, m.w-14)
		m.clampCursor()
		return m, nil

	case tickMsg:
		if m.msg != "" && m.msgLevel != lvErr && time.Since(m.msgAt) > 6*time.Second {
			m.msg = ""
		}
		return m, tea.Batch(m.refreshCmd(), tick())

	case repoMsg:
		if msg.repo != nil {
			m.repo = msg.repo
			return m, m.refreshCmd()
		}
		return m, nil

	case statusMsg:
		if msg.err != nil {
			m.statusErr = gitx.Describe(msg.err)
			return m, nil
		}
		m.statusErr = ""
		m.stats = msg.stats
		m.applyStatus(msg.st)
		return m, m.syncDetail()

	case graphMsg:
		if msg.refs != strings.Join(m.graphRefs, "\x00") || msg.limit != m.graphLimit {
			return m, nil // 筛选或加载量已经变了，这是过期结果
		}
		if msg.err == nil {
			m.graph = msg.lines
		} else {
			m.graph = nil
		}
		if m.tab == tabGraph {
			m.clampCursor()
			return m, m.syncDetail()
		}
		return m, nil

	case branchesMsg:
		if msg.err == nil {
			m.branches = msg.bs
			// 已删除的分支从筛选里去掉
			var keep []string
			for _, r := range m.graphRefs {
				for _, b := range m.branches {
					if b.Name == r {
						keep = append(keep, r)
						break
					}
				}
			}
			if len(keep) != len(m.graphRefs) {
				return m, m.setGraphRefs(keep)
			}
		}
		if m.picking {
			m.pickClamp()
		}
		return m, nil

	case detailMsg:
		if msg.view.key != m.detailKey {
			return m, nil // 过期结果
		}
		m.detail = msg.view
		m.clampOffset()
		return m, nil

	case spinner.TickMsg:
		if m.busy == "" {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case opDoneMsg:
		m.busy = ""
		// 图标由状态栏按级别加，这里只放文字
		if msg.err != nil {
			m.setMsg(lvErr, gitx.Describe(msg.err))
		} else if msg.ok != "" {
			m.setMsg(lvOK, msg.ok)
		}
		return m, m.refreshCmd()

	case tea.MouseMsg:
		if m.showHelp {
			if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
				m.showHelp = false
			}
			return m, nil
		}
		if m.picking {
			return m.handlePickMouse(msg)
		}
		return m.handleMouse(msg)

	case tea.KeyMsg:
		if m.showHelp {
			m.showHelp = false
			if msg.String() == "ctrl+c" && !m.opts.Embed {
				return m, tea.Quit
			}
			return m, nil
		}
		if m.committing {
			return m.handleCommitKey(msg)
		}
		if m.picking {
			return m.handlePickKey(msg)
		}
		return m.handleKey(msg)
	}
	return m, nil
}

// applyStatus 更新状态并尽量保持光标停在原来那一行。
func (m *Model) applyStatus(st *gitx.Status) {
	var prevKey string
	if r, ok := m.currentRowAny(); ok {
		prevKey = r.key()
	}
	first := m.status == nil
	m.status = st
	m.rows = buildRows(st, m.collapsed)
	if first {
		// 第一次加载时直接选中第一个文件，下方立刻有 diff 可看
		for i, r := range m.rows {
			if r.kind == rowFile {
				m.cursor[tabChanges] = i
				break
			}
		}
		defer m.ensureVisible()
	} else if prevKey != "" {
		found := false
		for i, r := range m.rows {
			if r.key() == prevKey {
				m.cursor[tabChanges] = i
				found = true
				break
			}
		}
		// 原来那行没了（比如刚被暂存走）：落到附近的文件上，详情区才有东西看
		if !found {
			m.cursor[tabChanges] = nearestFile(m.rows, m.cursor[tabChanges])
			defer m.ensureVisible()
		}
	}
	if m.tab == tabChanges {
		m.clampCursor()
	} else if m.cursor[tabChanges] >= len(m.rows) {
		m.cursor[tabChanges] = max(0, len(m.rows)-1)
	}
}

// nearestFile 返回 i 处或其后最近的文件行，后面没有就往前找。
func nearestFile(rows []row, i int) int {
	i = max(0, min(i, len(rows)-1))
	for j := i; j < len(rows); j++ {
		if rows[j].kind == rowFile {
			return j
		}
	}
	for j := i - 1; j >= 0; j-- {
		if rows[j].kind == rowFile {
			return j
		}
	}
	return i
}

func (m Model) currentRowAny() (row, bool) {
	i := m.cursor[tabChanges]
	if i < 0 || i >= len(m.rows) {
		return row{}, false
	}
	return m.rows[i], true
}

func (m *Model) switchTab(t tab) tea.Cmd {
	if m.tab == t {
		return nil
	}
	m.tab = t
	m.confirmKey = ""
	m.clampCursor()
	m.ensureVisible()
	return tea.Batch(m.refreshCmd(), m.syncDetail())
}

// ---------------------------------------------------------------- 键盘

func (m Model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	g := m.geom()
	cur := m.cursor[m.tab]
	switch k.String() {
	case "ctrl+c":
		if !m.opts.Embed || time.Since(m.quitArmedAt) < time.Second {
			return m, tea.Quit
		}
		m.quitArmedAt = time.Now()
		m.setMsg(lvInfo, "再按一次 Ctrl+C 关闭面板")
		return m, nil
	case "q":
		if !m.opts.Embed {
			return m, tea.Quit
		}
		m.setMsg(lvInfo, "面板模式下 q 不退出；要关闭请按两次 Ctrl+C")
		return m, nil
	case "?":
		m.showHelp = true
		return m, nil
	case "tab", "shift+tab":
		return m, m.switchTab((m.tab + 1) % numTabs)
	case "1":
		return m, m.switchTab(tabChanges)
	case "2":
		return m, m.switchTab(tabGraph)
	case "b":
		return m, m.openPicker()
	case "j", "down":
		return m, m.moveCursor(cur + 1)
	case "k", "up":
		return m, m.moveCursor(cur - 1)
	case "g", "home":
		return m, m.moveCursor(0)
	case "G", "end":
		return m, m.moveCursor(m.itemCount() - 1)
	case "pgdown":
		return m, m.moveCursor(cur + g.listH)
	case "pgup":
		return m, m.moveCursor(cur - g.listH)
	case "J", "ctrl+d":
		m.detailOff += max(1, g.detailH/2)
		m.clampOffset()
		return m, nil
	case "K", "ctrl+u":
		m.detailOff -= max(1, g.detailH/2)
		m.clampOffset()
		return m, nil
	case "r":
		m.setMsg(lvInfo, "已刷新")
		return m, m.refreshCmd()
	case "c":
		return m, m.beginCommit()
	case "p":
		return m, m.doAction("pull")
	case "P":
		return m, m.doAction("push")
	case "f":
		return m, m.doAction("fetch")
	case "a":
		return m, m.doAction("stageAll")
	case "u":
		return m, m.doAction("unstageAll")
	case "x":
		return m, m.doAction("discard")
	case " ":
		if m.tab == tabChanges {
			return m, m.toggleStage(cur)
		}
	case "h", "left":
		if m.tab == tabChanges {
			return m, m.collapseOrParent()
		}
	case "l", "right":
		if m.tab == tabChanges {
			return m, m.expandOrChild()
		}
	case "z":
		if m.tab == tabChanges {
			m.toggleAllFolders()
		} else {
			m.toggleAllFiles()
		}
		return m, nil
	case "enter":
		if m.tab == tabChanges {
			if r, ok := m.currentRow(); ok && r.kind != rowFile {
				m.toggleCollapse(r)
			}
		}
	}
	return m, nil
}

func (m Model) handleCommitKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc", "ctrl+c":
		m.committing = false
		m.input.Blur()
		return m, nil
	case "enter":
		return m, m.doAction("commitConfirm")
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(k)
	return m, cmd
}

// ---------------------------------------------------------------- 鼠标

func (m Model) handleMouse(ev tea.MouseMsg) (tea.Model, tea.Cmd) {
	g := m.geom()
	inList := g.inList(ev.X, ev.Y)
	inDetail := g.inDetail(ev.X, ev.Y)

	switch ev.Button {
	case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown:
		d := 3
		if ev.Button == tea.MouseButtonWheelUp {
			d = -3
		}
		if inList {
			m.offset[m.tab] += d
			m.clampOffset()
			return m, m.maybeLoadMore()
		}
		m.detailOff += d
		m.clampOffset()
		return m, nil
	}
	if ev.Action != tea.MouseActionPress || ev.Button != tea.MouseButtonLeft {
		return m, nil
	}

	switch {
	case m.repo == nil:
		if ev.Y == g.listY+2 {
			return m, m.doAction("init")
		}
		return m, nil

	case ev.Y == g.headerY:
		return m, m.openPicker()

	case ev.Y == g.tabsY:
		for _, b := range m.tabRightButtons() {
			if b.hit(ev.X) {
				return m, m.doAction(b.id)
			}
		}
		for _, b := range m.tabButtons() {
			if b.hit(ev.X) {
				return m, m.switchTab(tab(b.id[0] - '0'))
			}
		}

	case inList:
		idx := m.offset[m.tab] + ev.Y - g.listY
		if idx >= m.itemCount() {
			return m, nil
		}
		return m, m.clickRow(idx, ev.X-g.listX)

	case inDetail:
		if ev.Y == g.detailHdrY {
			if b, ok := m.detailFoldButton(g); ok && b.hit(ev.X) {
				m.toggleAllFiles()
			}
			return m, nil
		}
		vis := m.visibleDetail()
		if i := m.detailOff + ev.Y - g.detailY; i >= 0 && i < len(vis) && vis[i].kind == dFile {
			m.toggleFileFold(vis[i].path)
		}
		return m, nil

	case ev.Y == g.buttonsY:
		for _, b := range m.bottomButtons() {
			if b.hit(ev.X) {
				return m, m.doAction(b.id)
			}
		}
	}
	return m, nil
}

func (m *Model) clickRow(idx, x int) tea.Cmd {
	double := idx == m.lastClickRow && time.Since(m.lastClickAt) < 400*time.Millisecond
	m.lastClickRow, m.lastClickAt = idx, time.Now()

	switch m.tab {
	case tabChanges:
		r := m.rows[idx]
		bx := rowButtonX(r, m.geom().listW)
		if bx >= 0 && x >= bx {
			m.cursor[tabChanges] = idx
			return m.toggleStage(idx)
		}
		if r.kind == rowSection || r.kind == rowDir {
			m.cursor[tabChanges] = idx
			m.toggleCollapse(r)
			return nil
		}
		return m.moveCursor(idx)
	default:
		_ = double
		return m.moveCursor(idx)
	}
}

func (m *Model) toggleCollapse(r row) {
	if r.kind == rowFile || r.kind == rowEmpty {
		return
	}
	m.collapsed[r.key()] = !m.collapsed[r.key()]
	if m.status != nil {
		m.applyStatus(m.status)
	}
}

// setCollapsed 设置某个目录/分组的折叠状态，并让光标留在这一行上。
func (m *Model) setCollapsed(r row, v bool) {
	if v {
		m.collapsed[r.key()] = true
	} else {
		delete(m.collapsed, r.key())
	}
	if m.status == nil {
		return
	}
	m.rows = buildRows(m.status, m.collapsed)
	for i, x := range m.rows {
		if x.key() == r.key() && x.kind == r.kind {
			m.cursor[tabChanges] = i
			break
		}
	}
	m.clampCursor()
	m.ensureVisible()
}

// parentRow 返回第 i 行所属的目录或分组行。
func (m Model) parentRow(i int) int {
	r := m.rows[i]
	for j := i - 1; j >= 0; j-- {
		p := m.rows[j]
		if p.sec != r.sec {
			break
		}
		if (p.kind == rowDir && p.depth < r.depth) || p.kind == rowSection {
			return j
		}
	}
	return -1
}

// collapseOrParent：在展开的目录/分组上折叠它；否则跳到上一级目录（像文件管理器的 ←）。
func (m *Model) collapseOrParent() tea.Cmd {
	i := m.cursor[tabChanges]
	if i < 0 || i >= len(m.rows) {
		return nil
	}
	r := m.rows[i]
	if (r.kind == rowDir || r.kind == rowSection) && !r.collapsed {
		m.setCollapsed(r, true)
		return m.syncDetail()
	}
	if p := m.parentRow(i); p >= 0 {
		return m.moveCursor(p)
	}
	return nil
}

// expandOrChild：在折叠的目录/分组上展开它；已展开就进入第一个子项（像 →）。
func (m *Model) expandOrChild() tea.Cmd {
	i := m.cursor[tabChanges]
	if i < 0 || i >= len(m.rows) {
		return nil
	}
	r := m.rows[i]
	switch {
	case (r.kind == rowDir || r.kind == rowSection) && r.collapsed:
		m.setCollapsed(r, false)
		return m.syncDetail()
	case r.kind == rowDir || r.kind == rowSection:
		if i+1 < len(m.rows) && m.rows[i+1].sec == r.sec && m.rows[i+1].kind != rowSection {
			return m.moveCursor(i + 1)
		}
	}
	return nil
}

// toggleAllFolders 有展开的目录就全部折叠，否则全部展开（分组标题的折叠状态不动）。
func (m *Model) toggleAllFolders() {
	if m.status == nil {
		return
	}
	anyOpen := false
	for _, r := range m.rows {
		if r.kind == rowDir && !r.collapsed {
			anyOpen = true
			break
		}
	}
	// 操作后光标停在原来那行；它被折进去了就停在包含它的那个目录上
	anchor, hasAnchor := m.currentRow()

	if anyOpen {
		for _, f := range m.status.Files {
			dir := path.Dir(f.Path)
			for dir != "." && dir != "/" {
				for _, sec := range []section{secConflict, secStaged, secChanges} {
					m.collapsed[sec.title()+"\x00"+dir] = true
				}
				dir = path.Dir(dir)
			}
		}
		m.setMsg(lvInfo, "已折叠全部文件夹（z 展开）")
	} else {
		for k := range m.collapsed {
			if !strings.HasSuffix(k, "\x00") { // 分组标题的 key 以空路径结尾
				delete(m.collapsed, k)
			}
		}
		m.setMsg(lvInfo, "已展开全部文件夹")
	}
	m.rows = buildRows(m.status, m.collapsed)
	if hasAnchor && anchor.kind != rowSection {
		for i, r := range m.rows {
			if r.sec == anchor.sec && r.kind != rowSection &&
				(r.path == anchor.path || strings.HasPrefix(anchor.path, r.path+"/")) {
				m.cursor[tabChanges] = i
				break
			}
		}
	}
	m.clampCursor()
}

// toggleStage 对一行执行暂存或取消暂存（目录/分组为批量操作）。
func (m *Model) toggleStage(idx int) tea.Cmd {
	if idx < 0 || idx >= len(m.rows) || m.repo == nil {
		return nil
	}
	r := m.rows[idx]
	if len(r.files) == 0 {
		return nil
	}
	repo := m.repo
	files := r.files
	label := r.name
	if r.kind == rowSection {
		label = fmt.Sprintf("%d 个文件", len(files))
	}
	if r.sec == secStaged {
		return m.startOp("取消暂存", func() (string, error) {
			return "已取消暂存 " + label, repo.Unstage(files...)
		})
	}
	return m.startOp("暂存", func() (string, error) {
		return "已暂存 " + label, repo.Stage(files...)
	})
}

// ---------------------------------------------------------------- 操作

func (m *Model) beginCommit() tea.Cmd {
	if m.repo == nil {
		return nil
	}
	if m.stagedCount() == 0 {
		if m.changeCount() == 0 {
			m.setMsg(lvInfo, "工作区是干净的，没有可提交的内容")
		} else {
			m.setMsg(lvErr, "还没有暂存文件：点 + 暂存，或按 a 全部暂存")
		}
		return nil
	}
	m.committing = true
	m.input.SetValue("")
	m.input.Width = max(10, m.w-14)
	return m.input.Focus()
}

func (m *Model) doAction(id string) tea.Cmd {
	if id == "init" {
		dir := m.dir
		return m.startOp("初始化", func() (string, error) {
			if err := gitx.Init(dir); err != nil {
				return "", err
			}
			return "已初始化仓库", nil
		})
	}
	if id == "repoRetry" || m.repo == nil {
		return m.refreshCmd()
	}
	repo := m.repo
	st := m.status
	switch id {
	case "refresh":
		m.setMsg(lvInfo, "已刷新")
		return m.refreshCmd()
	case "commit":
		return m.beginCommit()
	case "commitConfirm":
		text := strings.TrimSpace(m.input.Value())
		if text == "" {
			m.setMsg(lvErr, "提交信息不能为空")
			return nil
		}
		m.committing = false
		m.input.Blur()
		return m.startOp("提交中…", func() (string, error) {
			s, err := repo.Commit(text)
			return "已提交 " + s, err
		})
	case "commitCancel":
		m.committing = false
		m.input.Blur()
		return nil
	case "stageAll":
		return m.startOp("暂存中…", func() (string, error) { return "已全部暂存", repo.StageAll() })
	case "unstageAll":
		return m.startOp("取消暂存中…", func() (string, error) { return "已全部取消暂存", repo.UnstageAll() })
	case "pull":
		return m.startOp("拉取中…", func() (string, error) {
			out, err := repo.Pull()
			if out == "" {
				out = "拉取完成"
			}
			return out, err
		})
	case "push":
		return m.startOp("推送中…", func() (string, error) { return "推送完成", repo.Push(st) })
	case "fetch":
		return m.startOp("抓取中…", func() (string, error) { return "抓取完成", repo.Fetch() })
	case "foldAll":
		m.toggleAllFolders()
		return m.syncDetail()
	case "filter", "picker":
		return m.openPicker()
	case "filterClear":
		return m.setGraphRefs(nil)
	case "discard":
		r, ok := m.currentRow()
		if !ok || r.kind != rowFile || r.sec != secChanges {
			m.setMsg(lvInfo, "选中「变更」下的文件后才能放弃改动")
			return nil
		}
		key := r.key()
		if m.confirmKey != key || time.Since(m.confirmAt) > 4*time.Second {
			m.confirmKey, m.confirmAt = key, time.Now()
			what := "改动"
			if r.file.Untracked() {
				what = "新文件（会被删除）"
			}
			m.setMsg(lvErr, "再按一次 x / 点「放弃」确认丢弃 "+r.name+" 的"+what)
			return nil
		}
		m.confirmKey = ""
		f := r.file
		return m.startOp("放弃改动…", func() (string, error) { return "已放弃 " + f.Path, repo.Discard(f) })
	}
	return nil
}
