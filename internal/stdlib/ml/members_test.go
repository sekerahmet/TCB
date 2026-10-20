package ml

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Görünüm metotları (şekillendir, devrik, eksenleri_sırala, dilim) ve birleştir tam sayılı
// tensörün (token kimlikleri, maks_indeks) tam sayılı olduğunu korur: .değer Tam verir.
func TestViewsKeepInts(t *testing.T) {
	out := mustRun(t, `değişken i = maks_indeks(tensör([[1.0, 5.0, 0.0], [7.0, 2.0, 9.0]]), eksen: 0)
değişken m = i.şekillendir([3, 1])
yazdır(m.değer, m.devrik().değer, m.eksenleri_sırala([1, 0]).değer, i.dilim(0, 0, 2).değer)
yazdır(birleştir([i, i]).değer, birleştir([i, tensör([1.0, 2.0, 3.0])]).değer)`)
	want := "[[1], [0], [1]] [[1, 0, 1]] [[1, 0, 1]] [1, 0]\n[1, 0, 1, 1, 0, 1] [1.0, 0.0, 1.0, 1.0, 2.0, 3.0]\n"
	if out != want {
		t.Fatalf("çıktı:\n%q\nbeklenen:\n%q", out, want)
	}
}

// Sıfır uzunluklu eksen eleman sınırını atlatmaz: [0, 10⁹, 10⁹] ve [10⁹, 0] gibi şekiller
// (veri yok ama 10⁹ boş liste ya da [4,0]@[0,N] ile dev çıktı) reddedilir.
func TestShapeLimitZeroAxis(t *testing.T) {
	for _, code := range []string{
		"sıfırlar([0, 1000000000, 1000000000])",
		"birler([1000000000000, 0])",
		"sıfırlar([0]).şekillendir([1000000000000, 0])",
	} {
		_, err := run(t, code)
		if err == nil || !messages.Has(err, messages.H6028) {
			t.Fatalf("%s: sınır hatası beklenirdi: %v", code, err)
		}
	}
	if out := mustRun(t, "yazdır(sıfırlar([0, 3]).şekil, sıfırlar([2, 0]).değer)"); out != "[0, 3] [[], []]\n" {
		t.Fatalf("küçük sıfır eksenli şekiller: %q", out)
	}
}
