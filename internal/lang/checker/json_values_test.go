package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (3.7a "Kurmak", "Okumak", "Açmak"): TR_json_parse(m), TR_JSON(x), j["k"] ve j[i] TR_JSON;
// TR_List(j) TR_List[TR_JSON]; .TR_get(k) TR_JSON?, .TR_keys() TR_List[TR_String], .TR_length TR_Int;
// tip çağrısı (TR_Int(j)) açar. TR_JSON başka tipin yerine verilmez, başka tip TR_JSON'un yerine de.
func TestJSONTypes(t *testing.T) {
	const j = "TR_var j = TR_json_parse(\"{}\")\n"
	expectTypes(t, map[string]string{
		"TR_var v = TR_json_parse(\"1\")": typeJSON,
		"TR_var v = TR_JSON([1, 2])":      typeJSON,
		j + "TR_var v = j[\"k\"]":         typeJSON,
		j + "TR_var v = j[\"k\"][0]":      typeJSON,
		j + "TR_var v = TR_List(j)":       listType(typeJSON),
		j + "TR_var v = j.TR_get(\"k\")":  optionalType(typeJSON),
		j + "TR_var v = j.TR_keys()":      listType(typeString),
		j + "TR_var v = j.TR_length":      typeInt,
		j + "TR_var v = TR_Int(j)":        typeInt,
		j + "TR_var v = TR_String(j[0])":  typeString,
	})
	singleError(t, "TR_var a: TR_Int = TR_json_parse(\"1\")", messages.H1001, 1, `"TR_JSON"`)
	singleError(t, "TR_func oku() -> TR_JSON {\n    TR_return TR_json_parse(\"1\")\n}\nTR_var y = oku()\ny = \"a\"", messages.H1005, 5)
	noErrors(t, j+"TR_var a: TR_Int = TR_Int(j[\"n\"])\nTR_var k: TR_JSON = j[\"k\"]\nk = TR_JSON(2)\nTR_var g: TR_JSON? = j.TR_get(\"g\")")
}

// Kural (3.7a "Kurmak"): TR_JSON(x)'te çevrilen değer ve öğeleri metin, sayı, mantık değeri,
// TR_none, liste, metin anahtarlı sözlük ve TR_JSON olabilir; tipi bilinen öbürü çalışma anındaki
// hatanın kendisidir (H2654), değişmezin öğesinde de. Tipi bilinmeyen değer ve öğe geçer.
func TestJSONConversionKinds(t *testing.T) {
	bad := []string{
		"TR_JSON((1, 2))",
		"TR_JSON([(1, 2)])",
		"TR_JSON([1, (1, 2)])",
		"TR_JSON(TR_Tensor([1.0]))",
		"TR_JSON({1: 2})",
		"TR_JSON({\"a\": {1: 2}})",
		"TR_JSON({\"a\": [TR_Tensor([1.0])]})",
		"TR_JSON(0..3)",
		"TR_func f() { }\nTR_print(TR_JSON(f))",
		"TR_struct P { a: TR_Int }\nTR_print(TR_JSON(P(1)))",
	}
	for _, src := range bad {
		_, errs := checkErrs(t, src)
		if len(errs) != 1 || !messages.Has(errs[0], messages.H2654) {
			t.Errorf("%q: tek H2654 bekleniyordu, alınan %v", src, errs)
		}
	}
	noErrors(t, "TR_var l = [1, 2]\nTR_var a = TR_JSON(l)\nTR_var b = TR_JSON({\"x\": [1, TR_none], \"y\": TR_true, \"z\": 2.5})")
	noErrors(t, "TR_var j = TR_JSON(TR_json_parse(\"1\"))\nTR_var k = TR_JSON([j, \"a\"])\nTR_var o: TR_Int? = TR_none\nTR_var m = TR_JSON(o)")
	noErrors(t, "TR_var u = TR_JSON("+unknownValue+")\nTR_var v = TR_JSON([1, "+unknownValue+"])\nTR_var karışık = [1, {1: 2}]\nTR_var w = TR_JSON(karışık)")
}

// Kural (3.7a "Kurmak"): beklenen tipi TR_JSON olan değişmezin içinde değişmez olmayan liste ya da
// sözlük (değişken, çağrının sonucu; iç içe değişmezde de) denetim hatasıdır, H1133: ileti kopyasını
// çevirmeyi önerir (`TR_JSON(l)`). JSON'a giremeyen, tipi bilinen öğe (tensör) çalışma anındaki
// hatanın kendisidir (H2654). Her JSON yerinde: tanım, dönüş, kullanıcı fonksiyonunun argümanı.
func TestJSONLiteralItems(t *testing.T) {
	const l = "TR_var l = [1]\n"
	cases := []struct {
		name, src string
		code      messages.Code
		line      int
		want      string
	}{
		{"liste değişkeni", l + "TR_var j: TR_JSON = {\"a\": l}", messages.H1133, 2, "`TR_JSON(l)`"},
		{"sözlük değişkeni", "TR_var d = {\"b\": 2}\nTR_var j: TR_JSON = [1, d]", messages.H1133, 2, "`TR_JSON(d)`"},
		{"iç içe", l + "TR_var j: TR_JSON = {\"a\": [1, {\"b\": l}]}", messages.H1133, 2, "`TR_JSON(l)`"},
		{"çağrının sonucu", "TR_var j: TR_JSON = {\"a\": \"x y\".TR_split(\" \")}", messages.H1133, 1, "`TR_JSON(\"x y\".TR_split(\" \"))`"},
		{"dönüş", l + "TR_func f() -> TR_JSON {\n    TR_return [l]\n}", messages.H1133, 3, "`TR_JSON(l)`"},
		{"argüman", l + "TR_func f(j: TR_JSON) { }\nf({\"a\": l})", messages.H1133, 3, "`TR_JSON(l)`"},
		{"tensör", "TR_var t = TR_Tensor([1.0])\nTR_var j: TR_JSON = {\"a\": t}", messages.H2654, 2, "TR_Tensor"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { fileErrorWith(t, tc.src, tc.code, tc.line, tc.want) })
	}
	noErrors(t, l+"TR_var d = {\"b\": 2}\nTR_var a: TR_JSON = {\"a\": TR_JSON(l), \"b\": [1, {\"c\": TR_none}], \"d\": TR_JSON(d)}")
	noErrors(t, l+"TR_var j: TR_JSON = TR_json_parse(\"1\")\nTR_var k: TR_JSON = [j, \"a\", 2.5]\nTR_var m = TR_JSON({\"a\": l})")
	noErrors(t, "TR_var u = ["+unknownValue+", 1][0]\nTR_var j: TR_JSON = {\"a\": u}\nTR_var x = {\"a\": [1]}")
}
