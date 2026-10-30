package checker

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// Kural (6.1, 6.6, 7.1): tipsiz parametre, tipsiz alan ve değer döndürüp '->' yazmayan TR_func denetim
// hatasıdır. İleti düzeltilmiş satırı verir: tip varsayılandan ya da dönüşlerden çıkarsa düzeltme
// uygulanabilir, çağrılardan tahmin edilirse uygulanamaz; bulunamazsa yazım biçimi ve neden söylenir.

// gateCase bir tipsiz yerin beklenen tanısıdır: kod, satır, iletide geçen parçalar ve düzeltme
// (fixed boşsa düzeltme yoktur; applicable uygulanabilir mi).
type gateCase struct {
	source     string
	code       messages.Code
	line       int
	parts      []string
	fixed      string
	applicable bool
}

// gateError kaynağı dosya olarak (satırlarıyla, SetFile) denetler; tek bir hata bekler ve onu verir.
func gateError(t *testing.T, source string) *messages.PosError {
	t.Helper()
	p := parser.New(lexer.NewBody(source, true))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v\nkaynak:\n%s", p.Errors(), source)
	}
	c := New()
	c.SetFile(ProgramFile, source)
	c.CheckProgram(prog)
	if len(c.Errs()) != 1 {
		t.Fatalf("tek hata bekleniyordu, gelen %d: %v\nkaynak:\n%s", len(c.Errs()), c.Errors(), source)
	}
	e, ok := c.Errs()[0].(*messages.PosError)
	if !ok {
		t.Fatalf("hata konumlu değil: %v", c.Errs()[0])
	}
	return e
}

func (g gateCase) run(t *testing.T) {
	t.Helper()
	e := gateError(t, g.source)
	expectCode(t, e, g.code, g.line)
	text := e.Error()
	for _, p := range g.parts {
		if !strings.Contains(text, p) {
			t.Errorf("ileti %q parçasını içermeli: %q", p, text)
		}
	}
	switch {
	case g.fixed == "" && len(e.Fixes) > 0:
		t.Errorf("düzeltme beklenmiyordu: %+v", e.Fixes)
	case g.fixed != "" && (len(e.Fixes) != 1 || e.Fixes[0].Applicable != g.applicable):
		t.Errorf("uygulanabilir=%v tek düzeltme bekleniyordu: %+v", g.applicable, e.Fixes)
	case g.fixed != "":
		if got := applyFixes(g.source, e.Fixes); !strings.Contains(got, g.fixed) || !strings.Contains(text, g.fixed) {
			t.Errorf("düzeltilmiş satır %q iletide ve düzeltilmiş kaynakta olmalı:\nileti: %s\nkaynak:\n%s", g.fixed, text, got)
		}
	}
}

// applyFixes düzeltmeleri kaynağa uygular (aynı satırda sondan başa).
func applyFixes(source string, fixes []messages.Fix) string {
	lines := strings.Split(source, "\n")
	for i := len(fixes) - 1; i >= 0; i-- {
		f := fixes[i]
		line := []rune(lines[f.Line-1])
		lines[f.Line-1] = string(line[:f.Col-1]) + f.Replacement + string(line[f.EndCol-1:])
	}
	return strings.Join(lines, "\n")
}

func TestUntypedFunctionParameter(t *testing.T) {
	for name, g := range map[string]gateCase{
		"varsayılandan": {source: "TR_func f(a: TR_Int, b = 0) -> TR_Int { # ERROR H1109\n    TR_return a + b\n}\nf(1)", code: messages.H1109, line: 1,
			parts: []string{`"b"`, "(6.1)"}, fixed: "TR_func f(a: TR_Int, b: TR_Int = 0) -> TR_Int {", applicable: true},
		"çağrılardan": {source: "TR_func f(a) -> TR_Float { # ERROR H1110\n    TR_return a * 2.0\n}\nf(1.5)\nf(2.0)", code: messages.H1110, line: 1,
			parts: []string{`"a"`, "2 çağrının hepsi", "tahmin"}, fixed: "TR_func f(a: TR_Float) -> TR_Float {"},
		"çağrısız": {source: "TR_func f(a) { # ERROR H1111\n    TR_print(a)\n}", code: messages.H1111, line: 1,
			parts: []string{`"a"`, "çağrı yok", "a: <tip>"}},
		"farklı çağrılar": {source: "TR_func f(a) { # ERROR H1111\n    TR_print(a)\n}\nf(1)\nf(\"x\")", code: messages.H1111, line: 1,
			parts: []string{"farklı ya da bilinmeyen"}},
		"bilinmeyen varsayılan": {source: "TR_func f(a: TR_Int, b = TR_none) { # ERROR H1111\n    TR_print(a, b)\n}\nf(1, 2)", code: messages.H1111, line: 1,
			parts: []string{`"b"`, "varsayılan değerin tipi"}},
		"öğe tipsiz varsayılan": {source: "TR_func f(l = []) { # ERROR H1111\n    TR_print(l)\n}\nf([1])", code: messages.H1111, line: 1,
			parts: []string{"öğe tipini vermiyor"}},
	} {
		t.Run(name, g.run)
	}
}

func TestUntypedMethodParameter(t *testing.T) {
	for name, g := range map[string]gateCase{
		"varsayılandan": {source: "TR_struct S {\n    TR_func m(k = 1.5) -> TR_Float { # ERROR H1109\n        TR_return k\n    }\n}", code: messages.H1109, line: 2,
			fixed: "TR_func m(k: TR_Float = 1.5) -> TR_Float {", applicable: true},
		"metot çağrısından": {source: "TR_struct S {\n    TR_func m(k) { # ERROR H1110\n        TR_print(k)\n    }\n}\nTR_var s = S()\ns.m(\"a\")", code: messages.H1110, line: 2,
			parts: []string{"tek çağrı", "TR_String"}, fixed: "TR_func m(k: TR_String) {"},
		"kurucu çağrısından": {source: "TR_struct S {\n    n: TR_Int\n    TR_func TR_init(k) { # ERROR H1110\n        TR_self.n = k\n    }\n}\nTR_var s = S(3)", code: messages.H1110, line: 3,
			fixed: "TR_func TR_init(k: TR_Int) {"},
	} {
		t.Run(name, g.run)
	}
}

// 6.6: fonksiyon ifadesinin parametresi tipini beklenen fonksiyon tipinden alır; beklenen tip yoksa
// (tipi açıklanmamış değişkenin ilk değeri) yazılır.
func TestUntypedFunctionExpressionParameter(t *testing.T) {
	gateCase{source: "TR_var f = (x) => x * 2 # ERROR H1111", code: messages.H1111, line: 1, parts: []string{`"x"`, "(6.1, 6.6)", "beklenen tip yok"}}.run(t)
	gateCase{source: "TR_var l = [(x) => x * 2] # ERROR H1111", code: messages.H1111, line: 1, parts: []string{"beklenen tip yok"}}.run(t)
	for _, src := range []string{
		"TR_var f: (TR_Int) -> TR_Int = (x) => x * 2",
		"TR_print([1, 2].TR_map((x) => x * 2))",
		"TR_var f = (x: TR_Int) => x * 2",
		"TR_var f = (_) => 0",
		"TR_func uygula(g: (TR_Int) -> TR_Int) -> TR_Int {\n    TR_return g(1)\n}\nTR_print(uygula((x) => x + 1))",
	} {
		noErrors(t, src)
	}
}

func TestUntypedStructField(t *testing.T) {
	for name, g := range map[string]gateCase{
		"varsayılandan": {source: "TR_struct S {\n    n = 0 # ERROR H1109\n}", code: messages.H1109, line: 2,
			parts: []string{`"n"`, "(7.1)"}, fixed: "n: TR_Int = 0", applicable: true},
		"kurmadan": {source: "TR_struct S {\n    ad # ERROR H1110\n}\nTR_var s = S(\"a\")\nTR_var t = S(ad: \"b\")", code: messages.H1110, line: 2,
			parts: []string{"2 çağrının hepsi", "(7.1)"}, fixed: "ad: TR_String"},
		"kurulmayan": {source: "TR_struct S {\n    ad # ERROR H1111\n}", code: messages.H1111, line: 2, parts: []string{"(7.1)", "ad: <tip>"}},
	} {
		t.Run(name, g.run)
	}
}

func TestUntypedEnumField(t *testing.T) {
	gateCase{source: "TR_enum D {\n    Bekliyor\n    Çalışıyor(adım) # ERROR H1111\n}", code: messages.H1111, line: 3, parts: []string{`"adım"`, "(7.1)"}}.run(t)
}

func TestMissingReturnType(t *testing.T) {
	for name, g := range map[string]gateCase{
		"dönüşlerden": {source: "TR_func f(a: TR_Bool) { # ERROR H1112\n    TR_if a {\n        TR_return 1\n    }\n    TR_return 2.5\n}", code: messages.H1112, line: 1,
			parts: []string{`"f"`, "(6.1)", "TR_Float"}, fixed: "TR_func f(a: TR_Bool) -> TR_Float {", applicable: true},
		"demet": {source: "TR_func f() { # ERROR H1112\n    TR_return 1, \"a\"\n}", code: messages.H1112, line: 1,
			fixed: "TR_func f() -> (TR_Int, TR_String) {", applicable: true},
		"metot": {source: "TR_struct S {\n    TR_func m() { # ERROR H1112\n        TR_return \"a\"\n    }\n}", code: messages.H1112, line: 2,
			fixed: "TR_func m() -> TR_String {", applicable: true},
		"farklı tipler": {source: "TR_func f(a: TR_Bool) { # ERROR H1113\n    TR_if a {\n        TR_return 1\n    }\n    TR_return \"x\"\n}", code: messages.H1113, line: 1,
			parts: []string{"farklı ya da bilinmiyor", "-> <tip>"}},
		"değersiz yol": {source: "TR_func f(a: TR_Bool) { # ERROR H1113\n    TR_if a {\n        TR_return 1\n    }\n    TR_return\n}", code: messages.H1113, line: 1,
			parts: []string{"değer döndürmüyor"}},
	} {
		t.Run(name, g.run)
	}
	// Değer döndürmeyen fonksiyon '->' yazmaz; iç fonksiyonun dönüşü dışınkine sayılmaz.
	noErrors(t, "TR_func f() {\n    TR_return\n}")
	noErrors(t, "TR_func f() {\n    TR_func g() -> TR_Int { TR_return 1 }\n    TR_print(g())\n}")
}
