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

// Cümle parçalardan kurulmaz: bir iletinin anlamı tek şablondadır, çünkü başka dillerde
// sözcük sırası, cinsiyet ve çekim farklıdır. Bir ileti koduna başka bir ileti kodunun metni
// (yüklem, nesne, iyelik parçası: "okunamadı", "bir değişkene") argüman olarak geçirilirse
// çevrilemez; her anlam kendi tam cümlesi olur, değişen yer şablonda `select` ile seçilir.
// Test, depodaki test dışı Go kodunda bir ileti çağrısına (`messages.New/Text`, `syntaxError`,
// `turkish.NewReason`) argüman olarak verilen `messages.Text(...)` çağrısını arar.
//
// Hata değeri taşıyan argümanlar (`messages.New(...)`, hata zinciri: "{fn}: {error}") parça
// sayılmaz. `interp.ArgCountError`'a verilen parametre adları (H2300-H2330: "bir değer",
// "dosya yolu") kendi başına anlamlı birer ad öbeğidir (messages README'si buna izin verir) ve
// `+` ile eklenen simge önekleri ("❌ ") sunumdur; ikisi de denetlenmez.
//
// fragmentUses, kuralın bugünkü istisnalarıdır (dosya → kullanım sayısı) ve yalnızca küçülür.
// Her kayıt gerekçesini yazar:
var fragmentUses = map[string]int{}

// messageBuilders ileti kuran işlevlerdir; argümanları iletinin parametreleridir.
var messageBuilders = map[string]bool{
	"New": true, "Text": true, "NewReason": true, "syntaxError": true, "Internalf": true, "Internal": true,
}

func TestMessageArgsAreNotMessageFragments(t *testing.T) {
	root := reporoot.Root()
	found := map[string]int{}
	var where []string
	for _, top := range []string{"cmd", "internal", "lib", "tools"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			fset := token.NewFileSet()
			f, err := goparser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(f, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.CallExpr:
					if !isMessageBuilder(x) {
						return true
					}
					for _, a := range x.Args {
						if isTextCall(a) {
							found[rel]++
							where = append(where, fset.Position(a.Pos()).String())
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
		switch allowed := fragmentUses[file]; {
		case n > allowed:
			t.Errorf("%s: bir iletinin metni başka bir iletiye parça olarak veriliyor (%d kez, izinli %d); anlamı tek şablonda, tam cümle olarak yazın (select ile)", file, n, allowed)
		case n < allowed:
			t.Errorf("%s: izin listesi eskimiş (%d kullanım kaldı, listede %d); kaydı küçültün", file, n, allowed)
		}
	}
	for file := range fragmentUses {
		if found[file] == 0 {
			t.Errorf("%s: artık ihlal yok; kaydı silin", file)
		}
	}
	if t.Failed() {
		t.Logf("kullanım yerleri:\n%s", strings.Join(where, "\n"))
	}
}

// isMessageBuilder çağrı `messages.New(...)` gibi bir ileti kuran işlev ya da yöntem mi?
func isMessageBuilder(call *ast.CallExpr) bool {
	switch f := call.Fun.(type) {
	case *ast.SelectorExpr:
		if pkg, ok := f.X.(*ast.Ident); ok && pkg.Name == "messages" {
			return messageBuilders[f.Sel.Name]
		}
		return f.Sel.Name == "syntaxError" || f.Sel.Name == "NewReason"
	case *ast.Ident:
		return f.Name == "NewReason"
	}
	return false
}

// isTextCall ifade `messages.Text(...)` çağrısı mı?
func isTextCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "messages" && sel.Sel.Name == "Text"
}
