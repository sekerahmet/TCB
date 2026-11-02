package interp

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (13.3, 13.10): anahtarsız ve anahtarlı sıralama aynı sıralama sırasını kullanır: -0.0 0.0'dan
// önce, NaN'lar sonda ve birbirine eşit, demette öğe öğe; eşitlerin sırası korunur.
func TestSortOrderIsTheSameWithAndWithoutAKey(t *testing.T) {
	for _, c := range []struct{ source, want string }{
		{"TR_print([0.0, TR_nan, -0.0, 1, -1.5].TR_sort())", "[-1.5, -0.0, 0.0, 1, nan]"},
		{"TR_print([0.0, TR_nan, -0.0, 1, -1.5].TR_sort((x) => x))", "[-1.5, -0.0, 0.0, 1, nan]"},
		{"TR_print([(0.0, 2.0), (-0.0, 3.0)].TR_sort())", "[(-0.0, 3.0), (0.0, 2.0)]"},
		{"TR_print([(0.0, 2.0), (-0.0, 3.0)].TR_sort((d) => d))", "[(-0.0, 3.0), (0.0, 2.0)]"},
		// Öğelerden biri değişken: değişmez yalnız sayı yazımı olsaydı tam sayılar ondalık olurdu (3.6).
		{"TR_var iki = 2\nTR_print([2.0, 1, iki, 0.5].TR_sort())", "[0.5, 1, 2.0, 2]"},
		{"TR_var iki = 2\nTR_print([2.0, 1, iki, 0.5].TR_sort((x) => x))", "[0.5, 1, 2.0, 2]"},
	} {
		out, _, err := runTROut(t, c.source+"\n")
		if err != nil || strings.TrimSpace(out) != c.want {
			t.Errorf("%s: %q beklenirdi, gelen %q (%v)", c.source, c.want, out, err)
		}
	}
}

// Kural (13.3): .TR_sort(f) öğeleri f'nin anahtarına göre kararlı dizer, yeni liste verir; anahtar
// demet olabilir; f her öğe için bir kez çağrılır.
func TestKeySort(t *testing.T) {
	for _, c := range []struct{ source, want string }{
		{`TR_print(["bb", "a", "ccc", "dd"].TR_sort((m) => m.TR_length))`, `["a", "bb", "dd", "ccc"]`},
		{"TR_print([1, 3, 2].TR_sort((x) => -x))", "[3, 2, 1]"},
		{`TR_print([(3, "b"), (1, "b"), (2, "a")].TR_sort((d) => (d.1, d.0)))`, `[(2, "a"), (1, "b"), (3, "b")]`},
		{"TR_print([].TR_sort((x) => x))", "[]"},
		{"TR_var n = 0\nTR_func k(x: TR_Int) -> TR_Int {\n    n += 1\n    TR_return x\n}\nTR_var l = [5, 4, 3, 4]\nTR_print(l.TR_sort(k), n, l)", "[3, 4, 4, 5] 4 [5, 4, 3, 4]"},
	} {
		out, _, err := runTROut(t, c.source+"\n")
		if err != nil || strings.TrimSpace(out) != c.want {
			t.Errorf("%s: %q beklenirdi, gelen %q (%v)", c.source, c.want, out, err)
		}
	}
}

// Kural (13.3, 5.7): iki parametreli karşılaştırıcı anahtar değildir (H2900); sıralanamayan anahtar
// (H2902) ve karşılaştırılamayan iki anahtar (H2903) hatadır; anahtar fonksiyonunun asıl listeye öğe
// eklemesi 5.7'deki gibi hatadır (H2703). İletiler kimlikle yazılır, görünüm adı ve <nil> girmez.
func TestKeySortErrors(t *testing.T) {
	grow := "TR_var l = [3, 1]\nTR_func k(x: TR_Int) -> TR_Int {\n    l.TR_append(x)\n    TR_return x\n}\nTR_print(l.TR_sort(k))"
	for _, c := range []struct {
		source string
		code   messages.Code
		want   string
	}{
		{"TR_print([3, 1].TR_sort((a, b) => b - a))", messages.H2900, "`l.TR_sort((x) => -x)`"},
		{"TR_print([3, 1].TR_sort((x) => [x]))", messages.H2902, "`.TR_sort(f)`"},
		{`TR_print([3, 1].TR_sort((x) => TR_if x > 2 { "a" } TR_else { 1 }))`, messages.H2903, "`.TR_sort(f)`"},
		{grow, messages.H2703, ""},
	} {
		_, _, err := runTROut(t, c.source+"\n")
		got := messages.ErrorIn(err, "en")
		if !messages.Has(err, c.code) || !strings.Contains(got, c.want) || strings.Contains(got, "<nil>") || strings.Contains(got, "list.") {
			t.Errorf("%s: %s beklenirdi (%s), gelen %q", c.source, c.code, c.want, got)
		}
	}
}
