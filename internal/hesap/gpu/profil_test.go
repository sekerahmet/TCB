package gpu

import (
	"bytes"
	"strings"
	"testing"

	"tr_dili/internal/hesap/tensor"
)

// TestProfilOlcer ölçüm açıkken çekirdeklerin ve sürücü bellek işlerinin sayılıp sürelerinin
// toplandığını, adım özetinin ve bellek durumunun yazıldığını denetler.
func TestProfilOlcer(t *testing.T) {
	ProfilAc(true)
	defer ProfilAc(false)
	g := gpuArkaUc()
	a := tensor.NewOn(gpuCihaz, []float32{1, 2, 3, 4, 5, 6}, 2, 3).SetRequiresGrad(true)
	b := tensor.NewOn(gpuCihaz, []float32{1, 0, 0, 1, 1, 1}, 3, 2)
	once := map[string]int64{}
	for _, o := range CekirdekOlcumleri() {
		once[o.Ad] = o.Cagri
	}
	for adim := 0; adim < 4; adim++ {
		g.gir()
		g.adimIsaretle()
		g.cik()
		y := tensor.MatMul(a, b).SumAll()
		y.Backward()
		_ = y.Item()
	}
	g.gir()
	g.adimIsaretle()
	g.cik()
	var carpim, toplam int64
	for _, o := range CekirdekOlcumleri() {
		yeni := o.Cagri - once[o.Ad]
		toplam += yeni
		if o.Aile == "carpim" {
			carpim += yeni
		}
		if o.GPUSuresi < 0 {
			t.Fatalf("%s: negatif süre", o.Ad)
		}
	}
	if carpim < 4 || toplam < 8 {
		t.Fatalf("ölçüm eksik: %d çarpım, %d toplam başlatma", carpim, toplam)
	}
	var w bytes.Buffer
	ProfilOzetiYaz(&w)
	t.Log("\n" + w.String())
	for _, parca := range []string{"GPU profili", "adım başına", "aile", "carpim", "bellek: ayrılmış"} {
		if !strings.Contains(w.String(), parca) {
			t.Fatalf("özette %q yok:\n%s", parca, w.String())
		}
	}
	if b := BellekOlcumleri(); len(b) == 0 || b[0].Ayrilmis == 0 || b[0].Canli > b[0].Ayrilmis+b[0].Havuzda {
		t.Fatalf("bellek ölçümü: %+v", b)
	}
}

func TestCekirdekAilesi(t *testing.T) {
	for ad, aile := range map[string]string{
		"carpim_128_128_8_8_8_false_true_false": "carpim", "tekli_turev_3_": "tekli_turev",
		"ce_ileri_256": "ce_ileri", "gomme": "gomme", "softmax_256_true": "softmax", "surucu_kopya": "surucu_kopya",
	} {
		if a := cekirdekAilesi(ad); a != aile {
			t.Fatalf("%s: aile %q, %q bekleniyordu", ad, a, aile)
		}
	}
}

// TestSinifBoyu boyut sınıflarının istenen boydan küçük olmadığını ve 1 MB üstünde boşluğun
// %25'i aşmadığını denetler.
func TestSinifBoyu(t *testing.T) {
	for _, b := range []uint64{1, 256, 257, 1000, 1 << 20, 1<<20 + 1, 3 << 20, 5<<20 + 7, 63 << 20, 64 << 20, 64<<20 + 1, 1 << 30} {
		s := sinifBoyu(b)
		if s < b {
			t.Fatalf("%d baytın sınıfı %d küçük", b, s)
		}
		if b > 1<<20 && float64(s) > 1.25*float64(b)+float64(2<<20) {
			t.Fatalf("%d baytın sınıfı %d: boşluk çok", b, s)
		}
		if sinifBoyu(s) != s {
			t.Fatalf("sınıf %d kendi sınıfı değil", s)
		}
	}
}
