package logging_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Adım kancası ucuzdur: kanca yokken İlerleme.güncelle'nin maliyeti tek bir atomik okumadır.
// Yapısal denetim: stepHook bir atomic.Pointer'dır; SetStepHook dışında yalnızca Load ile
// okunur (kilit, tahsis ya da kancayı hazırlayan iş eklenirse bu test düşer).
func TestStepHookCostsOneAtomicRead(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), reporoot.Path("internal", "stdlib", "logging", "hook.go"), nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	declared := false
	for _, d := range file.Decls {
		gen, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, s := range gen.Specs {
			v, ok := s.(*ast.ValueSpec)
			if !ok || len(v.Names) != 1 || v.Names[0].Name != "stepHook" {
				continue
			}
			declared = true
			idx, ok := v.Type.(*ast.IndexExpr) // atomic.Pointer[...]
			if !ok {
				t.Fatalf("stepHook atomic.Pointer olmalı")
			}
			if sel, ok := idx.X.(*ast.SelectorExpr); !ok || sel.Sel.Name != "Pointer" {
				t.Errorf("stepHook atomic.Pointer olmalı")
			}
		}
	}
	if !declared {
		t.Fatal("hook.go'da stepHook tanımı yok")
	}
	// Kullanımlar tüm paketin üretim dosyalarında: dışarıdan yalnızca Load.
	pkgDir := reporoot.Path("internal", "stdlib", "logging")
	names, err := reporoot.Glob(filepath.Join(pkgDir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	loads := 0
	fset := token.NewFileSet()
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "stepHook" {
				switch sel.Sel.Name {
				case "Load":
					loads++
				case "Store":
					if filepath.Base(name) != "hook.go" {
						t.Errorf("%s: stepHook.Store yalnızca hook.go'daki SetStepHook'ta olmalı", name)
					}
				default:
					t.Errorf("%s: stepHook.%s: yalnızca Load/Store izinli", name, sel.Sel.Name)
				}
			}
			return true
		})
	}
	if loads == 0 {
		t.Error("stepHook.Load çağrısı bulunamadı; kanca hiç okunmuyor")
	}
}
