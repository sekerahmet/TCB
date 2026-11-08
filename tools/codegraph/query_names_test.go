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

// Kural: test dışı bir çağrının argümanlarında ve bir düzey iç çağrı argümanlarında tek ayrı bir ad kodu (codes sabiti
// ya da takma sabiti; codes paketinin işlevine verilenler hariç) ve argüman olarak geçen tek bir işlev ya da somut
// alıcılı metot değeri varsa, işlevden koda INFERRED uses_name kenarı eklenir; gövdede EXTRACTED kenar varsa yeri
// korunur; iki ayrı kod, iki işlev ya da test dosyasındaki çağrı bağlanmaz (DENETİM 285, 286).
func TestAdjacentNameCodeInCallIsImplementedByArgumentCallable(t *testing.T) {
	root := auditModule(t, map[string]string{
		"internal/lang/codes/tr.json": `{
  "TR_mem_a": { "name": "üye_a", "family": "x" },
  "TR_fn_b":  { "name": "işlev_b", "family": "x" },
  "TR_fn_c":  { "name": "işlev_c", "family": "x" },
  "TR_fn_d":  { "name": "işlev_d", "family": "x" }
}
`,
		"internal/lang/codes/codes.go": `package codes

type Code int

const (
	MemA Code = 1 // TR_mem_a: member_a
	FnB  Code = 2 // TR_fn_b: fn_b
	FnC  Code = 3 // TR_fn_c: fn_c
	FnD  Code = 4 // TR_fn_d: fn_d
)

func Read(c Code) Code { return c }
`,
		"app/a_reg.go": `package app

import "example.com/audit/internal/lang/codes"

const aliasA = codes.MemA

type box struct{}

func (box) toA() {}

func fnB() {}
func fnC() {}
func fnTwo() {}
func fnRead() {}

func reg(c codes.Code, f func())       {}
func wrap(f func()) func()             { return f }
func safe(c codes.Code, f func()) func() { return f }
func twoCodes(a, b codes.Code, f func()) {}
func twoFuncs(c codes.Code, f, g func()) {}

func Register(b box) {
	reg(aliasA, b.toA)
	reg(codes.FnB, fnB)
	reg(codes.FnC, safe(codes.FnC, fnC))
	reg(codes.FnD, wrap(fnBody))
	twoCodes(codes.FnB, codes.FnC, fnTwo)
	twoFuncs(codes.FnB, fnB, fnTwo)
	reg(codes.Read(codes.FnB), fnRead)
}
`,
		"app/z_body.go": `package app

import "example.com/audit/internal/lang/codes"

func fnBody() { _ = codes.FnD }
`,
		"app/app_test.go": `package app

import (
	"testing"

	"example.com/audit/internal/lang/codes"
)

func testHelper() {}

func TestHelperDoesNotImplement(t *testing.T) { reg(codes.FnB, testHelper) }
`,
	})
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	const p = "go:func:example.com/audit/app."
	for _, w := range []struct{ fn, code, conf, file string }{
		{p + "box.toA", "name:TR_mem_a", inferred, "app/a_reg.go"},
		{p + "fnB", "name:TR_fn_b", inferred, "app/a_reg.go"},
		{p + "fnC", "name:TR_fn_c", inferred, "app/a_reg.go"},
		{p + "fnBody", "name:TR_fn_d", extracted, "app/z_body.go"},
	} {
		if e := edge(g, w.fn, w.code, edgeUsesName); e == nil || e.Confidence != w.conf || e.File != w.file || e.Count != 1 {
			t.Errorf("%s -> %s: got %+v, want conf=%s file=%s count=1", w.fn, w.code, e, w.conf, w.file)
		}
	}
	for _, fn := range []string{p + "fnTwo", p + "fnRead", p + "testHelper"} {
		for _, e := range g.Edges {
			if e.From == fn && e.Kind == edgeUsesName {
				t.Errorf("%s must not get a uses_name edge: %+v", fn, e)
			}
		}
	}
}
