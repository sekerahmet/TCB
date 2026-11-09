package interp

import (
	"math"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Eşitliğin iç içelik sınırı yoktur ve kendine bağlanan liste ve sözlükte de biter (4.5, 7.6). Eski sınır
// (10 000) 11 000 derin iki eşit listede, 9 950 halkalı listede ve 101 ile 103 halkalı iki listede sessizce
// TR_false veriyordu. TR testleri (tests/esitlik_test.tr) aynısını yapı düğümleriyle sınar; liste ve sözlük
// halkası yalnız genel öğe tipiyle kurulur, burada Go'da.
func TestEqualityHasNoDepthLimit(t *testing.T) {
	deep := func(n int, last int64) Value {
		var l Value = &ListVal{Elements: []Value{NewInt(last)}}
		for range n {
			l = &ListVal{Elements: []Value{l}}
		}
		return l
	}
	ring := func(n, diff int) *ListVal {
		first := &ListVal{Elements: []Value{NewInt(1)}}
		last := first
		for i := 1; i < n; i++ {
			v := int64(1)
			if i == diff {
				v = 2
			}
			y := &ListVal{Elements: []Value{NewInt(v)}}
			last.Elements = append(last.Elements, y)
			last = y
		}
		last.Elements = append(last.Elements, first)
		return first
	}
	dictRing := func(n int) *DictVal {
		node := func() *DictVal {
			return &DictVal{Order: []string{"x"}, Pairs: map[string]Value{"x": NewInt(1)}}
		}
		first := node()
		last := first
		for i := 1; i < n; i++ {
			y := node()
			last.Order, last.Pairs["s"] = append(last.Order, "s"), y
			last = y
		}
		last.Order, last.Pairs["s"] = append(last.Order, "s"), first
		return first
	}
	a := &ListVal{Elements: []Value{NewInt(1)}}
	a.Elements = append(a.Elements, a)
	b1 := &ListVal{Elements: []Value{NewInt(1)}}
	b := &ListVal{Elements: []Value{NewInt(1), b1}}
	b1.Elements = append(b1.Elements, b)
	c := &ListVal{Elements: []Value{&FloatVal{Val: math.NaN()}}}
	c.Elements = append(c.Elements, c)
	cases := []struct {
		name string
		x, y Value
		want bool
	}{
		{"11 000 derin, eşit", deep(11000, 1), deep(11000, 1), true},
		{"11 000 derin, son öğe ayrı", deep(11000, 1), deep(11000, 2), false},
		{"9 950 halka", ring(9950, -1), ring(9950, -1), true},
		{"101 ve 103 halka", ring(101, -1), ring(103, -1), true},
		{"101 ve 103 halka, bir öğe ayrı", ring(101, -1), ring(103, 50), false},
		{"101 ve 103 sözlük halkası", dictRing(101), dictRing(103), true},
		{"a = [1, a], b = [1, [1, b]]", a, b, true},
		{"b ile a", b, a, true},
		{"TR_nan tutan kendine bağlı liste kendisiyle", c, c, false},
		{"[a, a] ile [b, a]", &ListVal{Elements: []Value{a, a}}, &ListVal{Elements: []Value{b, a}}, true},
	}
	for _, tc := range cases {
		got, err := EqualValues(tc.x, tc.y)
		if err != nil || got != tc.want {
			t.Errorf("%s: beklenen %v, gelen %v %v", tc.name, tc.want, got, err)
		}
	}
}

// Karşılaştırılamayan öğe değerin neresinde olursa olsun hatadır, ilk farkta durulmaz (4.5); kendine
// bağlanan değerde de aranır, karşısındaki öğeye bakılmaz.
func TestUncomparableMemberIsFoundBeforeTheFirstDifference(t *testing.T) {
	cases := map[string]messages.Code{
		`TR_var f = (x) => x; TR_var s = [1, f] == [2, f]`:                                                      messages.H2706,
		`TR_var f = (x) => x; TR_var s = [f] == []`:                                                             messages.H2706,
		`TR_var f = (x) => x; TR_var s = [1] == [1, [f]]`:                                                       messages.H2706,
		`TR_var f = (x) => x; TR_var l = [1, f]; l.TR_append(l); TR_var s = l == [2]`:                           messages.H2706,
		`TR_var f = (x) => x; TR_var l = [f]; TR_for i TR_in 0..11000 { l = [l] }; TR_var s = l == [1]`:         messages.H2706,
		`TR_var f = (x) => x; TR_var l = [f]; TR_for i TR_in 0..11000 { l = [l] }; TR_var s = l == [1, 2]`:      messages.H2706,
		`TR_var f = (x) => x; TR_var l = [f]; TR_for i TR_in 0..11000 { l = [l] }; TR_var s = [1, l] == [1, 2]`: messages.H2706,
		`TR_var f = (x) => x; TR_var s = {"a": 1, "f": f} == {"a": 2, "g": 1}`:                                  messages.H2706,
	}
	for code, want := range cases {
		_, _, err := runTROut(t, code)
		if got, _ := messages.InnerCode(err); got != want {
			t.Errorf("%s: beklenen %v, gelen %v (%v)", code, want, got, err)
		}
	}
	// Değerin kendisi TR_none'la karşılaştırılabilir (4.5, 3.5): içinde fonksiyon olsa da. Kendini içeren
	// listeyi tutan demetin karşılaştırması da biter.
	_, val, err := runTROut(t, `TR_var f = (x) => x; TR_var l = [1]; l.TR_append(l)
		TR_var result = [[f] == TR_none, TR_none == [f], f == TR_none, (l, 1) == (l, 1)]`)
	if err != nil || val.Inspect() != `[false, false, false, true]` {
		t.Fatalf("TR_none'la karşılaştırma: %v %v", val, err)
	}
}

// Birleşim-bulun "aynı sınıf" kısa yolu yalnız iki düğüm de daha önce bir çifte girmişse işler: kayıtsız
// düğümde a == a'nın öğeleri karşılaştırılır (TR_nan).
func TestUnionFindHasNoIdentityShortcut(t *testing.T) {
	c := &ListVal{Elements: []Value{&FloatVal{Val: math.NaN()}}}
	c.Elements = append(c.Elements, c)
	if graphEqual(c, c) {
		t.Fatal("TR_nan tutan kendine bağlı liste kendisine eşit çıktı")
	}
	d := &ListVal{}
	d.Elements = []Value{d, &FloatVal{Val: math.NaN()}}
	if graphEqual(d, d) {
		t.Fatal("öğesi kendisi olan, TR_nan tutan liste kendisine eşit çıktı")
	}
	// Henüz hiçbir çifte girmemiş aynı düğüm (x, x) de öğe öğe: kimlik kısa yolu c == c'yi doğru yapardı.
	x := &ListVal{Elements: []Value{&FloatVal{Val: math.NaN()}}}
	if graphEqual(&ListVal{Elements: []Value{x}}, &ListVal{Elements: []Value{x}}) {
		t.Fatal("TR_nan tutan ortak öğe eşit çıktı")
	}
	e := &ListVal{Elements: []Value{NewInt(1)}}
	e.Elements = append(e.Elements, e)
	if !graphEqual(e, e) || !valuesEqual(e, e) {
		t.Fatal("kendine bağlı liste kendisine eşit çıkmadı")
	}
}
