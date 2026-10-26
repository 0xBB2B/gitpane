package gitx

import (
	"strconv"
	"strings"
)

// 提交图谱：不用 git log --graph 的 ASCII 输出，而是自己按泳道（lane）布局。
// 每个提交固定占一行，分叉/合并用同一行里的圆角线连接，没有纯连线行；
// 每条泳道分配后颜色不变，一眼能看出哪条线是哪个分支。

// Cell 是图谱区的一个字符格。Color 是泳道颜色序号（由界面映射到调色板），-1 表示空白。
type Cell struct {
	Glyph string
	Color int
}

// GraphLine 是图谱的一行：左侧连线格 + 提交信息。
type GraphLine struct {
	Cells                           []Cell
	Hash, Full, Refs, Subject, When string
	Author                          string
	Merge, Head                     bool
}

// Graph 返回提交图谱。refs 为空时显示全部分支（含远程和标签），否则只显示这些分支能到达的提交。
func (r *Repo) Graph(limit int, refs []string) ([]GraphLine, error) {
	args := []string{"-n", strconv.Itoa(limit)}
	if len(refs) == 0 {
		args = append(args, "--all")
	} else {
		args = append(append(args, refs...), "--")
	}
	return r.log(args...)
}

type rawCommit struct {
	full, short, refs, subject, when, author string
	parents                                  []string
}

func (r *Repo) log(args ...string) ([]GraphLine, error) {
	// --date-order：子提交一定在父提交之前，其余按时间交错，和 VS Code 等图谱一致
	out, err := r.git(append([]string{"log", "--date-order", "--no-color",
		"--format=%H%x1f%h%x1f%P%x1f%D%x1f%s%x1f%at%x1f%an"}, args...)...)
	if err != nil {
		return nil, err
	}
	var cs []rawCommit
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if l == "" {
			continue
		}
		f := strings.SplitN(l, "\x1f", 7)
		for len(f) < 7 {
			f = append(f, "")
		}
		cs = append(cs, rawCommit{full: f[0], short: f[1], parents: strings.Fields(f[2]), refs: f[3], subject: f[4], when: f[5], author: f[6]})
	}
	return layout(cs), nil
}

// 方向位：一个格子里线从哪些方向经过。
const (
	up = 1 << iota
	down
	left
	right
)

var boxGlyph = map[int]string{
	up | down: "│", left | right: "─",
	up | left: "╯", up | right: "╰", down | left: "╮", down | right: "╭",
	up | down | left: "┤", up | down | right: "├",
	up | left | right: "┴", down | left | right: "┬",
	up | down | left | right: "┼",
	up:                       "│", down: "│", left: "─", right: "─",
}

type lane struct {
	hash  string
	color int
}

// layout 为每个提交计算一行连线格。每条泳道占两列：线 + 间隔。
func layout(cs []rawCommit) []GraphLine {
	var lanes []lane
	nextColor := 0
	out := make([]GraphLine, 0, len(cs))

	for _, c := range cs {
		before := append([]lane(nil), lanes...)

		// 1. 找到在等这个提交的泳道：第一条作为它的列，其余在这里汇入（分支起点）
		col := -1
		var closing []int
		for i, l := range lanes {
			if l.hash == c.full {
				if col < 0 {
					col = i
				} else {
					closing = append(closing, i)
				}
			}
		}
		if col < 0 { // 分支末端（没有子提交显示过）：占一条空泳道
			col = freeLane(lanes, before, nil)
			if col == len(lanes) {
				lanes = append(lanes, lane{})
			}
			lanes[col] = lane{hash: c.full, color: nextColor}
			nextColor++
		}
		myColor := lanes[col].color
		for _, j := range closing {
			lanes[j] = lane{}
		}

		// 2. 父提交：第一个父提交沿用本列；其余父提交（合并来源）接到已有泳道或新开泳道
		type link struct {
			idx   int
			color int
		}
		var opens []link
		if len(c.parents) == 0 {
			lanes[col] = lane{}
		} else {
			lanes[col].hash = c.parents[0]
		}
		busy := map[int]bool{col: true}
		for _, j := range closing {
			busy[j] = true
		}
		var merged []string
		if len(c.parents) > 1 {
			merged = c.parents[1:]
		}
		for _, p := range merged {
			k := -1
			for i, l := range lanes {
				if l.hash == p && i != col {
					k = i
					break
				}
			}
			if k < 0 {
				k = freeLane(lanes, before, busy)
				if k == len(lanes) {
					lanes = append(lanes, lane{})
				}
				lanes[k] = lane{hash: p, color: nextColor}
				nextColor++
			}
			busy[k] = true
			opens = append(opens, link{k, lanes[k].color})
		}

		// 3. 画格子
		n := max(len(before), len(lanes))
		bits := make([]int, 2*n)
		color := make([]int, 2*n)
		for i := range color {
			color[i] = -1
		}
		mark := func(cell, b, col int) {
			bits[cell] |= b
			if color[cell] < 0 {
				color[cell] = col
			}
		}
		// 穿过本行的泳道
		for i, l := range before {
			if l.hash == "" || i == col || contains(closing, i) {
				continue
			}
			mark(2*i, up|down, l.color)
		}
		// 横线：从提交列连到目标列
		hline := func(target, color, from int) {
			for x := 2*min(from, target) + 1; x < 2*max(from, target); x++ {
				mark(x, left|right, color)
			}
		}
		for _, j := range closing {
			c := before[j].color
			hline(j, c, col)
			if j > col {
				mark(2*j, up|left, c)
			} else {
				mark(2*j, up|right, c)
			}
		}
		for _, o := range opens {
			hline(o.idx, o.color, col)
			d := down
			if o.idx < len(before) && before[o.idx].hash != "" && !contains(closing, o.idx) {
				d = up | down // 接到一条本来就在的泳道上
			}
			if o.idx > col {
				mark(2*o.idx, d|left, o.color)
			} else {
				mark(2*o.idx, d|right, o.color)
			}
		}

		cells := make([]Cell, 2*n)
		for i := range cells {
			if bits[i] == 0 {
				cells[i] = Cell{Glyph: " ", Color: -1}
				continue
			}
			cells[i] = Cell{Glyph: boxGlyph[bits[i]], Color: color[i]}
		}
		head := strings.HasPrefix(c.refs, "HEAD") || strings.Contains(c.refs, "HEAD ->")
		dot := "●"
		switch {
		case head:
			dot = "◉"
		case len(c.parents) > 1:
			dot = "○"
		}
		cells[2*col] = Cell{Glyph: dot, Color: myColor}
		// 去掉行尾空格
		for len(cells) > 0 && cells[len(cells)-1].Color < 0 {
			cells = cells[:len(cells)-1]
		}

		out = append(out, GraphLine{
			Cells: cells, Hash: c.short, Full: c.full, Refs: c.refs, Subject: c.subject,
			When: ago(c.when), Author: c.author, Merge: len(c.parents) > 1, Head: head,
		})

		// 收起末尾的空泳道
		for len(lanes) > 0 && lanes[len(lanes)-1].hash == "" {
			lanes = lanes[:len(lanes)-1]
		}
	}
	return out
}

// freeLane 找一条本行没被用过的空泳道；没有就返回 len(lanes) 表示新开一条。
func freeLane(lanes, before []lane, busy map[int]bool) int {
	for i, l := range lanes {
		if l.hash != "" || busy[i] {
			continue
		}
		if i < len(before) && before[i].hash != "" {
			continue // 这一行里它上半截还有线，不能复用
		}
		return i
	}
	return len(lanes)
}

func contains(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
