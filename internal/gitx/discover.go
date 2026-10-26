package gitx

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MaxRepos 是一次扫描最多返回的仓库数，防止在巨大的目录树里扫太久。
const MaxRepos = 500

// skipDirs 是扫描时不进入的目录（依赖、构建缓存等，里面几乎不会有要管理的仓库）。
var skipDirs = map[string]bool{
	"node_modules": true, "vendor": true, "Pods": true, "__pycache__": true,
	"target": true, "venv": true,
}

// FindRepos 在 root 下最多 depth 层子目录里查找 Git 仓库，返回按路径排序的绝对路径。
//
// 规则：
//   - 目录里有 .git（目录或文件，后者是 worktree / submodule）就算仓库，并且不再往它里面找；
//   - 跳过隐藏目录、skipDirs 里的目录和符号链接（避免循环）；
//   - 最多返回 MaxRepos 个，truncated 表示因此提前停止。
func FindRepos(root string, depth int) (repos []string, truncated bool) {
	if depth <= 0 {
		return nil, false
	}
	type item struct {
		dir   string
		level int
	}
	queue := []item{{root, 0}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.level >= depth {
			continue
		}
		entries, err := os.ReadDir(cur.dir)
		if err != nil {
			continue // 没权限之类，跳过
		}
		for _, e := range entries {
			name := e.Name()
			if !e.IsDir() || strings.HasPrefix(name, ".") || skipDirs[name] {
				continue // e.IsDir() 对符号链接返回 false，所以不会跟随链接
			}
			p := filepath.Join(cur.dir, name)
			if isRepo(p) {
				repos = append(repos, p)
				if len(repos) >= MaxRepos {
					sort.Strings(repos)
					return repos, true
				}
				continue
			}
			queue = append(queue, item{p, cur.level + 1})
		}
	}
	sort.Strings(repos)
	return repos, false
}

func isRepo(dir string) bool {
	_, err := os.Lstat(filepath.Join(dir, ".git"))
	return err == nil
}

// Summary 是仓库选择面板里显示的概要。
type Summary struct {
	Branch  string
	Changes int // 有改动的文件数（含未跟踪）
	Err     bool
}

// Summarize 读取仓库的当前分支和改动文件数；不扫描未跟踪目录内部，保持快速。
func Summarize(dir string) Summary {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := run(ctx, dir, "status", "--porcelain=v1", "-b", "-z", "--untracked-files=normal")
	if err != nil {
		return Summary{Err: true}
	}
	var s Summary
	parts := strings.Split(out, "\x00")
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		switch {
		case p == "":
		case strings.HasPrefix(p, "## "):
			st := &Status{}
			parseBranch(st, p[3:])
			s.Branch = st.Branch
		case len(p) > 3:
			s.Changes++
			if p[0] == 'R' || p[0] == 'C' || p[1] == 'R' || p[1] == 'C' {
				i++ // 重命名 / 复制后面紧跟原路径，跳过
			}
		}
	}
	return s
}

// Rel 返回 p 相对 root 的路径（用于显示）；失败时返回原路径。
func Rel(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil {
		return filepath.ToSlash(r)
	}
	return p
}
