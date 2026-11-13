package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/0xBB2B/gitpane/internal/gitx"
)

func menuTestModel(t *testing.T) Model {
	m := newTestModel(t)
	m.repo = &gitx.Repo{Root: t.TempDir()}
	m.tab = tabGraph
	m.branches = []gitx.Branch{
		{Name: "main", Current: true},
		{Name: "feature/a"},
		{Name: "origin/main", Remote: true},
		{Name: "origin/feature/a", Remote: true},
		{Name: "origin/feature/b", Remote: true},
	}
	m.graph = []gitx.GraphLine{
		{Hash: "aaaaaaa", Full: "aaaaaaa1", Subject: "一个很长的中文提交说明，用来测试菜单盖住宽字符", Head: true,
			Refs: "HEAD -> main, origin/main, feature/a, origin/feature/a, origin/feature/b, tag: v1.0.0, origin/HEAD"},
		{Hash: "bbbbbbb", Full: "bbbbbbb1", Subject: "第二个提交"},
	}
	return m
}

func TestCommitBranches(t *testing.T) {
	m := menuTestModel(t)
	var got []string
	for _, b := range m.commitBranches(m.graph[0]) {
		got = append(got, b.Name)
	}
	// 当前分支 main、有同名本地分支的 origin/main / origin/feature/a、标签、origin/HEAD 都不列
	if want := "feature/a,origin/feature/b"; strings.Join(got, ",") != want {
		t.Fatalf("可切换的分支 = %v，想要 %s", got, want)
	}
	if len(m.commitBranches(m.graph[1])) != 0 {
		t.Fatal("没有分支的提交不应列出分支")
	}
}

func TestMenuOverlayKeepsLineWidth(t *testing.T) {
	m := menuTestModel(t)
	// 在不同位置打开菜单（包括贴着右边和底边，要自动挪回来），每一行都必须还是正好 m.w 宽
	for _, p := range [][2]int{{3, 2}, {7, 3}, {115, 2}, {40, 28}} {
		m.closeMenu()
		m.openMenu(0, p[0], p[1])
		if !m.menu.open {
			t.Fatal("菜单没有打开")
		}
		if m.menu.x+m.menu.width() > m.w || m.menu.y+m.menu.height() > m.h {
			t.Fatalf("菜单超出窗口：%+v", m.menu)
		}
		for i, l := range strings.Split(m.View(), "\n") {
			if w := textWidth(ansi.Strip(l)); w != m.w {
				t.Fatalf("在 %v 打开菜单后第 %d 行宽 %d，应为 %d：%q", p, i, w, m.w, ansi.Strip(l))
			}
		}
	}
}

func TestMenuConfirmAndClose(t *testing.T) {
	m := menuTestModel(t)
	m.openMenu(0, 5, 3)
	// Esc 关闭
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m = nm.(Model); m.menu.open {
		t.Fatal("Esc 应关闭菜单")
	}
	// 需要确认的项：第一次只上膛，菜单不关
	m.openMenu(0, 5, 3)
	m.menu.items = append(m.menu.items, menuItem{id: "discard", label: "x", confirm: "再点一次"})
	i := len(m.menu.items) - 1
	m.runMenu(i)
	if !m.menu.open || m.menu.armed != i {
		t.Fatal("第一次点需要确认的项应只上膛")
	}
	// 点菜单外面只关菜单
	nm, _ = m.Update(tea.MouseMsg{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if m = nm.(Model); m.menu.open {
		t.Fatal("点菜单外面应关闭菜单")
	}
}

// 菜单 / 选区叠在中英文混排的行上：无论边界切在哪一格（包括宽字中间），整行宽度都不能变。
func TestSpliceCellsWideChars(t *testing.T) {
	const total = 40
	line := "\x1b[33mab\x1b[0m中文提交说明x \x1b[1m重叠拒绝（#529）\x1b[0m时间"
	line += strings.Repeat(" ", total-textWidth(ansi.Strip(line)))
	for x := 0; x < total; x++ {
		for w := 1; x+w <= total; w++ {
			got := spliceCells(line, x, w, total, strings.Repeat("@", w))
			plain := ansi.Strip(got)
			if gw := textWidth(plain); gw != total {
				t.Fatalf("x=%d w=%d：行宽 %d，应为 %d：%q", x, w, gw, total, plain)
			}
			if idx := strings.Index(plain, strings.Repeat("@", w)); textWidth(plain[:idx]) != x {
				t.Fatalf("x=%d w=%d：菜单没有从第 %d 格开始：%q", x, w, x, plain)
			}
		}
	}
}
