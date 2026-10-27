"""把 tmux capture-pane -e 的输出转成网站用的终端数据。

用法: ansi2site.py ANSI目录 输出目录 [文件名后缀，如 .light]

每种语言输出 screens/<lang>.json：
  {"cols":128, "rows":30,
   "screens": {"changes": [行HTML,...], "graph": [...], "picker": [...], "graphf": [...]},
   "hot": {"changes": [{"r":行,"c":起始列,"w":宽,"to":目标}, ...], ...}}

框线（│─╭…）、提交圆点（● ○ ◉）、半格方块（▌）输出成带 class 的空 span，由 CSS 画，
这样在浏览器里和本地终端一样上下左右严丝合缝，不依赖字体。
"""
import html
import json
import os
import re
import sys
import unicodedata

COLS, ROWS = 128, 30
SRC, OUT = sys.argv[1], sys.argv[2]
SUFFIX = sys.argv[3] if len(sys.argv) > 3 else ""
LANGS = ["zh-CN", "zh-TW", "en", "ja", "ko"]
SCREENS = ["changes", "graph", "picker", "graphf"]

# 按钮文字（各语言），用来找可点击区域
LABELS = {
    "zh-CN": {"changes": "变更", "graph": "图谱", "branch": "分支", "done": "完成", "all": "✕"},
    "zh-TW": {"changes": "變更", "graph": "圖譜", "branch": "分支", "done": "完成", "all": "✕"},
    "en": {"changes": "Changes", "graph": "Graph", "branch": "Branches", "done": "Done", "all": "✕"},
    "ja": {"changes": "変更", "graph": "グラフ", "branch": "ブランチ", "done": "完了", "all": "✕"},
    "ko": {"changes": "변경", "graph": "그래프", "branch": "브랜치", "done": "완료", "all": "✕"},
}

SGR = re.compile(r"\x1b\[([0-9;:]*)m")
OTHER_ESC = re.compile(r"\x1b(\[[0-9;?]*[A-Za-z]|\][^\x07]*\x07|[()][A-Za-z0-9])")

BOX = {"│": "v", "─": "h", "┼": "x", "├": "lt", "┤": "rt", "┬": "tt", "┴": "bt",
       "└": "bl", "┌": "tl", "┐": "tr", "┘": "br", "╭": "rtl", "╮": "rtr", "╰": "rbl", "╯": "rbr"}
DOTS = {"●": "dot", "○": "ring", "◉": "head"}
BLOCKS = {"▌": "half", "▍": "bar3"}


def c256(n):
    if n < 16:
        return None
    if n < 232:
        n -= 16
        v = [0, 95, 135, 175, 215, 255]
        return "#%02x%02x%02x" % (v[n // 36], v[(n // 6) % 6], v[n % 6])
    g = 8 + (n - 232) * 10
    return "#%02x%02x%02x" % (g, g, g)


class St:
    __slots__ = ("fg", "bg", "bold", "faint", "italic", "strike")

    def __init__(self):
        self.fg = self.bg = None
        self.bold = self.faint = self.italic = self.strike = False

    def key(self):
        return (self.fg, self.bg, self.bold, self.faint, self.italic, self.strike)

    def copy(self):
        s = St()
        for k in self.__slots__:
            setattr(s, k, getattr(self, k))
        return s


def sgr(st, params):
    ps = [int(p) if p else 0 for p in re.split("[;:]", params)] if params else [0]
    i = 0
    while i < len(ps):
        p = ps[i]
        if p == 0:
            st.__init__()
        elif p == 1:
            st.bold = True
        elif p == 2:
            st.faint = True
        elif p == 3:
            st.italic = True
        elif p == 9:
            st.strike = True
        elif p == 22:
            st.bold = st.faint = False
        elif p == 23:
            st.italic = False
        elif p == 29:
            st.strike = False
        elif p == 39:
            st.fg = None
        elif p == 49:
            st.bg = None
        elif p in (38, 48) and i + 1 < len(ps):
            c = None
            if ps[i + 1] == 5 and i + 2 < len(ps):
                c = c256(ps[i + 2])
                i += 2
            elif ps[i + 1] == 2 and i + 4 < len(ps):
                c = "#%02x%02x%02x" % tuple(ps[i + 2:i + 5])
                i += 4
            if p == 38:
                st.fg = c
            else:
                st.bg = c
        i += 1


def width(ch):
    if unicodedata.combining(ch):
        return 0
    return 2 if unicodedata.east_asian_width(ch) in "WF" else 1


def parse(text):
    grid = []
    for line in text.split("\n")[:ROWS]:
        row = [(" ", St()) for _ in range(COLS)]
        st, x, i = St(), 0, 0
        while i < len(line) and x < COLS:
            m = SGR.match(line, i)
            if m:
                sgr(st, m.group(1))
                i = m.end()
                continue
            m = OTHER_ESC.match(line, i)
            if m:
                i = m.end()
                continue
            ch = line[i]
            i += 1
            w = width(ch)
            if w == 0 or x + w > COLS:
                continue
            row[x] = (ch, st.copy())
            if w == 2:
                row[x + 1] = None
            x += w
        grid.append(row)
    while len(grid) < ROWS:
        grid.append([(" ", St()) for _ in range(COLS)])
    return grid


def style(st):
    css = []
    if st.fg:
        css.append("color:" + st.fg)
    if st.bg:
        css.append("background:" + st.bg)
    cls = []
    if st.bold:
        cls.append("B")
    if st.faint:
        cls.append("F")
    if st.italic:
        cls.append("I")
    if st.strike:
        cls.append("S")
    return ";".join(css), cls


def span(text, st, extra=None, aria=None):
    css, cls = style(st)
    if extra:
        cls = [extra] + cls
    a = ' class="%s"' % " ".join(cls) if cls else ""
    s = ' style="%s"' % css if css else ""
    h = ' aria-hidden="true"' if aria else ""
    if not a and not s and not h:
        return html.escape(text)
    return "<span%s%s%s>%s</span>" % (a, s, h, html.escape(text))


def row_html(row):
    out, buf, bst = [], "", None

    def flush():
        nonlocal buf, bst
        if buf:
            out.append(span(buf, bst))
        buf, bst = "", None

    for cell in row:
        if cell is None:
            continue
        ch, st = cell
        if ch in BOX:
            flush()
            out.append(span("", st, "k " + BOX[ch], True))
        elif ch in DOTS:
            flush()
            out.append(span("", st, "k " + DOTS[ch], True))
        elif ch in BLOCKS:
            flush()
            out.append(span("", st, "k " + BLOCKS[ch], True))
        elif width(ch) == 2:
            flush()
            out.append(span(ch, st, "w"))
        elif ord(ch) > 0x7E:
            flush()
            out.append(span(ch, st, "n"))  # 其他窄符号（⎇ ▾ ✓ ↑ …）：固定一格宽
        else:
            if bst is not None and bst.key() != st.key():
                flush()
            bst = st
            buf += ch
    flush()
    # 去掉行尾没有底色的空格，减小体积
    s = "".join(out)
    return s.rstrip(" ")


def row_text(row):
    """返回 (纯文本, 每个字符所在的列)。"""
    t, cols = "", []
    for x, cell in enumerate(row):
        if cell is None:
            continue
        t += cell[0]
        cols.append(x)
    return t, cols


def bg_run(row, x):
    """包含第 x 列、底色相同的一段（按钮的范围）。"""
    def bg(i):
        c = row[i]
        while c is None and i > 0:
            i -= 1
            c = row[i]
        return c[1].bg
    b = bg(x)
    if b is None:
        # 没有底色（未选中的标签页）：按文字边界扩展，两侧各带一格空白，遇到两个连续空格停
        def ch(i):
            c = row[i]
            return "x" if c is None else c[0]
        e = x
        while e < COLS and not (ch(e) == " " and (e + 1 >= COLS or ch(e + 1) == " ")):
            e += 1
        return max(0, x - 1), min(COLS, e + 1)
    a, e = x, x + 1
    while a > 0 and bg(a - 1) == b:
        a -= 1
    while e < COLS and bg(e) == b:
        e += 1
    return a, e


def find(grid, label, rows, nth=0):
    hits = []
    for r in rows:
        t, cols = row_text(grid[r])
        i = t.find(label)
        while i >= 0:
            hits.append((r, cols[i]))
            i = t.find(label, i + 1)
    return hits[nth] if len(hits) > nth else None


def hot(grid, lab, label, rows, to, nth=0):
    h = find(grid, lab[label], rows, nth)
    if not h:
        print("  !! 没找到", label, file=sys.stderr)
        return None
    r, c = h
    a, e = bg_run(grid[r], c)
    return {"r": r, "c": a, "w": e - a, "to": to}


def right_runs(row):
    """标签栏最右边的几个按钮（从右往左）。"""
    runs, x = [], COLS - 1
    while x >= 0:
        c = row[x]
        if c is None or c[1].bg is None:
            x -= 1
            continue
        a, e = bg_run(row, x)
        runs.append((a, e))
        x = a - 1
        if len(runs) >= 2:
            break
    return runs


os.makedirs(OUT, exist_ok=True)
for L in LANGS:
    lab = LABELS[L]
    data = {"cols": COLS, "rows": ROWS, "screens": {}, "hot": {}}
    grids = {}
    for sc in SCREENS:
        txt = open(os.path.join(SRC, "%s-%s.ansi" % (L, sc)), encoding="utf-8").read()
        grids[sc] = parse(txt)
        data["screens"][sc] = [row_html(r) for r in grids[sc]]
    bottom = [ROWS - 2]
    hs = {}
    g = grids["changes"]
    hs["changes"] = [hot(g, lab, "graph", [1], "graph")]
    for sc, back in (("graph", None), ("graphf", "graph")):
        g = grids[sc]
        lst = [hot(g, lab, "changes", [1], "changes"), hot(g, lab, "branch", bottom, "picker")]
        runs = right_runs(g[1])
        if runs:  # 最右边是分支筛选按钮
            a, e = runs[0]
            lst.append({"r": 1, "c": a, "w": e - a, "to": "picker"})
        if back and len(runs) > 1:  # 「✕ 全部」清除筛选
            a, e = runs[1]
            lst.append({"r": 1, "c": a, "w": e - a, "to": back})
        hs[sc] = lst
    g = grids["picker"]
    hs["picker"] = [hot(g, lab, "done", range(ROWS - 3, ROWS), "graphf")]
    data["hot"] = {k: [h for h in v if h] for k, v in hs.items()}
    p = os.path.join(OUT, "%s%s.json" % (L, SUFFIX))
    with open(p, "w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False, separators=(",", ":"))
    print(L, os.path.getsize(p) // 1024, "KB", {k: len(v) for k, v in data["hot"].items()})
