package tensor

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Gradyan kapısının kapsam denetimi: türev kaydeden her işlemin float64 sonlu fark ve bf16
// tolerans testi (gradSpec kaydı) olmalı. İşlem listesi koddan çıkar: BinOp/UnOp üyeleri
// (numBinOps/numUnOps sayaçlarıyla, yeni üye kendiliğinden eklenir) ve track/trackPass
// çağrılarının işlem adları (kaynak ayrıştırılarak). Listede olup kaydı olmayan ya da
// kayıtta olup listede olmayan işlem testi düşürür.

// dynamicTrackSites adı sabit bir metin olmayan track çağrılarının (dosya başına sayı) ve
// onların kaydettiği işlem adlarının tablosudur. Yeni bir dinamik çağrı eklenirse sayı tutmaz
// ve test, adların buraya (ve bir gradSpec'e) yazılmasını ister.
var dynamicTrackSites = map[string]struct {
	count int
	names []string
}{
	"custom_op.go":   {3, []string{"custom"}},
	"elementwise.go": {2, nil}, // binopName[op] ve unopName[op]: numaralandırmadan gelir
	"reduce.go":      {3, []string{"sum", "mean", "sum_all", "mean_all", "softmax", "log_softmax"}},
}

// enumOpNames BinOp ve UnOp üyelerinin ad tablosundaki adlarıdır.
func enumOpNames(t *testing.T) []string {
	t.Helper()
	if len(binopName) != int(numBinOps) {
		t.Fatalf("binopName %d ad içeriyor, BinOp %d üyeli", len(binopName), numBinOps)
	}
	names := append([]string(nil), binopName[:]...)
	for i := UnOp(0); i < numUnOps; i++ {
		n, ok := unopName[i]
		if !ok {
			t.Fatalf("UnOp %d için unopName kaydı yok", i)
		}
		names = append(names, n)
	}
	return names
}

// trackOpNames paketin üretim kaynağındaki track/trackPass çağrılarının sabit işlem adlarını
// ve dosya başına dinamik çağrı sayısını verir.
func trackOpNames(t *testing.T) (literal map[string]bool, dynamic map[string]int) {
	t.Helper()
	literal, dynamic = map[string]bool{}, map[string]int{}
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
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil || fd.Name.Name == "trackPass" {
				continue // trackPass kendi çağrılarına adı iletir
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok || len(c.Args) < 2 {
					return true
				}
				if id, ok := c.Fun.(*ast.Ident); !ok || (id.Name != "track" && id.Name != "trackPass") {
					return true
				}
				if lit, ok := c.Args[1].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					s, _ := strconv.Unquote(lit.Value)
					literal[s] = true
				} else {
					dynamic[f]++
				}
				return true
			})
		}
	}
	return literal, dynamic
}

// gradGroupsRun test dosyalarındaki runGradGroup(t, "grup") çağrılarının grup adlarıdır.
func gradGroupsRun(t *testing.T) map[string]bool {
	t.Helper()
	run := map[string]bool{}
	files, _ := filepath.Glob("*_test.go")
	fset := token.NewFileSet()
	for _, f := range files {
		tree, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(tree, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok || len(c.Args) != 2 {
				return true
			}
			if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "runGradGroup" {
				if lit, ok := c.Args[1].(*ast.BasicLit); ok {
					s, _ := strconv.Unquote(lit.Value)
					run[s] = true
				}
			}
			return true
		})
	}
	return run
}

func sortedKeys(m map[string]bool) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// TestGradCoverage türev kaydeden her işlemin gradyan kaydı vardır ve her kayıt bir testte koşar.
func TestGradCoverage(t *testing.T) {
	expected := map[string]bool{}
	for _, n := range enumOpNames(t) {
		expected[n] = true
	}
	literal, dynamic := trackOpNames(t)
	for n := range literal {
		expected[n] = true
	}
	for f, c := range dynamic {
		want := dynamicTrackSites[f]
		if want.count != c {
			t.Errorf("%s: adı sabit olmayan %d track çağrısı var, dynamicTrackSites %d diyor: yeni işlemin adlarını tabloya ve bir gradSpec'e ekleyin", f, c, want.count)
		}
	}
	for f, want := range dynamicTrackSites {
		if dynamic[f] == 0 {
			t.Errorf("%s: dynamicTrackSites'ta kayıt var ama dosyada adı sabit olmayan çağrı yok", f)
		}
		for _, n := range want.names {
			expected[n] = true
		}
	}

	covered, groupsRun, names, seen := map[string]bool{}, gradGroupsRun(t), map[string]bool{}, map[string]bool{}
	for group, specs := range gradGroups {
		if !groupsRun[group] {
			t.Errorf("%q grubunun kayıtları var ama hiçbir testte runGradGroup(t, %q) çağrısı yok", group, group)
		}
		for _, s := range specs {
			if names[group+"/"+s.name] {
				t.Errorf("%s/%s adı iki kez kayıtlı", group, s.name)
			}
			names[group+"/"+s.name] = true
			if s.build == nil || s.inputs == nil || s.ref == nil || len(s.ops) == 0 {
				t.Errorf("%s/%s: build, inputs, ref ve ops alanları dolu olmalı", group, s.name)
			}
			key := group + "/" + s.name
			tol, ok := bf16Tolerances[key]
			if !ok {
				t.Errorf("%s: bf16 toleransı yok (TR_MEASURE_BF16=write ile ölçüp tabloyu yenileyin)", key)
			}
			if tol > 0.1 || s.gradTol > 1e-3 {
				t.Errorf("%s: tolerans (bf16 %g, float64 %g) belgelenmiş üst sınırı aşıyor (0.1 / 1e-3)", key, tol, s.gradTol)
			}
			seen[key] = true
			for _, op := range s.ops {
				covered[op] = true
			}
		}
	}
	for key := range bf16Tolerances {
		if !seen[key] {
			t.Errorf("bf16Tolerances %q kaydını içeriyor ama böyle bir gradyan kaydı yok (tabloyu yenileyin)", key)
		}
	}
	for key := range bf16Tolerances {
		if !seen[key] {
			t.Errorf("bf16Tolerances %q kaydını içeriyor ama böyle bir gradyan kaydı yok (tabloyu yenileyin)", key)
		}
	}
	for _, n := range sortedKeys(expected) {
		if !covered[n] {
			t.Errorf("%q işleminin gradyan testi (gradSpec) yok", n)
		}
	}
	for _, n := range sortedKeys(covered) {
		if !expected[n] {
			t.Errorf("gradSpec bilinmeyen işlemi %q anıyor (yazım hatası ya da silinmiş işlem)", n)
		}
	}
}
