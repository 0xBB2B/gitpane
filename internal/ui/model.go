// Package ui 是 gitpane 的界面：变更树 / 提交图谱 / 分支，支持鼠标。
package ui

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xBB2B/gitpane/internal/gitx"
	"github.com/0xBB2B/gitpane/internal/i18n"
)

const refreshEvery = 2 * time.Second

type tab int

const (
	tabChanges tab = iota
	tabGraph
	numTabs
)

// 图谱一次加载的提交数；滚到底时再加载这么多。
const graphPage = 500

type msgLevel int

const (
	lvInfo msgLevel = iota
	lvOK
	lvErr
)

// Options 控制运行方式。
type Options struct {
	// Embed 表示嵌入在别的分屏工具里当面板运行：q 不退出，Ctrl+C 要按两次。
	Embed  bool
	Layout Layout
	// Depth 是启动目录不是 Git 仓库时向下探测子仓库的层数；0 表示不探测。
	Depth int
}

// Model 是面板的全部状态。
type Model struct {
	dir      string
	opts     Options
	repo     *gitx.Repo
	w, h     int
	tab      tab
	showHelp bool

	// 列表 / 详情分隔线：每个标签页的比例，以及是否正在拖动
	split [numTabs]splitPref
	drag  dragKind

	// 子仓库探测（启动目录不是仓库时）：workspace 是扫描根目录，为空表示没有启用
	workspace     string
	repos         []repoEntry
	scanning      bool
	scanTrunc     bool
	scanGen       int
	repoPicking   bool
	repoCursor    int
	repoOffset    int
	repoFilter    string
	repoFiltering bool

	status    *gitx.Status
	statusErr string
	stats     [3]map[string]gitx.Stat // 按分组的每文件增删行数
	rows      []row
	collapsed map[string]bool

	graph      []gitx.GraphLine
	graphLimit int
	graphRefs  []string // 图谱只显示这些分支；空表示全部
	branches   []gitx.Branch

	// 分支选择面板
	picking    bool
	pickCursor int
	pickOffset int

	// 提交详情里各文件的折叠状态（key: 详情 key + 路径）；没记录时按默认
	fileFold map[string]bool

	// 每个标签页各自的光标与滚动位置
	cursor [3]int
	offset [3]int

	detail    detailView
	detailKey string
	detailOff int

	busy     string
	spin     spinner.Model
	msg      string
	msgLevel msgLevel
	msgAt    time.Time

	committing bool
	input      textinput.Model

	confirmKey string
	confirmAt  time.Time

	lastClickRow int
	lastClickAt  time.Time
	quitArmedAt  time.Time
}

// New 创建界面。dir 可以是仓库内任意目录。
func New(dir string, opts Options) Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = i18n.T("写点什么…  回车提交 · Esc 取消")
	ti.CharLimit = 0
	ti.TextStyle = fg(cText)
	ti.PlaceholderStyle = fg(cFaint)
	ti.Cursor.Style = fg(cAccent)

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = fg(cAccent)

	m := Model{dir: dir, opts: opts, collapsed: map[string]bool{}, fileFold: map[string]bool{},
		graphLimit: graphPage, input: ti, spin: sp, lastClickRow: -1, split: loadSplits()}
	m.repo, _ = gitx.Open(dir)
	if m.repo == nil && opts.Depth > 0 {
		m.workspace, m.scanning = dir, true
	}
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.refreshCmd(), tick()}
	if m.scanning {
		cmds = append(cmds, scanCmd(m.workspace, m.opts.Depth))
	}
	return tea.Batch(cmds...)
}

// ---------------------------------------------------------------- 消息

type tickMsg struct{}

type statusMsg struct {
	st    *gitx.Status
	stats [3]map[string]gitx.Stat
	err   error
}

type graphMsg struct {
	lines []gitx.GraphLine
	err   error
	limit int
	refs  string // 发起请求时的筛选，用来丢弃过期结果
}

type branchesMsg struct {
	bs  []gitx.Branch
	err error
}

type detailMsg struct {
	view detailView
}

type opDoneMsg struct {
	ok  string
	err error
}

type repoMsg struct{ repo *gitx.Repo }

func tick() tea.Cmd {
	return tea.Tick(refreshEvery, func(time.Time) tea.Msg { return tickMsg{} })
}

// refreshCmd 刷新当前标签页需要的数据。
func (m Model) refreshCmd() tea.Cmd {
	if m.repo == nil {
		dir := m.dir
		return func() tea.Msg {
			r, _ := gitx.Open(dir)
			return repoMsg{r}
		}
	}
	repo := m.repo
	cmds := []tea.Cmd{func() tea.Msg {
		st, err := repo.Status()
		if err != nil {
			return statusMsg{err: err}
		}
		var stats [3]map[string]gitx.Stat
		stats[secChanges], stats[secStaged] = repo.NumStat()
		stats[secConflict] = map[string]gitx.Stat{}
		n := 0
		for _, f := range st.Files {
			if f.Untracked() && n < 200 {
				n++
				if c := repo.UntrackedLines(f.Path); c >= 0 {
					stats[secChanges][f.Path] = gitx.Stat{Add: c}
				}
			}
		}
		return statusMsg{st: st, stats: stats}
	}}
	if m.tab == tabGraph {
		limit, refs := m.graphLimit, append([]string(nil), m.graphRefs...)
		cmds = append(cmds, func() tea.Msg {
			lines, err := repo.Graph(limit, refs)
			return graphMsg{lines: lines, err: err, limit: limit, refs: strings.Join(refs, "\x00")}
		}, func() tea.Msg {
			bs, err := repo.Branches()
			return branchesMsg{bs, err}
		})
	}
	if c := m.detailCmd(); c != nil {
		cmds = append(cmds, c)
	}
	return tea.Batch(cmds...)
}

// detailTarget 根据当前光标决定下方详情区显示什么。
func (m Model) detailTarget() (string, func() detailView) {
	if m.repo == nil {
		return "", nil
	}
	repo := m.repo
	switch m.tab {
	case tabChanges:
		r, ok := m.currentRow()
		if !ok || r.kind != rowFile {
			return "", nil
		}
		f, sec := r.file, r.sec
		code := statusCode(f, sec)
		where, col := i18n.T("工作区"), cYellow
		switch {
		case sec == secStaged:
			where, col = i18n.T("已暂存"), cGreen
		case sec == secConflict:
			where, col = i18n.T("冲突"), cRed
		case f.Untracked():
			where, col = i18n.T("新文件"), cGreen
		}
		dir, name := path.Split(f.Path)
		head := detailHead{icon: fileIcon(name), name: name, dir: strings.TrimSuffix(dir, "/"), badge: where + " · " + statusWord(code), badgeCol: col}
		if f.OrigPath != "" {
			head.dir = f.OrigPath + " →"
		}
		if st, ok := m.stats[sec][f.Path]; ok {
			head.add, head.del = st.Add, st.Del
		}
		key := "f\x00" + r.key()
		return key, func() detailView {
			raw, err := repo.Diff(f, sec == secStaged)
			v := detailView{key: key, head: head}
			if err != nil && raw == "" {
				v.lines = []dline{{kind: dErr, text: gitx.Describe(err)}}
				return v
			}
			v.lines, v.digits = parseDiff(raw, false)
			if len(v.lines) == 0 {
				v.lines = []dline{{kind: dText, text: i18n.T("没有文本差异（可能只是权限或空文件）")}}
			}
			return v
		}
	case tabGraph:
		i := m.cursor[tabGraph]
		if i < 0 || i >= len(m.graph) || m.graph[i].Hash == "" {
			return "", nil
		}
		g := m.graph[i]
		key := "c\x00" + g.Hash
		return key, func() detailView {
			v := detailView{key: key, head: detailHead{icon: icons.commit + " ", name: g.Hash, dir: g.Author + " · " + g.When}}
			ci, patch, err := repo.Show(g.Hash)
			if err != nil || ci == nil {
				v.lines = []dline{{kind: dErr, text: gitx.Describe(err)}}
				return v
			}
			lines, digits := parseDiff(patch, true)
			for _, l := range lines {
				if l.kind == dFile {
					v.head.add += l.add
					v.head.del += l.del
				}
			}
			v.lines, v.digits = append(commitLines(ci), lines...), digits
			v.files = indexFiles(v.lines)
			return v
		}
	}
	return "", nil
}

func (m Model) detailCmd() tea.Cmd {
	_, load := m.detailTarget()
	if load == nil {
		return nil
	}
	return func() tea.Msg { return detailMsg{view: load()} }
}

// ---------------------------------------------------------------- 工具

func (m Model) itemCount() int {
	if m.tab == tabGraph {
		return len(m.graph)
	}
	return len(m.rows)
}

func (m Model) currentRow() (row, bool) {
	i := m.cursor[tabChanges]
	if m.tab != tabChanges || i < 0 || i >= len(m.rows) {
		return row{}, false
	}
	return m.rows[i], true
}

func (m *Model) setMsg(level msgLevel, s string) {
	m.msg, m.msgLevel, m.msgAt = s, level, time.Now()
}

func (m *Model) clampCursor() {
	n := m.itemCount()
	t := m.tab
	if m.cursor[t] >= n {
		m.cursor[t] = n - 1
	}
	if m.cursor[t] < 0 {
		m.cursor[t] = 0
	}
	// 只夹住范围，不把视口拉回光标：自动刷新时用户可能正在用滚轮翻看
	m.clampOffset()
}

func (m *Model) ensureVisible() {
	g := m.geom()
	t := m.tab
	if m.cursor[t] < m.offset[t] {
		m.offset[t] = m.cursor[t]
	}
	if m.cursor[t] >= m.offset[t]+g.listH {
		m.offset[t] = m.cursor[t] - g.listH + 1
	}
	m.clampOffset()
}

func (m *Model) clampOffset() {
	g := m.geom()
	t := m.tab
	if mx := max(0, m.itemCount()-g.listH); m.offset[t] > mx {
		m.offset[t] = mx
	}
	if m.offset[t] < 0 {
		m.offset[t] = 0
	}
	if dmax := max(0, len(m.visibleDetail())-g.detailH); m.detailOff > dmax {
		m.detailOff = dmax
	}
	if m.detailOff < 0 {
		m.detailOff = 0
	}
}

// moveCursor 移动光标并在目标变化时加载详情。
func (m *Model) moveCursor(to int) tea.Cmd {
	m.cursor[m.tab] = to
	m.clampCursor()
	m.ensureVisible()
	return tea.Batch(m.syncDetail(), m.maybeLoadMore())
}

// maybeLoadMore 在图谱看到接近底部、且上次加载已满额时，加载更多提交。
func (m *Model) maybeLoadMore() tea.Cmd {
	if m.tab != tabGraph || m.repo == nil || len(m.graph) < m.graphLimit {
		return nil
	}
	g := m.geom()
	if m.offset[tabGraph]+g.listH < len(m.graph)-10 && m.cursor[tabGraph] < len(m.graph)-10 {
		return nil
	}
	m.graphLimit += graphPage
	m.setMsg(lvInfo, fmt.Sprintf(i18n.T("加载更多历史…（%d 条）"), m.graphLimit))
	return m.refreshCmd()
}

// visibleDetail 返回详情区当前可见的行：折叠的文件只留标题行。
func (m Model) visibleDetail() []dline {
	lines := m.detail.lines
	if m.detail.files == 0 {
		return lines
	}
	out := make([]dline, 0, len(lines))
	for _, l := range lines {
		if l.file >= 0 {
			hdr := lines[l.file]
			folded := m.fileFolded(hdr.path)
			if l.kind == dFile {
				l.collapsed = folded
			} else if folded {
				continue
			}
		}
		out = append(out, l)
	}
	return out
}

// fileFolded：提交只改了一个文件时默认展开，多个文件默认折叠，先看清单。
func (m Model) fileFolded(path string) bool {
	if v, ok := m.fileFold[m.detailKey+"\x00"+path]; ok {
		return v
	}
	return m.detail.files > 1
}

func (m *Model) toggleFileFold(path string) {
	m.fileFold[m.detailKey+"\x00"+path] = !m.fileFolded(path)
	m.clampOffset()
}

// toggleAllFiles 有展开的文件就全部折叠，否则全部展开。
func (m *Model) toggleAllFiles() {
	var paths []string
	anyOpen := false
	for _, l := range m.detail.lines {
		if l.kind == dFile {
			paths = append(paths, l.path)
			if !m.fileFolded(l.path) {
				anyOpen = true
			}
		}
	}
	for _, p := range paths {
		m.fileFold[m.detailKey+"\x00"+p] = anyOpen
	}
	m.clampOffset()
}

// syncDetail 在详情目标改变时清空并重新加载。
func (m *Model) syncDetail() tea.Cmd {
	key, _ := m.detailTarget()
	if key == m.detailKey {
		return nil
	}
	m.detailKey, m.detailOff = key, 0
	m.detail = detailView{}
	return m.detailCmd()
}

func (m *Model) startOp(label string, fn func() (string, error)) tea.Cmd {
	if m.busy != "" {
		m.setMsg(lvErr, i18n.Tf("上一个操作还在进行：%s", m.busy))
		return nil
	}
	m.busy = label
	return tea.Batch(m.spin.Tick, func() tea.Msg {
		ok, err := fn()
		return opDoneMsg{ok: ok, err: err}
	})
}

func (m Model) stagedCount() int {
	if m.status == nil {
		return 0
	}
	n := 0
	for _, f := range m.status.Files {
		if f.Staged() {
			n++
		}
	}
	return n
}

func (m Model) changeCount() int {
	if m.status == nil {
		return 0
	}
	return len(m.status.Files)
}

// detectBackground 决定用深色还是浅色主题：GITPANE_THEME=light|dark 优先，
// 否则在 bubbletea 接管输入前向终端查询背景色。
func detectBackground() {
	switch strings.ToLower(os.Getenv("GITPANE_THEME")) {
	case "light":
		lipgloss.SetHasDarkBackground(false)
	case "dark":
		lipgloss.SetHasDarkBackground(true)
	default:
		_ = lipgloss.HasDarkBackground()
	}
}
