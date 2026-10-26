package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

var tupleFiles = map[string]string{
	"lang/lang.go": `package lang

func load() ([]string, error) { return nil, nil }
func pick() int               { return 1 }
func other() int              { return 2 }

var list, listErr = load()
var first, second = pick(), other()
`,
}

// Kural: çok adlı bir paket düzeyi belirtimde her ad kendi değerini, tek ve çok değerli değerde her ad o değeri kullanır.
// Eskiden yalnız ilk ad kullanıyordu: `var display, active, displayErr, activeErr = startLanguages(...)`'te active'den
// geçen zincir (codes.Codes → active → buildLanguage → tr.json) kopuktu ve listeyi gezen testler kayboluyordu.
func TestEveryNameOfAValueSpecUsesItsOwnValue(t *testing.T) {
	g, err := buildGraph(auditModule(t, tupleFiles))
	if err != nil {
		t.Fatal(err)
	}
	pkg := "go:var:example.com/audit/lang."
	calls := func(from, to string) bool {
		return slices.ContainsFunc(g.Edges, func(e Edge) bool {
			return e.From == pkg+from && e.To == "go:func:example.com/audit/lang."+to && e.Kind == edgeCalls
		})
	}
	for _, c := range []struct {
		from, to string
		want     bool
	}{{"list", "load", true}, {"listErr", "load", true}, {"first", "pick", true}, {"first", "other", false},
		{"second", "other", true}, {"second", "pick", false}} {
		if calls(c.from, c.to) != c.want {
			t.Errorf("%s calls %s: %v, want %v", c.from, c.to, !c.want, c.want)
		}
	}
}

// Kural: depodaki her çok adlı paket düzeyi belirtimde ortak değeri paylaşan adların kullanım kenarları aynıdır ve
// kendi değeri bir çağrı olan her adın en az bir kenarı vardır. Bağımsız okuma: go/parser; kenarlar grafikten.
func TestEveryMultiNameValueSpecInTheRepositoryHasItsUses(t *testing.T) {
	_, m, g := repoGraph(t)
	uses := map[string][]string{} // düğüm → kullanım kenarlarının hedefleri (inits_var ve contains hariç)
	for _, e := range g.Edges {
		if e.Kind != edgeContains && e.Kind != edgeInitsVar && strings.HasPrefix(e.From, "go:") {
			uses[e.From] = append(uses[e.From], e.Kind+" "+e.To)
		}
	}
	for k := range uses {
		slices.Sort(uses[k])
		uses[k] = slices.Compact(uses[k])
	}
	checked := 0
	root := reporoot.Root()
	err := reporoot.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") || strings.Contains(filepath.ToSlash(p), "/testdata/") {
			return err
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, filepath.Dir(p))
		pkgPath := m.Path + "/" + filepath.ToSlash(rel)
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || (gd.Tok != token.VAR && gd.Tok != token.CONST) {
				continue
			}
			kind := map[token.Token]string{token.VAR: kindVar, token.CONST: kindConst}[gd.Tok]
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				if len(vs.Names) < 2 || len(vs.Values) == 0 {
					continue
				}
				checked++
				first := ""
				for i, n := range vs.Names {
					if n.Name == "_" {
						continue
					}
					id := valueID(kind, pkgPath, n.Name)
					if first == "" {
						first = id
					}
					if len(vs.Values) == 1 && !slices.Equal(uses[id], uses[first]) {
						t.Errorf("%s and %s share one value but not its uses:\n  %v\n  %v", id, first, uses[id], uses[first])
					}
					if len(vs.Values) == len(vs.Names) && isCall(vs.Values[i]) && len(uses[id]) == 0 {
						t.Errorf("%s: its value is a call, but it uses nothing", id)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no multi-name value spec found; the rule is not exercised")
	}
}

func isCall(e ast.Expr) bool { _, ok := ast.Unparen(e).(*ast.CallExpr); return ok }
