package ogrenme

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tr_dili/internal/hesap/tensor"
)

// kayitBaytlari girdileri tensör dosyası baytlarına çevirir (bulanıklaştırma tohumları için).
func kayitBaytlari(tb testing.TB, girdiler []kayitGirdisi) []byte {
	tb.Helper()
	var b bytes.Buffer
	if err := girdileriYaz(&b, girdiler); err != nil {
		tb.Fatal(err)
	}
	return b.Bytes()
}

// Güvenlik denetimi (2026-10-18), bulgu: sıfır uzunluklu bir eksen tensör dosyasındaki
// eleman sınırını (yurut.MaxCollectionSize) devre dışı bırakıyordu. 126 baytlık bir dosya
// [2, 0], [1048576, 1, 0] ve [1048576, 0, 8192] şekilli tensörler bildirebiliyor, ornekler/05
// tarzı bir ileri geçiş (X @ W1 + b1 ... @ W2) 137 GB ayırmaya çalışıp süreci
// kurtarılamaz biçimde ("fatal error: out of memory") çökertiyordu.
func TestKayitSifirEksenSiniri(t *testing.T) {
	kotu := kayitBaytlari(t, []kayitGirdisi{
		{ad: "W1", sekil: []int{2, 0}, tur: tensor.Float32, veri: []float32{}},
		{ad: "b1", sekil: []int{1 << 20, 1, 0}, tur: tensor.Float32, veri: []float32{}},
		{ad: "W2", sekil: []int{1 << 20, 0, 8192}, tur: tensor.Float32, veri: []float32{}},
	})
	if _, err := kayitCoz(bytes.NewReader(kotu), int64(len(kotu))); err == nil || !strings.Contains(err.Error(), "çok büyük") {
		t.Fatalf("sıfır eksenle dev şekil reddedilmeliydi: %v", err)
	}

	// Sınırın altındaki sıfır eksenli tensörler (ör. boş tampon) okunmaya devam eder.
	iyi := kayitBaytlari(t, []kayitGirdisi{
		{ad: "boş", sekil: []int{0, 3}, tur: tensor.Float32, veri: []float32{}},
		{ad: "sınırda", sekil: []int{0, 5000, 10000}, tur: tensor.BFloat16, veri: []float32{}},
	})
	g, err := kayitCoz(bytes.NewReader(iyi), int64(len(iyi)))
	if err != nil || len(g) != 2 || g[1].sekil[2] != 10000 {
		t.Fatalf("sınır içindeki sıfır eksenli tensörler okunmalı: %v %+v", err, g)
	}

	// Kullanıcının gördüğü hata: tensör_yükle açık bir hata verir, süreç çökmez.
	yol := filepath.Join(t.TempDir(), "kotu.trt")
	if err := os.WriteFile(yol, kotu, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = calistir(t, `değişken p = tensör_yükle("`+filepath.ToSlash(yol)+`")`)
	if err == nil || !strings.Contains(err.Error(), "tensör çok büyük") {
		t.Fatalf("tensör_yükle açık bir hata vermeli: %v", err)
	}
}
