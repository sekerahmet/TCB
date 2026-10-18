package veriyolu

import (
	"context"
	"io"
	"slices"
	"testing"
)

// siraliHepsi okuyucunun kalan bütün satırlarını ve her toplu işten sonraki durumu verir.
func siraliHepsi(t *testing.T, so *SiraliOkuyucu) ([][]int32, []Durum) {
	t.Helper()
	var satirlar [][]int32
	var durumlar []Durum
	var ti TopluIs
	for {
		err := so.Doldur(context.Background(), &ti)
		if err == io.EOF {
			return satirlar, durumlar
		}
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < ti.B; i++ {
			satirlar = append(satirlar, slices.Clone(ti.Satir(i)))
		}
		durumlar = append(durumlar, ti.Durum)
	}
}

// Sıralı okuyucu, bir toplu işin Durum'u Baslangic olarak verilince tam o noktadan sürer:
// kesilip sürdürülen okuma, kesintisiz okumayla aynı satırları verir (kısa parça atlanır,
// parça sınırında ve sonunda da).
func TestSiraliBaslangictanSurer(t *testing.T) {
	parcaSizintiDenetle(t)
	d := kodluKume(t, 100, 5, 37, 9)
	k, err := KumeAc(d, KumeSecenekleri{})
	if err != nil {
		t.Fatal(err)
	}
	sec := SiraliSecenekleri{B: 3, T: 8}
	so, err := k.Sirali(sec)
	if err != nil {
		t.Fatal(err)
	}
	tum, durumlar := siraliHepsi(t, so)
	so.Kapat()
	okunan := 0
	for i, dm := range durumlar {
		okunan += sec.B
		okunan = min(okunan, len(tum))
		s := sec
		s.Baslangic = &dm
		so2, err := k.Sirali(s)
		if err != nil {
			t.Fatal(err)
		}
		kalan, _ := siraliHepsi(t, so2)
		so2.Kapat()
		if !slices.EqualFunc(kalan, tum[okunan:], slices.Equal[[]int32]) {
			t.Fatalf("%d. toplu işten (durum %+v) sonra sürdürülen okuma farklı: %d satır, beklenen %d", i+1, dm, len(kalan), len(tum)-okunan)
		}
	}
	if _, err := k.Sirali(SiraliSecenekleri{B: 1, T: 8, Baslangic: &Durum{Sira: 9}}); err == nil {
		t.Error("parça sayısını aşan başlangıç hata vermeliydi")
	}
	if _, err := k.Sirali(SiraliSecenekleri{B: 1, T: 8, Baslangic: &Durum{Sira: 4, Pencere: 3}}); err == nil {
		t.Error("bitmiş okuyucunun pencereli başlangıcı hata vermeliydi")
	}
}
