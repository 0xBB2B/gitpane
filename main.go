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
	"strconv"
	"strings"

	"github.com/0xBB2B/gitpane/internal/i18n"
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

// usageText 是 --help 的内容（简体中文原文，按当前语言翻译）。
const usageText = `gitpane —— 鼠标可点的 Git TUI

用法：
  gitpane [选项] [目录]

选项：
  --lang zh-CN|zh-TW|en|ja|ko  界面语言（默认按系统语言自动选择）
  --layout auto|wide|stack     排布方式（默认 auto：宽度 ≥ 100 列时左右并排）
  --depth N                    当前目录不是仓库时，向下探测几层子目录找仓库（默认 2，0 = 不探测）
  --embed                      面板模式：q 不退出，Ctrl+C 需按两次（嵌入其他分屏工具时用）
  --version                    显示版本

环境变量：
  GITPANE_LANG=zh-CN|zh-TW|en|ja|ko  界面语言（--lang 优先）
  GITPANE_DEPTH=N                    子仓库探测深度（--depth 优先）
  GITPANE_THEME=light|dark           指定配色（默认自动检测终端背景）
  GITPANE_NERD_FONT=1                使用 Nerd Font 图标

进入后按 ? 查看全部快捷键。`

// langFromArgs 在解析参数之前先找出 --lang，这样连 --help 都能用指定的语言显示。
func langFromArgs(args []string) string {
	for i, a := range args {
		for _, p := range []string{"--lang=", "-lang="} {
			if strings.HasPrefix(a, p) {
				return strings.TrimPrefix(a, p)
			}
		}
		if (a == "--lang" || a == "-lang") && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// defaultDepth 是 --depth 的默认值：GITPANE_DEPTH，否则 2。
func defaultDepth() int {
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv("GITPANE_DEPTH"))); err == nil && v >= 0 {
		return v
	}
	return 2
}

func main() {
	i18n.Set(i18n.Detect())
	if v := langFromArgs(os.Args[1:]); v != "" {
		l, ok := i18n.Parse(v)
		if !ok {
			fmt.Fprintf(os.Stderr, "gitpane: unsupported language %q (zh-CN, zh-TW, en, ja, ko)\n", v)
			os.Exit(2)
		}
		i18n.Set(l)
	}

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, i18n.T(usageText))
	}
	flag.String("lang", "", "zh-CN | zh-TW | en | ja | ko")
	layout := flag.String("layout", "auto", "auto | wide | stack")
	depth := flag.Int("depth", defaultDepth(), "0..")
	embed := flag.Bool("embed", false, i18n.T("作为面板嵌入运行"))
	showVersion := flag.Bool("version", false, i18n.T("显示版本"))
	flag.Parse()
	if *showVersion {
		fmt.Println(versionString())
		return
	}

	if *depth < 0 {
		*depth = 0
	}
	opts := ui.Options{Embed: *embed, Depth: *depth}
	switch *layout {
	case "auto", "":
		opts.Layout = ui.LayoutAuto
	case "wide":
		opts.Layout = ui.LayoutWide
	case "stack":
		opts.Layout = ui.LayoutStack
	default:
		fmt.Fprintln(os.Stderr, "gitpane: "+i18n.Tf("--layout 只能是 auto / wide / stack，收到 %q", *layout))
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
		fmt.Fprintln(os.Stderr, "gitpane: "+i18n.Tf("%s 不是目录", dir))
		os.Exit(1)
	}

	if err := ui.Run(dir, opts); err != nil {
		fmt.Fprintln(os.Stderr, "gitpane:", err)
		os.Exit(1)
	}
}
