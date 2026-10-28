package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func selModel(t *testing.T) Model {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := New(t.TempDir(), Options{})
	m.w, m.h = 120, 30
	m.wrap = true
	m.detailKey = "k"
	long := "return fmt.Sprintf(\"%s-%s\", " + strings.Repeat("veryLongIdentifier, ", 6) + "end)"
	m.detail = detailView{key: "k", digits: 3, lines: []dline{
		{kind: dHunk, new: 10, text: "func main() {"},
		{kind: dCtx, old: 10, new: 10, text: "\tx := 1"},
		{kind: dAdd, new: 11, text: "\t" + long},
		{kind: dDel, old: 11, text: "\ty := 2"},
	}}
	return m
}

func drag(m Model, x0, y0, x1, y1 int) (Model, tea.Cmd) {
	press := tea.MouseMsg{X: x0, Y: y0, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
	move := tea.MouseMsg{X: x1, Y: y1, Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion}
	rel := tea.MouseMsg{X: x1, Y: y1, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease}
	nm, _ := m.Update(press)
	nm, _ = nm.(Model).Update(move)
	nm, cmd := nm.(Model).Update(rel)
	return nm.(Model), cmd
}

func TestSelectCopyFromDiff(t *testing.T) {
	m := selModel(t)
	g := m.geom()
	vis := m.visibleDetail()
	if len(vis) < 5 {
		t.Fatalf("长行应该被折成多行：%d", len(vis))
	}
	// 从第二行（x := 1）的行号栏按下，拖到最后一行（y := 2）的末尾
	m, cmd := drag(m, g.detailX+1, g.detailY+1, g.detailX+g.detailW-1, g.detailY+len(vis)-1)
	if cmd == nil || !m.sel.shown {
		t.Fatal("拖动后松开应该复制并保留高亮")
	}
	got := m.selectedText()
	want := "\tx := 1\n\treturn fmt.Sprintf(\"%s-%s\", " + strings.Repeat("veryLongIdentifier, ", 6) + "end)\n\ty := 2"
	if got != want {
		t.Fatalf("复制的内容不对：\n got %q\nwant %q", got, want)
	}
	for _, bad := range []string{" 10 ", " 11 ", "+ ", "- "} {
		if strings.Contains(got, bad) {
			t.Fatalf("不应该带上行号或 +/- 栏（%q）：%q", bad, got)
		}
	}
	// 单行里选一部分：只复制选中的那几格
	m2, _ := drag(selModel(t), g.detailX+dlinePrefix(dCtx, g.detailW, 3)+4, g.detailY+1, g.detailX+dlinePrefix(dCtx, g.detailW, 3)+9, g.detailY+1)
	if s := m2.selectedText(); s != "x := 1" {
		t.Fatalf("部分选中应得到 %q，得到 %q", "x := 1", s)
	}
	// 按任意键取消高亮
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if nm.(Model).sel.visible() {
		t.Fatal("按键后选区应该消失")
	}
}

func TestClickIsNotSelection(t *testing.T) {
	m := selModel(t)
	g := m.geom()
	m, cmd := drag(m, g.detailX+10, g.detailY+1, g.detailX+10, g.detailY+1)
	if cmd != nil || m.sel.visible() {
		t.Fatal("原地按下松开是单击，不应复制")
	}
}

func TestSelectionStaysInPane(t *testing.T) {
	m := selModel(t)
	g := m.geom()
	// 从详情区拖到左边的列表里：终点被夹在详情区内
	m, _ = drag(m, g.detailX+20, g.detailY+1, 2, g.detailY+2)
	if m.sel.ex < g.detailX {
		t.Fatalf("选区不应跑到详情区左边：ex=%d detailX=%d", m.sel.ex, g.detailX)
	}
	// 高亮画在主界面上，且不改变每行宽度
	for i, line := range strings.Split(m.View(), "\n") {
		if w := textWidth(line); w != m.w {
			t.Fatalf("第 %d 行宽度 %d，应为 %d", i, w, m.w)
		}
	}
}
