package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

var caseFiles = map[string]string{
	"cli/cli.go": `package cli

func alpha() int { return 1 }
func beta() int  { return 2 }

func answer(cmd string) int {
	switch cmd {
	case "alpha":
		return alpha()
	case "beta":
		return beta()
	}
	return 0
}

func Run(args []string) int { return answer(args[0]) }
`,
	"cli/cli_test.go": `package cli

import "testing"

func runIt(cmd string) int { return Run([]string{cmd}) }

func TestAlpha(t *testing.T) { _ = Run([]string{"alpha"}) }

func TestBetaThroughHelper(t *testing.T) { _ = runIt("beta") }
`,
}

// Kural: bir test (ya da test dosyasındaki yardımcısına verdiği metin) bir dize kolunun değerini taşıyor ve kolun
// işlevine ulaşıyorsa, kolun çağırdıklarını doğrudan sınar: komut adıyla çalıştırılan komutun işlevi o testin doğrudan
// testidir (eksikler listesi, ince ayar: query_fields.go'nun en yakın testi 5 adımdı). Başka kolun işlevi o testin olmaz.
func TestACommandRunByNameIsTestedDirectly(t *testing.T) {
	g, err := buildGraph(auditModule(t, caseFiles))
	if err != nil {
		t.Fatal(err)
	}
	pkg := "go:func:example.com/audit/cli."
	tests := func(test, fn string) bool {
		return slices.ContainsFunc(g.Edges, func(e Edge) bool { return e.From == pkg+test && e.To == pkg+fn && e.Kind == edgeTests })
	}
	for _, c := range []struct {
		test, fn string
		want     bool
	}{{"TestAlpha", "alpha", true}, {"TestAlpha", "beta", false},
		{"TestBetaThroughHelper", "beta", true}, {"TestBetaThroughHelper", "alpha", false}} {
		if tests(c.test, c.fn) != c.want {
			t.Errorf("%s tests %s: %v, want %v", c.test, c.fn, !c.want, c.want)
		}
	}
}

// Kural: codegraph'ın kendi komutları için de: `answer`'ın her dize kolunun çağırdığı işlevin, kolun değerini taşıyan bir
// test varsa en az bir doğrudan testi vardır. Kollar ve testlerdeki metinler go/parser ile bağımsız okunur.
func TestEveryCodegraphCommandIsTestedDirectlyByTheTestsThatNameIt(t *testing.T) {
	_, _, g := repoGraph(t)
	carried := map[string]bool{} // test dosyalarındaki dize değişmezleri
	files, err := reporoot.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		file, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if v, err := strconv.Unquote(lit.Value); err == nil {
					carried[v] = true
				}
			}
			return true
		})
	}
	direct := map[string]bool{} // doğrudan (tests kenarıyla) sınanan düğümler
	for _, e := range g.Edges {
		if e.Kind == edgeTests {
			direct[e.To] = true
		}
	}
	src, err := parser.ParseFile(token.NewFileSet(), reporoot.Path("tools", "codegraph", "main_query.go"), nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	ast.Inspect(src, func(n ast.Node) bool {
		fd, ok := n.(*ast.FuncDecl)
		if !ok || fd.Name.Name != "answer" {
			return true
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			cc, ok := n.(*ast.CaseClause)
			if !ok {
				return true
			}
			named := false
			for _, e := range cc.List {
				if lit, ok := e.(*ast.BasicLit); ok {
					v, _ := strconv.Unquote(lit.Value)
					named = named || carried[v]
				}
			}
			if !named {
				return false
			}
			for _, st := range cc.Body {
				ast.Inspect(st, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					if id, isIdent := sel.X.(*ast.Ident); isIdent && id.Name == "ix" { // index'in metotları
						checked++
						if !direct["go:func:"+g.Module+"/tools/codegraph.index."+sel.Sel.Name] {
							t.Errorf("index.%s is called for a command a test names, but has no direct test", sel.Sel.Name)
						}
					}
					return true
				})
			}
			return false
		})
		return false
	})
	if checked == 0 {
		t.Fatal("no command case found in answer; the rule is not exercised")
	}
}
