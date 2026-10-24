package messages

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Cümle parçalardan kurulmaz, bir iletinin metnini Go kodu düzenlemez: `strings.Replace`,
// `TrimPrefix`, `Contains` gibi işlevlere doğrudan bir iletinin metni (`err.Error()` ya da
// `messages.Text(...)`) verilmez; ayrıntı gerekiyorsa iletinin parametresinden okunur
// (messages.As(err).Args). Test, depodaki test dışı Go kodunda bu doğrudan kullanımı arar
// (metni önce bir değişkene alan dolaylı kullanımı görmez).
//
// textEditing, kuralın bugünkü ihlalleridir (dosya → kullanım sayısı) ve yalnızca küçülür.
//
//	interp/code_names.go: tipin kendi hata iletisi asıl adı zaten öneriyor mu diye iletinin metnine bakıyor.
//	stdlib/dataset/sentinel.go: iletinin metninden ": " önekini kırpıyor.
var textEditing = map[string]int{
	"internal/lang/interp/code_names.go":  1,
	"internal/stdlib/dataset/sentinel.go": 1,
}

// textFuncs ileti metnini bölen, kırpan ya da içinde arayan strings işlevleridir.
var textFuncs = map[string]bool{
	"Replace": true, "ReplaceAll": true, "TrimPrefix": true, "TrimSuffix": true, "HasPrefix": true,
	"HasSuffix": true, "Contains": true, "Index": true, "Split": true, "Cut": true, "CutPrefix": true,
	"CutSuffix": true, "TrimLeft": true, "TrimRight": true, "Trim": true,
}

func TestGoCodeDoesNotEditMessageText(t *testing.T) {
	root := reporoot.Root()
	found := map[string]int{}
	for _, top := range []string{"cmd", "internal", "lib", "tools"} {
		err := reporoot.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			f, err := goparser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(f, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok && isTextFunc(call) {
					for _, a := range call.Args {
						if isMessageText(a) {
							found[rel]++
						}
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for file, n := range found {
		switch allowed := textEditing[file]; {
		case n > allowed:
			t.Errorf("%s: iletinin metni Go'da düzenleniyor (%d kez, izinli %d); parametresini messages.As(err).Args'tan okuyun", file, n, allowed)
		case n < allowed:
			t.Errorf("%s: izin listesi eskimiş (%d kullanım kaldı, listede %d); kaydı küçültün", file, n, allowed)
		}
	}
	for file := range textEditing {
		if found[file] == 0 {
			t.Errorf("%s: artık ihlal yok; kaydı silin", file)
		}
	}
}

// isTextFunc çağrı strings paketinin metin düzenleyen işlevlerinden biri mi?
func isTextFunc(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "strings" && textFuncs[sel.Sel.Name]
}

// isMessageText ifade `x.Error()` ya da `messages.Text(...)` çağrısı mı?
func isMessageText(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if sel.Sel.Name == "Error" && len(call.Args) == 0 {
		return true
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "messages" && sel.Sel.Name == "Text"
}
