package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/0xBB2B/gitpane/internal/gitx"
)

// 带 \r / ESC / 换行的文字渲染后必须正好占 w 列、且只有一行。
func TestPaintSanitizes(t *testing.T) {
	for _, in := range []string{
		"* add products page\r",
		"evil \x1b[2J\x1b[H clear",
		"two\nlines",
		"tab\there",
		"bidi ‮evil",
	} {
		out := paint(40, nil, sg(in, lipgloss.NewStyle()))
		if strings.ContainsAny(out, "\r\n") {
			t.Errorf("%q 渲染后仍含换行/回车：%q", in, out)
		}
		if w := lipgloss.Width(out); w != 40 {
			t.Errorf("%q 渲染宽度 %d，期望 40", in, w)
		}
	}
	if got := clean("ok 中文"); got != "ok 中文" {
		t.Errorf("正常文字不应被改动：%q", got)
	}
}

// CRLF 提交正文按行拆开，不能留下 \r，也不能多出空行。
func TestCommitBodyCRLF(t *testing.T) {
	lines := commitLines(&gitxCommit)
	body := 0
	for _, l := range lines {
		if strings.Contains(l.text, "\r") {
			t.Fatalf("正文行里还有 \\r：%q", l.text)
		}
		if l.kind == dBody {
			body++
		}
	}
	if body != 3 {
		t.Fatalf("正文应为 3 行，得到 %d", body)
	}
}

var gitxCommit = gitx.CommitInfo{Subject: "squash", Body: "* add products page\r\n* server api\r\n* buy button"}
