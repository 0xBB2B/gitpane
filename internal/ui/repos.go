package ui

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xBB2B/gitpane/internal/gitx"
	"github.com/0xBB2B/gitpane/internal/i18n"
)

// 子仓库探测：启动目录不是 Git 仓库时，在 Options.Depth 层子目录里找仓库，
// 找到一个就直接打开，找到多个就弹出仓库选择面板；之后随时可以按 o 或点右上角切换。

type repoEntry struct {
	path   string
	rel    string
	sum    gitx.Summary
	loaded bool
}

type scanMsg struct {
	repos     []string
	truncated bool
}

// summaryMsg 是一批仓库概要；gen 用来丢弃重新扫描之前发出的过期结果。
type summaryMsg struct {
	gen   int
	start int
	sums  []gitx.Summary
}

const summaryBatch = 8

func scanCmd(root string, depth int) tea.Cmd {
	return func() tea.Msg {
		repos, trunc := gitx.FindRepos(root, depth)
		return scanMsg{repos, trunc}
	}
}

// summaryCmd 并发读取从 start 开始的一批仓库概要，读完再由 Update 发起下一批。
func (m Model) summaryCmd(start int) tea.Cmd {
	if start >= len(m.repos) {
		return nil
	}
	end := min(len(m.repos), start+summaryBatch)
	paths := make([]string, 0, end-start)
	for _, r := range m.repos[start:end] {
		paths = append(paths, r.path)
	}
	gen := m.scanGen
	return func() tea.Msg {
		sums := make([]gitx.Summary, len(paths))
		var wg sync.WaitGroup
		for i, p := range paths {
			wg.Add(1)
			go func(i int, p string) {
				defer wg.Done()
				sums[i] = gitx.Summarize(p)
			}(i, p)
		}
		wg.Wait()
		return summaryMsg{gen: gen, start: start, sums: sums}
	}
}

func (m *Model) applyScan(msg scanMsg) tea.Cmd {
	m.scanning = false
	m.scanTrunc = msg.truncated
	m.scanGen++
	m.repos = m.repos[:0]
	for _, p := range msg.repos {
		m.repos = append(m.repos, repoEntry{path: p, rel: gitx.Rel(m.workspace, p)})
	}
	m.repoCursor, m.repoOffset = 0, 0
	m.repoClamp()
	switch {
	case m.repo == nil && len(m.repos) == 1:
		return tea.Batch(m.switchRepo(0), m.summaryCmd(0))
	case m.repo == nil && len(m.repos) > 1:
		m.repoPicking = true
	}
	return m.summaryCmd(0)
}

func (m *Model) applySummary(msg summaryMsg) tea.Cmd {
	if msg.gen != m.scanGen {
		return nil
	}
	for i, s := range msg.sums {
		if j := msg.start + i; j < len(m.repos) {
			m.repos[j].sum, m.repos[j].loaded = s, true
		}
	}
	return m.summaryCmd(msg.start + len(msg.sums))
}

// switchRepo 打开第 i 个仓库，清空和上一个仓库有关的状态。
func (m *Model) switchRepo(i int) tea.Cmd {
	if i < 0 || i >= len(m.repos) {
		return nil
	}
	if m.busy != "" {
		m.setMsg(lvErr, i18n.Tf("上一个操作还在进行：%s", m.busy))
		return nil
	}
	r, err := gitx.Open(m.repos[i].path)
	if err != nil {
		m.setMsg(lvErr, gitx.Describe(err))
		return nil
	}
	m.repoPicking, m.repoFiltering = false, false
	if m.repo != nil && m.repo.Root == r.Root {
		return nil
	}
	m.repo = r
	m.status, m.statusErr, m.rows = nil, "", nil
	m.stats = [3]map[string]gitx.Stat{}
	m.graph, m.graphLimit, m.graphRefs, m.branches = nil, graphPage, nil, nil
	m.cursor, m.offset = [3]int{}, [3]int{}
	m.detail, m.detailKey, m.detailOff = detailView{}, "", 0
	m.collapsed, m.fileFold = map[string]bool{}, map[string]bool{}
	m.committing, m.picking, m.confirmKey = false, false, ""
	m.input.Blur()
	m.setMsg(lvOK, i18n.Tf("已打开 %s", m.repos[i].rel))
	return m.refreshCmd()
}

// openRepoPicker 打开仓库选择面板，并刷新各仓库的分支和改动数。
func (m *Model) openRepoPicker() tea.Cmd {
	if m.workspace == "" {
		return nil
	}
	m.repoPicking = true
	m.repoFilter, m.repoFiltering = "", false
	for i, r := range m.repos {
		if m.repo != nil && r.path == m.repo.Root {
			m.repoCursor = i
		}
	}
	m.repoClamp()
	if m.scanning {
		return nil
	}
	m.scanGen++
	return m.summaryCmd(0)
}

func (m *Model) rescan() tea.Cmd {
	if m.scanning {
		return nil
	}
	m.scanning = true
	return scanCmd(m.workspace, m.opts.Depth)
}

// ---------------------------------------------------------------- 筛选与光标

// visibleRepos 返回符合筛选条件的仓库下标；筛选是不区分大小写的子串匹配。
func (m Model) visibleRepos() []int {
	q := strings.ToLower(m.repoFilter)
	var out []int
	for i, r := range m.repos {
		if q == "" || strings.Contains(strings.ToLower(r.rel), q) {
			out = append(out, i)
		}
	}
	return out
}

func (m Model) repoListH() int { return max(1, m.h-4) }

func (m *Model) repoClamp() {
	n := len(m.visibleRepos())
	m.repoCursor = max(0, min(m.repoCursor, n-1))
	h := m.repoListH()
	if m.repoCursor < m.repoOffset {
		m.repoOffset = m.repoCursor
	}
	if m.repoCursor >= m.repoOffset+h {
		m.repoOffset = m.repoCursor - h + 1
	}
	m.repoOffset = max(0, min(m.repoOffset, n-h))
}

func (m *Model) openSelectedRepo() tea.Cmd {
	vis := m.visibleRepos()
	if m.repoCursor < 0 || m.repoCursor >= len(vis) {
		return nil
	}
	return m.switchRepo(vis[m.repoCursor])
}

func (m *Model) closeRepoPicker() {
	m.repoPicking, m.repoFiltering, m.repoFilter = false, false, ""
}

// ---------------------------------------------------------------- 键盘与鼠标

func (m Model) handleRepoKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.repoFiltering {
		switch k.Type {
		case tea.KeyEsc:
			m.repoFiltering, m.repoFilter = false, ""
		case tea.KeyEnter:
			m.repoFiltering = false
			return m, m.openSelectedRepo()
		case tea.KeyBackspace:
			if r := []rune(m.repoFilter); len(r) > 0 {
				m.repoFilter = string(r[:len(r)-1])
			}
		case tea.KeyUp:
			m.repoCursor--
		case tea.KeyDown:
			m.repoCursor++
		case tea.KeyRunes, tea.KeySpace:
			for _, r := range k.Runes {
				if unicode.IsPrint(r) {
					m.repoFilter += string(r)
				}
			}
			m.repoCursor = 0
		case tea.KeyCtrlC:
			if !m.opts.Embed {
				return m, tea.Quit
			}
		}
		m.repoClamp()
		return m, nil
	}
	switch k.String() {
	case "ctrl+c":
		if !m.opts.Embed {
			return m, tea.Quit
		}
		m.closeRepoPicker()
	case "esc", "q", "o":
		m.closeRepoPicker()
	case "j", "down":
		m.repoCursor++
	case "k", "up":
		m.repoCursor--
	case "g", "home":
		m.repoCursor = 0
	case "G", "end":
		m.repoCursor = len(m.repos)
	case "pgdown":
		m.repoCursor += m.repoListH()
	case "pgup":
		m.repoCursor -= m.repoListH()
	case "/":
		m.repoFiltering = true
	case "r":
		return m, m.rescan()
	case "enter", " ", "l", "right":
		return m, m.openSelectedRepo()
	}
	m.repoClamp()
	return m, nil
}

func (m Model) handleRepoMouse(ev tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch ev.Button {
	case tea.MouseButtonWheelUp:
		m.repoOffset = max(0, m.repoOffset-3)
		return m, nil
	case tea.MouseButtonWheelDown:
		m.repoOffset = max(0, min(m.repoOffset+3, len(m.visibleRepos())-m.repoListH()))
		return m, nil
	}
	if ev.Action != tea.MouseActionPress || ev.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if ev.Y == m.h-2 {
		for _, b := range m.repoButtons() {
			if b.hit(ev.X) {
				switch b.id {
				case "repoOpen":
					return m, m.openSelectedRepo()
				case "repoRescan":
					return m, m.rescan()
				case "repoBack":
					m.closeRepoPicker()
				}
			}
		}
		return m, nil
	}
	i := m.repoOffset + ev.Y - 2
	if ev.Y < 2 || i >= len(m.visibleRepos()) {
		return m, nil
	}
	// 单击直接打开（像菜单一样）
	m.repoCursor = i
	return m, m.openSelectedRepo()
}

// ---------------------------------------------------------------- 渲染

func (m Model) repoButtons() []button {
	bs := []button{{id: "repoOpen", label: icons.check + i18n.T(" 打开"), kind: btnPrimary}, {id: "repoRescan", label: i18n.T("↻ 重新扫描")}}
	if m.repo != nil {
		bs = append(bs, button{id: "repoBack", label: i18n.T("返回")})
	}
	return place(1, m.w, bs)
}

// headerRepoButton 是头部右侧的仓库名；在子仓库模式下可以点它切换仓库。
func (m Model) headerRepoButton() (button, bool) {
	if m.workspace == "" || m.repo == nil {
		return button{}, false
	}
	label := "⌂ " + gitx.Rel(m.workspace, m.repo.Root) + " ▾"
	w := lipgloss.Width(label) + 2
	b := button{id: "repos", label: label, x0: m.w - w - 1, x1: m.w - 1}
	return b, b.x0 > m.w/2 // 太窄时不显示，用 o 键
}

func homeShort(p string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rel, err := filepath.Rel(home, p); err == nil && !strings.HasPrefix(rel, "..") {
			if rel == "." {
				return "~"
			}
			return "~/" + filepath.ToSlash(rel)
		}
	}
	return p
}

func (m Model) viewRepoPicker() string {
	vis := m.visibleRepos()
	lines := make([]string, 0, m.h)

	info := i18n.Tf("%d 个仓库 · 深度 %d", len(m.repos), m.opts.Depth)
	if m.scanTrunc {
		info += i18n.T(" · 已达上限")
	}
	lines = append(lines, rightAlign(m.w, cBar,
		[]seg{sg(" ⌂ "+i18n.T("选择仓库")+" ", lipgloss.NewStyle().Background(cAccent).Foreground(cOnPill).Bold(true)), sg("  "+homeShort(m.workspace), fg(cMuted))},
		[]seg{sg(info+" ", fg(cYellow))}))

	if m.repoFiltering || m.repoFilter != "" {
		cursor := ""
		if m.repoFiltering {
			cursor = "█"
		}
		lines = append(lines, paint(m.w, nil, sg("  / ", fg(cAccent).Bold(true)), sg(m.repoFilter+cursor, fg(cText)),
			sg("   "+i18n.Tf("匹配 %d 个", len(vis)), fg(cFaint))))
	} else {
		lines = append(lines, paint(m.w, nil, sg(i18n.T("  ↑↓ 选择 · 回车或单击打开 · / 筛选 · r 重新扫描 · Esc 返回"), fg(cFaint))))
	}

	h := m.repoListH()
	for vi := m.repoOffset; vi < m.repoOffset+h && vi < len(vis); vi++ {
		r := m.repos[vis[vi]]
		var bg lipgloss.TerminalColor
		if vi == m.repoCursor {
			bg = cSel
		}
		cur := m.repo != nil && r.path == m.repo.Root
		dot, nameSt := "   ", fg(cText)
		if cur {
			dot, nameSt = " ● ", fg(cGreen).Bold(true)
		}
		dir, base := filepath.Split(r.rel)
		left := []seg{sg(dot, fg(cGreen)), sg(dir, fg(cMuted)), sg(base, nameSt)}
		var right []seg
		switch {
		case !r.loaded:
			right = []seg{sg("… ", fg(cFaint))}
		case r.sum.Err:
			right = []seg{sg(i18n.T("读取失败")+" ", fg(cRed))}
		default:
			if r.sum.Branch != "" {
				right = append(right, sg(icons.branch+" "+r.sum.Branch+"  ", fg(cPurple)))
			}
			if r.sum.Changes > 0 {
				right = append(right, sg(i18n.Tf("%d 处改动", r.sum.Changes)+" ", fg(cYellow)))
			} else {
				right = append(right, sg(icons.clean+" "+i18n.T("干净")+" ", fg(cGreen)))
			}
		}
		lines = append(lines, rightAlign(m.w, bg, left, right))
	}
	switch {
	case m.scanning:
		lines = append(lines, paint(m.w, nil, sg(i18n.T("  正在扫描子目录…"), fg(cMuted))))
	case len(m.repos) == 0:
		lines = append(lines, paint(m.w, nil, sg("  "+i18n.Tf("在 %d 层子目录内没有找到 Git 仓库", m.opts.Depth), fg(cMuted))))
	case len(vis) == 0:
		lines = append(lines, paint(m.w, nil, sg(i18n.T("  没有匹配的仓库"), fg(cMuted))))
	}
	for len(lines) < m.h-2 {
		lines = append(lines, strings.Repeat(" ", m.w))
	}
	lines = lines[:m.h-2]
	lines = append(lines, renderButtons(m.w, m.repoButtons()))
	if m.msg != "" && time.Since(m.msgAt) < 6*time.Second {
		col := cMuted
		if m.msgLevel == lvErr {
			col = cRed
		}
		lines = append(lines, paint(m.w, nil, sg(" "+m.msg, fg(col))))
	} else {
		lines = append(lines, strings.Repeat(" ", m.w))
	}
	return strings.Join(lines, "\n")
}
