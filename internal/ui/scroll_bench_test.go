package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// 大 diff（5000 行，三分之一是超长行）。
func bigDetailModel(t testing.TB, lines int, wrap bool) Model {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := New(t.TempDir(), Options{})
	m.w, m.h = 160, 45
	m.wrap = wrap
	m.detailKey = "k"
	m.detail = detailView{key: "k", digits: 4}
	long := strings.Repeat("abcdefghij ", 25)
	for i := 0; i < lines; i++ {
		txt := fmt.Sprintf("line %d", i)
		if i%3 == 0 {
			txt = long
		}
		m.detail.lines = append(m.detail.lines, dline{kind: dAdd, new: i + 1, text: txt})
	}
	return m
}

// 滚轮一次 = Update + View。开着自动换行时曾经每次要 70ms+，快速滚动会卡住几十秒。
func BenchmarkWheelWrapped(b *testing.B) {
	m := bigDetailModel(b, 5000, true)
	g := m.geom()
	ev := tea.MouseMsg{X: g.detailX + 5, Y: g.detailY + 3, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nm, _ := m.Update(ev)
		m = nm.(Model)
		_ = m.View()
	}
}

func TestVisibleDetailCache(t *testing.T) {
	m := bigDetailModel(t, 300, true)
	a := m.visibleDetail()
	if b := m.visibleDetail(); &a[0] != &b[0] {
		t.Fatal("内容没变时应该直接用缓存")
	}
	// 窗口宽度变了：重新折行
	m.w = 100
	if b := m.visibleDetail(); &a[0] == &b[0] || len(b) <= len(a) {
		t.Fatal("宽度变窄后应该重新计算，而且行数变多")
	}
	// 关掉换行：行数等于原始行数
	m.wrap = false
	if b := m.visibleDetail(); len(b) != 300 {
		t.Fatalf("关掉换行后应是 300 行，得到 %d", len(b))
	}
	// 换了一份内容（同样行数）：不能用旧缓存
	m.wrap = true
	old := m.visibleDetail()
	nd := make([]dline, len(m.detail.lines))
	copy(nd, m.detail.lines)
	nd[1].text = "changed"
	m.detail.lines = nd
	if b := m.visibleDetail(); &old[0] == &b[0] {
		t.Fatal("内容换了一份后应该重新计算")
	}

	// 多文件提交：折叠 / 展开文件后应该重新计算
	m.detail = detailView{key: "k", digits: 3, lines: []dline{
		{kind: dFile, path: "a.go"}, {kind: dAdd, new: 1, text: "x"},
		{kind: dFile, path: "b.go"}, {kind: dAdd, new: 1, text: "y"},
	}}
	m.detail.files = indexFiles(m.detail.lines)
	if n := len(m.visibleDetail()); n != 2 {
		t.Fatalf("多文件默认折叠，应只有 2 个标题行，得到 %d", n)
	}
	m.toggleFileFold("a.go")
	if n := len(m.visibleDetail()); n != 3 {
		t.Fatalf("展开 a.go 后应有 3 行，得到 %d", n)
	}
	m.toggleAllFiles()
	m.toggleAllFiles()
	if n := len(m.visibleDetail()); n != 4 {
		t.Fatalf("全部展开后应有 4 行，得到 %d", n)
	}
}
