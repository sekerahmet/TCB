package main

import (
	"bytes"
	"go/ast"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/devtools/goload"
)

var valueFiles = map[string]string{
	"lexer/token.go": `package lexer

const (
	TokPercent = "%"
	TokDot     = "."
	tokHidden  = "#"
)
`,
	"lexer/token_test.go": "package lexer\n\nconst TokTest = \"?\"\n",
	"interp/ops.go": `package interp

import "example.com/audit/lexer"

var _ = lexer.TokPercent

func opCode(op string) int {
	switch op {
	case "%":
		return 1
	case "#", "?":
		return 2
	}
	return 0
}

func isMod(op string) bool { return op == "%" }

var names = map[string]int{"%": 1}
`,
	"paths/clean.go": `package paths

func hidden(name string) bool { return name == "." || name == "%" }
`,
}

// Kural: değeri dışa açık bir dize sabitine eşit bir metin sabiti `case`'te, `==`/`!=`'de ya da dize anahtarlı haritanın
// anahtarında geçiyorsa o yer sabitin `compares_value` [INFERRED] kullanıcısıdır; yalnız sabitin paketine (geçişli)
// bağımlı paketlerde (başka paketteki aynı metin başka anlamdadır), test dosyasındaki ve dışa kapalı sabit hedef olmaz.
// `query "%"` değeri eşit sabiti bulur (eksikler listesi, kritik 1: who-uses lexer.TokPercent yorumlayıcının `case "%"`'sini ve
// denetleyicinin `case "+", ..., "%"`'sini göstermiyordu).
func TestTextComparesUseTheConstantOfTheirValue(t *testing.T) {
	root := auditModule(t, valueFiles)
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	ix := newIndex(g)
	const pct = "go:const:example.com/audit/lexer.TokPercent"
	got := map[string]string{}
	for _, e := range g.Edges {
		if e.Kind == edgeComparesValue {
			got[strings.TrimPrefix(e.From, "go:")+" -> "+strings.TrimPrefix(e.To, "go:const:example.com/audit/")] = e.Confidence
		}
	}
	for _, want := range []string{"func:example.com/audit/interp.opCode -> lexer.TokPercent",
		"func:example.com/audit/interp.isMod -> lexer.TokPercent", "var:example.com/audit/interp.names -> lexer.TokPercent"} {
		if got[want] != inferred {
			t.Errorf("missing INFERRED %s (got %v)", want, got)
		}
	}
	if len(got) != 3 {
		t.Errorf("only the three uses in interp, not paths (no dependency), tokHidden (unexported) or TokTest (test file): %v", got)
	}
	var out, errb bytes.Buffer
	if n, code := ix.resolve("%", &queryOpts{stdout: &out, stderr: &errb}); code != exitOK || n.ID != pct {
		t.Errorf(`query "%%" resolves to the constant of that value: %v %d %s`, n, code, errb.String())
	}
	ix.printWhoUses(&out, ix.byID[pct], 12)
	if !strings.Contains(out.String(), "compares_value (3)") || !strings.Contains(out.String(), "interp.opCode [func]") {
		t.Errorf("who-uses lacks the text comparisons:\n%s", out.String())
	}
}

// Kural (sınıf): sözcük çözümleyicinin her dize sabiti için yorumlayıcıda ve denetleyicide değeri `case`'te ya da
// `==`/`!=`'de metinle anan her işlev who-uses'ta görünür. Beklenen küme codegraph'tan bağımsız, sözdizimi ağacından
// kurulur (go/types ve çıkarıcı kullanılmaz): sabitler token.go'nun değişmezlerinden, yerler işlev gövdelerinden.
func TestEveryOperatorTheInterpreterOrCheckerNamesByTextIsListed(t *testing.T) {
	_, m, g := repoGraph(t)
	lexerPkg := g.Module + "/internal/lang/lexer"
	byValue := map[string][]string{} // değer -> sabit kimlikleri
	edges := map[string]bool{}
	for _, e := range g.Edges {
		if e.Kind == edgeComparesValue {
			edges[e.From+" "+e.To] = true
		}
	}
	for _, u := range m.Units {
		if u.PkgPath != lexerPkg {
			continue
		}
		for _, f := range u.Files {
			if strings.HasSuffix(m.Fset.Position(f.Pos()).Filename, "_test.go") {
				continue
			}
			for _, c := range constLiterals(f) {
				byValue[c[1]] = append(byValue[c[1]], valueID(kindConst, lexerPkg, c[0]))
			}
		}
	}
	checked := 0
	for _, u := range m.Units {
		if u.PkgPath != g.Module+"/internal/lang/interp" && u.PkgPath != g.Module+"/internal/lang/checker" {
			continue
		}
		for _, f := range u.Files {
			if strings.HasSuffix(m.Fset.Position(f.Pos()).Filename, "_test.go") {
				continue
			}
			for _, d := range f.Decls {
				fn, ok := d.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				recv := ""
				if fn.Recv != nil {
					recv = goload.RecvExprName(fn.Recv.List[0].Type)
				}
				id := memberID(u.PkgPath, recv, fn.Name.Name)
				for _, v := range comparedLiterals(fn.Body) {
					for _, c := range byValue[v] {
						checked++
						if !edges[id+" "+c] {
							t.Errorf("%s names %q by text, who-uses %s does not list it", id, v, c)
						}
					}
				}
			}
		}
	}
	if checked < 20 {
		t.Errorf("only %d operator uses checked; the interpreter and the checker name more by text", checked)
	}
}

// constLiterals bir dosyanın dışa açık, değeri dize değişmezi olan sabitleridir (ad, değer).
func constLiterals(f *ast.File) [][2]string {
	var out [][2]string
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, s := range gd.Specs {
			vs := s.(*ast.ValueSpec)
			for i, n := range vs.Names {
				if i >= len(vs.Values) || !n.IsExported() {
					continue
				}
				if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if v, err := strconv.Unquote(lit.Value); err == nil && v != "" {
						out = append(out, [2]string{n.Name, v})
					}
				}
			}
		}
	}
	return out
}

// comparedLiterals bir gövdede `case`'te ve `==`/`!=`'de geçen dize değişmezleridir.
func comparedLiterals(body ast.Node) []string {
	var out []string
	add := func(e ast.Expr) {
		if lit, ok := ast.Unparen(e).(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if v, err := strconv.Unquote(lit.Value); err == nil && v != "" {
				out = append(out, v)
			}
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CaseClause:
			for _, e := range n.List {
				add(e)
			}
		case *ast.BinaryExpr:
			if n.Op == token.EQL || n.Op == token.NEQ {
				add(n.X)
				add(n.Y)
			}
		}
		return true
	})
	return out
}
