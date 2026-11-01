package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (3.7a "Karşılaştırmak", 4.5, 3.5): JSON değeri tipi bilinen JSON olmayan değerle (H1131) ve
// TR_none'la (H1132) karşılaştırılmaz: üst düzeyde, kabın içinde, .TR_contains'te ve eşleme deseninde.
// TR_Type(j) == TR_JSON hep yanlıştır (H1134). TR_JSON'a indeksle atama H2403'tür. JSON ile JSON,
// TR_JSON? ile TR_none, açılmış değer ve tipi bilinmeyen değer geçer.
func TestJSONComparison(t *testing.T) {
	const j = "TR_var j = TR_json_parse(\"{}\")\n"
	cases := []struct {
		src  string
		code messages.Code
	}{
		{"TR_var a = j[\"k\"] == 3", messages.H1131},
		{"TR_var a = \"x\" != j[\"s\"]", messages.H1131},
		{"TR_var a = j == [1]", messages.H1131},
		{"TR_var a = j[\"n\"] == TR_none", messages.H1132},
		{"TR_var a = TR_none != j", messages.H1132},
		{"TR_var a = [j[\"a\"]] == [1]", messages.H1131},
		{"TR_var a = TR_List(j) == [1, 2]", messages.H1131},
		{"TR_var a = [j] != [TR_none]", messages.H1132},
		{"TR_var a = {\"x\": j} == {\"x\": 1}", messages.H1131},
		{"TR_var a = (1, j) == (1, 2)", messages.H1131},
		{"TR_var a = TR_List(j).TR_contains(1)", messages.H1131},
		{"TR_var a = TR_List(j).TR_contains(TR_none)", messages.H1132},
		{"TR_var a = [1, 2].TR_contains(j)", messages.H1131},
		{"TR_var a = TR_match j {\n    1 => \"bir\"\n    _ => \"başka\"\n}", messages.H1131},
		{"TR_var a = TR_match j {\n    TR_none => \"yok\"\n    _ => \"başka\"\n}", messages.H1132},
		{"TR_var a = TR_Type(j) == TR_JSON", messages.H1134},
		{"TR_var a = TR_JSON != TR_Type(j[\"k\"])", messages.H1134},
		{"j[\"k\"] = 2", messages.H2403},
		{"j[0][\"k\"] += 2", messages.H2403},
	}
	for _, tc := range cases {
		_, errs := checkErrs(t, j+tc.src)
		if len(errs) != 1 || !messages.Has(errs[0], tc.code) {
			t.Errorf("%q: tek %s bekleniyordu, alınan %v", tc.src, tc.code, errs)
		}
	}
	noErrors(t, j+"TR_var a = j == TR_json_parse(\"1\")\nTR_var b = j[\"k\"] != TR_JSON(3)\nTR_var c = TR_Int(j[\"k\"]) == 3\nTR_var d = j.TR_get(\"k\") == TR_none")
	noErrors(t, j+"TR_var a = TR_Type(j) == TR_Dict\nTR_var b = TR_Type(j[\"n\"]) == TR_NoneType\nTR_var c = TR_List(j) == [TR_JSON(1)]\nTR_var d = TR_List(j).TR_contains(TR_JSON(1))")
	noErrors(t, j+"TR_var u = "+unknownValue+"\nTR_var a = j == u\nTR_var b = [j] == [u]\nTR_var c = TR_match j {\n    TR_JSON(1) => 1\n    _ => 2\n}\nTR_var k = TR_Dict\nTR_var e = TR_Type(1) == TR_Int")
}
