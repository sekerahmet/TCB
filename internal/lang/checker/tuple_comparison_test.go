package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (3.6a, 4.4, 15.5, 13.11): tipi bilinen demette sınır dışı indeks H2774; sıralanamayan öğe
// çiftli demetin <, >, <=, >='si H2772 (mantık değeri, liste, ayrı türler, iç içe demette de). Çalışma
// anının kodlarıdır. Tensör tutan demetin == ve !='si eşitlenebilirliğin kuralıdır (H1148, iç içe demette
// ve öğenin listesinde de; TestUnequatableComparisonIsACheckError). Tipi bilinmeyen ve isteğe bağlı demet
// ile tipi bilinmeyen öğe geçer; sıralanan öğeler (sayı, metin, onların demetleri) ve tensörsüz demetin
// ==si geçer.
func TestTupleTypeRules(t *testing.T) {
	const x = "TR_var x = TR_Tensor([1.0])\n"
	cases := []struct {
		src  string
		code messages.Code
		line int
	}{
		{"TR_var t = (1, 2)\nTR_var a = t.2", messages.H2774, 2},
		{"TR_var t = ((1, 2), 3)\nTR_var a = t.0.5", messages.H2774, 2},
		{"TR_func f(t: (TR_Int, TR_Int)) -> TR_Int {\n    TR_return t.2\n}", messages.H2774, 2},
		{x + "TR_var a = (x, 1) == (x, 1)", messages.H1148, 2},
		{x + "TR_var a = (1, (2, x)) != (1, (2, x))", messages.H1148, 2},
		{x + "TR_var a = (1, [x]) == (1, [x])", messages.H1148, 2},
		{"TR_var a = (1, [1]) < (1, [2])", messages.H2772, 1},
		{"TR_var a = (1.0, TR_true) > (1.0, TR_false)", messages.H2772, 1},
		{"TR_var a = (1, \"a\") <= (1, 2)", messages.H2772, 1},
		{"TR_var a = ((1, TR_true), 2) >= ((1, TR_false), 2)", messages.H2772, 1},
	}
	for _, tc := range cases {
		singleError(t, tc.src, tc.code, tc.line)
	}
	noErrors(t, "TR_var t = (1, 2)\nTR_var a = t.1\nTR_var b = (1, \"a\") < (2, \"b\")\nTR_var c = ((1, 2), 3.5) >= ((1, 2), 3)\nTR_var d = (1, [1]) == (1, [1])")
	noErrors(t, x+"TR_var u = "+unknownValue+"\nTR_var a = u.5\nTR_var b = (u, 1) == (u, 1)\nTR_var c = (u, [1]) < (1, [2])")
	noErrors(t, "TR_var o: (TR_Int, TR_Bool)? = TR_none\nTR_var a = o == (1, TR_true)\nTR_var n: (TR_Int?, TR_Int) = (1, 2)\nTR_var b = n < (1, 2)")
}
