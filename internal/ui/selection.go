package ui

import (
	"encoding/base64"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/0xBB2B/gitpane/internal/i18n"
)

// 鼠标拖动选中文字并复制。
//
// gitpane 接管了鼠标（点击、滚轮、拖分隔线），终端自带的拖动选择就用不了了，所以自己实现：
//   - 选择限定在起点所在的那一栏（列表 / 详情 / 其他区域），不会把左右两栏混在一起；
//   - 在详情区复制代码时去掉行号和 +/- 栏，自动换行折开的行拼回一行；
//   - 松开鼠标就复制：本机写系统剪贴板，SSH 上用 OSC 52 让终端写到本地剪贴板；
//   - 单击、双击照旧；按住拖动超过一格才算选择。

type rect struct{ x0, y0, x1, y1 int } // 右、下边界不含

func (r rect) clampX(x int) int { return max(r.x0, min(x, r.x1-1)) }
func (r rect) clampY(y int) int { return max(r.y0, min(y, r.y1-1)) }

type selState struct {
	pressed bool // 左键按下、还没松开
	active  bool // 拖动过：这是一次选择，不是单击
	shown   bool // 选择完成，保留高亮直到下一次点击 / 按键 / 滚动
	detail  bool // 选择在详情区里（复制时去掉行号栏）
	pane    rect
	sx, sy  int // 起点
	ex, ey  int // 终点（已夹在 pane 里）
}

func (s selState) visible() bool { return s.active || s.shown }

// selPane 返回 (x, y) 所在的那一栏，以及它是不是详情区。
func (m Model) selPane(x, y int) (rect, bool) {
	g := m.geom()
	if x >= g.detailX && x < g.detailX+g.detailW && y >= g.detailHdrY && y < g.detailY+g.detailH {
		return rect{g.detailX, g.detailHdrY, g.detailX + g.detailW, g.detailY + g.detailH}, true
	}
	if g.inList(x, y) {
		return rect{g.listX, g.listY, g.listX + g.listW, g.listY + g.listH}, false
	}
	return rect{0, 0, m.w, m.h}, false
}

// selRow 是选区在某一屏幕行上的部分：列 [a, b)。
type selRow struct {
	y, a, b int
	text    string // 非空时复制这段原文（详情区里整行选中的代码等），不用屏幕上的字
	cont    bool   // 自动换行折出来的后续行：复制时和上一行拼在一起
	src     int    // 自动换行时这一屏幕行属于哪一行原文；-1 表示不适用
}

func (m Model) selRows() []selRow {
	s := m.sel
	if !s.visible() {
		return nil
	}
	sx, sy, ex, ey := s.sx, s.sy, s.ex, s.ey
	if ey < sy || (ey == sy && ex < sx) {
		sx, sy, ex, ey = ex, ey, sx, sy
	}
	var rows []selRow
	g := m.geom()
	var vis []dline
	if s.detail {
		vis = m.visibleDetail()
	}
	for y := sy; y <= ey; y++ {
		r := selRow{y: y, a: s.pane.x0, b: s.pane.x1, src: -1}
		if y == sy {
			r.a = sx
		}
		if y == ey {
			r.b = ex + 1
		}
		// 详情区的内容行：起点在行号栏里时从正文开始；整行选中时复制原文
		if s.detail && y >= g.detailY {
			if i := m.detailOff + y - g.detailY; i >= 0 && i < len(vis) {
				l := vis[i]
				if pre := dlinePrefix(l.kind, g.detailW, m.detail.digits); pre >= 0 {
					start := g.detailX + pre
					r.a = max(r.a, start)
					r.cont = l.cont
					if r.a == start && r.b >= s.pane.x1 {
						r.text = l.text
						if m.wrap {
							r.src = l.src
						}
					}
				} else if l.kind == dFile && r.a <= s.pane.x0 && r.b >= s.pane.x1 {
					r.text = l.path
				}
			}
		}
		if r.a < r.b {
			rows = append(rows, r)
		}
	}
	return rows
}

// selectedText 拼出选中的文字。
func (m Model) selectedText() string {
	rows := m.selRows()
	if len(rows) == 0 {
		return ""
	}
	screen := strings.Split(ansi.Strip(m.viewMain()), "\n")
	// 自动换行时，一行原文的所有折行都被整行选中，就直接用原文（保留制表符等原样）
	if m.wrap {
		orig := m.foldedDetail()
		var merged []selRow
		for i := 0; i < len(rows); i++ {
			r := rows[i]
			if r.src >= 0 && !r.cont && r.src < len(orig) {
				j := i + 1
				for j < len(rows) && rows[j].cont && rows[j].src == r.src {
					j++
				}
				// 这一行原文的折行要全部在选区里：选区之后不能还有它的折行
				lastScreen := rows[j-1].y
				g := m.geom()
				vis := m.visibleDetail()
				next := m.detailOff + lastScreen - g.detailY + 1
				if next >= len(vis) || !vis[next].cont || vis[next].src != r.src {
					r.text = orig[r.src].text
					merged = append(merged, r)
					i = j - 1
					continue
				}
			}
			merged = append(merged, r)
		}
		rows = merged
	}
	var b strings.Builder
	for i, r := range rows {
		piece := r.text
		if piece == "" && r.y < len(screen) {
			piece = ansi.Cut(screen[r.y], r.a, r.b)
		}
		// 下一行是这一行折出来的，就不去掉行尾空格（空格可能是正文的一部分）
		if i+1 >= len(rows) || !rows[i+1].cont {
			piece = strings.TrimRight(piece, " ")
		}
		if i > 0 && !r.cont {
			b.WriteByte('\n')
		}
		b.WriteString(piece)
	}
	return b.String()
}

// overlaySelection 在已经渲染好的各行上画出选区高亮。
func (m Model) overlaySelection(lines []string) {
	open := styleOpen(lipgloss.NewStyle().Background(cAccent).Foreground(cOnPill))
	for _, r := range m.selRows() {
		if r.y < 0 || r.y >= len(lines) {
			continue
		}
		line := lines[r.y]
		mid := ansi.Strip(ansi.Cut(line, r.a, r.b))
		lines[r.y] = ansi.Cut(line, 0, r.a) + sgrReset + open + mid + sgrReset + ansi.Cut(line, r.b, m.w)
	}
}

// ---------------------------------------------------------------- 鼠标

// handleSelect 处理选择相关的鼠标事件；返回 true 表示事件已经处理完。
// 按下时只记下起点，不消费事件（单击照常生效）。
func (m *Model) handleSelect(ev tea.MouseMsg) (bool, tea.Cmd) {
	switch ev.Action {
	case tea.MouseActionPress:
		if ev.Button == tea.MouseButtonLeft {
			pane, detail := m.selPane(ev.X, ev.Y)
			m.sel = selState{pressed: true, pane: pane, detail: detail, sx: ev.X, sy: ev.Y, ex: ev.X, ey: ev.Y}
		} else {
			m.sel = selState{} // 滚轮等：内容会动，取消选区
		}
		return false, nil
	case tea.MouseActionMotion:
		if !m.sel.pressed {
			return false, nil
		}
		x, y := m.sel.pane.clampX(ev.X), m.sel.pane.clampY(ev.Y)
		if x != m.sel.sx || y != m.sel.sy {
			m.sel.active = true
		}
		m.sel.ex, m.sel.ey = x, y
		return true, nil
	case tea.MouseActionRelease:
		if !m.sel.pressed {
			return false, nil
		}
		m.sel.pressed = false
		if !m.sel.active {
			m.sel = selState{}
			return true, nil
		}
		m.sel.active, m.sel.shown = false, true
		text := m.selectedText()
		if text == "" {
			m.sel = selState{}
			return true, nil
		}
		return true, copyCmd(text)
	}
	return false, nil
}

// ---------------------------------------------------------------- 剪贴板

type copiedMsg struct {
	lines int
	err   error
}

// copyCmd 把文字放进剪贴板：本机用系统剪贴板；通过 SSH 连接时（或系统剪贴板不可用时）
// 用 OSC 52 让终端写到它所在电脑的剪贴板。
func copyCmd(text string) tea.Cmd {
	return func() tea.Msg {
		n := strings.Count(text, "\n") + 1
		if os.Getenv("SSH_TTY") == "" && os.Getenv("SSH_CONNECTION") == "" {
			if err := clipboard.WriteAll(text); err == nil {
				return copiedMsg{lines: n}
			}
		}
		seq := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(text)) + "\a"
		if os.Getenv("TMUX") != "" {
			// tmux 里要用 passthrough 包一层，终端才收得到
			seq = "\x1bPtmux;" + strings.ReplaceAll(seq, "\x1b", "\x1b\x1b") + "\x1b\\"
		}
		_, err := os.Stdout.WriteString(seq)
		return copiedMsg{lines: n, err: err}
	}
}

func (m *Model) applyCopied(msg copiedMsg) {
	if msg.err != nil {
		m.setMsg(lvErr, i18n.Tf("复制失败：%s", msg.err.Error()))
		return
	}
	m.setMsg(lvOK, i18n.Tf("已复制 %d 行", msg.lines))
}
