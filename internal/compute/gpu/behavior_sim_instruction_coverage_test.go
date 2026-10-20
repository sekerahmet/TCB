package gpu

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

// Çekirdeklere eklenen her PTX komutu benzeticiye de eklenir: gerçek kart varken sınamalar
// benzetici yerine kartta koşar, bu yüzden benzeticinin bir komutu tanımadığı ancak GPU'suz
// makinede anlaşılır. Bu test, yazıcının (emitter) komut metodlarına verilen her komut
// adının benzetici tarafından tanındığını kaynak üstünden denetler. Komutun veri düzenini
// sınayan testler (TestMmaFragmentLayout, TestShflXor) hem benzeticide hem kartta koşar.

// warpOps benzeticide sim_warp.go'nun ayrıca çözdüğü warp düzeyi komutlardır (simOpNames'te
// değildir).
var warpOps = map[string]bool{"mma": true, "shfl": true}

// literalOp bir komut argümanının başındaki sabit metindir: düz sabit, fmt.Sprintf'in
// biçimi ya da bir + zincirinin ilk sabiti. Çözülemezse "" verir.
func literalOp(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind == token.STRING {
			s, _ := strconv.Unquote(x.Value)
			return s
		}
	case *ast.BinaryExpr:
		return literalOp(x.X)
	case *ast.CallExpr:
		if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Sprintf" && len(x.Args) > 0 {
			return literalOp(x.Args[0])
		}
	}
	return ""
}

// emittedOps yazıcının komut metodlarına verilen komut adlarını (ilk noktaya kadar) toplar;
// ikinci değer çözülemeyen (değişkenle verilen) çağrıların sayısıdır.
func emittedOps(t *testing.T) (ops map[string][]string, unresolved int) {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	ops = map[string][]string{}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			arg := -1 // komut adının argüman sırası
			switch sel.Sel.Name {
			case "e":
				arg = 0
			case "ifCase", "otherwise":
				arg = 1
			}
			if arg < 0 || len(call.Args) <= arg {
				return true
			}
			op := literalOp(call.Args[arg])
			base, _, _ := strings.Cut(op, ".")
			if base == "" || strings.Contains(base, "%") {
				unresolved++
				return true
			}
			ops[base] = append(ops[base], name)
			return true
		})
	}
	return ops, unresolved
}

func TestEveryEmittedInstructionIsSimulated(t *testing.T) {
	ops, unresolved := emittedOps(t)
	if len(ops) < 20 {
		t.Fatalf("yalnızca %d komut adı bulundu; tarama bozulmuş olabilir", len(ops))
	}
	t.Logf("%d farklı komut adı, %d çözülemeyen çağrı", len(ops), unresolved)
	var names []string
	for op := range ops {
		names = append(names, op)
	}
	sort.Strings(names)
	for _, op := range names {
		if _, ok := simOpNames[op]; !ok && !warpOps[op] {
			t.Errorf("%s komutu %v dosyalarında üretiliyor ama benzetici tanımıyor (sim_parse.go simOpNames / sim_warp.go)", op, ops[op])
		}
	}
	// Değişkenle verilen komutlar (ör. 16 bitlik çift dönüşümü) kaynaktan çözülemez; sayıları
	// küçük kalmalı, yoksa tarama anlamını yitirir.
	if unresolved > 24 {
		t.Errorf("%d komut çağrısı çözülemedi; tarama yetersiz kalıyor", unresolved)
	}
}
