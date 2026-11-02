package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSuggestTag(t *testing.T) {
	cases := []struct {
		tags []string
		want string
	}{
		{nil, "v0.1.0"},
		{[]string{"v0.6.0", "v0.5.0"}, "v0.6.1"},
		{[]string{"nightly", "v1.2.9", "v1.2.8"}, "v1.2.10"},
		{[]string{"2.0.3"}, "2.0.4"},
		{[]string{"release-1"}, "v0.1.0"},
	}
	for _, c := range cases {
		if got := SuggestTag(c.tags); got != c.want {
			t.Errorf("%v: got %s want %s", c.tags, got, c.want)
		}
	}
}

func TestTagCreateAndPush(t *testing.T) {
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
	// 附注标签要记录打标签的人；CI 机器上没有全局身份，在仓库里配一个
	git(work, "config", "user.name", "t")
	git(work, "config", "user.email", "t@t")
	git(work, "commit", "-q", "--allow-empty", "-m", "one")
	git(work, "commit", "-q", "--allow-empty", "-m", "two")
	git(work, "push", "-q", "-u", "origin", "main")

	r, err := Open(work)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "has space", "a..b", "-x", "end."} {
		if r.CheckTagName(bad) == nil {
			t.Errorf("%q 应该是不合法的标签名", bad)
		}
	}
	if err := r.CheckTagName("v1.0.0"); err != nil {
		t.Fatalf("v1.0.0 应该可用：%v", err)
	}
	if err := r.CreateTag("v1.0.0", "HEAD~1"); err != nil {
		t.Fatal(err)
	}
	if r.CheckTagName("v1.0.0") == nil {
		t.Fatal("已存在的标签应该报错")
	}
	if typ := strings.TrimSpace(git(work, "cat-file", "-t", "v1.0.0")); typ != "tag" {
		t.Fatalf("应该是附注标签（tag 对象），实际 %s", typ)
	}
	if tags, _ := r.Tags(); len(tags) != 1 || SuggestTag(tags) != "v1.0.1" {
		t.Fatalf("标签列表 / 建议不对：%v", tags)
	}
	// 单独推送一个标签
	if err := r.PushTag("v1.0.0"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(git(work, "ls-remote", "--tags", "origin"), "refs/tags/v1.0.0") {
		t.Fatal("v1.0.0 应该已经推送到远程")
	}
	// 分支已经是最新时，推送分支也会带上新建的附注标签
	if err := r.CreateTag("v1.1.0", "HEAD"); err != nil {
		t.Fatal(err)
	}
	st, err := r.Status()
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Push(st); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(git(work, "ls-remote", "--tags", "origin"), "refs/tags/v1.1.0") {
		t.Fatal("推送分支时应该顺带推送 v1.1.0")
	}
}
