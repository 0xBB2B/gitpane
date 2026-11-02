package gitx

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/0xBB2B/gitpane/internal/i18n"
)

// Tags 返回所有标签，按版本号从新到旧排序。
func (r *Repo) Tags() ([]string, error) {
	out, err := r.git("tag", "--list", "--sort=-v:refname")
	if err != nil {
		return nil, err
	}
	var tags []string
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			tags = append(tags, l)
		}
	}
	return tags, nil
}

var semverRe = regexp.MustCompile(`^(v?)(\d+)\.(\d+)\.(\d+)$`)

// SuggestTag 根据已有标签建议下一个版本号：找到最新的 vX.Y.Z（或 X.Y.Z），补丁号加一；
// 没有这种格式的标签时建议 v0.1.0。tags 要按版本号从新到旧排好（Tags 的返回值）。
func SuggestTag(tags []string) string {
	for _, t := range tags {
		m := semverRe.FindStringSubmatch(t)
		if m == nil {
			continue
		}
		patch, _ := strconv.Atoi(m[4])
		return m[1] + m[2] + "." + m[3] + "." + strconv.Itoa(patch+1)
	}
	return "v0.1.0"
}

// CheckTagName 检查标签名是否合法、是否已经存在。
func (r *Repo) CheckTagName(name string) error {
	if name == "" {
		return errors.New(i18n.T("标签名不能为空"))
	}
	if _, err := r.git("check-ref-format", "refs/tags/"+name); err != nil || strings.HasPrefix(name, "-") {
		return errors.New(i18n.Tf("「%s」不是合法的标签名", name))
	}
	if _, err := r.git("rev-parse", "--verify", "--quiet", "refs/tags/"+name); err == nil {
		return errors.New(i18n.Tf("标签 %s 已经存在", name))
	}
	return nil
}

// CreateTag 在 rev 上创建附注标签（附注就是标签名）。用附注标签是因为
// git push --follow-tags 只会带上附注标签，而且发版时 git describe 等也只认它。
func (r *Repo) CreateTag(name, rev string) error {
	_, err := r.git("tag", "-a", name, "-m", name, rev)
	return err
}

// defaultRemote 返回推送用的远程：有 origin 就用 origin，否则用第一个。
func (r *Repo) defaultRemote() (string, error) {
	remotes, _ := r.git("remote")
	remote := firstLine(remotes)
	if remote == "" {
		return "", errors.New(i18n.T("没有配置远程仓库"))
	}
	for _, rm := range strings.Fields(remotes) {
		if rm == "origin" {
			remote = "origin"
		}
	}
	return remote, nil
}

// PushTag 把一个标签推送到远程。
func (r *Repo) PushTag(name string) error {
	remote, err := r.defaultRemote()
	if err != nil {
		return err
	}
	_, err = r.gitSlow("push", remote, "refs/tags/"+name)
	return err
}
