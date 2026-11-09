package interp

import (
	"math"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Eşitliğin iç içelik sınırı yoktur ve kendine bağlanan liste ve sözlükte de biter (4.5, 7.6). Eski sınır
// (10 000) 11 000 derin iki eşit listede, 9 950 halkalı listede ve 101 ile 103 halkalı iki listede sessizce
// TR_false veriyordu. TR testleri (tests/esitlik_test.tr) aynısını yapı düğümleriyle sınar; burada genel öğe
// tipli liste ve sözlük kurulur.
func TestEqualityHasNoDepthLimit(t *testing.T) {
	_, val, err := runTROut(t, `
		TR_func derin(n, son) {
			TR_var l = [son]
			TR_for i TR_in 1..n {
				l = [l]
			}
			TR_return l
		}
		TR_func halka(n, fark) {
			TR_var ilk = [1]
			TR_var son = ilk
			TR_for i TR_in 1..n {
				TR_var y = [TR_if i == fark { 2 } TR_else { 1 }]
				son.TR_append(y)
				son = y
			}
			son.TR_append(ilk)
			TR_return ilk
		}
		TR_func sözlük_halka(n) {
			TR_var ilk = {"x": 1}
			TR_var son = ilk
			TR_for i TR_in 1..n {
				TR_var y = {"x": 1}
				son["s"] = y
				son = y
			}
			son["s"] = ilk
			TR_return ilk
		}
		TR_var a = [1]
		a.TR_append(a)
		TR_var b = [1, [1]]
		b[1].TR_append(b)
		TR_var c = [TR_nan]
		c.TR_append(c)
		TR_var result = [
			derin(11000, 1) == derin(11000, 1), derin(11000, 1) == derin(11000, 2),
			halka(9950, -1) == halka(9950, -1), halka(101, -1) == halka(103, -1),
			halka(101, -1) == halka(103, 50), sözlük_halka(101) == sözlük_halka(103),
			a == b, b == a, c == c, c != c, [a, a] == [b, a]]
	`)
	want := `[true, false, true, true, false, true, true, true, false, true, true]`
	if err != nil || val.Inspect() != want {
		t.Fatalf("beklenen %s, gelen %v %v", want, val, err)
	}
}

// Karşılaştırılamayan öğe değerin neresinde olursa olsun hatadır, ilk farkta durulmaz (4.5); kendine
// bağlanan değerde de aranır, karşısındaki öğeye bakılmaz.
func TestUncomparableMemberIsFoundBeforeTheFirstDifference(t *testing.T) {
	cases := map[string]messages.Code{
		`TR_var f = (x) => x; TR_var s = [1, f] == [2, f]`:                                              messages.H2706,
		`TR_var f = (x) => x; TR_var s = [f] == []`:                                                     messages.H2706,
		`TR_var f = (x) => x; TR_var s = [1] == [1, [f]]`:                                               messages.H2706,
		`TR_var f = (x) => x; TR_var l = [1, f]; l.TR_append(l); TR_var s = l == [2]`:                   messages.H2706,
		`TR_var f = (x) => x; TR_var l = [f]; TR_for i TR_in 0..11000 { l = [l] }; TR_var s = l == [1]`: messages.H2706,
		`TR_var f = (x) => x; TR_var s = {"a": 1, "f": f} == {"a": 2, "g": 1}`:                          messages.H2706,
	}
	for code, want := range cases {
		_, _, err := runTROut(t, code)
		if got, _ := messages.InnerCode(err); got != want {
			t.Errorf("%s: beklenen %v, gelen %v (%v)", code, want, got, err)
		}
	}
	// Değerin kendisi TR_none'la karşılaştırılabilir (4.5, 3.5): içinde fonksiyon olsa da. Demetin tensör
	// araması (H2777, holdsTensor) kendini içeren listede de biter.
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
