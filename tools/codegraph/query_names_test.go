package main

import (
	"go/ast"
	"go/constant"
	"go/types"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Kural: bir ad kodu bileşik değerde bir işlevin anahtarıysa (`codes.CmdFormat: formatCommand`) kodun Go'daki
// uygulaması o çiftin değeridir, aynı satırdaki öbür çiftlerinki değil (keyedImplementations, implementationsAt). Çiftler
// go/types'tan bulunur, grafiğe bakılmadan; kodun kullanıldığı her satırda sorgu yalnız o işlevi adlandırır.
func TestAKeyedNameCodeIsImplementedByItsPairsValue(t *testing.T) {
	_, m, g := repoGraph(t)
	ix := newIndex(g)
	checked := 0
	for _, u := range m.Units {
		for _, f := range u.Files {
			if strings.HasSuffix(m.Fset.Position(f.Pos()).Filename, "_test.go") {
				continue
			}
			ast.Inspect(f, func(n ast.Node) bool {
				kv, ok := n.(*ast.KeyValueExpr)
				if !ok {
					return true
				}
				key, value := nameIdent(kv.Key), nameIdent(kv.Value)
				if key == nil || value == nil {
					return true
				}
				c, isConst := u.Info.Uses[key].(*types.Const)
				fn, isFunc := u.Info.Uses[value].(*types.Func)
				if !isConst || !isFunc || c.Pkg() == nil || c.Pkg().Path() != m.Path+"/"+codesDir || ix.byID[funcRef(fn)] == nil {
					return true
				}
				rel, line := m.Rel(m.Fset.Position(key.Pos()).Filename), m.Fset.Position(key.Pos()).Line
				want := ix.byID[funcRef(fn)].Name + " ["
				v, exact := constant.Int64Val(c.Val())
				if !exact {
					return true
				}
				code := "name:" + codes.Code(v).String()
				for _, e := range g.Edges {
					if e.Kind != edgeUsesName || e.To != code || e.File != rel || e.Line != line || ix.byID[e.From] == nil {
						continue
					}
					checked++
					if impl := ix.implementationsAt(ix.byID[e.From], &e); len(impl) != 1 || !strings.HasPrefix(impl[0], want) {
						t.Errorf("%s:%d: %s is implemented by %s, got %v", rel, line, e.To, want, impl)
					}
				}
				return true
			})
		}
	}
	if checked == 0 {
		t.Fatal("no name code keys a function in a composite literal: the scan no longer sees them")
	}
}
