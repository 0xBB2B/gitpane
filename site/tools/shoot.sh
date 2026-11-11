#!/bin/bash
# 为网站的终端演示截取界面（每种语言 5 个：changes / graph / switch / picker / graphf），再转成 site/screens/*.json。
#
# 用法: site/tools/shoot.sh <gitpane 可执行文件> <演示仓库目录> [dark|light]
# 深色输出 site/screens/<语言>.json，浅色输出 site/screens/<语言>.light.json
# 需要 tmux、python3。演示仓库最好有已暂存 / 未暂存的改动、几条分支和合并，截图更好看。
set -e
BIN=$(realpath "$1"); DEMO=$(realpath "$2"); THEME=${3:-dark}
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
TMP=$(mktemp -d)
click(){ tmux -L ws send-keys -t w -l $'\e[<0;'"$1"';'"$2"'M'; tmux -L ws send-keys -t w -l $'\e[<0;'"$1"';'"$2"'m'; sleep 0.7; }
key(){ tmux -L ws send-keys -t w "$@"; sleep 0.5; }
cap(){ tmux -L ws capture-pane -p -e -N -t w > "$TMP/$1"; }
# 点标签栏最右边的按钮（图谱页的「⎇ 全部分支 ▾」）：按显示宽度找到第 2 行最后一个字的位置
rclick(){ local x; x=$(tmux -L ws capture-pane -p -t w | sed -n 2p | python3 -c "import sys,unicodedata as u;l=sys.stdin.read().rstrip();print(sum(2 if u.east_asian_width(c) in 'WF' else 1 for c in l)-1)"); click "$x" 2; }
for L in zh-CN zh-TW en ja ko; do
  tmux -L ws kill-server 2>/dev/null || true; sleep 0.4
  LANG=C.UTF-8 tmux -u -L ws -f /dev/null new-session -d -s w -x 128 -y 30 \
    "cd '$DEMO' && LANG=C.UTF-8 TERM=xterm-256color COLORTERM=truecolor GITPANE_THEME=$THEME XDG_CONFIG_HOME='$TMP/cfg' '$BIN' --lang $L"
  sleep 1.8
  key j; key j; key j; sleep 0.6; cap $L-changes.ansi        # 变更页，选中一个已暂存的文件
  key 2; sleep 1; key G; sleep 1; click 30 10; cap $L-graph.ansi  # 图谱页，选中一个提交
  key b; sleep 0.6; cap $L-switch.ansi; key Escape; sleep 0.5  # 分支切换面板
  rclick; sleep 0.5; click 10 6; click 10 7; key k; cap $L-picker.ansi  # 右上角的分支筛选面板，勾选两条分支
  key Enter; sleep 1; cap $L-graphf.ansi                     # 只看这两条分支的图谱
  echo "$L done"
done
tmux -L ws kill-server 2>/dev/null || true
SUFFIX=""; [ "$THEME" = light ] && SUFFIX=".light"
python3 "$ROOT/site/tools/ansi2site.py" "$TMP" "$ROOT/site/screens" "$SUFFIX"
rm -rf "$TMP"
