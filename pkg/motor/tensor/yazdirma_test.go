package tensor

import (
	"math/rand"
	"strings"
	"testing"

	ct "tr_dili/pkg/tensor"
)

func TestYazdirmaOzelDegerler(t *testing.T) {
	out := mustCalistir(t, `yazdır(tensör([0.00001, 1]))
yazdır(tensör([1000000000.0, 2]))
yazdır(log(tensör([-1, 0, 1])), üstel(tensör([100])))
yazdır(toplam(tensör([0.00002])))
yazdır(min(tensör([3, -2, 5])), min(tensör([3, -2], türev_izle: doğru)))`)
	want := "Tensör[2] [1.0000e-05, 1.0000e+00]\n" +
		"Tensör[2] [1.0000e+09, 2.0000e+00]\n" +
		"Tensör[3] [Tanımsız,  -Sonsuz,   0.0000] Tensör[1] [Sonsuz]\n" +
		"Tensör(2.0000e-05)\n" +
		"Tensör(-2.0000) Tensör(-2.0000, türev izleniyor)\n"
	if out != want {
		t.Fatalf("özel değerler:\n%q\nbeklenen:\n%q", out, want)
	}
}

func TestBuyukTensorYazdirmaDogru(t *testing.T) {
	// Kısaltılmış yazdırma yalnızca gösterilen hücreleri okur; değerler doğru yerde olmalı.
	d := Yeni(ct.Arange(2000).Reshape(2, 10, 100))
	s := d.Inspect()
	for _, parca := range []string{"[   0.0000,    1.0000,    2.0000, …,   97.0000,   98.0000,   99.0000]", "1999.0000", "1900.0000", "…"} {
		if !strings.Contains(s, parca) {
			t.Fatalf("eksik %q:\n%s", parca, s)
		}
	}
	if strings.Count(s, "\n") > 25 {
		t.Fatalf("kısaltılmadı:\n%s", s)
	}
	// 1000×1000 yazdırma hızlı ve az bellekli olmalı.
	b := Yeni(ct.Randn(rand.New(rand.NewSource(1)), 1000, 1000))
	if ayirma := testing.AllocsPerRun(3, func() { _ = b.Inspect() }); ayirma > 500 {
		t.Fatalf("büyük tensör yazdırması %v ayırma yaptı", ayirma)
	}
}
