package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSwitchBranches(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("没有 git")
	}
	base := t.TempDir()
	git := func(dir string, args ...string) string {
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
	remote := filepath.Join(base, "remote.git")
	work := filepath.Join(base, "work")
	git(base, "init", "-q", "--bare", "-b", "main", remote)
	git(base, "init", "-q", "-b", "main", work)
	git(work, "remote", "add", "origin", remote)
	write := func(name, s string) {
		if err := os.WriteFile(filepath.Join(work, name), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.txt", "main\n")
	git(work, "add", ".")
	git(work, "commit", "-q", "-m", "init")
	git(work, "switch", "-q", "-c", "feature")
	write("a.txt", "feature\n")
	git(work, "commit", "-q", "-am", "feature change")
	git(work, "push", "-q", "origin", "feature", "main")
	git(work, "switch", "-q", "main")
	git(work, "branch", "-q", "-D", "feature") // 本地没有 feature，只剩 origin/feature

	r, err := Open(work)
	if err != nil {
		t.Fatal(err)
	}
	git(work, "fetch", "-q", "origin")
	// 有冲突的未提交改动：给出说明怎么办的错误，改动还在
	write("a.txt", "local edit\n")
	err = r.Switch(Branch{Name: "origin/feature", Remote: true})
	if err == nil || !strings.Contains(err.Error(), "a.txt") || strings.Contains(err.Error(), "error:") {
		t.Fatalf("冲突时应给出带文件名的说明：%v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(work, "a.txt")); string(b) != "local edit\n" {
		t.Fatal("切换失败时未提交的改动不能丢")
	}
	// 去掉冲突后：远程分支切过去会建立同名本地跟踪分支
	git(work, "checkout", "--", "a.txt")
	if err := r.Switch(Branch{Name: "origin/feature", Remote: true}); err != nil {
		t.Fatal(err)
	}
	if cur := strings.TrimSpace(git(work, "branch", "--show-current")); cur != "feature" {
		t.Fatalf("应该在本地 feature 分支上：%s", cur)
	}
	if up := strings.TrimSpace(git(work, "rev-parse", "--abbrev-ref", "feature@{u}")); up != "origin/feature" {
		t.Fatalf("应该跟踪 origin/feature：%s", up)
	}
	// 切回本地分支
	if err := r.Switch(Branch{Name: "main"}); err != nil {
		t.Fatal(err)
	}
}
