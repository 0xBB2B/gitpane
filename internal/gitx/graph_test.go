package gitx

import (
	"strings"
	"testing"
)

func render(lines []GraphLine) string {
	var b strings.Builder
	for _, l := range lines {
		for _, c := range l.Cells {
			b.WriteString(c.Glyph)
		}
		b.WriteString(" " + l.Hash + "\n")
	}
	return b.String()
}

func rc(h string, parents ...string) rawCommit {
	return rawCommit{full: h, short: h, parents: parents}
}

// 每个提交只占一行，没有纯连线行；行数等于提交数。
func TestLayoutOneRowPerCommit(t *testing.T) {
	// M 合并了 feature(F2,F1) 到 main(A)，F1 从 B 分出；C 是并行的另一条分支末端
	cs := []rawCommit{
		rc("C", "B"),
		rc("M", "A", "F2"),
		rc("F2", "F1"),
		rc("A", "B"),
		rc("F1", "B"),
		rc("B", "R"),
		rc("R"),
	}
	got := layout(cs)
	if len(got) != len(cs) {
		t.Fatalf("行数 %d，期望 %d", len(got), len(cs))
	}
	out := render(got)
	t.Log("\n" + out)
	for _, want := range []string{"╮", "╯"} {
		if !strings.Contains(out, want) {
			t.Errorf("缺少连线字符 %s：\n%s", want, out)
		}
	}
	// 合并提交用空心点
	if got[1].Cells[2*0].Glyph == "●" && !got[1].Merge {
		t.Errorf("M 应该是合并提交")
	}
}

// 线性历史只有一条泳道。
func TestLayoutLinear(t *testing.T) {
	got := layout([]rawCommit{rc("3", "2"), rc("2", "1"), rc("1")})
	for _, l := range got {
		if len(l.Cells) != 1 {
			t.Fatalf("线性历史应该只有一列：\n%s", render(got))
		}
	}
}
