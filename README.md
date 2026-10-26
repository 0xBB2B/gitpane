# gitpane

鼠标可点的 Git TUI：变更树、diff、提交图谱（可按分支筛选），单个二进制。

宽屏（≥ 100 列）时，左边列表、右边详情：

```
  ⎇ main  → origin/main ↑3                                                          demo
  变更 6   图谱  ─────────────────────────────────────────────────────────────────────────
▍▾ 已暂存  1                       全部取消  │ ▌ server.go  internal/server   +8 -3  工作区 · 修改
 └ README.md                       +8  M  -  │ ⋯ 第 1 行
▍▾ 变更  5                         全部暂存  │   1   1   package server
 ├ ▾ cmd/app/                          1  +  │   2   2
 │ └ main.go                    +1 -2  M  +  │   3     - import "fmt"
 ├ ▾ internal/server/                  1  +  │       3 + import (
 │ └ server.go                  +8 -3  M  +  │       4 +     "fmt"
  ✓ 提交   ↓ 拉取   ↑ 推送 3   + 全部暂存   ↻
  空格  暂存   c  提交   a  全暂存   x  放弃   Tab  切换                    ?  帮助   q  退出
```

窄屏时自动变成上下布局，适合放在分屏的一侧当面板用。

## 安装

**macOS（Homebrew）**

```bash
brew tap 0xBB2B/gitpane https://github.com/0xBB2B/gitpane   # 只需一次
brew install 0xBB2B/gitpane/gitpane
```

以后 `brew upgrade` 就会更新。

**Windows（Scoop）**

```powershell
scoop bucket add gitpane https://github.com/0xBB2B/gitpane   # 只需一次
scoop install gitpane
```

Windows 建议在 Windows Terminal 里用；老的 cmd 控制台对鼠标和线框字符支持不好。

**Linux / 其他**

从 [Releases](https://github.com/0xBB2B/gitpane/releases) 下载对应平台的压缩包，或者：

```bash
go install github.com/0xBB2B/gitpane@latest
```

运行时需要系统里有 `git`。

## 用法

```bash
gitpane                  # 当前目录
gitpane ~/code/foo       # 指定目录（仓库里任意子目录都行）
gitpane --layout stack   # 强制上下布局；wide 强制左右
gitpane --embed          # 面板模式：q 不退出，Ctrl+C 要按两次（嵌入其他分屏工具时用）
```

进入后按 `?` 看全部快捷键。

## 功能

- **变更**：冲突 / 已暂存 / 变更三组，按目录树展示，每个文件显示 `+增 -删` 和状态字母。
  点文件看 diff，点右侧 `+` / `-` 暂存或取消，点目录的 `+` 整个目录一起，分组标题右侧按钮批量操作。
  暂存后光标自动落到下一个文件。
- **diff**：自绘，带新旧行号、增删行底色、hunk 位置；只渲染可见部分，大文件也不卡。
- **图谱**：自己计算泳道布局，每个提交一行，分叉 / 合并用 `╭ ╮ ╰ ╯ ├ ┤ ┴ ┼` 在同一行连好，每条分支颜色固定；
  `◉` 是 HEAD，`○` 是合并提交（标题淡显）。HEAD / 本地分支 / 远程分支 / 标签分色显示。
  滚到底自动加载更多，整个历史都能翻到。
- **分支筛选**：默认显示全部分支；点标签栏右侧「⎇ 全部分支 ▾」（或按 `b`）打开分支面板，
  本地 / 远程分支都能多选，只看这些分支的历史；点「✕ 全部」恢复。面板里双击或按 `s` 切换到该分支（远程分支会自动建跟踪分支）。
- **提交详情**：提交信息 + 每个文件一行（带 `+增 -删`），点文件名展开 / 折叠它的改动；
  只改一个文件时默认展开，多个文件默认折叠，`z` 或标题栏右侧按钮全部展开 / 折叠。
- **操作**：提交、pull、push（没有上游时自动 `-u origin HEAD`）、fetch、放弃改动（两次确认）、git init。
- 每 2 秒自动刷新，外部改了文件（比如 AI 助手在写代码）马上能看到。

## 快捷键

| 键 | 作用 |
|---|---|
| `Tab` `1` `2` | 切换 变更 / 图谱 |
| `j` `k` `↑` `↓` / `g` `G` | 移动 / 到顶到底 |
| `空格` | 暂存 / 取消暂存当前行 |
| `回车` `←` `→` | 折叠 / 展开目录（在文件上按 `←` 回到所在目录） |
| `z` | 变更页：全部折叠 / 展开目录；图谱页：提交详情里全部展开 / 折叠文件 |
| `b` | 打开分支筛选面板（空格勾选、`a` 显示全部、`s` 切换分支、Esc 返回） |
| `c` | 提交（底部输入框，回车确认，Esc 取消） |
| `a` / `u` | 全部暂存 / 全部取消暂存 |
| `x` | 放弃当前文件改动（按两次确认） |
| `p` / `P` / `f` | pull / push / fetch |
| `J` `K` | 详情区翻页 |
| `r` | 立即刷新 |
| `?` | 帮助 |
| `q` / `Ctrl+C` | 退出（`--embed` 时 Ctrl+C 按两次） |

## 环境变量

| 变量 | 说明 |
|---|---|
| `GITPANE_THEME` | `dark` / `light`，自动检测不准时手动指定 |
| `GITPANE_NERD_FONT` | 设为 `1` 显示 Nerd Font 文件 / 目录 / 分支图标 |

## 说明

- 执行 git 时设置了 `GIT_OPTIONAL_LOCKS=0`，和其他工具同时操作仓库也不会抢 `index.lock`。
- `GIT_TERMINAL_PROMPT=0`：需要输密码的 push/pull 会直接报错而不是卡住，建议用 SSH key 或 credential helper。

## 结构

```
main.go                  命令行参数、版本信息
.goreleaser.yaml         发布配置（六平台编译、Release、Homebrew cask、Scoop）
.github/workflows/
  ci.yml                 推送 / PR：三系统测试 + 打包演练
  release.yml            推送 v* 标签：正式发布
internal/gitx/           git 命令封装（status、numstat、diff、提交、远程、分支）
  graph.go               提交图谱的泳道布局（带单元测试）
internal/ui/
  model.go / update.go   状态、异步加载、键盘鼠标操作
  view.go                布局（上下 / 左右）与渲染；同一个布局函数用于点击命中判断
  diffview.go            diff / 提交详情 / 图谱行的解析与绘制
  picker.go              分支筛选面板
  tree.go                变更文件 → 带连线的目录树
  theme.go               Tokyo Night 深浅两套配色、图标
```

## 发布

发布由 GitHub Actions + [GoReleaser](https://goreleaser.com) 自动完成：推送 `v*` 标签后，
编译 macOS / Linux / Windows × amd64 / arm64 六个版本，创建 GitHub Release，
并把 Homebrew cask 写进本仓库的 `Casks/`、Scoop 清单写进 `bucket/`，提交到 `main`。

本仓库同时就是 Homebrew tap 和 Scoop bucket，不需要另建仓库，也不需要额外的 token。

### 注意

- 每次发版后机器人会往 `main` 推一个 `chore: …` 提交，本地下次推送前先 `git pull`。
- 如果给 `main` 开了分支保护，需要允许 GitHub Actions 推送，否则 cask / 清单更新会失败。
- `Casks/` 和 `bucket/` 是生成的，不要手改。

### 每次发版

```bash
git tag v0.1.0
git push origin v0.1.0
```

在 Actions 页面可以看到 release 流程。带 `-rc` / `-beta` 等后缀的标签（如 `v0.2.0-rc1`）
会标成预发布，不推送到 brew / scoop。

### 本地试打包

```bash
goreleaser check                                  # 检查配置
goreleaser release --snapshot --clean --skip=publish   # 只在 dist/ 里生成产物，不发布
```

## 许可

MIT
