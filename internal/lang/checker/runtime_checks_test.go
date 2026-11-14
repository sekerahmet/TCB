package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// markedProgram kaynağı ayrıştırır ve denetler; hata varsa testi düşürür.
func markedProgram(t *testing.T, source string) *ast.Program {
	t.Helper()
	p := parser.New(lexer.NewBody(source, true))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi: %v\n%s", p.Errors(), source)
	}
	c := New()
	if errs := c.CheckProgram(prog); len(errs) > 0 {
		t.Fatalf("hata beklenmiyordu: %v\n%s", errs, source)
	}
	return prog
}

// Kural (1.2): tipli yere giren değer ona yalnız tipi bilinmediği için uyuyorsa (verilen yanda
// TR_Any ya da öğe tipi bilinmeyen kap) denetleyici onu yerin koduyla işaretler: tanım, atama,
// programın fonksiyonuna argüman, dönüş, alan yazma, kurucu ve kap öğesi; bileşik atamada atamanın
// kendisi. Tipi bilinen değere, beklenen yanı TR_Any ya da öğe tipi yazılmamış kap olan yere ve
// TR_JSON yere işaret konmaz.
func TestRuntimeCheckPlaces(t *testing.T) {
	m := "TR_var m = [1, \"a\"]\n"
	k := "TR_struct K { x: TR_Int }\n"
	for _, c := range []struct {
		name, src string
		code      messages.Code
		expected  string
	}{
		{"tanım", m + "TR_var y: TR_Int = m[1]\n", messages.H1001, typeInt},
		{"atama", m + "TR_var y: TR_Int = 0\ny = m[1]\n", messages.H1005, typeInt},
		{"bileşik atama", m + "TR_var y: TR_Int = 0\ny += m[1]\n", messages.H1005, typeInt},
		{"argüman", m + "TR_func f(x: TR_Int) -> TR_Int { TR_return x }\nTR_print(f(m[1]))\n", messages.H1004, typeInt},
		{"dönüş", m + "TR_func f() -> TR_Int { TR_return m[1] }\nTR_print(f())\n", messages.H1003, typeInt},
		{"alan", m + k + "TR_var o = K(x: 1)\no.x = m[1]\n", messages.H1124, typeInt},
		{"kurucu", m + k + "TR_var o = K(x: m[1])\n", messages.H1125, typeInt},
		{"kap öğesi", m + "TR_var l: TR_List[TR_Int] = [1]\nl[0] = m[1]\n", messages.H1136, typeInt},
		{"öğe tipsiz kap", m + "TR_var y: TR_List[TR_String] = m\n", messages.H1001, listType(typeString)},
		{"iç öğe tipsiz kap", m + "TR_var y: TR_List[TR_List[TR_Int]] = [m]\n", messages.H1001, listType(listType(typeInt))},
	} {
		prog := markedProgram(t, c.src)
		if len(prog.RuntimeChecks) != 1 {
			t.Errorf("%s: bir işaret bekleniyordu, gelen %d", c.name, len(prog.RuntimeChecks))
			continue
		}
		for n, r := range prog.RuntimeChecks {
			if r.Code != c.code || r.Expected != c.expected || r.Given < 0 || r.Hint < 0 || r.At == nil {
				t.Errorf("%s: işaret %v %q (given %d, hint %d), beklenen %v %q", c.name, r.Code, r.Expected, r.Given, r.Hint, c.code, c.expected)
			}
			if _, compound := n.(*ast.AssignStatement); compound != (r.Op != "") {
				t.Errorf("%s: bileşik atamanın işareti atamanın kendisinde olmalı: %T, işleç %q", c.name, n, r.Op)
			}
		}
	}
	for _, src := range []string{
		"TR_var y: TR_Int = 1\n",
		m + "TR_var y: TR_JSON = m[1]\n",
		m + "TR_var y = m[1]\n",
		"TR_var l: TR_List[TR_Float] = [1, 2]\n",
		"TR_var d: TR_Dict[TR_String, TR_Float] = {\"a\": 1}\n",
		m + "TR_print(m[1])\n",
	} {
		if prog := markedProgram(t, src); len(prog.RuntimeChecks) != 0 {
			t.Errorf("işaret beklenmiyordu (%d):\n%s", len(prog.RuntimeChecks), src)
		}
	}
	// Uygunluğun iki kolu yalnız verilen yanda sayılır: beklenen yan TR_Any ya da öğe tipi yazılmamış
	// kapsa tipi bilinen değer katı uygunluktan da geçer (bu yazımlar depoda tipsiz yer sayıldığı için
	// TR programı yerine doğrudan, TestTRCodeIsTyped).
	strict := compat{}
	for _, c := range []struct {
		expected, actual string
		strict           bool
	}{
		{typeList, typeList, true},
		{typeList, listType(typeInt), true},
		{typeAny, typeString, true},
		{listType(typeInt), typeList, false},
		{listType(typeInt), typeAny, false},
		{tupleType([]string{typeInt, typeString}), typeAny, false},
		{listType(listType(typeInt)), listType(typeList), false},
		{typeInt + "?", typeAny, false},
		{typeInt + "?", typeNull, true},
	} {
		if got := strict.strict(c.expected, c.actual, false); got != c.strict {
			t.Errorf("strict(%s, %s) = %v, beklenen %v", c.expected, c.actual, got, c.strict)
		}
	}
	// Bir sonraki denetim önceki programın işaretlerini taşımaz.
	c := New()
	first := parser.New(lexer.NewBody(m+"TR_var y: TR_Int = m[1]\n", true)).ParseProgram()
	second := parser.New(lexer.NewBody("TR_var z = 1\n", true)).ParseProgram()
	c.CheckProgram(first)
	c.CheckProgram(second)
	if len(first.RuntimeChecks) != 1 || len(second.RuntimeChecks) != 0 {
		t.Errorf("işaretler denetlenen programındır: %d, %d", len(first.RuntimeChecks), len(second.RuntimeChecks))
	}
}

// Kural (3.13): TR_Float yere yalnız tam sayı yazımı (değişmezin içinde de) geçer; TR_Int tipli değer
// (ad, işlem, değişmezin içindeki ad) yerin koduyla denetim hatasıdır. Tipli yerin dışında (işleç,
// karşılaştırma) en dıştaki tam sayı ondalığa uyar.
func TestIntValueIsNoFloat(t *testing.T) {
	n := "TR_var n: TR_Int = 1\n"
	for _, c := range []struct {
		name, src string
		code      messages.Code
		line      int
	}{
		{"tanım", n + "TR_var r: TR_Float = n\n", messages.H1001, 2},
		{"atama", n + "TR_var r: TR_Float = 0.5\nr = n\n", messages.H1005, 3},
		{"argüman", "TR_func f(x: TR_Float) -> TR_Float { TR_return x }\n" + n + "TR_print(f(n))\n", messages.H1004, 3},
		{"dönüş", "TR_func f(n: TR_Int) -> TR_Float { TR_return n }\nTR_print(f(1))\n", messages.H1003, 1},
		{"kurucu", "TR_struct K { x: TR_Float }\n" + n + "TR_var k = K(x: n)\n", messages.H1125, 3},
		{"işlem", n + "TR_var r: TR_Float = n + 1\n", messages.H1001, 2},
		{"kabın içi", n + "TR_var l: TR_List[TR_Float] = [n]\n", messages.H1001, 2},
		{"alan", "TR_struct K { x: TR_Float }\n" + n + "TR_var k = K(x: 0.5)\nk.x = n\n", messages.H1124, 4},
	} {
		t.Run(c.name, func(t *testing.T) { singleError(t, c.src, c.code, c.line) })
	}
	for _, src := range []string{
		"TR_var r: TR_Float = 1\n",
		"TR_var r: TR_Float = -2\n",
		"TR_var l: TR_List[TR_Float] = [1, 2]\n",
		"TR_var d: TR_Dict[TR_String, TR_Float] = {\"a\": 1}\n",
		"TR_var l: TR_List[TR_Float] = [1.5]\nl[0] = 2\n",
		"TR_var p: (TR_Float, TR_String) = (1, \"a\")\n",
		"TR_func f(x: TR_Float) -> TR_Float { TR_return x }\nTR_print(f(1))\n",
		"TR_var x = 0.5\nTR_var l: TR_List[TR_Float] = [x, 1]\n",
		"TR_var n = 1\nTR_print(n < 0.5)\n",
	} {
		noErrors(t, src)
	}
}
