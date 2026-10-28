package interp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Durumu değiştiren işlerin listesi (1.5) tek yerde yazılır: her kodun bir adı vardır ve bu işlerin
// hiçbiri bırakılabilir bir bekleme (Abandonable, AwaitNew) yapmaz: bırakılan iş yarıda kalmış bir
// durum değişikliği olurdu.
func TestStateChangesAreNeverAbandoned(t *testing.T) {
	listed := map[int]string{}
	for _, k := range StateChanges {
		if k == codes.None || codes.Name(k) == "" {
			t.Errorf("%v: kodun adı yok", k)
		}
		listed[int(k)] = k.String()
	}
	values := constantValues(t)
	for _, dir := range []string{"internal/lang/interp", "internal/stdlib"} {
		walkNonTestGoFiles(t, filepath.Join(reporoot.Root(), filepath.FromSlash(dir)), func(path string, file *ast.File, fset *token.FileSet) {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || !callsAbandonable(fn) {
					continue
				}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					sel, ok := n.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "codes" {
						if what, isChange := listed[values[sel.Sel.Name]]; isChange {
							t.Errorf("%s: %s durumu değiştirir (%s) ama bırakılabilir bir bekleme yapıyor",
								fset.Position(sel.Pos()), sel.Sel.Name, what)
						}
					}
					return true
				})
			}
		})
	}
}

// callsAbandonable fonksiyonun bırakılabilir bir bekleme çağırdığını söyler.
func callsAbandonable(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && (sel.Sel.Name == "Abandonable" || sel.Sel.Name == "AwaitNew") {
				found = true
			}
		}
		return !found
	})
	return found
}

// constantValues codes paketinin üretilmiş sabitlerinin değerlerini verir (ad → sayı).
func constantValues(t *testing.T) map[string]int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(reporoot.Root(), "internal", "lang", "codes", "codes_generated.go"))
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]int{}
	pattern := regexp.MustCompile(`^\s*(\w+)\s+Code\s*=\s*(\d+)`)
	for _, line := range strings.Split(string(data), "\n") {
		if m := pattern.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[2])
			values[m[1]] = n
		}
	}
	if len(values) < 100 {
		t.Fatalf("sabitler okunamadı: %d", len(values))
	}
	return values
}

// walkNonTestGoFiles dir altındaki test olmayan Go dosyalarını ayrıştırıp fn'e verir.
func walkNonTestGoFiles(t *testing.T, dir string, fn func(path string, file *ast.File, fset *token.FileSet)) {
	t.Helper()
	err := reporoot.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		fn(path, file, fset)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
