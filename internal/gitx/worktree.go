package gitx

import "strings"

// Worktree 是一个工作树（git worktree）。同一个仓库的工作树共享提交历史，
// 但各自检出一个分支、有各自的未提交改动。
type Worktree struct {
	Path     string // 工作树根目录（git 给的格式，Windows 上也是正斜杠）
	Branch   string // 检出的分支（短名）；游离 HEAD 时为空
	Head     string // HEAD 提交的完整哈希
	Detached bool
	Locked   bool
}

// Worktrees 列出这个仓库的所有工作树，第一个是主工作树。
// 裸仓库和目录已经不存在的工作树（prunable）不列出来：它们没法切过去看改动。
func (r *Repo) Worktrees() ([]Worktree, error) {
	out, err := r.git("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parseWorktrees(out), nil
}

func parseWorktrees(out string) []Worktree {
	var list []Worktree
	var cur *Worktree
	skip := false
	flush := func() {
		if cur != nil && !skip {
			list = append(list, *cur)
		}
		cur, skip = nil, false
	}
	for _, line := range strings.Split(strings.ReplaceAll(out, "\r", ""), "\n") {
		key, val, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			flush()
			cur = &Worktree{Path: val}
		case "HEAD":
			if cur != nil {
				cur.Head = val
			}
		case "branch":
			if cur != nil {
				cur.Branch = strings.TrimPrefix(val, "refs/heads/")
			}
		case "detached":
			if cur != nil {
				cur.Detached = true
			}
		case "locked":
			if cur != nil {
				cur.Locked = true
			}
		case "bare", "prunable":
			skip = true
		case "":
			flush()
		}
	}
	flush()
	return list
}

// Label 是工作树在界面上的名字：分支名，游离时是短哈希。
func (w Worktree) Label() string {
	if w.Branch != "" {
		return w.Branch
	}
	if len(w.Head) >= 7 {
		return w.Head[:7]
	}
	return w.Head
}
