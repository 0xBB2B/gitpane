package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/0xBB2B/gitpane/internal/gitx"
)

func TestSwitchWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("没有 git")
	}
	base := t.TempDir()
	main := filepath.Join(base, "app")
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	_ = os.MkdirAll(main, 0o755)
	git(main, "init", "-q", "-b", "main")
	git(main, "commit", "-q", "--allow-empty", "-m", "init")
	feat := filepath.Join(base, "app-feat")
	git(main, "worktree", "add", "-q", "-b", "feat", feat)

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := New(main, Options{})
	m.w, m.h = 120, 30
	repo := m.repo
	m.worktrees, _ = repo.Worktrees()

	if b, ok := m.worktreeButton(); !ok || b.id != "worktree" {
		t.Fatal("有两个工作树时，变更页应该显示工作树按钮")
	}
	// 图谱的状态应该在切换后保留
	m.graphRefs = []string{"feat"}
	m.cursor[tabGraph] = 3
	m.cursor[tabChanges] = 5
	m.collapsed["x"] = true

	m.openWorktreePicker()
	if !m.wtPicking || m.wtCursor != 0 {
		t.Fatalf("打开面板时光标应在当前工作树：picking=%v cursor=%d", m.wtPicking, m.wtCursor)
	}
	m.switchWorktree(1)
	if m.wtPicking || !samePath(m.repo.Root, m.worktrees[1].Path) {
		t.Fatalf("应该切到 feat 工作树：%s", m.repo.Root)
	}
	if m.cursor[tabChanges] != 0 || len(m.collapsed) != 0 || m.tab != tabChanges {
		t.Fatal("切换后变更页的状态应该清空，并停在变更页")
	}
	if len(m.graphRefs) != 1 || m.cursor[tabGraph] != 3 {
		t.Fatal("切换工作树不应该影响图谱的筛选和位置")
	}
	if m.currentWorktree() != 1 {
		t.Fatal("当前工作树应该是第二个")
	}
	// 只有一个工作树时不显示按钮、按 t 只给提示
	m.worktrees = m.worktrees[:1]
	if _, ok := m.worktreeButton(); ok {
		t.Fatal("只有一个工作树时不应显示按钮")
	}
	m.openWorktreePicker()
	if m.wtPicking {
		t.Fatal("只有一个工作树时不应打开面板")
	}
	_ = gitx.Worktree{}
}
