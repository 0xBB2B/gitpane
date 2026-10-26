package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func mkdirs(t *testing.T, root string, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFindRepos(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root,
		"a/.git",              // 第 1 层
		"a/sub/.git",          // 仓库里面的仓库：不应列出
		"group/b/.git",        // 第 2 层
		"group/deep/c/.git",   // 第 3 层：depth=2 时找不到
		".hidden/d/.git",      // 隐藏目录：跳过
		"node_modules/e/.git", // 依赖目录：跳过
		"plain/nothing",
	)
	// worktree / submodule 用 .git 文件
	mkdirs(t, root, "wt")
	if err := os.WriteFile(filepath.Join(root, "wt", ".git"), []byte("gitdir: x"), 0o644); err != nil {
		t.Fatal(err)
	}

	rel := func(ps []string) []string {
		var out []string
		for _, p := range ps {
			out = append(out, Rel(root, p))
		}
		return out
	}
	got, trunc := FindRepos(root, 2)
	want := []string{"a", "group/b", "wt"}
	if !reflect.DeepEqual(rel(got), want) || trunc {
		t.Errorf("depth=2 得到 %v（截断 %v），期望 %v", rel(got), trunc, want)
	}
	got, _ = FindRepos(root, 3)
	want = []string{"a", "group/b", "group/deep/c", "wt"}
	if !reflect.DeepEqual(rel(got), want) {
		t.Errorf("depth=3 得到 %v，期望 %v", rel(got), want)
	}
	if got, _ := FindRepos(root, 0); len(got) != 0 {
		t.Errorf("depth=0 应该不扫描，得到 %v", got)
	}
}

func TestSummarize(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("没有 git")
	}
	dir := t.TempDir()
	gitc := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	gitc("init", "-q", "-b", "trunk")
	os.WriteFile(filepath.Join(dir, "a b.txt"), []byte("1"), 0o644)
	gitc("add", ".")
	gitc("commit", "-qm", "init")
	gitc("mv", "a b.txt", "c d.txt")                                // 重命名（路径带空格）
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("2"), 0o644) // 未跟踪
	s := Summarize(dir)
	if s.Err || s.Branch != "trunk" || s.Changes != 2 {
		t.Errorf("得到 %+v，期望 branch=trunk changes=2", s)
	}
}
