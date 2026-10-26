package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// keys 扫描整个项目，收集所有 i18n.T / i18n.Tf 调用里的原文（字面量或包级常量）。
func keys(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()
	consts := map[string]string{}
	type call struct {
		arg ast.Expr
		pos string
	}
	var calls []call
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "dist" || strings.HasPrefix(d.Name(), ".")) && p != root {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.ValueSpec:
				for i, name := range n.Names {
					if i < len(n.Values) {
						if lit, ok := n.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
							if v, err := strconv.Unquote(lit.Value); err == nil {
								consts[name.Name] = v
							}
						}
					}
				}
			case *ast.CallExpr:
				sel, ok := n.Fun.(*ast.SelectorExpr)
				if !ok || len(n.Args) == 0 {
					return true
				}
				pkg, ok := sel.X.(*ast.Ident)
				if !ok || pkg.Name != "i18n" || (sel.Sel.Name != "T" && sel.Sel.Name != "Tf") {
					return true
				}
				calls = append(calls, call{n.Args[0], fset.Position(n.Pos()).String()})
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, c := range calls {
		var s string
		switch a := c.arg.(type) {
		case *ast.BasicLit:
			s, _ = strconv.Unquote(a.Value)
		case *ast.Ident:
			s = consts[a.Name]
		default:
			continue // 变量参数（比如已经翻译过的文字）不检查
		}
		s = strings.TrimFunc(s, unicode.IsSpace)
		if s != "" {
			out[s] = c.pos
		}
	}
	return out
}

// 每一条界面文字都必须有全部语言的翻译。
func TestCatalogsComplete(t *testing.T) {
	ks := keys(t)
	if len(ks) < 100 {
		t.Fatalf("只找到 %d 条文字，扫描可能出错了", len(ks))
	}
	var missing []string
	for _, l := range All[1:] {
		for k, pos := range ks {
			if !Has(l, k) {
				missing = append(missing, string(l)+"\t"+pos+"\t"+strconv.Quote(k))
			}
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		if os.Getenv("I18N_DUMP") != "" {
			for _, m := range missing {
				t.Log(m)
			}
		}
		t.Fatalf("%d 条翻译缺失（I18N_DUMP=1 查看明细），例如：\n%s", len(missing), strings.Join(missing[:min(10, len(missing))], "\n"))
	}
}

// 翻译里的 %s / %d 等占位符数量和顺序必须和原文一致。
func TestPlaceholders(t *testing.T) {
	for l, cat := range catalogs {
		for k, v := range cat {
			if verbs(k) != verbs(v) {
				t.Errorf("%s: 占位符不一致\n原文：%q\n译文：%q", l, k, v)
			}
		}
	}
}

func verbs(s string) string {
	var b strings.Builder
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '%' {
			b.WriteByte(s[i+1])
			i++
		}
	}
	return b.String()
}

// 目录里没人用的翻译条目（文案改了但翻译没删）。
func TestNoStaleEntries(t *testing.T) {
	ks := keys(t)
	for l, cat := range catalogs {
		for k := range cat {
			if _, ok := ks[k]; !ok {
				t.Errorf("%s: 多余的翻译条目 %q", l, k)
			}
		}
	}
}

func TestParse(t *testing.T) {
	cases := map[string]Lang{
		"zh_CN.UTF-8": ZhCN, "zh-Hans-CN": ZhCN, "zh": ZhCN,
		"zh_TW.UTF-8": ZhTW, "zh-Hant": ZhTW, "zh_HK": ZhTW,
		"en_US.UTF-8": En, "en": En,
		"ja_JP.UTF-8": Ja, "ja": Ja,
		"ko_KR.UTF-8": Ko, "ko-KR": Ko,
	}
	for in, want := range cases {
		if got, ok := Parse(in); !ok || got != want {
			t.Errorf("Parse(%q) = %q, %v；期望 %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "C", "POSIX", "fr_FR.UTF-8"} {
		if _, ok := Parse(in); ok {
			t.Errorf("Parse(%q) 不应识别", in)
		}
	}
}

func TestTKeepsSpaces(t *testing.T) {
	register("xx", map[string]string{"提交": "Commit"})
	defer delete(catalogs, "xx")
	Set("xx")
	defer Set(ZhCN)
	if got := T("  提交 "); got != "  Commit " {
		t.Errorf("T 应保留首尾空格，得到 %q", got)
	}
	if got := T("没有翻译"); got != "没有翻译" {
		t.Errorf("没有翻译时应返回原文，得到 %q", got)
	}
}
