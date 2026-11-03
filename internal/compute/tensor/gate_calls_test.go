package tensor

import (
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kural (10.2, S-kilit; DENETİM istek 64, Ş2): tensör paketi arka uca veri okuyan ya da yazan
// çağrıyı yalnız kapıdan yapar (gate_calls.go). Veri çağrısı: arka uç arayüzlerinden birinin
// (Backend ve isteğe bağlı yetenekleri) ya da arka uç tipinin depo ya da görünüm parametresi
// alan bir yöntemi. İstisnalar: kapının kendisi, arka uçların kendi yöntemleri (kapının altı)
// ve newKernel'e verilen gövde (izinleri çekirdek tutar). Paket go/types ile denetlenir; yeni
// bir doğrudan çağrı testi düşürür ve yerini söyler.
func TestBackendDataCallsGoThroughTheGate(t *testing.T) {
	fset, files, info, pkg := checkTensorPackage(t)
	backend := pkg.Scope().Lookup("Backend").Type().Underlying().(*types.Interface)
	storage, view := pkg.Scope().Lookup("Storage").Type(), pkg.Scope().Lookup("View").Type()
	backendSide := func(t types.Type) bool {
		if types.Implements(t, backend) || types.Implements(types.NewPointer(t), backend) {
			return true
		}
		n, ok := t.(*types.Named)
		if !ok || n.Obj().Pkg() != pkg || n.Obj().Name() == "Storage" || n.Obj().Name() == "Optimizer" {
			return false
		}
		_, isInterface := n.Underlying().(*types.Interface)
		return isInterface
	}
	dataCall := func(sig *types.Signature) bool {
		for i := 0; i < sig.Params().Len(); i++ {
			pt := sig.Params().At(i).Type()
			if p, ok := pt.(*types.Pointer); ok {
				pt = p.Elem()
			}
			if s, ok := pt.(*types.Slice); ok {
				pt = s.Elem()
			}
			if types.Identical(pt, storage) || types.Identical(pt, view) {
				return true
			}
		}
		return false
	}
	gated := 0
	for _, f := range files {
		name := filepath.Base(fset.Position(f.Pos()).Filename)
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				if n.Recv != nil && backendSide(derefType(info.Types[n.Recv.List[0].Type].Type)) {
					return false // arka ucun kendi uygulaması
				}
			case *ast.CallExpr:
				if id, ok := n.Fun.(*ast.Ident); ok && id.Name == "newKernel" {
					return false // çekirdeğin gövdesi: izinleri çekirdek tutar
				}
				se, ok := n.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				sel := info.Selections[se]
				if sel == nil || sel.Kind() != types.MethodVal || !backendSide(derefType(sel.Recv())) || !dataCall(sel.Type().(*types.Signature)) {
					return true
				}
				if name == "gate_calls.go" {
					gated++
					return true
				}
				t.Errorf("%s: %s doğrudan arka uca çağrılıyor; gate%s(...) kullanın (gate_calls.go)", fset.Position(n.Pos()), se.Sel.Name, se.Sel.Name)
			}
			return true
		})
	}
	if gated < 50 {
		t.Fatalf("kapıda yalnız %d veri çağrısı bulundu; tarama arka uç çağrılarını görmüyor", gated)
	}
}

func derefType(t types.Type) types.Type {
	if p, ok := t.(*types.Pointer); ok {
		return p.Elem()
	}
	return t
}

// checkTensorPackage paketin (testsiz, bu yapılandırmadaki) dosyalarını tip denetiminden geçirir.
func checkTensorPackage(t *testing.T) (*token.FileSet, []*ast.File, *types.Info, *types.Package) {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := reporoot.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, e := range entries {
		n := e.Name()
		if !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		if ok, err := build.Default.MatchFile(".", n); err != nil || !ok {
			continue
		}
		f, err := parser.ParseFile(fset, n, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	info := &types.Info{Selections: map[*ast.SelectorExpr]*types.Selection{}, Types: map[ast.Expr]types.TypeAndValue{}}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check("github.com/sekerahmet/TCB/internal/compute/tensor", fset, files, info)
	if err != nil {
		t.Fatal(err)
	}
	return fset, files, info, pkg
}
