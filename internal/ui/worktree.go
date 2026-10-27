package ui

import (
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xBB2B/gitpane/internal/gitx"
	"github.com/0xBB2B/gitpane/internal/i18n"
)

// 工作树（git worktree）：同一个仓库可以同时检出多个分支到不同目录并行开发，
// 每个工作树有自己的未提交改动。变更页可以在这些工作树之间切换（标签栏右侧按钮或 t 键），
// 切过去之后暂存、提交、拉取、推送都作用在那个工作树上；图谱是共享的，位置和筛选保留。

// wtSumMsg 是选择面板里各工作树的分支和改动数；gen 用来丢弃过期结果。
type wtSumMsg struct {
	gen  int
	sums map[string]gitx.Summary
}

func wtSumCmd(gen int, wts []gitx.Worktree) tea.Cmd {
	paths := make([]string, len(wts))
	for i, w := range wts {
		paths[i] = w.Path
	}
	return func() tea.Msg {
		sums := make(map[string]gitx.Summary, len(paths))
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, p := range paths {
			wg.Add(1)
			go func(p string) {
				defer wg.Done()
				s := gitx.Summarize(p)
				mu.Lock()
				sums[p] = s
				mu.Unlock()
			}(p)
		}
		wg.Wait()
		return wtSumMsg{gen: gen, sums: sums}
	}
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(filepath.FromSlash(a)), filepath.Clean(filepath.FromSlash(b))
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b) // 这两个系统的文件名默认不区分大小写
	}
	return a == b
}

// currentWorktree 返回当前打开的是第几个工作树；不在列表里时返回 -1。
func (m Model) currentWorktree() int {
	if m.repo == nil {
		return -1
	}
	for i, w := range m.worktrees {
		if samePath(w.Path, m.repo.Root) {
			return i
		}
	}
	return -1
}

// worktreeButton 是变更页标签栏右侧的「工作树：xxx ▾」，只有多个工作树时才显示。
func (m Model) worktreeButton() (button, bool) {
	if m.tab != tabChanges || len(m.worktrees) < 2 {
		return button{}, false
	}
	label := ""
	if i := m.currentWorktree(); i >= 0 {
		label = m.worktrees[i].Label()
	} else if m.status != nil {
		label = m.status.Branch
	}
	return button{id: "worktree", label: icons.branch + " " + i18n.Tf("工作树：%s", label) + " ▾", kind: btnPrimary}, true
}

func (m *Model) openWorktreePicker() tea.Cmd {
	if m.repo == nil {
		return nil
	}
	if len(m.worktrees) < 2 {
		m.setMsg(lvInfo, i18n.T("这个仓库只有一个工作树（可以用 git worktree add 新建）"))
		return nil
	}
	m.wtPicking = true
	m.wtCursor = max(0, m.currentWorktree())
	m.wtClamp()
	m.wtGen++
	return wtSumCmd(m.wtGen, m.worktrees)
}

func (m *Model) switchWorktree(i int) tea.Cmd {
	if i < 0 || i >= len(m.worktrees) {
		return nil
	}
	w := m.worktrees[i]
	m.wtPicking = false
	if m.repo != nil && samePath(m.repo.Root, w.Path) {
		return nil
	}
	if m.busy != "" {
		m.setMsg(lvErr, i18n.Tf("上一个操作还在进行：%s", m.busy))
		return nil
	}
	r, err := gitx.Open(w.Path)
	if err != nil {
		m.setMsg(lvErr, gitx.Describe(err))
		return nil
	}
	m.repo = r
	// 只清空和改动有关的状态；图谱是共享的历史，位置和筛选都保留
	m.status, m.statusErr, m.rows = nil, "", nil
	m.stats = [3]map[string]gitx.Stat{}
	m.cursor[tabChanges], m.offset[tabChanges] = 0, 0
	m.detail, m.detailKey, m.detailOff = detailView{}, "", 0
	m.collapsed = map[string]bool{}
	m.committing, m.confirmKey = false, ""
	m.input.Blur()
	m.tab = tabChanges // 切工作树就是为了看它的改动
	m.setMsg(lvOK, i18n.Tf("已切换到工作树 %s", w.Label()))
	return m.refreshCmd()
}

func wtPath(base, p string) string {
	p = filepath.FromSlash(p)
	if rel := gitx.Rel(base, p); !strings.HasPrefix(rel, "..") {
		return rel
	}
	return homeShort(p)
}

func (m Model) wtListH() int { return max(1, m.h-4) }

func (m *Model) wtClamp() {
	n := len(m.worktrees)
	m.wtCursor = max(0, min(m.wtCursor, n-1))
	h := m.wtListH()
	if m.wtCursor < m.wtOffset {
		m.wtOffset = m.wtCursor
	}
	if m.wtCursor >= m.wtOffset+h {
		m.wtOffset = m.wtCursor - h + 1
	}
	m.wtOffset = max(0, min(m.wtOffset, n-h))
}

// ---------------------------------------------------------------- 键盘与鼠标

func (m Model) handleWorktreeKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "ctrl+c":
		if !m.opts.Embed {
			return m, tea.Quit
		}
		m.wtPicking = false
	case "esc", "q", "t":
		m.wtPicking = false
	case "j", "down":
		m.wtCursor++
	case "k", "up":
		m.wtCursor--
	case "g", "home":
		m.wtCursor = 0
	case "G", "end":
		m.wtCursor = len(m.worktrees)
	case "pgdown":
		m.wtCursor += m.wtListH()
	case "pgup":
		m.wtCursor -= m.wtListH()
	case "enter", " ", "l", "right":
		return m, m.switchWorktree(m.wtCursor)
	}
	m.wtClamp()
	return m, nil
}

func (m Model) handleWorktreeMouse(ev tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch ev.Button {
	case tea.MouseButtonWheelUp:
		m.wtOffset = max(0, m.wtOffset-3)
		return m, nil
	case tea.MouseButtonWheelDown:
		m.wtOffset = max(0, min(m.wtOffset+3, len(m.worktrees)-m.wtListH()))
		return m, nil
	}
	if ev.Action != tea.MouseActionPress || ev.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if ev.Y == m.h-2 {
		for _, b := range m.worktreeButtons() {
			if b.hit(ev.X) {
				if b.id == "wtOpen" {
					return m, m.switchWorktree(m.wtCursor)
				}
				m.wtPicking = false
			}
		}
		return m, nil
	}
	i := m.wtOffset + ev.Y - 2
	if ev.Y < 2 || i >= len(m.worktrees) {
		return m, nil
	}
	m.wtCursor = i // 单击直接切换（像菜单一样）
	return m, m.switchWorktree(i)
}

// ---------------------------------------------------------------- 渲染

func (m Model) worktreeButtons() []button {
	return place(1, m.w, []button{
		{id: "wtOpen", label: icons.check + i18n.T(" 打开"), kind: btnPrimary},
		{id: "wtBack", label: i18n.T("返回")},
	})
}

func (m Model) viewWorktreePicker() string {
	lines := make([]string, 0, m.h)
	cur := m.currentWorktree()
	// 工作树一般都放在主工作树旁边：路径显示成相对主工作树所在目录的样子（app、app-login）
	base := ""
	if len(m.worktrees) > 0 {
		base = filepath.Dir(filepath.FromSlash(m.worktrees[0].Path))
	}
	where := homeShort(base)
	lines = append(lines, rightAlign(m.w, cBar,
		[]seg{sg(" "+icons.branch+" "+i18n.T("选择工作树")+" ", lipgloss.NewStyle().Background(cAccent).Foreground(cOnPill).Bold(true)), sg("  "+where, fg(cMuted))},
		[]seg{sg(i18n.Tf("%d 个工作树", len(m.worktrees))+" ", fg(cYellow))}))
	lines = append(lines, paint(m.w, nil, sg(i18n.T("  ↑↓ 选择 · 回车或单击切换 · Esc 返回"), fg(cFaint))))

	h := m.wtListH()
	for i := m.wtOffset; i < m.wtOffset+h && i < len(m.worktrees); i++ {
		w := m.worktrees[i]
		var bg lipgloss.TerminalColor
		if i == m.wtCursor {
			bg = cSel
		}
		dot, nameSt := "   ", fg(cText).Bold(true)
		if i == cur {
			dot, nameSt = " ● ", fg(cGreen).Bold(true)
		}
		name := w.Label()
		if w.Detached {
			name = i18n.Tf("游离 %s", name)
		}
		left := []seg{sg(dot, fg(cGreen)), sg(icons.branch+" ", fg(cPurple)), sg(name, nameSt), sg("   "+wtPath(base, w.Path), fg(cMuted))}
		var right []seg
		if w.Locked {
			right = append(right, sg(i18n.T("已锁定")+"  ", fg(cFaint)))
		}
		switch s, ok := m.wtSums[w.Path]; {
		case !ok:
			right = append(right, sg("… ", fg(cFaint)))
		case s.Err:
			right = append(right, sg(i18n.T("读取失败")+" ", fg(cRed)))
		case s.Changes > 0:
			right = append(right, sg(i18n.Tf("%d 处改动", s.Changes)+" ", fg(cYellow)))
		default:
			right = append(right, sg(icons.clean+" "+i18n.T("干净")+" ", fg(cGreen)))
		}
		lines = append(lines, rightAlign(m.w, bg, left, right))
	}
	for len(lines) < m.h-2 {
		lines = append(lines, strings.Repeat(" ", m.w))
	}
	lines = lines[:m.h-2]
	lines = append(lines, renderButtons(m.w, m.worktreeButtons()))
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
