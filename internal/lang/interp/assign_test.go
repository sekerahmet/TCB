package interp

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// assignInPlace sade değerde (plain) kancaya bakmadan döner; bu, hiçbir sade tipin
// InPlaceOperator olmamasına dayanır. Paket dışındaki bir tip interp.Value'yu kullanmak için
// interp'i aktarır, öyleyse plain'in tip listesine giremez (döngülü aktarma); paketin içinde
// InPlaceOp yöntemi tanımlı değildir. Bugün kancayı yalnız tensör (ml.TensorValue) uygular.
func TestNoPlainValueIsAnInPlaceOperator(t *testing.T) {
	for _, val := range []Value{NewInt(1), NewInt(5000), &FloatVal{Val: 1.5}, True, &StringVal{Val: "a"}} {
		if !plain(val) {
			t.Errorf("%T sade sayılmalı", val)
		}
		if _, ok := val.(InPlaceOperator); ok {
			t.Errorf("%T sade bir değer, InPlaceOperator olmamalı", val)
		}
	}
	dir := filepath.Join(reporoot.Root(), "internal", "lang", "interp")
	entries, err := reporoot.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv != nil && fn.Name.Name == "InPlaceOp" {
				t.Errorf("%s: interp'te InPlaceOp yöntemi; sade bir tipse assignInPlace'in erken dönüşü onu atlar", fset.Position(fn.Pos()))
			}
		}
	}
}

// taskAssignable SetIndexAt'i de uygulayan dış değer: her yazmanın görevini ve satırını kaydeder.
type taskAssignable struct {
	assignableValue
	lines []int
	vms   []*VM
}

func (a *taskAssignable) SetIndexAt(m *VM, line int, i, d Value) error {
	a.lines, a.vms = append(a.lines, line), append(a.vms, m)
	return a.SetIndex(i, d)
}

// Kural (A12, T1 dilim 2a; 15.45): indeksli atamada değer TaskIndexAssignable'ı uyguluyorsa VM onu
// görev ve atamanın satırıyla çağırır, SetIndex'in çağrıldığı her yerde (düz atama ve kancasız
// bileşik atamanın yazması); uygulamayan değerde SetIndex yolu aynen sürer; kancanın hatası satır
// öneklidir.
func TestIndexAssignTaskHookGetsTheLine(t *testing.T) {
	a := &taskAssignable{assignableValue: assignableValue{cell: map[int64]Value{}}}
	plain := &assignableValue{cell: map[int64]Value{}}
	vmx := New()
	vmx.GlobalEnv.Define("a", a, false)
	vmx.GlobalEnv.Define("p", plain, false)
	prog := mustParseProgram(t, "a[0] = 5\np[0] = 7\na[1] += 2\na[-1] = 3")
	_, err := vmx.Run(prog)
	if !atLine(err, 4, "negatif indeks") {
		t.Fatalf("kancanın hatası satır önekli olmalı: %v", err)
	}
	if fmt.Sprint(a.lines) != "[1 3 4]" || a.vms[0] == nil {
		t.Fatalf("kancanın satırları %v (görev %v), beklenen [1 3 4]", a.lines, a.vms)
	}
	if got := a.PlainValue().Inspect(); got != "[5, 2]" {
		t.Fatalf("kancalı atamalar: %v", got)
	}
	if got := plain.cell[0].Inspect(); got != "7" {
		t.Fatalf("kancasız değerde SetIndex yolu: %v", got)
	}
}
