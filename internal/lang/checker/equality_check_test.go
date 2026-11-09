package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (4.5, 5.6, 7.6–7.8, 13.3, 15.5): eşitlenemeyen değerin '==' ve '!='si (tensör H1148, isteğe bağlı
// tensör H1151, fonksiyon H1152, arayüz H1153), '==' isteyen değer deseni (H1150) ve .TR_contains,
// .TR_index_of, .TR_unique'i (H1149) denetim hatasıdır; her iki yanda da. İleti biçime göre öneri verir:
// demette ve yapıda parça parça yazım (işlenenler adsa), isteğe bağlı değerde TR_none'la açma, listede ve
// sözlükte öğe öğe, durumlu tipte durumu açma, fonksiyonda sonuçlar, arayüzde alanlar. Geçenler: isteğe bağlı değerin TR_none değişmeziyle karşılaştırması
// (3.5), iki tensörün öğe öğe '=='si, durum açan desen, TR_equal ve TR_assert_equal (12.4).
func TestUnequatableComparisonIsACheckError(t *testing.T) {
	const defs = "TR_struct Kutu {\n    t: TR_Tensor\n    n: TR_Int\n}\n" +
		"TR_enum C {\n    Var(t: TR_Tensor)\n    Yok\n}\n" +
		"TR_interface Katman {\n    TR_func ileri(x: TR_Tensor) -> TR_Tensor\n}\n" +
		"TR_func f(x: TR_Int) -> TR_Int {\n    TR_return x\n}\n" +
		"TR_var t = TR_Tensor([1.0])\nTR_var g: TR_Tensor? = t\n"
	const at = 17 // defs'ten sonraki ilk satır
	cases := []struct {
		src   string
		code  messages.Code
		off   int // hatanın satırı: at + off
		parts []string
	}{
		{"TR_var a = [t] == [t]", messages.H1148, 0, []string{"TR_List[TR_Tensor]", "`TR_equal(a, b)`"}},
		{"TR_var a = [[t]] != [[t]]", messages.H1148, 0, nil},
		{"TR_var a = ([t], 1) == ([t], 1)", messages.H1148, 0, nil},
		{"TR_var a = {\"x\": t} == {\"x\": t}", messages.H1148, 0, nil},
		{"TR_var a = Kutu(t: t, n: 1) == Kutu(t: t, n: 1)", messages.H1148, 0, []string{"öğelerini"}},
		{"TR_func h(c: C) -> TR_Bool {\n    TR_return c == C.Yok\n}", messages.H1148, 1, []string{"eşlemeyle"}},
		{"TR_func h(a: Katman, b: Katman) -> TR_Bool {\n    TR_return [a] == [b]\n}", messages.H1153, 1, []string{"alanlarını"}},
		{"TR_var a = g == t", messages.H1151, 0, []string{"`TR_none`"}},
		{"TR_var h: TR_Tensor? = TR_none\nTR_var a = g == h", messages.H1151, 1, nil},
		{"TR_var n = TR_none\nTR_var a = g == n", messages.H1151, 1, nil},
		{"TR_var a = f == f", messages.H1152, 0, []string{"f(x) == g(x)"}},
		{"TR_var a = [f] == [f]", messages.H1152, 0, nil},
		{"TR_var l: TR_List[((TR_Int) -> TR_Int)?] = [TR_none]\nTR_var a = l == l", messages.H1152, 1, nil},
		{"TR_var a = (f, 1) != (f, 1)", messages.H1152, 0, nil},
		{"TR_var a = [t].TR_contains(t)", messages.H1149, 0, []string{"`.TR_contains`", "`TR_for`"}},
		{"TR_var a = [t].TR_index_of(t)", messages.H1149, 0, []string{"`.TR_index_of`"}},
		{"TR_var a = [[t]].TR_unique()", messages.H1149, 0, []string{"`.TR_unique`"}},
		{"TR_var a = [[1], \"a\"].TR_contains([t])", messages.H1149, 0, nil},
		{"TR_var a = [f].TR_contains(f)", messages.H1149, 0, nil},
		{"TR_var a = TR_match [t] {\n    ([t]) => 1\n    _ => 2\n}", messages.H1150, 1, nil},
		{"TR_var a = TR_match t {\n    (t) => 1\n    _ => 2\n}", messages.H1150, 1, []string{"`TR_if`"}},
	}
	for _, tc := range cases {
		singleError(t, defs+tc.src, tc.code, at+tc.off, tc.parts...)
	}
	// Ad olan işlenende yazım dosyadan kurulur (SetFile)
	fileErrorWith(t, defs+"TR_var p = (t, 1)\nTR_var a = p == p", messages.H1148, at+1, "`TR_equal(p.0, p.0) TR_and p.1 == p.1`")
	fileErrorWith(t, defs+"TR_var p = Kutu(t: t, n: 1)\nTR_var a = p != p", messages.H1148, at+1, "`TR_equal(p.t, p.t) TR_and p.n == p.n`")
	fileErrorWith(t, defs+"TR_var p = ((t, 2), [1])\nTR_var q = p\nTR_var a = p == q", messages.H1148, at+2,
		"`TR_equal(p.0.0, q.0.0) TR_and p.0.1 == q.0.1 TR_and p.1 == q.1`")
	// Yazımı olmayan öğe (tensör listesi) sözle
	fileErrorWith(t, defs+"TR_var p = ([t], 1)\nTR_var a = p == p", messages.H1148, at+1, "compare the elements one by one")
	noErrors(t, defs+"TR_var a = g == TR_none\nTR_var b = TR_none != g\nTR_var c = t == t\nTR_var d = t == 1.0\n"+
		"TR_var e = TR_equal(t, t)\nTR_assert_equal([t], [t])\nTR_var k: Kutu? = TR_none\nTR_var l = k == TR_none\n"+
		"TR_func h(c: C) -> TR_Int {\n    TR_return TR_match c {\n        C.Var(x) => 1\n        C.Yok => 2\n    }\n}\n"+
		"TR_var m = [1, 2] == [1, 2]\nTR_var o = [1].TR_contains(1)\nTR_var r = (1, \"a\") == (1, \"a\")")
}
