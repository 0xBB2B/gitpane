package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestWrapDLine(t *testing.T) {
	const w, digits = 40, 3 // w < 44：单列行号，前缀 digits+3 = 6，正文 34 列
	long := strings.Repeat("abcdefghij", 8)

	if wrapDLine(dline{kind: dAdd, text: "short"}, w, digits) != nil {
		t.Fatal("短行不应折行")
	}
	parts := wrapDLine(dline{kind: dAdd, text: long}, w, digits)
	if len(parts) != 3 || strings.Join(parts, "") != long {
		t.Fatalf("80 列代码应折成 34+34+12：%q", parts)
	}
	for _, p := range parts {
		if lipgloss.Width(p) > w-6 {
			t.Fatalf("折出的段超宽：%q", p)
		}
	}
	// 中文按显示宽度（每字 2 列）折
	cjk := strings.Repeat("中", 30)
	for _, p := range wrapDLine(dline{kind: dCtx, text: cjk}, w, digits) {
		if lipgloss.Width(p) > w-6 {
			t.Fatalf("中文折行超宽：%q", p)
		}
	}
	// 文件标题、hunk 不折
	if wrapDLine(dline{kind: dFile, path: long}, w, digits) != nil || wrapDLine(dline{kind: dHunk, text: long}, w, digits) != nil {
		t.Fatal("文件标题和 hunk 标题不参与折行")
	}
	// 渲染：每一屏幕行恰好 w 列，后续行不带行号和 + 号
	first := renderDLine(dline{kind: dAdd, new: 7, text: parts[0]}, w, digits)
	cont := renderDLine(dline{kind: dAdd, new: 7, text: parts[1], cont: true}, w, digits)
	if lipgloss.Width(first) != w || lipgloss.Width(cont) != w {
		t.Fatal("渲染宽度不对")
	}
	if !strings.Contains(stripANSI(first), "7 +") || strings.Contains(stripANSI(cont), "7") || strings.Contains(stripANSI(cont), "+") {
		t.Fatalf("后续行不应有行号和符号：%q / %q", stripANSI(first), stripANSI(cont))
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	esc := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			esc = true
		case esc && (r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'):
			esc = false
		case !esc:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestToggleWrapKeepsTopLine(t *testing.T) {
	m := newTestModel(t)
	m.tab = tabGraph
	long := strings.Repeat("x", 300)
	m.detailKey = "k"
	m.detail = detailView{key: "k", digits: 3}
	for i := 0; i < 60; i++ {
		m.detail.lines = append(m.detail.lines, dline{kind: dAdd, new: i + 1, text: long})
	}
	if !m.wrap {
		t.Fatal("没有保存过偏好时应默认开启换行")
	}
	m.wrap = false
	m.detailOff = 10
	m.toggleWrap()
	vis := m.visibleDetail()
	if !m.wrap || len(vis) <= 60 {
		t.Fatalf("开启换行后屏幕行应变多：%d", len(vis))
	}
	if vis[m.detailOff].src != 10 || vis[m.detailOff].cont {
		t.Fatalf("顶部应仍是第 10 行的开头，实际 src=%d cont=%v", vis[m.detailOff].src, vis[m.detailOff].cont)
	}
	m.detailOff += 2 // 停在第 10 行的折行中间
	m.toggleWrap()
	if m.wrap || m.detailOff != 10 {
		t.Fatalf("关闭换行后应回到第 10 行，实际 %d", m.detailOff)
	}
	// 偏好写进了配置文件，重新读取能拿到
	m.toggleWrap()
	m.toggleWrap() // 再关掉：关闭也要能保存下来，下次启动不会回到默认的开启
	if _, dp := loadLayout(); dp.wrapOn() {
		t.Fatal("关闭换行的偏好没有保存")
	}
}

func TestLayoutFileCompat(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	p := layoutFile()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	// 旧版本写的文件（没有 detail 键）照常读取
	if err := os.WriteFile(p, []byte(`{"changes":{"wide":0.3},"graph":{"stack":0.5}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	sp, dp := loadLayout()
	if sp[tabChanges].Wide != 0.3 || sp[tabGraph].Stack != 0.5 || !dp.wrapOn() {
		t.Fatalf("读旧文件出错：%+v %+v", sp, dp)
	}
	// 新文件里的 detail 键，按旧版本的结构解析也不会出错（旧版本会忽略它）
	m := Model{split: sp, wrap: true}
	m.saveLayout()
	data, _ := os.ReadFile(p)
	var old map[string]splitPref
	if err := json.Unmarshal(data, &old); err != nil {
		t.Fatalf("旧版本无法解析新文件：%v\n%s", err, data)
	}
	if old["changes"].Wide != 0.3 {
		t.Fatalf("旧版本读到的比例不对：%s", data)
	}
}

func TestStatSummary(t *testing.T) {
	cases := []struct {
		in       string
		text     string
		add, del int
	}{
		{"2 files changed, 74 insertions(+), 7 deletions(-)", "改动了 2 个文件", 74, 7},
		{"1 file changed, 1 insertion(+)", "改动了 1 个文件", 1, 0},
		{"1 file changed, 3 deletions(-)", "改动了 1 个文件", 0, 3},
		{"something else", "something else", 0, 0},
	}
	for _, c := range cases {
		l := statSummary(c.in)
		if l.text != c.text || l.add != c.add || l.del != c.del {
			t.Errorf("%q → %+v", c.in, l)
		}
	}
}

func TestRightAlignCJKTruncate(t *testing.T) {
	left := []seg{sg(" ● 4c63f65 ", fg(cYellow)), sg("状态面板第一行显示任务进度，决策包里提示轮数上限", fg(cText))}
	right := []seg{sg(" 7 天前 ", fg(cFaint))}
	for w := 20; w <= 60; w++ {
		line := stripANSI(rightAlign(w, nil, left, right))
		if got := lipgloss.Width(line); got != w {
			t.Fatalf("w=%d：行宽 %d", w, got)
		}
		if !strings.HasSuffix(line, " 7 天前 ") {
			t.Fatalf("w=%d：时间没有贴右：%q", w, line)
		}
	}
}
