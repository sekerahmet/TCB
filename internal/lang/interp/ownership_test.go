package interp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// ownActive'in değişmezi (ownership.go): bir kez açılınca kapanmaz ve açılmadan hiçbir değer
// sahipli (ownHeld) olmaz. Sıcak yollar (releaseTemp, afterIndex) kapalıyken işi buna dayanarak
// atlar. Test paketin kaynağını okur: ownActive'e yalnız true yazılır; ownHeld yalnız adopt'ta
// (ownActive açıldıktan sonra) ve adoptContainer'da kurulur; adoptContainer'ı yalnız sahipli
// bir eleman gören ownContainer çağırır.
func TestOwnershipIsNeverSwitchedOffAndNothingIsHeldBeforeIt(t *testing.T) {
	dir := filepath.Join(reporoot.Root(), "internal", "lang", "interp")
	entries, err := reporoot.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var stores, holds []string
	var containerCallers []string
	var adoptStore, adoptHold token.Pos
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.CallExpr:
					sel, ok := x.Fun.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					if id, ok := sel.X.(*ast.Ident); ok && id.Name == "ownActive" && sel.Sel.Name == "Store" {
						arg, _ := x.Args[0].(*ast.Ident)
						if arg == nil || arg.Name != "true" {
							t.Errorf("%s: ownActive'e true'dan başka bir değer yazılıyor", fset.Position(x.Pos()))
						}
						stores = append(stores, fn.Name.Name)
						if fn.Name.Name == "adopt" {
							adoptStore = x.Pos()
						}
					}
					if sel.Sel.Name == "adoptContainer" {
						containerCallers = append(containerCallers, fn.Name.Name)
					}
				case *ast.AssignStmt:
					for _, r := range x.Rhs {
						if id, ok := r.(*ast.Ident); ok && id.Name == "ownHeld" {
							holds = append(holds, fn.Name.Name)
							if fn.Name.Name == "adopt" {
								adoptHold = x.Pos()
							}
						}
					}
				}
				return true
			})
		}
	}
	if strings.Join(stores, ",") != "adopt" {
		t.Errorf("ownActive yalnız adopt'ta açılmalı: %v", stores)
	}
	if strings.Join(holds, ",") != "adopt,adoptContainer" {
		t.Errorf("ownHeld yalnız adopt'ta ve adoptContainer'da kurulmalı: %v", holds)
	}
	if adoptStore == 0 || adoptHold == 0 || adoptStore > adoptHold {
		t.Errorf("adopt ownHeld kurmadan önce ownActive'i açmalı")
	}
	if strings.Join(containerCallers, ",") != "ownContainer" {
		t.Errorf("adoptContainer'ı yalnız ownContainer çağırmalı: %v", containerCallers)
	}
}
