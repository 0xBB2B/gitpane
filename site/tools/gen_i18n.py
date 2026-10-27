"""生成 site/i18n.js：从 5 种语言的 README 提取标语、亮点、用法，再加上网站独有的文案。

用法: python3 site/tools/gen_i18n.py .（仓库根目录）
"""
import html
import json
import re
import sys

ROOT = sys.argv[1]
FILES = {"zh-CN": "README.md", "zh-TW": "README.zh-TW.md", "en": "README.en.md", "ja": "README.ja.md", "ko": "README.ko.md"}
HL = {"zh-CN": "亮点", "zh-TW": "亮點", "en": "Highlights", "ja": "特長", "ko": "특징"}
USE = {"zh-CN": "用法", "zh-TW": "用法", "en": "Usage", "ja": "使い方", "ko": "사용법"}

# 网站独有的文案
SITE = {
    "zh-CN": {
        "toLight": "切换到浅色主题", "toDark": "切换到深色主题",
        "skip": "跳到正文",
        "title": "Gitpane：鼠标可点的 Git 终端界面",
        "h1": "鼠标可点的 Git 终端界面",
        "linux": "Linux / 源码",
        "copy": "复制", "copied": "已复制",
        "latest": "最新版本 {v}", "archives": "也可以直接下载各平台的压缩包",
        "termLabel": "gitpane 界面演示",
        "hint": "这是 gitpane 真实输出的界面，不是图片。点标签「变更」「图谱」或底部的「⎇ 分支」切换看看，键盘上的 1、2、b、Esc 也能用。",
        "after": "在任意 Git 仓库里运行 gitpane，按 ? 查看全部快捷键，按 q 退出。",
        "license": "MIT 许可证", "readme": "完整说明", "lang": "界面语言", "source": "源代码",
        "to": {"changes": "切换到变更页", "graph": "切换到图谱页", "graphf": "完成并返回图谱", "picker": "打开分支面板"},
    },
    "zh-TW": {
        "toLight": "切換到淺色主題", "toDark": "切換到深色主題",
        "skip": "跳到正文",
        "title": "Gitpane：滑鼠可點的 Git 終端介面",
        "h1": "滑鼠可點的 Git 終端介面",
        "linux": "Linux / 原始碼",
        "copy": "複製", "copied": "已複製",
        "latest": "最新版本 {v}", "archives": "也可以直接下載各平台的壓縮檔",
        "termLabel": "gitpane 介面示範",
        "hint": "這是 gitpane 實際輸出的介面，不是圖片。點分頁「變更」「圖譜」或底部的「⎇ 分支」切換看看，鍵盤上的 1、2、b、Esc 也能用。",
        "after": "在任何 Git 儲存庫裡執行 gitpane，按 ? 查看所有快捷鍵，按 q 退出。",
        "license": "MIT 授權", "readme": "完整說明", "lang": "介面語言", "source": "原始碼",
        "to": {"changes": "切換到變更頁", "graph": "切換到圖譜頁", "graphf": "完成並回到圖譜", "picker": "開啟分支面板"},
    },
    "en": {
        "toLight": "Switch to light theme", "toDark": "Switch to dark theme",
        "skip": "Skip to content",
        "title": "Gitpane: a Git terminal UI you can drive with your mouse",
        "h1": "A Git terminal UI you can drive with your mouse",
        "linux": "Linux / source",
        "copy": "Copy", "copied": "Copied",
        "latest": "Latest release {v}", "archives": "or download an archive for your platform",
        "termLabel": "Gitpane interface demo",
        "hint": "This is real Gitpane output, not an image. Click the Changes and Graph tabs or the ⎇ Branches button at the bottom, or press 1, 2, b and Esc.",
        "after": "Run gitpane inside any Git repository. Press ? for every key binding, q to quit.",
        "license": "MIT License", "readme": "Full documentation", "lang": "Language", "source": "Source code",
        "to": {"changes": "Go to Changes", "graph": "Go to Graph", "graphf": "Done, back to the graph", "picker": "Open the branch picker"},
    },
    "ja": {
        "toLight": "ライトテーマに切り替え", "toDark": "ダークテーマに切り替え",
        "skip": "本文へスキップ",
        "title": "Gitpane：マウスで操作できる Git ターミナル UI",
        "h1": "マウスで操作できる Git ターミナル UI",
        "linux": "Linux / ソースから",
        "copy": "コピー", "copied": "コピーしました",
        "latest": "最新バージョン {v}", "archives": "各プラットフォーム向けのアーカイブも直接ダウンロードできます",
        "termLabel": "gitpane の画面デモ",
        "hint": "画像ではなく、gitpane が実際に出力した画面です。タブの「変更」「グラフ」や下部の「⎇ ブランチ」をクリックしてみてください。キーボードの 1、2、b、Esc でも操作できます。",
        "after": "Git リポジトリの中で gitpane を実行します。? ですべてのキー操作を表示、q で終了します。",
        "license": "MIT ライセンス", "readme": "詳しい説明", "lang": "表示言語", "source": "ソースコード",
        "to": {"changes": "変更画面へ", "graph": "グラフ画面へ", "graphf": "完了してグラフへ戻る", "picker": "ブランチ選択パネルを開く"},
    },
    "ko": {
        "toLight": "라이트 테마로 전환", "toDark": "다크 테마로 전환",
        "skip": "본문으로 건너뛰기",
        "title": "Gitpane: 마우스로 조작하는 Git 터미널 UI",
        "h1": "마우스로 조작하는 Git 터미널 UI",
        "linux": "Linux / 소스",
        "copy": "복사", "copied": "복사됨",
        "latest": "최신 버전 {v}", "archives": "플랫폼별 압축 파일을 직접 내려받을 수도 있습니다",
        "termLabel": "gitpane 화면 데모",
        "hint": "이미지가 아니라 gitpane이 실제로 출력한 화면입니다. 「변경」「그래프」 탭이나 하단의 「⎇ 브랜치」를 눌러 보세요. 키보드 1, 2, b, Esc로도 조작할 수 있습니다.",
        "after": "Git 저장소 안에서 gitpane을 실행하세요. ?로 모든 단축키를 보고, q로 종료합니다.",
        "license": "MIT 라이선스", "readme": "전체 문서", "lang": "표시 언어", "source": "소스 코드",
        "to": {"changes": "변경 화면으로", "graph": "그래프 화면으로", "graphf": "완료하고 그래프로", "picker": "브랜치 선택 패널 열기"},
    },
}


def inline(md):
    """README 里的行内 Markdown（**粗体**、`代码`）转成 HTML。"""
    s = html.escape(md, quote=False)
    s = re.sub(r"`([^`]+)`", r"<code>\1</code>", s)
    s = re.sub(r"\*\*([^*]+)\*\*", r"<strong>\1</strong>", s)
    return s


def section(text, title):
    m = re.search(r"^## " + re.escape(title) + r"\n(.*?)(?=^## )", text, re.S | re.M)
    return m.group(1)


out = {}
for lang, fn in FILES.items():
    t = open(f"{ROOT}/{fn}", encoding="utf-8").read()
    d = dict(SITE[lang])
    # 标语下面那一行：「变更树 · …<br>一个可执行文件…」
    m = re.search(r"<p><strong>(.*?)</strong></p>\s*<p>(.*?)</p>", t, re.S)
    d["h1"] = m.group(1).strip()
    d["title"] = "Gitpane: " + d["h1"] if lang in ("en", "ko") else "Gitpane：" + d["h1"]
    d["lede"] = m.group(2).split("<br>")[-1].strip()  # 第一句是功能清单，和亮点重复，只取第二句
    d["hlTitle"] = HL[lang]
    items = []
    for line in section(t, HL[lang]).strip().splitlines():
        m = re.match(r"- \*\*(.+?)\*\*[:：]\s*(.+)", line)
        if m:
            d_ = m.group(2)
            if lang == "en":  # README 里跟在冒号后面是小写，网页上单独成段，首字母大写
                d_ = d_[0].upper() + d_[1:]
            items.append({"t": inline(m.group(1)), "d": inline(d_)})
    d["hl"] = items
    d["useTitle"] = USE[lang]
    m = re.search(r"```bash\n(.*?)```", section(t, USE[lang]), re.S)
    d["usage"] = m.group(1).rstrip("\n")
    out[lang] = d
    assert len(items) == 7, (lang, len(items))

js = "// 由 site/tools/gen_i18n.py 从各语言 README 生成，请勿手改；改 README 后重新生成。\nwindow.I18N = " + \
    json.dumps(out, ensure_ascii=False, indent=1) + ";\n"
open(f"{ROOT}/site/i18n.js", "w", encoding="utf-8").write(js)
print({k: (len(v["hl"]), v["lede"][:30]) for k, v in out.items()})
