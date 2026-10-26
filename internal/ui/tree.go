package ui

import (
	"path"
	"sort"
	"strings"

	"github.com/0xBB2B/gitpane/internal/gitx"
	"github.com/0xBB2B/gitpane/internal/i18n"
)

type section int

const (
	secConflict section = iota
	secStaged
	secChanges
)

func (s section) title() string {
	switch s {
	case secConflict:
		return i18n.T("冲突")
	case secStaged:
		return i18n.T("已暂存")
	default:
		return i18n.T("变更")
	}
}

type rowKind int

const (
	rowSection rowKind = iota
	rowDir
	rowFile
	rowEmpty
	rowClean // 工作区干净时的占位行
)

// row 是变更列表里渲染的一行。
type row struct {
	kind      rowKind
	sec       section
	depth     int
	name      string
	path      string    // 目录或文件的完整相对路径
	file      gitx.File // 仅 rowFile 有效
	count     int       // 目录/分组下的文件数
	files     []string  // 目录/分组下所有文件路径，用于批量暂存
	collapsed bool
	guide     string // 树形连线前缀，如 "│ ├ "
}

// key 用分组编号而不是显示名，切换界面语言不影响折叠状态。
func (r row) key() string { return r.sec.id() + "\x00" + r.path }

func (s section) id() string { return string(rune('0' + int(s))) }

type node struct {
	name  string
	path  string
	dirs  map[string]*node
	files []gitx.File
}

func newNode(name, p string) *node { return &node{name: name, path: p, dirs: map[string]*node{}} }

func (n *node) add(f gitx.File) {
	parts := strings.Split(f.Path, "/")
	cur := n
	for _, d := range parts[:len(parts)-1] {
		next, ok := cur.dirs[d]
		if !ok {
			next = newNode(d, path.Join(cur.path, d))
			cur.dirs[d] = next
		}
		cur = next
	}
	cur.files = append(cur.files, f)
}

func (n *node) allFiles() []string {
	var out []string
	for _, f := range n.files {
		out = append(out, f.Path)
	}
	for _, d := range n.dirs {
		out = append(out, d.allFiles()...)
	}
	return out
}

// buildRows 把状态分成冲突/已暂存/变更三组，每组按目录树展开。
func buildRows(st *gitx.Status, collapsed map[string]bool) []row {
	groups := map[section][]gitx.File{}
	for _, f := range st.Files {
		if f.Conflicted() {
			groups[secConflict] = append(groups[secConflict], f)
			continue
		}
		if f.Staged() {
			groups[secStaged] = append(groups[secStaged], f)
		}
		if f.Unstaged() {
			groups[secChanges] = append(groups[secChanges], f)
		}
	}

	if len(st.Files) == 0 {
		return []row{{kind: rowClean}}
	}
	var rows []row
	for _, sec := range []section{secConflict, secStaged, secChanges} {
		files := groups[sec]
		if sec == secConflict && len(files) == 0 {
			continue
		}
		root := newNode("", "")
		for _, f := range files {
			root.add(f)
		}
		hdr := row{kind: rowSection, sec: sec, name: sec.title(), count: len(files), files: root.allFiles()}
		hdr.collapsed = collapsed[hdr.key()]
		rows = append(rows, hdr)
		if hdr.collapsed {
			continue
		}
		if len(files) == 0 {
			rows = append(rows, row{kind: rowEmpty, sec: sec, depth: 1})
			continue
		}
		rows = appendNode(rows, root, sec, 1, "", collapsed)
	}
	return rows
}

func appendNode(rows []row, n *node, sec section, depth int, prefix string, collapsed map[string]bool) []row {
	names := make([]string, 0, len(n.dirs))
	for k := range n.dirs {
		names = append(names, k)
	}
	sort.Strings(names)
	sort.Slice(n.files, func(i, j int) bool { return n.files[i].Path < n.files[j].Path })
	total := len(names) + len(n.files)
	idx := 0
	branch := func() (self, child string) {
		idx++
		if idx == total {
			return prefix + "└ ", prefix + "  "
		}
		return prefix + "├ ", prefix + "│ "
	}
	for _, name := range names {
		d := n.dirs[name]
		label := d.name
		// 只有一个子目录且没有文件时压缩成 a/b/c
		for len(d.files) == 0 && len(d.dirs) == 1 {
			for _, only := range d.dirs {
				label += "/" + only.name
				d = only
			}
		}
		self, child := branch()
		files := d.allFiles()
		r := row{kind: rowDir, sec: sec, depth: depth, name: label, path: d.path, count: len(files), files: files, guide: self}
		r.collapsed = collapsed[r.key()]
		rows = append(rows, r)
		if !r.collapsed {
			rows = appendNode(rows, d, sec, depth+1, child, collapsed)
		}
	}
	for _, f := range n.files {
		self, _ := branch()
		rows = append(rows, row{kind: rowFile, sec: sec, depth: depth, name: path.Base(f.Path), path: f.Path, file: f, count: 1, files: []string{f.Path}, guide: self})
	}
	return rows
}

// statusCode 返回该文件在对应分组下显示的状态字母。
func statusCode(f gitx.File, sec section) byte {
	switch sec {
	case secConflict:
		return '!'
	case secStaged:
		return f.X
	default:
		if f.Untracked() {
			return 'U'
		}
		return f.Y
	}
}
