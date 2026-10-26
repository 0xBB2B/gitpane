// gitpane —— 鼠标可点的 Git TUI：变更树、diff、泳道式提交图谱、分支筛选。
//
// 用法：
//
//	gitpane [目录]               全屏打开（宽屏左右布局，窄屏上下布局）
//	gitpane --layout stack       强制上下布局；wide 强制左右
//	gitpane --embed              作为面板嵌入别的工具（q 不退出）
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/0xBB2B/gitpane/internal/ui"
)

// 发布时由 GoReleaser 通过 -ldflags "-X main.version=… -X main.commit=… -X main.date=…" 写入。
var (
	version = "dev"
	commit  = ""
	date    = ""
)

// versionString 返回版本信息；go install 安装的没有 ldflags，就从模块信息里取版本号。
func versionString() string {
	v := version
	if v == "dev" {
		if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			v = bi.Main.Version
		}
	}
	s := "gitpane " + v
	if commit != "" {
		s += " (" + commit
		if date != "" {
			s += ", " + date
		}
		s += ")"
	}
	return s
}

func main() {
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, `gitpane —— 鼠标可点的 Git TUI

用法：
  gitpane [选项] [目录]

选项：
  --layout auto|wide|stack   排布方式（默认 auto：宽度 ≥ 100 列时左右并排）
  --embed                    面板模式：q 不退出，Ctrl+C 需按两次（嵌入其他分屏工具时用）
  --version                  显示版本

环境变量：
  GITPANE_THEME=light|dark   指定配色（默认自动检测终端背景）
  GITPANE_NERD_FONT=1        使用 Nerd Font 图标

进入后按 ? 查看全部快捷键。
`)
	}
	layout := flag.String("layout", "auto", "auto | wide | stack")
	embed := flag.Bool("embed", false, "作为面板嵌入运行")
	showVersion := flag.Bool("version", false, "显示版本")
	flag.Parse()

	if *showVersion {
		fmt.Println(versionString())
		return
	}

	opts := ui.Options{Embed: *embed}
	switch *layout {
	case "auto", "":
		opts.Layout = ui.LayoutAuto
	case "wide":
		opts.Layout = ui.LayoutWide
	case "stack":
		opts.Layout = ui.LayoutStack
	default:
		fmt.Fprintf(os.Stderr, "gitpane: --layout 只能是 auto / wide / stack，收到 %q\n", *layout)
		os.Exit(2)
	}

	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}
	abs, err := filepath.Abs(dir)
	if err == nil {
		dir = abs
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		fmt.Fprintf(os.Stderr, "gitpane: %s 不是目录\n", dir)
		os.Exit(1)
	}

	if err := ui.Run(dir, opts); err != nil {
		fmt.Fprintln(os.Stderr, "gitpane:", err)
		os.Exit(1)
	}
}
