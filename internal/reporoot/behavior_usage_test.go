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
		err := WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
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

// listingCalls dosya listeleyen ve reporoot'ta izlenen-dosya eşi olan çağrılardır.
var listingCalls = map[string]string{
	"filepath.Walk": "WalkDir", "filepath.WalkDir": "WalkDir", "filepath.Glob": "Glob", "os.ReadDir": "ReadDir",
}

// Testler (ve ortak test yardımcıları, internal/testkit) dosya listelemeyi reporoot ile yapar: WalkDir, Glob, ReadDir
// (filepath.Walk, filepath.WalkDir, filepath.Glob, os.ReadDir ya da os.DirFS üzerinde fs.WalkDir değil). Depo içindeki
// yolda yalnızca git'in izlediği dosyalar görünür, depo testleri geliştiricinin izlenmeyen ve yok sayılan dosyalarına
// bağlı olmaz (bağımsız denetim grup 6: WalkDir; grup 7: Glob ve ReadDir ile okuyan formatter TestRepoFiles ve e2e
// TestExamplesCoverLibraryNames izlenmeyen dosyayla düşüyordu). Depo dışındaki yolda (t.TempDir) bunlar standart
// kitaplığın kendisidir. Geride dosya bırakılmadığını arayan test reporoot.GlobAll'u adıyla çağırır; istisna listesi
// yoktur.
func TestTestsListFilesThroughReporoot(t *testing.T) {
	root := Root()
	scanned := 0
	for _, top := range []string{"cmd", "internal", "lib", "tools"} {
		err := WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			helper := strings.HasPrefix(rel, "internal/testkit/") && strings.HasSuffix(rel, ".go")
			if err != nil || d.IsDir() || !strings.HasSuffix(path, "_test.go") && !helper {
				return err
			}
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			scanned++
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch name := selectorName(call.Fun); {
				case listingCalls[name] != "":
					t.Errorf("%s: %s; reporoot.%s kullanın", rel, name, listingCalls[name])
				case name == "fs.WalkDir" && len(call.Args) > 0 && selectorName(callFun(call.Args[0])) == "os.DirFS":
					t.Errorf("%s: fs.WalkDir(os.DirFS(...)); klasörü reporoot.WalkDir ile gezin", rel)
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
}

// selectorName `paket.ad` biçimli ifadenin adıdır ("" değilse).
func selectorName(e ast.Expr) string {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	return pkg.Name + "." + sel.Sel.Name
}

// callFun çağrı ifadesinin çağrılanıdır (çağrı değilse nil).
func callFun(e ast.Expr) ast.Expr {
	if c, ok := e.(*ast.CallExpr); ok {
		return c.Fun
	}
	return nil
}
