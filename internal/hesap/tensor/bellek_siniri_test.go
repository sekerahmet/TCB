package tensor

import (
	"runtime"
	"strings"
	"testing"
)

// sinirKoy testin süresince tek ayırmanın sınırını bayt olarak koyar (gerçek bellek
// ayrılmadan sınırın iki yanı sınanır); test bitince eski değer geri gelir.
func sinirKoy(t *testing.T, bayt int64) {
	t.Helper()
	eski := ayirmaSiniri()
	sinirBayt.Store(bayt)
	t.Cleanup(func() { sinirBayt.Store(eski) })
}

// beklenenPanik fn'in, parcalar'ın hepsini içeren bir mesajla panik etmesini ister
// (panikMesaji ozel_turev_test.go'dadır).
func beklenenPanik(t *testing.T, ad string, fn func(), parcalar ...string) {
	t.Helper()
	m := panikMesaji(fn)
	if m == "" {
		t.Fatalf("%s: hata bekleniyordu, işlem başarılı oldu", ad)
	}
	for _, p := range parcalar {
		if !strings.Contains(m, p) {
			t.Fatalf("%s: mesajda %q yok: %s", ad, p, m)
		}
	}
}

func TestFizikselBellekOkunur(t *testing.T) {
	b, ok := fizikselBellek()
	switch runtime.GOOS {
	case "windows", "linux", "darwin":
		if !ok || b < 256<<20 {
			t.Fatalf("fiziksel bellek okunamadı ya da akla yatkın değil: %d bayt (ok=%v)", b, ok)
		}
	}
	if s := ayirmaSiniri(); s <= 0 {
		t.Fatalf("ayırma sınırı pozitif olmalı: %d", s)
	}
}

// Güvenlik denetimi Y2: [185000, 1, 1] + [4, 46000] yayınlaması 34 milyar eleman (yaklaşık
// 127 GB) isterdi ve süreç yakalanamaz bir bellek hatasıyla çökerdi. Sınır 16 GB'a
// sabitlenir ki test bellek miktarından bağımsız olsun (daha büyük makinede gerçekten
// ayırmaya çalışmasın).
func TestYayinlamaSiniri(t *testing.T) {
	sinirKoy(t, 16<<30)
	a, b := Zeros(185000, 1, 1), Zeros(4, 46000)
	beklenenPanik(t, "toplama", func() { Add(a, b) },
		"toplama: sonuç tensörü [185000, 4, 46000] şeklinde", "126,8 GB bellek ister", "bu makinede 16,0 GB var",
		"yayınlama beklenmedik bir boyut üretmiş olabilir")
	beklenenPanik(t, "çarpma", func() { Mul(b, a) }, "çarpma: sonuç tensörü [185000, 4, 46000]")
	// Yayınlamasız işlemlerin mesajı yayınlamadan söz etmez.
	m := panikMesaji(func() { MatMul(Zeros(100000, 1), Zeros(1, 100000)) })
	if !strings.Contains(m, "matris çarpımı: sonuç tensörü [100000, 100000]") || strings.Contains(m, "yayınlama") {
		t.Fatalf("matris çarpımı mesajı: %s", m)
	}
}

// Taşan şekil sessizce küçük bir sayıya dönmez: [2^40, 2^40] 2^80 eleman ister.
func TestTasanSekil(t *testing.T) {
	beklenenPanik(t, "sıfırlar", func() { Zeros(1<<40, 1<<40) },
		"tensör oluşturma: istenen tensör [1099511627776, 1099511627776] şeklinde", "tam sayı sınırını aşıyor", "× 10^24")
	beklenenPanik(t, "rastgele", func() { Randn(nil, 1<<40, 1<<40) }, "tam sayı sınırını aşıyor")
	beklenenPanik(t, "aralık", func() { Arange(-1) }, "negatif boyut")
	// Sıfır uzunluklu eksenler veri tutmaz; öbür eksenler dev olsa da tensör geçerlidir.
	if n := Zeros(1<<40, 0, 1<<40).Numel(); n != 0 {
		t.Fatalf("boş tensörün eleman sayısı 0 olmalı: %d", n)
	}
	// Ama böyle iki boş tensörün çarpımı dev bir çıktı ister: [2^40, 0] @ [0, 2^40].
	beklenenPanik(t, "boş matris çarpımı", func() { MatMul(Zeros(1<<40, 0), Zeros(0, 1<<40)) },
		"matris çarpımı: sonuç tensörü [1099511627776, 1099511627776]", "tam sayı sınırını aşıyor")
	// Verisiz bir dosyanın bildirdiği taşan şekil (yükleyici New çağırır) eskiden 0 elemana
	// sarıp kabul ediliyordu.
	beklenenPanik(t, "yükleme", func() { New(nil, 1<<62, 4) }, "tam sayı sınırını aşıyor")
	beklenenPanik(t, "16 bit yükleme", func() { NewOf(BFloat16, nil, 1<<62, 4) }, "tam sayı sınırını aşıyor")
}

// Sınırın hemen altı geçer, hemen üstü hata verir (tür boyu hesaba katılır).
func TestSinirinIkiYani(t *testing.T) {
	sinirKoy(t, 4096)
	if Zeros(1024).Numel() != 1024 || ZerosOf(BFloat16, 2048).Numel() != 2048 || Full(2, 32, 32).Numel() != 1024 {
		t.Fatal("sınırdaki tensörler oluşturulmalı")
	}
	beklenenPanik(t, "float32 üstü", func() { Zeros(1025) }, "[1025]", "4,0 KB bellek ister; bu makinede 4,0 KB var")
	beklenenPanik(t, "bf16 üstü", func() { ZerosOf(BFloat16, 2049) }, "[2049]")
	beklenenPanik(t, "birler", func() { Ones(33, 32) }, "[33, 32]")
	beklenenPanik(t, "rastgele", func() { Randn(nil, 1025) }, "[1025]")
	// 16 bitten float32'ye dönüşüm belleği iki katına çıkarır.
	h := ZerosOf(BFloat16, 2048)
	beklenenPanik(t, "tür dönüşümü", func() { h.Cast(Float32) }, "tür dönüşümü: sonuç tensörü [2048]")
	// Arka uç ayırmaları (gradyanlar, iç tamponlar) şekilsiz denetlenir.
	beklenenPanik(t, "arka uç", func() { cpuBackend{}.Alloc(1025, Float32) }, "arka uç: 1025 elemanlık float32 tampon")
	beklenenPanik(t, "konaktan", func() { New(make([]float32, 1025), 1025) }, "[1025]")
}

// İşlemlerin çıktıları: matris çarpımı, yeniden şekillendirme, kopya, birleştirme,
// indirgeme, gömme ve dikkat aynı denetimden geçer.
func TestIslemCiktilariSinirli(t *testing.T) {
	sinirKoy(t, 64<<10) // 64 KB: 16384 float32
	a, b := Zeros(200, 1), Zeros(1, 200)
	beklenenPanik(t, "matris çarpımı", func() { MatMul(a, b) }, "matris çarpımı: sonuç tensörü [200, 200]")
	beklenenPanik(t, "toplu matris çarpımı", func() { MatMul(Zeros(2, 100, 1), Zeros(2, 1, 100)) }, "[2, 100, 100]")
	x := Zeros(100, 100)
	beklenenPanik(t, "birleştirme", func() { Concat([]*Tensor{x, x}, 0) }, "birleştirme: sonuç tensörü [200, 100]")
	// Verisiz girdilerin eksen toplamı taşıp küçük bir şekle sarmaz (5 × 2^62 ≡ 2^62).
	bos := Zeros(0, 1<<62)
	beklenenPanik(t, "birleştirme taşma", func() { Concat([]*Tensor{bos, bos, bos, bos, bos}, 1) }, "tam sayı sınırını aşıyor")
	beklenenPanik(t, "yeniden şekil", func() { Zeros(0).Reshape(1<<62, 4) }, "yeniden şekillendirme", "tam sayı sınırını aşıyor")
	beklenenPanik(t, "yeniden şekil -1", func() { Zeros(0).Reshape(-1, 1<<62, 4) }, "tam sayı sınırını aşıyor")
	// Bitişik olmayan görünümün yeniden şekli kopya ister; kopya sınır içinde kalır.
	y := Zeros(64, 64)
	if got := y.T().Reshape(-1).Numel(); got != 4096 {
		t.Fatalf("sınır içindeki kopya: %d", got)
	}
	tablo := Zeros(2, 5000)
	beklenenPanik(t, "gömme", func() { Embedding(tablo, []int{0, 1, 0, 1}) }, "gömme: sonuç tensörü [4, 5000]")
	q, k, v := Zeros(1, 3000, 1), Zeros(1, 1, 1), Zeros(1, 1, 8)
	beklenenPanik(t, "dikkat", func() { Attention(q, k, v, AttentionOptions{}) }, "dikkat: sonuç tensörü [1, 3000, 8]")
	// Taşan indeks şekli sessizce 0'a sarıp boş indeks listesine uymaz.
	beklenenPanik(t, "gömme taşma", func() { Embedding(Zeros(2, 4), nil, 1<<62, 4) }, "Embedding: 0 indeks")
}

// BenchmarkAyirmaDenetimi denetimin kendi maliyeti: sıcak yolda her çıktı için bir şekil
// denetimi ve bir arka uç denetimi yapılır.
func BenchmarkAyirmaDenetimi(b *testing.B) {
	sh := []int{8, 64, 64}
	n := 0
	for i := 0; i < b.N; i++ {
		n += ayirmaBoyu(isToplama, sh, Float32)
		n += elemanDenetle(4096, Float32)
	}
	if n == 0 {
		b.Fatal("beklenmeyen sonuç")
	}
}

// Sınır içindeki her işlem sonucu değişmeden çalışır (denetim yalnızca sınırı korur).
func TestSinirIciDegismez(t *testing.T) {
	a := New([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	b := New([]float32{10, 20, 30}, 3)
	got := Add(a, b).Data()
	want := []float32{11, 22, 33, 14, 25, 36}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("toplama: %v, beklenen %v", got, want)
		}
	}
	if s := a.Reshape(3, -1).Shape(); s[0] != 3 || s[1] != 2 {
		t.Fatalf("yeniden şekil: %v", s)
	}
	if s := Zeros(0, 5).Reshape(5, 0, 7).Shape(); s[2] != 7 {
		t.Fatalf("boş yeniden şekil: %v", s)
	}
}
