package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseWorktrees(t *testing.T) {
	out := "worktree /code/app\nHEAD 1111111111111111111111111111111111111111\nbranch refs/heads/main\n\n" +
		"worktree /code/app-feat\nHEAD 2222222222222222222222222222222222222222\nbranch refs/heads/feature/login\nlocked\n\n" +
		"worktree /code/app-fix\nHEAD 3333333333333333333333333333333333333333\ndetached\n\n" +
		"worktree /code/gone\nHEAD 4444444444444444444444444444444444444444\nbranch refs/heads/old\nprunable gitdir file points to non-existent location\n\n"
	ws := parseWorktrees(out)
	if len(ws) != 3 {
		t.Fatalf("应该有 3 个工作树（prunable 的不算），得到 %d：%+v", len(ws), ws)
	}
	if ws[0].Path != "/code/app" || ws[0].Label() != "main" {
		t.Errorf("主工作树不对：%+v", ws[0])
	}
	if ws[1].Branch != "feature/login" || !ws[1].Locked {
		t.Errorf("第二个工作树不对：%+v", ws[1])
	}
	if !ws[2].Detached || ws[2].Label() != "3333333" {
		t.Errorf("游离 HEAD 的工作树应显示短哈希：%+v", ws[2])
	}
}

func TestWorktreesReal(t *testing.T) {
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
	if err := os.MkdirAll(main, 0o755); err != nil {
		t.Fatal(err)
	}
	git(main, "init", "-q", "-b", "main")
	git(main, "commit", "-q", "--allow-empty", "-m", "init")
	git(main, "worktree", "add", "-q", "-b", "feat", filepath.Join(base, "app-feat"))

	r, err := Open(main)
	if err != nil {
		t.Fatal(err)
	}
	ws, err := r.Worktrees()
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 2 || ws[0].Branch != "main" || ws[1].Branch != "feat" {
		t.Fatalf("工作树列表不对：%+v", ws)
	}
	// 从另一个工作树打开，Root 应该是那个工作树的目录，列表一样
	r2, err := Open(ws[1].Path)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Root != ws[1].Path {
		t.Fatalf("Root 应该是工作树目录：%s vs %s", r2.Root, ws[1].Path)
	}
	if ws2, _ := r2.Worktrees(); len(ws2) != 2 {
		t.Fatalf("从工作树里看到的列表也应该有 2 个：%+v", ws2)
	}
}
