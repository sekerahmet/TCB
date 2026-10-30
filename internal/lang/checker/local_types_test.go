package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// firstValueType kaynağın son deyimi olan TR_var'ın ilk değerinin çıkarılan tipidir.
func firstValueType(t *testing.T, source string) string {
	t.Helper()
	p := parser.New(lexer.NewBody(source, true))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("beklenmeyen sözdizimi hataları: %v\nkaynak:\n%s", p.Errors(), source)
	}
	c := New()
	c.CheckProgram(prog)
	last, ok := prog.Statements[len(prog.Statements)-1].(*ast.VarStatement)
	if !ok {
		t.Fatalf("son deyim TR_var değil:\n%s", source)
	}
	return c.valueTypes[last.Value]
}

// expectTypes her kaynağın son TR_var'ının ilk değerinin tipini sınar.
func expectTypes(t *testing.T, cases map[string]string) {
	t.Helper()
	for source, want := range cases {
		if got := firstValueType(t, source); got != want {
			t.Errorf("tip %q beklenirdi, gelen %q\nkaynak:\n%s", want, got, source)
		}
	}
}

// Kural (1.2, 3.13): açıklamasız yerel değişkenin tipi ilk değerinden çıkar ve değişmez; sonraki
// atama o tipe uyar. Tam sayı değişkene ondalık atanamaz; ondalık değişkene tam sayı atanabilir.
func TestLocalTypeIsPermanent(t *testing.T) {
	singleError(t, "TR_var toplam = 0\ntoplam += 0.5", messages.H1005, 2, `"toplam" değişkeni (TR_Int)`, `"TR_Float"`)
	singleError(t, "TR_var x = 5\nx = \"beş\"", messages.H1005, 2, `"x" değişkeni (TR_Int)`, `"TR_String"`)
	singleError(t, "TR_var m = \"a\"\nTR_func f() {\n    m = TR_true\n}", messages.H1005, 3, `"m" değişkeni (TR_String)`, `"TR_Bool"`)
	singleError(t, "TR_var x = 10\nx /= 4", messages.H1005, 2, `"TR_Float"`, "'//'")
	noErrors(t, "TR_var toplam = 0.0\ntoplam += 1\ntoplam = 2")
	noErrors(t, "TR_var s = \"a\"\ns += 1\ns = \"b\"")
	noErrors(t, "TR_var n = 0\nn += 1\nn = n * 2 - 1\nn //= 2")
}

// Kural (yanlış alarm vermez): tipi çıkarılamayan ilk değer (bilinmeyen çağrı, TR_none) ve
// denetleyicinin tanımadığı tip (ör. TR_JSON; tip parametresi, arayüz) değişkene verilmez; değişken TR_Any kalır.
func TestLocalTypeUnknownStaysAny(t *testing.T) {
	noErrors(t, "TR_var a = TR_none\na = 5\na = \"x\"")
	noErrors(t, "TR_var a = TR_input()\na = 5\na = \"x\"")
	noErrors(t, "TR_func oku() -> TR_JSON {\n    TR_return TR_json_parse(\"1\")\n}\nTR_var y = oku()\ny = \"a\"")
}

// Kural (4.2, 4.10, 15.6): değişmezin, aritmetiğin ve karşılaştırmanın tipi çalışma anının
// kurallarıyla çıkar; işlenenin tipi bilinmiyorsa ya da işlem tanımsızsa sonuç TR_Any'dir.
func TestOperatorTypes(t *testing.T) {
	const n, f, s, ten = "TR_var n = 2\n", "TR_var f = 0.5\n", "TR_var s = \"a\"\n", "TR_var t = TR_Tensor([1.0]) @ TR_Tensor([1.0])\n"
	expectTypes(t, map[string]string{
		"TR_var v = 1":                      typeInt,
		"TR_var v = 1.5":                    typeFloat,
		"TR_var v = \"a\"":                  typeString,
		"TR_var v = TR_true":                typeBool,
		n + "TR_var v = -n":                 typeInt,
		f + "TR_var v = -f":                 typeFloat,
		s + "TR_var v = -s":                 typeAny,
		n + "TR_var v = TR_not n > 1":       typeBool,
		n + "TR_var v = n / 2":              typeFloat,
		n + "TR_var v = n // 2":             typeInt,
		n + f + "TR_var v = n * f":          typeFloat,
		n + "TR_var v = n == 2":             typeBool,
		n + "TR_var v = n > 1 TR_and n < 3": typeBool,
		s + "TR_var v = s * 3":              typeString,
		s + "TR_var v = 3 * s":              typeString,
		"TR_var v = [1] + [2]":              typeList,
		"TR_var v = [0] * 4":                typeList,
		ten + "TR_var v = t * 2":            typeTensor,
		ten + "TR_var v = 1.0 - t":          typeTensor,
		ten + "TR_var v = -t":               typeTensor,
		s + "TR_var v = s - 1":              typeAny,
		"TR_var v = TR_input() + 1":         typeAny,
	})
}
