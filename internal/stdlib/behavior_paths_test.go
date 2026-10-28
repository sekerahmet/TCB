package stdlib

import (
	"go/ast"
	"sort"
	"testing"
)

// osPathFuncs yol alıp dosya sistemine dokunan os işlevleridir.
var osPathFuncs = map[string]bool{
	"Open": true, "OpenFile": true, "OpenRoot": true, "ReadFile": true, "WriteFile": true,
	"Create": true, "Stat": true, "Lstat": true, "ReadDir": true, "Mkdir": true, "MkdirAll": true,
	"Remove": true, "RemoveAll": true, "Rename": true, "MkdirTemp": true,
}

// resolvers kullanıcının yolunu dilin tek yol kuralıyla çözen çağrılardır.
var resolvers = map[string]bool{"ResolvePath": true, "ResolvePathTried": true, "resolve": true}

// notUserPaths yorumlayıcı alıp os çağıran ama kullanıcının yazdığı bir yola dokunmayan işlevlerdir
// (gerekçeleriyle). Liste yalnızca küçülür: yeni kayıt eklenmez.
var notUserPaths = map[string]string{
	"internal/stdlib/checkpoint/read.go:loadNewest": "koşu klasörü TR_checkpoint() kurulurken bind.ResolvePath ile çözülmüştür",
	"internal/stdlib/files/copy.go:checkConflict":   "job.src ve job.last findSource/findTarget'ta çözülmüş yollardır",
	"internal/stdlib/files/temp.go:tempDirBuiltin":  "yol işletim sisteminin geçici klasöründen üretilir, kullanıcı yolu almaz",
}

// Yol alan bir yerleşik ya da üye (yorumlayıcıyı parametre olarak alan işlev), dosya sistemine
// dokunmadan önce yolu dilin tek kuralıyla (VM.ResolvePath) çözmelidir: kendi başına
// çalışma klasörüne ya da programın klasörüne bakan bir yerleşik, aynı göreli yolu başka
// yerleşikten farklı bulurdu. Yorumlayıcısız işlevler (m almayanlar) çözülmüş yolu alır.
func TestUserPathsResolvedByLanguageRule(t *testing.T) {
	var violations []string
	seen := map[string]bool{}
	for path, f := range productionFiles(t) {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || !takesVM(fn) {
				continue
			}
			var touches, resolves bool
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch f := c.Fun.(type) {
				case *ast.SelectorExpr:
					if x, ok := f.X.(*ast.Ident); ok && x.Name == "os" && osPathFuncs[f.Sel.Name] {
						touches = true
					}
					resolves = resolves || resolvers[f.Sel.Name]
				case *ast.Ident:
					resolves = resolves || resolvers[f.Name]
				}
				return true
			})
			key := path + ":" + fn.Name.Name
			if _, ok := notUserPaths[key]; ok {
				seen[key] = true
				continue
			}
			if touches && !resolves {
				violations = append(violations, key)
			}
		}
	}
	sort.Strings(violations)
	for _, v := range violations {
		t.Errorf("%s: dosya sistemine dokunuyor ama yolu VM.ResolvePath ile çözmüyor", v)
	}
	for key := range notUserPaths {
		if !seen[key] {
			t.Errorf("notUserPaths'taki %s artık yok; kaydı silin", key)
		}
	}
}

// takesVM işlevin *interp.VM parametresi olup olmadığını söyler.
func takesVM(fn *ast.FuncDecl) bool {
	for _, p := range fn.Type.Params.List {
		star, ok := p.Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		if sel, ok := star.X.(*ast.SelectorExpr); ok && sel.Sel.Name == "VM" {
			return true
		}
	}
	return false
}
