package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (3.13, yanlış alarm vermez): iki kullanıcı tipi ancak ikisi de bilinen yapıysa ve ayrıysa
// uyumsuzdur; arayüz ya da bilinmeyen bir tip başka bir kullanıcı tipini karşılayabilir. Çekirdek
// tip ile kullanıcı tipi uyumsuzdur.
func TestUserTypeCompatibility(t *testing.T) {
	structs := compat{isStruct: func(n string) bool { return n == "Nokta" || n == "Kare" }}
	for _, tt := range []struct {
		k                compat
		expected, actual string
		want             bool
	}{
		{structs, "Nokta", "Nokta", true},
		{structs, "Nokta", "Kare", false},
		{structs, "Şekil", "Kare", true},
		{structs, "Kare", "Şekil", true},
		{compat{}, "Nokta", "Kare", true},
		{structs, typeInt, "Kare", false},
		{structs, "Kare", typeInt, false},
		{structs, "Kare", typeNull, false},
		{structs, "Kare?", typeNull, true},
		{structs, "Kare?", "Kare", true},
		{structs, typeNull, "Kare", false},
	} {
		if got := tt.k.types(tt.expected, tt.actual); got != tt.want {
			t.Errorf("types(%q, %q) = %v, beklenen %v", tt.expected, tt.actual, got, tt.want)
		}
	}
}

// Kural: kabın tip metni en dış düzeyde bölünür; ileti adları tek tek çevirir, ayraçlar kalır.
func TestTypeShapeText(t *testing.T) {
	d := containerType(typeDict, []string{typeString, listType(typeInt)})
	if s := shapeOf(d); s.base != typeDict || len(s.args) != 2 || s.args[1] != listType(typeInt) {
		t.Errorf("shapeOf(%q) = %+v", d, s)
	}
	if got := mapTypeNames(d, func(n string) string { return "<" + n + ">" }); got != "<"+typeDict+">[<"+typeString+">, <"+typeList+">[<"+typeInt+">]]" {
		t.Errorf("mapTypeNames(%q) = %q", d, got)
	}
	if got := typeSpelling(d).MessageText("tr"); got != "TR_Dict[TR_String, TR_List[TR_Int]]" {
		t.Errorf("typeSpelling(%q) = %q", d, got)
	}
	pair := tupleType([]string{typeInt, d})
	if s := shapeOf(pair); s.base != "" || len(s.elems) != 2 || s.elems[1] != d {
		t.Errorf("shapeOf(%q) = %+v", pair, s)
	}
}

// Kural (3.6, 3.6a): kap tip argümanında değişmezdir; değişmezde tam sayı yazımı ondalıktır; kabın
// içinde bilinmeyen ad (tip parametresi, arayüz) her tipi karşılayabilir. Demet ancak aynı sayıda ve
// sırayla uyumlu öğeli demete uyar.
func TestContainerCompatibility(t *testing.T) {
	ints, floats := listType(typeInt), listType(typeFloat)
	for _, tt := range []struct {
		k                compat
		expected, actual string
		want             bool
	}{
		{compat{}, floats, ints, false},
		{compat{literal: true}, floats, ints, true},
		{compat{}, ints, floats, false},
		{compat{}, ints, typeList, true},
		{compat{}, typeList, ints, true},
		{compat{}, listType("T"), ints, true},
		{compat{}, ints, listType(typeString), false},
		{compat{}, ints, typeDict, false},
		{compat{}, tupleType([]string{typeInt, ints}), tupleType([]string{typeInt, typeList}), true},
		{compat{}, tupleType([]string{typeInt, typeInt}), tupleType([]string{typeInt, typeString}), false},
		{compat{}, tupleType([]string{typeInt, typeInt}), tupleType([]string{typeInt, typeInt, typeInt}), false},
		{compat{}, tupleType([]string{typeFloat, typeInt}), tupleType([]string{typeInt, typeInt}), false},
		{compat{literal: true}, tupleType([]string{typeFloat, typeInt}), tupleType([]string{typeInt, typeInt}), true},
		{compat{}, typeInt, tupleType([]string{typeInt, typeInt}), false},
		{compat{}, "T", tupleType([]string{typeInt, typeInt}), true},
		{compat{}, tupleType([]string{typeInt, typeInt}), typeAny, true},
	} {
		if got := tt.k.types(tt.expected, tt.actual); got != tt.want {
			t.Errorf("types(%q, %q) = %v, beklenen %v", tt.expected, tt.actual, got, tt.want)
		}
	}
}

// Kural (3.7a "Kurmak"): beklenen tipi TR_JSON olan yere verilen liste ve sözlük değişmezi, iç içe
// değişmezleriyle, JSON değeri olarak kurulur: bildirimde, varsayılanda, parametrede, dönüşte, alanda
// ve yeniden atamada. Değişmez olmayan kap ve tek değer (TR_JSON(x) ile çevrilir) atanamaz.
func TestJSONLiteralFitsAJSONPlace(t *testing.T) {
	for _, source := range []string{
		`TR_var g: TR_JSON = {"a": 1, "l": [1, "x", TR_none], "o": {"k": TR_true}}`,
		`TR_var g: TR_JSON? = []`,
		"TR_func f(j: TR_JSON = [1, \"a\"]) {\n}\nf({\"a\": 1})",
		"TR_func f() -> TR_JSON {\n    TR_return [1, 2.5]\n}",
		"TR_struct P {\n    j: TR_JSON\n}\nTR_var p = P(j: {\"a\": 1})\np.j = [\"b\"]",
		"TR_var g: TR_JSON = {\"a\": 1}\ng = {\"b\": [2]}",
	} {
		noErrors(t, source)
	}
	singleError(t, "TR_var l = [1]\nTR_var g: TR_JSON = l", messages.H1001, 2)
	singleError(t, "TR_var g: TR_JSON = 1", messages.H1001, 1)
}
