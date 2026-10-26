// Package gitx 封装面板需要的 git 命令。所有命令都在仓库根目录执行，
// 并禁用可选锁，避免和 Claude Code 同时操作仓库时抢 index.lock。
package gitx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/0xBB2B/gitpane/internal/i18n"
)

// ErrNotRepo 表示当前目录不在 git 仓库中。
var ErrNotRepo = errors.New("不是 Git 仓库")

// Repo 是一个 git 工作区。
type Repo struct {
	Root string
}

// Open 从任意子目录定位仓库根目录。
func Open(dir string) (*Repo, error) {
	out, err := run(context.Background(), dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, ErrNotRepo
	}
	return &Repo{Root: strings.TrimSpace(out)}, nil
}

// Init 在 dir 中执行 git init。
func Init(dir string) error {
	_, err := run(context.Background(), dir, "init")
	return err
}

func run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0", // 需要输密码时直接失败，而不是卡住面板
		"GIT_OPTIONAL_LOCKS=0",
		"LC_ALL=C.UTF-8",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), errors.New(msg)
	}
	return stdout.String(), nil
}

func (r *Repo) git(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return run(ctx, r.Root, args...)
}

func (r *Repo) gitSlow(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	return run(ctx, r.Root, args...)
}

// ---------------------------------------------------------------- status

// File 是一条 porcelain 状态记录。X 为暂存区状态，Y 为工作区状态。
type File struct {
	Path     string
	OrigPath string
	X, Y     byte
}

func (f File) Untracked() bool { return f.X == '?' }

func (f File) Conflicted() bool {
	switch string([]byte{f.X, f.Y}) {
	case "DD", "AU", "UD", "UA", "DU", "AA", "UU":
		return true
	}
	return false
}

// Staged 表示暂存区有改动。
func (f File) Staged() bool {
	return !f.Conflicted() && f.X != ' ' && f.X != '?' && f.X != '!'
}

// Unstaged 表示工作区有改动（含未跟踪）。
func (f File) Unstaged() bool {
	return !f.Conflicted() && f.Y != ' '
}

// Status 是仓库整体状态。
type Status struct {
	Branch    string
	Upstream  string
	Ahead     int
	Behind    int
	Detached  bool
	NoCommits bool
	Gone      bool
	Files     []File
}

// Status 读取工作区状态。
func (r *Repo) Status() (*Status, error) {
	out, err := r.git("status", "--porcelain=v1", "-b", "-z", "-uall")
	if err != nil {
		return nil, err
	}
	st := &Status{}
	parts := strings.Split(out, "\x00")
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "## ") {
			parseBranch(st, p[3:])
			continue
		}
		if len(p) < 4 {
			continue
		}
		f := File{X: p[0], Y: p[1], Path: p[3:]}
		if f.X == 'R' || f.X == 'C' || f.Y == 'R' || f.Y == 'C' {
			if i+1 < len(parts) {
				f.OrigPath = parts[i+1]
				i++
			}
		}
		st.Files = append(st.Files, f)
	}
	return st, nil
}

func parseBranch(st *Status, s string) {
	switch {
	case strings.HasPrefix(s, "No commits yet on "):
		st.NoCommits = true
		st.Branch = strings.TrimPrefix(s, "No commits yet on ")
		return
	case strings.HasPrefix(s, "Initial commit on "):
		st.NoCommits = true
		st.Branch = strings.TrimPrefix(s, "Initial commit on ")
		return
	case strings.HasPrefix(s, "HEAD (no branch)"):
		st.Detached = true
		st.Branch = "HEAD"
		return
	}
	track := ""
	if i := strings.Index(s, " ["); i >= 0 {
		track = strings.TrimSuffix(s[i+2:], "]")
		s = s[:i]
	}
	if i := strings.Index(s, "..."); i >= 0 {
		st.Branch, st.Upstream = s[:i], s[i+3:]
	} else {
		st.Branch = s
	}
	for _, t := range strings.Split(track, ", ") {
		switch {
		case strings.HasPrefix(t, "ahead "):
			st.Ahead, _ = strconv.Atoi(strings.TrimPrefix(t, "ahead "))
		case strings.HasPrefix(t, "behind "):
			st.Behind, _ = strconv.Atoi(strings.TrimPrefix(t, "behind "))
		case t == "gone":
			st.Gone = true
		}
	}
}

// ---------------------------------------------------------------- diff

const maxDiffBytes = 2 << 20

// Diff 返回单个文件的彩色 diff。
func (r *Repo) Diff(f File, staged bool) (string, error) {
	var out string
	var err error
	switch {
	case f.Untracked():
		// --no-index 有差异时退出码为 1，属于正常情况
		out, err = r.git("diff", "--no-index", "--no-color", "--", "/dev/null", f.Path)
		if out != "" {
			err = nil
		}
	case staged:
		paths := []string{f.Path}
		if f.OrigPath != "" {
			paths = append(paths, f.OrigPath) // 带上原路径才能识别为重命名
		}
		out, err = r.git(append([]string{"diff", "--cached", "--no-color", "-M", "--"}, paths...)...)
	default:
		out, err = r.git("diff", "--no-color", "--", f.Path)
	}
	if len(out) > maxDiffBytes {
		out = out[:maxDiffBytes] + i18n.T("\n…（diff 过大，已截断）")
	}
	return out, err
}

// Commit 是一个提交的元信息。
type CommitInfo struct {
	Hash, Short, Author, Email, When, Date, Refs, Subject, Body string
}

// Show 返回提交元信息，以及（无颜色的）统计 + 补丁文本。
func (r *Repo) Show(rev string) (*CommitInfo, string, error) {
	head, err := r.git("show", "-s", "--format=%H%x1f%h%x1f%an%x1f%ae%x1f%at%x1f%ad%x1f%D%x1f%s%x1f%b", "--date=format:%Y-%m-%d %H:%M", rev)
	if err != nil {
		return nil, "", err
	}
	f := strings.SplitN(strings.TrimRight(head, "\n"), "\x1f", 9)
	for len(f) < 9 {
		f = append(f, "")
	}
	ci := &CommitInfo{Hash: f[0], Short: f[1], Author: f[2], Email: f[3], When: ago(f[4]), Date: f[5], Refs: f[6], Subject: f[7], Body: strings.TrimSpace(f[8])}
	patch, err := r.git("show", "--no-color", "--format=", "--stat=1000", "-p", "-M", rev)
	if len(patch) > maxDiffBytes {
		patch = patch[:maxDiffBytes] + i18n.T("\n…（输出过大，已截断）")
	}
	return ci, patch, err
}

// Stat 是单个文件的增删行数；二进制文件为 -1。
type Stat struct{ Add, Del int }

// NumStat 返回工作区（未暂存）和暂存区每个文件的增删行数。
func (r *Repo) NumStat() (unstaged, staged map[string]Stat) {
	parse := func(out string) map[string]Stat {
		m := map[string]Stat{}
		parts := strings.Split(out, "\x00")
		for i := 0; i < len(parts); i++ {
			p := parts[i]
			if p == "" {
				continue
			}
			f := strings.SplitN(p, "\t", 3)
			if len(f) < 3 {
				continue
			}
			st := Stat{Add: -1, Del: -1}
			if f[0] != "-" {
				st.Add, _ = strconv.Atoi(f[0])
				st.Del, _ = strconv.Atoi(f[1])
			}
			path := f[2]
			if path == "" && i+2 < len(parts) { // 重命名：路径为空，后面跟 旧路径\0新路径
				path = parts[i+2]
				i += 2
			}
			m[path] = st
		}
		return m
	}
	u, _ := r.git("diff", "--numstat", "-z")
	s, _ := r.git("diff", "--cached", "--numstat", "-z", "-M")
	return parse(u), parse(s)
}

// UntrackedLines 统计未跟踪文本文件的行数（太大的文件跳过）。
func (r *Repo) UntrackedLines(path string) int {
	fi, err := os.Stat(r.Root + "/" + path)
	if err != nil || fi.Size() > 1<<20 || fi.IsDir() {
		return -1
	}
	b, err := os.ReadFile(r.Root + "/" + path)
	if err != nil || bytes.IndexByte(b, 0) >= 0 {
		return -1
	}
	n := bytes.Count(b, []byte("\n"))
	if len(b) > 0 && b[len(b)-1] != '\n' {
		n++
	}
	return n
}

// ---------------------------------------------------------------- 暂存与提交

func (r *Repo) hasHead() bool {
	_, err := r.git("rev-parse", "--verify", "-q", "HEAD")
	return err == nil
}

// Stage 暂存指定路径（含删除）。
func (r *Repo) Stage(paths ...string) error {
	_, err := r.git(append([]string{"add", "-A", "--"}, paths...)...)
	return err
}

// Unstage 取消暂存指定路径。
func (r *Repo) Unstage(paths ...string) error {
	if !r.hasHead() {
		_, err := r.git(append([]string{"rm", "-r", "-q", "--cached", "--"}, paths...)...)
		return err
	}
	_, err := r.git(append([]string{"reset", "-q", "HEAD", "--"}, paths...)...)
	return err
}

func (r *Repo) StageAll() error {
	_, err := r.git("add", "-A")
	return err
}

func (r *Repo) UnstageAll() error {
	if !r.hasHead() {
		_, err := r.git("rm", "-r", "-q", "--cached", ".")
		return err
	}
	_, err := r.git("reset", "-q")
	return err
}

// Discard 丢弃工作区改动；未跟踪文件会被删除。
func (r *Repo) Discard(f File) error {
	if f.Untracked() {
		_, err := r.git("clean", "-f", "-q", "--", f.Path)
		return err
	}
	_, err := r.git("checkout", "-q", "--", f.Path)
	return err
}

func (r *Repo) Commit(msg string) (string, error) {
	out, err := r.git("commit", "-q", "-m", msg)
	if err != nil {
		return "", err
	}
	_ = out
	sha, _ := r.git("log", "-1", "--format=%h %s")
	return strings.TrimSpace(sha), nil
}

// ---------------------------------------------------------------- 远程

func (r *Repo) Pull() (string, error) {
	out, err := r.gitSlow("pull", "--no-edit")
	return lastLine(out), err
}

func (r *Repo) Fetch() error {
	_, err := r.gitSlow("fetch", "--all", "--prune")
	return err
}

// Push 推送当前分支；没有上游时自动设置为 origin/<当前分支>。
func (r *Repo) Push(st *Status) error {
	if st != nil && st.Upstream == "" && !st.Detached {
		remotes, _ := r.git("remote")
		remote := firstLine(remotes)
		if remote == "" {
			return errors.New(i18n.T("没有配置远程仓库"))
		}
		for _, rm := range strings.Fields(remotes) {
			if rm == "origin" {
				remote = "origin"
			}
		}
		_, err := r.gitSlow("push", "-u", remote, "HEAD")
		return err
	}
	_, err := r.gitSlow("push")
	return err
}

// ---------------------------------------------------------------- 分支与图谱

type Branch struct {
	Name     string
	Current  bool
	Remote   bool
	Upstream string
	Track    string
	When     string
}

// Branches 返回本地分支（按最近提交排序），后面跟远程分支。
func (r *Repo) Branches() ([]Branch, error) {
	out, err := r.git("for-each-ref", "--sort=-committerdate",
		"--format=%(HEAD)%00%(refname)%00%(refname:short)%00%(upstream:short)%00%(upstream:track)%00%(committerdate:unix)",
		"refs/heads", "refs/remotes")
	if err != nil {
		return nil, err
	}
	var local, remote []Branch
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		f := strings.Split(line, "\x00")
		if len(f) < 6 {
			continue
		}
		b := Branch{Current: f[0] == "*", Name: f[2], Upstream: f[3], Track: f[4], When: ago(f[5])}
		if strings.HasPrefix(f[1], "refs/remotes/") {
			if strings.HasSuffix(f[1], "/HEAD") {
				continue
			}
			b.Remote = true
			remote = append(remote, b)
			continue
		}
		local = append(local, b)
	}
	return append(local, remote...), nil
}

// Switch 切换分支；远程分支会自动建立同名的本地跟踪分支。
func (r *Repo) Switch(b Branch) error {
	if !b.Remote {
		_, err := r.git("switch", b.Name)
		return err
	}
	short := b.Name
	if i := strings.IndexByte(short, '/'); i >= 0 {
		short = short[i+1:]
	}
	if _, err := r.git("rev-parse", "--verify", "-q", "refs/heads/"+short); err == nil {
		_, err := r.git("switch", short)
		return err
	}
	_, err := r.git("switch", "--track", b.Name)
	return err
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
}

// ago 把 git 给的 Unix 时间戳转成当前语言的相对时间（git 自己的 %ar 只有英文）。
func ago(unix string) string {
	sec, err := strconv.ParseInt(strings.TrimSpace(unix), 10, 64)
	if err != nil {
		return unix
	}
	return i18n.Ago(time.Unix(sec, 0))
}

// Describe 用于错误提示中截短 git 的多行输出。
func Describe(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprint(firstLine(err.Error()))
}
