package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSwitchPicker(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("没有 git")
	}
	base := t.TempDir()
	dir := filepath.Join(base, "app")
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
	git("commit", "-q", "--allow-empty", "-m", "init")
	git("branch", "dev")
	git("branch", "feature/login")
	git("worktree", "add", "-q", filepath.Join(base, "app-wt"), "feature/login")

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := New(dir, Options{})
	m.w, m.h = 120, 30
	m.worktrees, _ = m.repo.Worktrees()

	// 在变更页打开切换面板：分支列表异步加载，到了之后光标在当前分支上
	cmd := m.openSwitchPicker()
	if !m.picking || !m.pickSwitchMode || cmd == nil {
		t.Fatal("应该打开切换模式的分支面板，并加载分支列表")
	}
	nm, _ := m.Update(cmd())
	m = nm.(Model)
	rows := m.pickRows()
	if b := m.branches[rows[m.pickCursor].bi]; !b.Current || b.Name != "main" {
		t.Fatalf("光标应在当前分支 main 上：%+v", b)
	}
	// 筛选：/ 输入 log
	for _, k := range []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune{'/'}}, {Type: tea.KeyRunes, Runes: []rune("log")}} {
		nm, _ = m.Update(k)
		m = nm.(Model)
	}
	if rows := m.pickRows(); len(rows) != 2 || m.branches[rows[1].bi].Name != "feature/login" {
		t.Fatalf("筛选 log 后应只剩 feature/login：%d 行", len(rows))
	}
	// 它在另一个工作树里检出：回车直接切到那个工作树，而不是报错
	if m.branchWorktree(m.branches[m.pickRows()[1].bi]) < 0 {
		t.Fatal("feature/login 应该被认出是在另一个工作树里")
	}
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if m.picking || !strings.HasSuffix(filepath.ToSlash(m.repo.Root), "app-wt") {
		t.Fatalf("应该切到 app-wt 工作树：picking=%v root=%s", m.picking, m.repo.Root)
	}
}
