package checkpoint_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Durumlu değerler arayüzle bağlanır: StateDict ve LoadState yöntemlerini birlikte yazan her
// motor tipi, derleme anında `var _ Stateful = (*T)(nil)` ile arayüzü sağladığını bildirmelidir;
// yöntemlerden biri yeniden adlandırılırsa tip kontrol noktasından sessizce düşmesin.
func TestStatefulImplementersAreAsserted(t *testing.T) {
	root := reporoot.Path("internal", "stdlib")
	fset := token.NewFileSet()
	methods := map[string]map[string]bool{} // tip → yöntemler
	asserted := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		for _, decl := range f.Decls {
			switch x := decl.(type) {
			case *ast.FuncDecl:
				if x.Recv == nil || (x.Name.Name != "StateDict" && x.Name.Name != "LoadState") {
					continue
				}
				if name := receiverName(x.Recv.List[0].Type); name != "" {
					if methods[name] == nil {
						methods[name] = map[string]bool{}
					}
					methods[name][x.Name.Name] = true
				}
			case *ast.GenDecl:
				for _, s := range x.Specs {
					if v, ok := s.(*ast.ValueSpec); ok && len(v.Names) == 1 && v.Names[0].Name == "_" &&
						typeName(v.Type) == "Stateful" && len(v.Values) == 1 {
						if call, ok := v.Values[0].(*ast.CallExpr); ok {
							if p, ok := call.Fun.(*ast.ParenExpr); ok {
								asserted[receiverName(p.X)] = true
							}
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for name, m := range methods {
		if !m["StateDict"] || !m["LoadState"] {
			continue
		}
		found++
		if !asserted[name] {
			t.Errorf("%s StateDict ve LoadState yazıyor ama `var _ Stateful = (*%s)(nil)` denetimi yok", name, name)
		}
	}
	if found == 0 {
		t.Error("durumlu tip bulunamadı; tarama bozuk")
	}
}

// receiverName (*T) ya da T ifadesinden T'yi verir.
func receiverName(e ast.Expr) string {
	if s, ok := e.(*ast.StarExpr); ok {
		e = s.X
	}
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	if sel, ok := e.(*ast.SelectorExpr); ok {
		return sel.Sel.Name
	}
	return ""
}

// typeName Stateful ya da paket.Stateful türünün adını verir.
func typeName(e ast.Expr) string {
	return receiverName(e)
}
