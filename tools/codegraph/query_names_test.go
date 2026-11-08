package main

import (
	"go/ast"
	"go/constant"
	"go/types"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Kural: bir ad kodu bileşik değerde bir işlevin anahtarıysa (`codes.CmdFormat: formatCommand`) kodun Go'daki
// uygulaması o çiftin değeridir, aynı satırdaki öbür çiftlerinki değil (keyedImplementations, implementationsAt). Çiftler
// go/types'tan bulunur, grafiğe bakılmadan; kodun kullanıldığı her satırda sorgu yalnız o işlevi adlandırır.
func TestAKeyedNameCodeIsImplementedByItsPairsValue(t *testing.T) {
	_, m, g := repoGraph(t)
	ix := newIndex(g)
	checked := 0
	for _, u := range m.Units {
		for _, f := range u.Files {
			if strings.HasSuffix(m.Fset.Position(f.Pos()).Filename, "_test.go") {
				continue
			}
			ast.Inspect(f, func(n ast.Node) bool {
				kv, ok := n.(*ast.KeyValueExpr)
				if !ok {
					return true
				}
				key, value := nameIdent(kv.Key), nameIdent(kv.Value)
				if key == nil || value == nil {
					return true
				}
				c, isConst := u.Info.Uses[key].(*types.Const)
				fn, isFunc := u.Info.Uses[value].(*types.Func)
				if !isConst || !isFunc || c.Pkg() == nil || c.Pkg().Path() != m.Path+"/"+codesDir || ix.byID[funcRef(fn)] == nil {
					return true
				}
				rel, line := m.Rel(m.Fset.Position(key.Pos()).Filename), m.Fset.Position(key.Pos()).Line
				want := ix.byID[funcRef(fn)].Name + " ["
				v, exact := constant.Int64Val(c.Val())
				if !exact {
					return true
				}
				code := "name:" + codes.Code(v).String()
				for _, e := range g.Edges {
					if e.Kind != edgeUsesName || e.To != code || e.File != rel || e.Line != line || ix.byID[e.From] == nil {
						continue
					}
					checked++
					if impl := ix.implementationsAt(ix.byID[e.From], &e); len(impl) != 1 || !strings.HasPrefix(impl[0], want) {
						t.Errorf("%s:%d: %s is implemented by %s, got %v", rel, line, e.To, want, impl)
					}
				}
				return true
			})
		}
	}
	if checked == 0 {
		t.Fatal("no name code keys a function in a composite literal: the scan no longer sees them")
	}
}

type wantNameEdge struct{ fn, code, conf, file string }

func checkNameImplModule(t *testing.T, appSrc, testSrc string, want []wantNameEdge, noEdge []string) *Graph {
	t.Helper()
	root := auditModule(t, map[string]string{
		"internal/lang/codes/tr.json": `{
  "TR_mem_a": { "name": "üye_a", "family": "x" },
  "TR_fn_b":  { "name": "işlev_b", "family": "x" },
  "TR_fn_c":  { "name": "işlev_c", "family": "x" },
  "TR_fn_d":  { "name": "işlev_d", "family": "x" },
  "TR_fn_e":  { "name": "işlev_e", "family": "x" },
  "TR_fn_f":  { "name": "işlev_f", "family": "x" }
}
`,
		"internal/lang/codes/codes.go": `package codes

type Code int

const (
	MemA Code = 1 // TR_mem_a: member_a
	FnB  Code = 2 // TR_fn_b: fn_b
	FnC  Code = 3 // TR_fn_c: fn_c
	FnD  Code = 4 // TR_fn_d: fn_d
	FnE  Code = 5 // TR_fn_e: fn_e
	FnF  Code = 6 // TR_fn_f: fn_f
)

func Read(c Code) Code { return c }
`,
		"app/a_reg.go": appSrc,
		"app/z_body.go": `package app

import "example.com/audit/internal/lang/codes"

func fnBody() { _ = codes.FnD }
`,
		"app/app_test.go": testSrc,
	})
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	const p = "go:func:example.com/audit/app."
	for _, w := range want {
		if e := edge(g, p+w.fn, w.code, edgeUsesName); e == nil || e.Confidence != w.conf || e.File != w.file || e.Count != 1 {
			t.Errorf("%s -> %s: got %+v, want conf=%s file=%s count=1", w.fn, w.code, e, w.conf, w.file)
		}
	}
	for _, fn := range noEdge {
		for _, e := range g.Edges {
			if e.From == p+fn && e.Kind == edgeUsesName {
				t.Errorf("%s must not get a uses_name edge: %+v", fn, e)
			}
		}
	}
	return g
}

// Kural: test dışı bir çağrının argümanlarında ve bir düzey iç çağrı argümanlarında tek ayrı bir ad kodu (`codes.X` sabiti
// ya da takma sabiti; codes paketinin işlevine verilenler hariç) ve argüman olarak geçen tek bir işlev ya da somut
// alıcılı metot değeri/ifadesi varsa, işlevden koda INFERRED uses_name kenarı eklenir; gövdede EXTRACTED kenar varsa yeri
// korunur; iki ayrı kod, iki işlev (`unit` biçimi: metot ifadesi ve işlev aynı çağrıda) ya da test dosyasındaki çağrı
// bağlanmaz; modül dışı işlev (`math.Floor` gibi) eşleşmede sayılmaz ve düğüm almaz (DENETİM 285, 286 ve 293).
func TestAdjacentNameCodeInCallIsImplementedByArgumentCallable(t *testing.T) {
	g := checkNameImplModule(t, `package app

import (
	"math"

	"example.com/audit/internal/lang/codes"
)

const aliasA = codes.MemA

type box struct{}

func (box) toA()      {}
func (box) exprE()    {}
func (box) exprUnit() {}
func (box) exprMath() {}

func fnB() {}
func fnC() {}
func fnTwo() {}
func fnRead() {}

func reg(c codes.Code, f func())                  {}
func regExpr(c codes.Code, f func(box))           {}
func unit(c codes.Code, m func(box), f func())    {}
func regMath(c codes.Code, f func(float64) float64) {}
func unitMath(c codes.Code, m func(box), f func(float64) float64) {}
func wrap(f func()) func()                        { return f }
func safe(c codes.Code, f func()) func()          { return f }
func twoCodes(a, b codes.Code, f func())          {}
func twoFuncs(c codes.Code, f, g func())          {}

func Register(b box) {
	reg(aliasA, b.toA)
	reg(codes.FnB, fnB)
	reg(codes.FnC, safe(codes.FnC, fnC))
	reg(codes.FnD, wrap(fnBody))
	regExpr(codes.FnE, box.exprE)
	unit(codes.FnE, box.exprUnit, fnTwo)
	regMath(codes.FnE, math.Floor)
	unitMath(codes.FnF, box.exprMath, math.Floor)
	twoCodes(codes.FnB, codes.FnC, fnTwo)
	twoFuncs(codes.FnB, fnB, fnTwo)
	reg(codes.Read(codes.FnB), fnRead)
}
`, `package app

import (
	"testing"

	"example.com/audit/internal/lang/codes"
)

func testHelper() {}

func TestHelperDoesNotImplement(t *testing.T) { reg(codes.FnB, testHelper) }
`, []wantNameEdge{
		{"box.toA", "name:TR_mem_a", inferred, "app/a_reg.go"},
		{"fnB", "name:TR_fn_b", inferred, "app/a_reg.go"},
		{"fnC", "name:TR_fn_c", inferred, "app/a_reg.go"},
		{"fnBody", "name:TR_fn_d", extracted, "app/z_body.go"},
		{"box.exprE", "name:TR_fn_e", inferred, "app/a_reg.go"},
		{"box.exprMath", "name:TR_fn_f", inferred, "app/a_reg.go"},
	}, []string{"fnTwo", "fnRead", "testHelper", "box.exprUnit"})
	for _, n := range g.Nodes {
		if strings.Contains(n.ID, "Floor") || strings.Contains(n.Name, "Floor") {
			t.Errorf("external module function math.Floor must not get a node: %+v", n)
		}
	}
}

// Kural: test dışı bir bileşik değerin yapı satırı olan öğelerinde (`[]struct{...}{{codes.X, f}}`, `{k: codes.X, fn: f}`,
// `map[K]struct{...}{k: {codes.X, f}}`, `&T{...}`) tek ayrı ad kodu ve tek işlev değeri (paket işlevi, somut metot değeri
// ya da metot ifadesi) varsa, işlevden koda INFERRED uses_name kenarı eklenir; harita çifti (`codes.X: f`), çağrı içeren
// alan (`{codes.X, wrap(f)}`), iki kodlu/iki işlevli satır ve test dosyası bağlanmaz (Adım 2b).
func TestTableRowNameCodeIsImplementedByRowCallable(t *testing.T) {
	mustEdge(t, fixture(t), fixturePkg+"runIf", "name:TR_kw_if", edgeUsesName, inferred)
	checkNameImplModule(t, `package app

import "example.com/audit/internal/lang/codes"

type box struct{}

func (box) toA()   {}
func (box) exprE() {}

func rowPos()     {}
func rowKeyed()   {}
func rowPtr()     {}
func rowWrapped() {}
func rowTwo()     {}
func mapFn()      {}

func wrap(f func()) func() { return f }

type row struct {
	k  codes.Code
	fn func()
}

type twoCodeRow struct {
	a, b codes.Code
	fn   func()
}

type twoFuncRow struct {
	k    codes.Code
	f, g func()
}

func Tables(b box) {
	_ = []row{
		{codes.FnB, rowPos},
		{k: codes.FnC, fn: rowKeyed},
		{codes.FnD, fnBody},
		{codes.FnB, wrap(rowWrapped)},
	}
	_ = []*row{
		&row{codes.FnF, rowPtr},
	}
	_ = map[string]row{
		"a": {codes.MemA, b.toA},
	}
	_ = []struct {
		k  codes.Code
		fn func(box)
	}{
		{codes.FnE, box.exprE},
	}
	_ = []twoCodeRow{{codes.FnB, codes.FnC, rowTwo}}
	_ = []twoFuncRow{{codes.FnB, rowPos, rowTwo}}
	_ = map[codes.Code]func(){codes.FnB: mapFn}
}
`, `package app

import (
	"testing"

	"example.com/audit/internal/lang/codes"
)

func testRowFn() {}

func TestRowInTestFileSkipped(t *testing.T) {
	_ = []row{{codes.FnB, testRowFn}}
}
`, []wantNameEdge{
		{"rowPos", "name:TR_fn_b", inferred, "app/a_reg.go"},
		{"rowKeyed", "name:TR_fn_c", inferred, "app/a_reg.go"},
		{"fnBody", "name:TR_fn_d", extracted, "app/z_body.go"},
		{"rowPtr", "name:TR_fn_f", inferred, "app/a_reg.go"},
		{"box.toA", "name:TR_mem_a", inferred, "app/a_reg.go"},
		{"box.exprE", "name:TR_fn_e", inferred, "app/a_reg.go"},
	}, []string{"rowWrapped", "rowTwo", "mapFn", "testRowFn"})
}
