package parser

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Durumlu tip (7.7) yapının deyimiyle okunur: durumlar tanım sırasıyla, alanları 7.1'deki gibi; metotlar
// durumlardan sonra. Arayüz (7.8) gövdesiz metot imzalarıdır.
func TestEnumAndInterfaceDefinitions(t *testing.T) {
	p := New(codedLexer("TR_enum D {\n    A\n    B(n: TR_Int, m = 2)\n    TR_func f() { TR_return 1 }\n}\nTR_interface K {\n    TR_func f(x: TR_Float) -> TR_Float\n    TR_func g()\n}"))
	prog := p.ParseProgram()
	if errs := p.Errs(); len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	enum := prog.Statements[0].(*ast.StructStatement)
	if enum.Kind != ast.KindEnum || len(enum.Variants) != 2 || enum.Variants[1].Name != "B" || len(enum.Variants[1].Fields) != 2 || enum.Methods["f"] == nil {
		t.Errorf("enum %+v", enum)
	}
	iface := prog.Statements[1].(*ast.StructStatement)
	if iface.Kind != ast.KindInterface || len(iface.Methods) != 2 || iface.Methods["f"].Body != nil || iface.Methods["f"].ReturnType == nil {
		t.Errorf("interface %+v", iface)
	}
}

// Durumlu tip ve arayüz gövdesinde metot adı yapınınki gibi üyedir: kitaplığın metot adı yazılabilir.
func TestLibraryMethodNameInEnumAndInterface(t *testing.T) {
	for _, code := range []string{
		"TR_interface K {\n    TR_func TR_forward(x)\n}",
		"TR_enum D {\n    A\n    TR_func TR_forward(x) { TR_return x }\n}",
	} {
		if errs := parseErrs(code); len(errs) != 0 {
			t.Errorf("%q: %v", code, errs)
		}
	}
}

// Parantezsiz 'Durum.Durum(a, _)' deseni durumlu tip desenidir; parantezli ifade değeriyle
// karşılaştırılır (5.6).
func TestEnumPatternIsOnlyTheBareCall(t *testing.T) {
	p := New(codedLexer("TR_var x = TR_match d {\n    D.B(a, _) => a\n    (f(1)) => 0\n    _ => 1\n}"))
	prog := p.ParseProgram()
	if errs := p.Errs(); len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	m := prog.Statements[0].(*ast.VarStatement).Value.(*ast.MatchExpression)
	pat, ok := m.Cases[0].Patterns[0].(*ast.EnumPattern)
	if !ok || pat.Variant != "B" || len(pat.Names) != 2 || pat.Names[1].Value != "_" {
		t.Errorf("enum pattern %#v", m.Cases[0].Patterns[0])
	}
	if _, ok := m.Cases[1].Patterns[0].(*ast.CallExpression); !ok {
		t.Errorf("a parenthesized call stays a value: %#v", m.Cases[1].Patterns[0])
	}
}

// Durumlu tip ve arayüz tanımının yanlışları tek iletidir.
func TestEnumAndInterfaceErrors(t *testing.T) {
	for _, c := range []struct {
		code string
		want messages.Code
	}{
		{"TR_enum D {\n    A\n    TR_func TR_init() { }\n}", messages.H0285},
		{"TR_enum D {\n    TR_func f() { TR_return 1 }\n    A\n}", messages.H0286},
		{"TR_enum D {\n    A\n    A\n}", messages.H0293},
		{"TR_enum D {\n    A\n    TR_func f() { TR_return 1 }\n    TR_func f() { TR_return 2 }\n}", messages.H0293},
		{"TR_enum D {\n    A\n    TR_func A() { TR_return 1 }\n}", messages.H0294},
		{"TR_enum D {\n    A(n, n)\n}", messages.H0296},
		{"TR_enum D {\n    A(n = 1, m)\n}", messages.H0297},
		{"TR_interface K {\n    TR_func f()\n    TR_func f()\n}", messages.H0295},
		{"TR_interface K {\n}", messages.H0288},
		{"TR_interface K {\n    x\n}", messages.H0289},
		{"TR_interface K {\n    TR_func f() { TR_return 1 }\n}", messages.H0290},
		{"TR_var x = TR_match d {\n    D.B(n: a) => a\n}", messages.H0287},
		{"TR_var x = TR_match d {\n    D.B(1) => 1\n}", messages.H0291},
		// koşullu kol 5.5'in koşullu ifadesi değildir
		{"TR_var x = TR_match d {\n    D.B(a) TR_if a > 0 => a\n    _ => 0\n}", messages.H0292},
		// desenin '_'si bağlamaz; kolda okunması 5.3'ün hatasıdır
		{"TR_var x = TR_match d {\n    D.B(a, _) => _\n}", messages.H0253},
	} {
		if errs := parseErrs(c.code); len(errs) != 1 || !messages.Has(errs[0], c.want) {
			t.Errorf("%q: want one %v, got %v", c.code, c.want, errs)
		}
	}
}
