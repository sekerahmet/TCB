package interp

import (
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
