package ui

import (
	"testing"

	"github.com/0xBB2B/gitpane/internal/gitx"
)

func newTestModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	m := New(t.TempDir(), Options{})
	m.w, m.h = 120, 30
	return m
}

func TestGraphRefreshKeepsSelection(t *testing.T) {
	m := newTestModel(t)
	m.tab = tabGraph
	m.h = 6 // 列表只有 2 行高，才需要滚动
	mk := func(names ...string) []gitx.GraphLine {
		var out []gitx.GraphLine
		for _, n := range names {
			out = append(out, gitx.GraphLine{Hash: n, Full: n + "full"})
		}
		return out
	}
	m.graph = mk("c5", "c4", "c3", "c2", "c1")
	m.cursor[tabGraph], m.offset[tabGraph] = 2, 1 // 选中 c3

	// 刷新后顶上多了两个新提交
	nm, _ := m.Update(graphMsg{lines: mk("c7", "c6", "c5", "c4", "c3", "c2", "c1"), limit: m.graphLimit})
	m = nm.(Model)
	if got := m.graph[m.cursor[tabGraph]].Hash; got != "c3" {
		t.Fatalf("刷新后选中应仍是 c3，实际 %s", got)
	}
	if m.offset[tabGraph] != 3 {
		t.Fatalf("滚动位置应同步平移 2 行，实际 %d", m.offset[tabGraph])
	}
	// 选中的提交消失（如 amend）时保持原行号
	nm, _ = m.Update(graphMsg{lines: mk("c7", "c6", "c5", "c4", "c3x", "c2", "c1"), limit: m.graphLimit})
	m = nm.(Model)
	if m.cursor[tabGraph] != 4 {
		t.Fatalf("找不到原提交时应保持行号，实际 %d", m.cursor[tabGraph])
	}
}
