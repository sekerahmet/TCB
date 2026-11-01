package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

const pointStruct = "TR_struct Nokta {\n    x: TR_Float\n    ad: TR_String = \"\"\n}\n"

// Kural (7.5, 3.13): tipi bilinen yapı nesnesinin alanına atanan değer alanın tipindedir; uymayan
// değer denetim hatasıdır (H1124). Bileşik atamada sonucun tipi alanın tipidir; tam sayı yazımı ondalık
// alana atanır; tipsiz alan ve tipi bilinmeyen nesne denetlenmez.
func TestFieldAssignType(t *testing.T) {
	singleError(t, pointStruct+"TR_var p = Nokta(1.0)\np.x = \"b\"", messages.H1124, 6, "`x`", "TR_Float", "TR_String")
	singleError(t, pointStruct+"TR_func f(p: Nokta) {\n    p.ad = TR_true\n}", messages.H1124, 6, "`ad`", "TR_Bool")
	noErrors(t, pointStruct+"TR_var p = Nokta(1.0)\np.x = 2\np.x += 1\np.ad = \"a\"\nTR_var j = TR_json_parse(\"{}\")\nj.x = \"b\"\nTR_print(p, j)")
}

// Kural (7.2, 3.13): yapı kurulurken alana verilen değer (konumlu: tanım sırasıyla; adlı: alanın adıyla)
// alanın tipindedir; uymayan değer denetim hatasıdır (H1125). Kurucusu olan yapı kurucunun imzasıyla
// kurulur (7.4), burada denetlenmez.
func TestConstructionFieldTypes(t *testing.T) {
	singleError(t, pointStruct+"TR_var p = Nokta(\"a\")", messages.H1125, 5, "`x`", "TR_Float", "TR_String")
	singleError(t, pointStruct+"TR_var p = Nokta(1.0, ad: 2)", messages.H1125, 5, "`ad`", "TR_String", "TR_Int")
	noErrors(t, pointStruct+"TR_var p = Nokta(1, ad: \"a\")\nTR_var q = Nokta(x: 2.5)\nTR_print(p, q)")
	noErrors(t, "TR_struct Kat {\n    n: TR_Int\n    TR_func TR_init(s: TR_String) {\n        TR_self.n = s.TR_length\n    }\n}\nTR_print(Kat(\"abc\"))")
}
