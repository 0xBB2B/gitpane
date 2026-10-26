package ui

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xBB2B/gitpane/internal/i18n"
)

// 列表和详情之间的分隔线可以拖动（或用 < > 调整、= 复位），
// 每个标签页、每种布局（左右 / 上下）各自记住比例，并保存到配置目录。

// splitPref 是一个标签页的分隔比例；0 表示用默认值。
type splitPref struct {
	Wide  float64 `json:"wide,omitempty"`  // 左右布局：列表占整宽的比例
	Stack float64 `json:"stack,omitempty"` // 上下布局：列表占可用高度的比例
}

type dragKind int

const (
	dragNone  dragKind = iota
	dragWide           // 正在左右拖动竖分隔线
	dragStack          // 正在上下拖动详情标题栏
)

const (
	minListW   = 24 // 左右布局时列表 / 详情的最小宽度
	minDetailW = 24
)

var tabNames = [numTabs]string{"changes", "graph"}

func layoutFile() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "gitpane", "layout.json")
}

// detailPrefs 是详情区的显示偏好，和分隔比例存在同一个文件的 "detail" 键下。
// 旧版本读到它会当成一个空的分隔比例忽略掉，不影响兼容。
type detailPrefs struct {
	Wrap bool `json:"wrap,omitempty"` // 长行自动换行
}

// loadLayout 读取保存的比例和详情偏好；文件不存在或损坏时返回默认值。
func loadLayout() (out [numTabs]splitPref, dp detailPrefs) {
	p := layoutFile()
	if p == "" {
		return
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return
	}
	var saved map[string]json.RawMessage
	if json.Unmarshal(data, &saved) != nil {
		return
	}
	for i, name := range tabNames {
		var s splitPref
		if raw, ok := saved[name]; ok && json.Unmarshal(raw, &s) == nil {
			out[i] = splitPref{Wide: validRatio(s.Wide), Stack: validRatio(s.Stack)}
		}
	}
	if raw, ok := saved["detail"]; ok {
		_ = json.Unmarshal(raw, &dp)
	}
	return
}

func validRatio(r float64) float64 {
	if math.IsNaN(r) || r < 0.1 || r > 0.9 {
		return 0
	}
	return r
}

// saveLayout 写回配置文件；失败时静默忽略（只是偏好，不影响使用）。
func (m Model) saveLayout() {
	p := layoutFile()
	if p == "" {
		return
	}
	saved := map[string]any{}
	for i, name := range tabNames {
		saved[name] = m.split[i]
	}
	saved["detail"] = detailPrefs{Wrap: m.wrap}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, append(data, '\n'), 0o644)
}

// setListW 把左右布局的列表宽度设为 w 列（会夹到合理范围），记成比例。
func (m *Model) setListW(w int) {
	w = max(minListW, min(w, m.w-minDetailW-1))
	m.split[m.tab].Wide = float64(w) / float64(m.w)
	m.clampOffset()
}

// setListH 把上下布局的列表高度设为 h 行，记成比例。
func (m *Model) setListH(h int) {
	avail := m.stackAvail()
	h = max(1, min(h, avail-1))
	m.split[m.tab].Stack = float64(h) / float64(avail)
	m.clampOffset()
}

// stackAvail 是上下布局里列表 + 详情可用的总行数（和 geom 保持一致）。
func (m Model) stackAvail() int { return max(2, max(2, m.h-4)-1) }

// nudgeSplit 用键盘微调：左右布局每次 4 列，上下布局每次 2 行。
func (m *Model) nudgeSplit(dir int) {
	g := m.geom()
	if g.wide {
		m.setListW(g.listW + 4*dir)
	} else {
		m.setListH(g.listH + 2*dir)
	}
	m.saveLayout()
}

func (m *Model) resetSplit() {
	m.split[m.tab] = splitPref{}
	m.clampOffset()
	m.saveLayout()
	m.setMsg(lvInfo, i18n.T("已恢复默认比例"))
}

// handleDrag 处理拖动中的鼠标事件；返回 true 表示事件已被拖动消费。
func (m *Model) handleDrag(ev tea.MouseMsg) bool {
	if m.drag == dragNone {
		return false
	}
	switch ev.Action {
	case tea.MouseActionMotion:
		g := m.geom()
		if m.drag == dragWide {
			m.setListW(ev.X)
		} else {
			m.setListH(ev.Y - g.listY)
		}
	case tea.MouseActionRelease:
		m.drag = dragNone
		m.saveLayout()
	}
	return true
}

// startDrag 在按下的位置是分隔线时开始拖动。
func (m *Model) startDrag(g geom, x, y int) bool {
	switch {
	case g.wide && x == g.listW && y >= g.listY && y < g.listY+g.listH:
		m.drag = dragWide
	case !g.wide && m.repo != nil && y == g.detailHdrY:
		for _, b := range m.detailButtons(g) {
			if b.hit(x) {
				return false // 标题栏右端的按钮照常可点
			}
		}
		m.drag = dragStack
	default:
		return false
	}
	return true
}
