package ml

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Bileşik türev kapsam denetimi: türevli tensör işlemlerinden ikisini ya da fazlasını (karışık
// hassasiyet dönüşümü dahil) birleştiren her Go fonksiyonu bir compositeSpec'te anılmalı ya da
// gerekçesiyle burada istisna olmalı. Yeni bir bileşik testsiz eklenirse test düşer; kayıtta
// anılan fonksiyon yoksa (silinmiş, adı değişmiş) yine düşer.

// differentiableCalls türev kaydeden tensör çağrılarının (işlev ya da yöntem) adları ve
// highPrecision (karışık hassasiyette CastShared çağıran yardımcı) adıdır.
var differentiableCalls = map[string]bool{
	"Attention": true, "LayerNorm": true, "RMSNorm": true, "CrossEntropy": true, "Embedding": true,
	"Where": true, "Softmax": true, "LogSoftmax": true, "MatMul": true, "MatMulOut": true,
	"Add": true, "Sub": true, "Mul": true, "Div": true, "AddScalar": true, "MulScalar": true,
	"Neg": true, "Exp": true, "Log": true, "Sqrt": true, "Sum": true, "Mean": true, "Max": true,
	"Clamp": true, "Concat": true, "Reshape": true, "Cast": true, "CastShared": true, "To": true,
	"highPrecision": true,
}

// compositeExempt bileşik sayılıp kapıya girmeyenler ve gerekçeleri.
var compositeExempt = map[string]string{
	"checkGrad":     "sayısal denetleyici: kullanıcı işlevinin gradyanını sonlu farkla sınar, türev üretmez",
	"registerMath":  "her kapanış tek bir tensör işlemi sarar (tek işlem tensör paketinin kapısındadır); min/maks kapanışları minAll ve reduction kayıtlarında",
	"binary":        "dört işleçten birini seçer, tek bir tensör işlemi çağırır; işlemler tensör kapısındadır",
	"operator":      "sol işlenen sayıyken ters işleç: -, / ve skaler çıkarma/bölme arasından tek bir tensör işlemi seçer",
	"sampleBuiltin": "örnekleme: türevsiz",
}

// compositeFuncs ml paketinin test dışı kaynağında, differentiableCalls'tan en az ikisini
// çağıran fonksiyonların adlarıdır.
func compositeFuncs(t *testing.T) []string {
	t.Helper()
	files, _ := filepath.Glob("*.go")
	fset := token.NewFileSet()
	var out []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		tree, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range tree.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			names := map[string]bool{}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch c := n.(type) {
				case *ast.SelectorExpr:
					if differentiableCalls[c.Sel.Name] {
						names[c.Sel.Name] = true
					}
				case *ast.CallExpr:
					if id, ok := c.Fun.(*ast.Ident); ok && differentiableCalls[id.Name] {
						names[id.Name] = true
					}
				}
				return true
			})
			if len(names) >= 2 {
				out = append(out, fd.Name.Name)
			}
		}
	}
	sort.Strings(out)
	return out
}

func TestCompositeGradCoverage(t *testing.T) {
	covered := map[string]bool{}
	seen := map[string]bool{}
	for _, s := range compositeSpecs() {
		if seen[s.name] {
			t.Errorf("%s adı iki kez kayıtlı", s.name)
		}
		seen[s.name] = true
		if s.run == nil || s.ref == nil || len(s.shapes) == 0 || len(s.funcs) == 0 {
			t.Errorf("%s: run, ref, shapes ve funcs alanları dolu olmalı", s.name)
		}
		if s.bf16Tol <= 0 || s.bf16Tol > 0.1 {
			t.Errorf("%s: bf16 toleransı %g, (0, 0.1] içinde ve ölçülmüş olmalı", s.name, s.bf16Tol)
		}
		for _, f := range s.funcs {
			covered[f] = true
		}
	}
	found := compositeFuncs(t)
	exists := map[string]bool{}
	for _, f := range found {
		exists[f] = true
		if _, ok := compositeExempt[f]; !ok && !covered[f] {
			t.Errorf("%s birden çok türevli işlemi birleştiriyor ama bileşik gradyan kaydı (compositeSpecs) yok", f)
		}
	}
	for f, why := range compositeExempt {
		if !exists[f] {
			t.Errorf("istisna %q artık bileşik değil ya da silinmiş: kaydı kaldırın (%s)", f, why)
		}
	}
	for f := range covered {
		if !exists[f] && !fileHasFunc(t, f) {
			t.Errorf("compositeSpecs %q fonksiyonunu anıyor ama pakette böyle bir fonksiyon yok", f)
		}
	}
}

// fileHasFunc paketin test dışı kaynağında adı verilen fonksiyonun tanımlı olup olmadığını söyler.
func fileHasFunc(t *testing.T, name string) bool {
	t.Helper()
	files, _ := filepath.Glob("*.go")
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		tree, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range tree.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == name {
				return true
			}
		}
	}
	return false
}
