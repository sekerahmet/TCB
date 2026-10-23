package ml

import (
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestTruthValue(t *testing.T) {
	out := mustRun(t, `eğer tensör([0.0]) { yazdır("yanlış") } değilse { yazdır("sıfır yanlıştır") }
eğer toplam(tensör([1, -1])) { yazdır("yanlış") } değilse { yazdır("skaler sıfır yanlıştır") }
eğer tensör([2.5]) { yazdır("sıfır olmayan doğrudur") }
eğer sıfırlar([0]) { yazdır("yanlış") } değilse { yazdır("boş yanlıştır") }`)
	want := "sıfır yanlıştır\nskaler sıfır yanlıştır\nsıfır olmayan doğrudur\nboş yanlıştır\n"
	if out != want {
		t.Fatalf("doğruluk:\n%q", out)
	}
}

func TestValueShortForm(t *testing.T) {
	out := mustRun(t, `değişken a = tensör([0.1, 0.2, 1.5, 3])
yazdır(a.değer, a.değer[0] == 0.1, toplam(tensör([0.1])).değer)
değişken m = tensör([[1, 2], [3, 4]]).değer
m[0].ekle(9)
yazdır(m)`)
	want := "[0.1, 0.2, 1.5, 3.0] doğru 0.1\n[[1.0, 2.0, 9], [3.0, 4.0]]\n"
	if out != want {
		t.Fatalf("değer:\n%q\nbeklenen:\n%q", out, want)
	}
	// Yapı: 3 boyutlu tensör doğru iç içe listeye dönüşür.
	d := New(tensor.Arange(24).Reshape(2, 3, 4))
	v, _ := d.valueList()
	l := v.(*interp.ListVal)
	if got := l.Elements[1].(*interp.ListVal).Elements[2].(*interp.ListVal).Elements[3].(*interp.FloatVal).Val; got != 23 {
		t.Fatalf("[1][2][3] = %v", got)
	}
	if got := l.Inspect(); !strings.HasPrefix(got, "[[[0.0, 1.0, 2.0, 3.0], [4.0") {
		t.Fatalf("liste: %s", got)
	}
	// Boş eksenli tensör.
	if v, _ := New(tensor.Zeros(2, 0)).valueList(); v.Inspect() != "[[], []]" {
		got := v.Inspect()
		t.Fatalf("boş: %s", got)
	}
}

func TestValueAllocations(t *testing.T) {
	d := New(tensor.Zeros(100, 40))
	alloc := testing.AllocsPerRun(5, func() { _, _ = d.valueList() })
	if alloc > 10 {
		t.Fatalf(".değer %v ayırma yaptı; önceden ayrılmış dilimler bekleniyordu", alloc)
	}
}

func TestIntermediateTensorGrad(t *testing.T) {
	_, err := run(t, `değişken w = tensör([1.0, 2.0], türev_izle: doğru)
geri_yayılım(toplam(w * w))
w = w - w.gradyan * 0.1
yazdır(w.gradyan)`)
	if err == nil || !messages.Has(err, messages.H6700) {
		t.Fatalf("ara tensör gradyanı: %v", err)
	}
	// Geri yayılımdan sonra da ara tensör ara sayılır ve iyileştiriciye verilemez.
	_, err = run(t, `değişken w = tensör([1.0], türev_izle: doğru)
değişken y = w * 2
geri_yayılım(toplam(y))
AdamW([y])`)
	if err == nil || !messages.Has(err, messages.H6514) {
		t.Fatalf("ara tensör parametre: %v", err)
	}
	// Henüz geri yayılım yapılmamış yaprakta gradyan yoktur.
	out := mustRun(t, `yazdır(tensör([1.0], türev_izle: doğru).gradyan, sıfırlar([2]).gradyan)`)
	if out != "yok yok\n" {
		t.Fatalf("yaprak gradyanı: %q", out)
	}
}

// Kural: IntElements tensörün bütün elemanlarını tam sayı olarak verir; tam sayı olmayan ilk eleman
// bad ile bildirilir (ok yanlış).
func TestIntElements(t *testing.T) {
	ids, _, ok := New(tensor.NewOn(tensor.CPU, []float32{3, -1, 0}, 3)).IntElements()
	if !ok || !slices.Equal(ids, []int{3, -1, 0}) {
		t.Errorf("IntElements = %v, %v", ids, ok)
	}
	if _, bad, ok := New(tensor.NewOn(tensor.CPU, []float32{1, 2.5, 3.5}, 3)).IntElements(); ok || bad != 2.5 {
		t.Errorf("tam sayı olmayan eleman: bad = %v, ok = %v", bad, ok)
	}
	if ids, _, ok := New(tensor.NewOn(tensor.CPU, []float32{}, 0)).IntElements(); !ok || len(ids) != 0 {
		t.Errorf("boş tensör: %v, %v", ids, ok)
	}
}
