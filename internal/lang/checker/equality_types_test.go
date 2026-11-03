package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (4.5, H1141): == ve != tipleri bilinen ve ayrı iki değeri karşılaştırmaz: "1" == 1, öğe tipleri
// ayrı listeler ([1] == [1.0]), ayrı iki yapı. Sayılar, isteğe bağlı ile asıl tipi ve TR_none, JSON,
// fonksiyon, tip değeri, tensör, tipi bilinmeyen değer ve öğe tipsiz kap bu kurala girmez.
func TestDistinctTypesAreNotCompared(t *testing.T) {
	cases := []struct {
		name, src string
		line      int
	}{
		{"metin ve sayı", "TR_print(\"1\" == 1)", 1},
		{"eşit değil", "TR_print(\"1\" != 1)", 1},
		{"öğe tipleri ayrı listeler", "TR_print([1] == [1.0])", 1},
		{"sözlük", "TR_print({\"a\": 1} == {\"a\": \"b\"})", 1},
		{"demet uzunluğu", "TR_print((1, 2) == (1, 2, 3))", 1},
		{"iki yapı", "TR_struct A { x: TR_Int }\nTR_struct B { x: TR_Int }\nTR_print(A(1) == B(1))", 3},
		{"isteğe bağlı", "TR_var l: TR_List[TR_String] = []\nTR_print(l.TR_first == 1)", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { singleError(t, tc.src, messages.H1141, tc.line) })
	}
	noErrors(t, "TR_print(1 == 1.0, [1] == [], [1] == [2], (1, \"a\") == (2, \"b\"))\nTR_var l: TR_List[TR_Int] = []\nTR_print(l.TR_first == 1, l.TR_first == TR_none)")
	noErrors(t, "TR_var u = [1, \"a\"][0]\nTR_print(u == 1, u == \"a\")\nTR_var t = TR_Tensor([1.0])\nTR_print(TR_Type(5) == TR_Int)")
}

// Kural (3.7a, 4.5; H1131): TR_JSON? (j.TR_get(k)) tipi bilinen JSON olmayan değerle karşılaştırılmaz;
// TR_none'la ve TR_JSON'la karşılaştırılır. Öneri öbür yanı JSON'a çevirir, açmayı önermez.
func TestOptionalJSONComparison(t *testing.T) {
	const j = "TR_var j = TR_json_parse(\"{}\")\n"
	singleError(t, j+"TR_print(j.TR_get(\"q\") == 3)", messages.H1131, 2)
	singleError(t, j+"TR_print(\"a\" != j.TR_get(\"q\"))", messages.H1131, 2)
	noErrors(t, j+"TR_print(j.TR_get(\"q\") == TR_none, j.TR_get(\"q\") == TR_JSON(3), j.TR_get(\"q\") == j[\"a\"])")
}
