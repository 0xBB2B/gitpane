package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0xBB2B/gitpane/internal/gitx"
)

func TestTagFlow(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("没有 git")
	}
	dir := filepath.Join(t.TempDir(), "app")
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	_ = os.MkdirAll(dir, 0o755)
	git("init", "-q", "-b", "main")
	git("commit", "-q", "--allow-empty", "-m", "one")
	git("tag", "-a", "v0.6.0", "-m", "v0.6.0")

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := New(dir, Options{})
	m.w, m.h = 120, 30

	// 不在图谱页：只给提示
	m.beginTag()
	if m.tagging {
		t.Fatal("不在图谱页时不应打开标签输入框")
	}

	m.tab = tabGraph
	m.graph, _ = m.repo.Graph(50, nil)
	m.cursor[tabGraph] = 0
	m.beginTag()
	if !m.tagging || m.tagSuggest != "v0.6.1" {
		t.Fatalf("应该打开输入框并建议 v0.6.1：tagging=%v suggest=%s", m.tagging, m.tagSuggest)
	}
	// 重名：报错，输入框保持打开
	m.input.SetValue("v0.6.0")
	if cmd := m.doTag(false); cmd != nil || !m.tagging || m.msgLevel != lvErr {
		t.Fatal("重名时应报错并保持输入框打开")
	}
	// 空输入回车 = 用建议的版本号
	m.input.SetValue("")
	cmd := m.doTag(false)
	if cmd == nil || m.tagging {
		t.Fatal("合法的标签名应该开始创建并关闭输入框")
	}
	// 执行这个操作（startOp 返回的批量命令里有真正干活的那个）
	for _, c := range flattenCmds(cmd) {
		if msg, ok := c().(opDoneMsg); ok && msg.err != nil {
			t.Fatal(msg.err)
		}
	}
	if !strings.Contains(git("tag", "--list"), "v0.6.1") {
		t.Fatal("v0.6.1 应该已经创建")
	}
	_ = gitx.SuggestTag
}

// flattenCmds 展开 tea.Batch，返回其中的各个命令（跳过 spinner 的 Tick 等不返回 opDoneMsg 的）。
func flattenCmds(cmd tea.Cmd) []tea.Cmd {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if b, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Cmd
		for _, c := range b {
			out = append(out, flattenCmds(c)...)
		}
		return out
	}
	return []tea.Cmd{func() tea.Msg { return msg }}
}
