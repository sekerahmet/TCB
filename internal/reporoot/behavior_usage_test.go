package reporoot

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fileCalls dosya sistemine yol veren çağrılardır ("paket.işlev"). Bu çağrılara verilen bir
// metin sabiti ".." ile başlıyorsa test depo dosyasına göreli yolla ulaşıyordur.
var fileCalls = map[string]bool{
	"os.ReadFile": true, "os.Open": true, "os.OpenFile": true, "os.ReadDir": true, "os.Stat": true,
	"os.Lstat": true, "os.DirFS": true, "filepath.Join": true, "filepath.Walk": true,
	"filepath.WalkDir": true, "filepath.Glob": true, "filepath.Abs": true,
}

// Testler depodaki dosyalara (docs/, tests/, examples/, test verileri) `../../` gibi göreli
// yollarla değil reporoot.Root() ile ulaşır; paket taşınınca testler kırılmaz. Test, depodaki
// bütün *_test.go dosyalarında dosya çağrılarının ".." ile başlayan metin sabitlerini arar;
// bugün ihlal yoktur ve istisna listesi de yoktur.

func TestTestsUseRepoRootNotRelativePaths(t *testing.T) {
	root := Root()
	found := map[string]int{}
	scanned := 0
	for _, top := range []string{"cmd", "internal", "lib", "tools"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, "_test.go") {
				return err
			}
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			scanned++
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkg, ok := sel.X.(*ast.Ident)
				if !ok || !fileCalls[pkg.Name+"."+sel.Sel.Name] {
					return true
				}
				for _, a := range call.Args {
					if lit, ok := a.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						if s, err := strconv.Unquote(lit.Value); err == nil && (s == ".." || strings.HasPrefix(s, "../") || strings.HasPrefix(s, `..\`)) {
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
	if scanned < 100 {
		t.Fatalf("çok az test dosyası tarandı: %d", scanned)
	}
	for file, n := range found {
		t.Errorf("%s: %d göreli yol (`..`) kullanımı; depo dosyalarına reporoot.Root() ile ulaşın", file, n)
	}
}

// Yardımcı yalnızca testler içindir ve hiçbir iç paketi içe aktarmaz: test dışı hiçbir Go dosyası
// reporoot'u içe aktarmaz (ortak test yardımcıları internal/testkit hariç: onlar da yalnızca
// testlerden içe aktarılır, internal/testkit TestHelpersAreImportedOnlyByTests), reporoot'un
// kendisi de iç paket içe aktarmaz. (internal/layers_test.go
// araçlara izin verir; bugün hiçbir araç kullanmıyor, kural README'deki gibi katıdır.)
func TestHelperImportedOnlyByTests(t *testing.T) {
	root := Root()
	const self = "github.com/sekerahmet/TCB/internal/reporoot"
	for _, top := range []string{"cmd", "internal", "lib", "tools"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range f.Imports {
				p, _ := strconv.Unquote(imp.Path.Value)
				switch {
				case strings.HasPrefix(rel, "internal/reporoot/") && strings.HasPrefix(p, "github.com/sekerahmet/TCB/"):
					t.Errorf("%s: reporoot hiçbir iç paketi içe aktarmaz (%s)", rel, p)
				case p == self && !strings.HasSuffix(rel, "_test.go") && !strings.HasPrefix(rel, "internal/testkit/"):
					t.Errorf("%s: reporoot yalnızca testlerden içe aktarılır", rel)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
