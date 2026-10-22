package tensor

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// setLimit testin süresince tek ayırmanın sınırını bayt olarak koyar (gerçek bellek
// ayrılmadan sınırın iki yanı sınanır); test bitince eski değer geri gelir.
func setLimit(t *testing.T, byteN int64) {
	t.Helper()
	old := allocLimit()
	limitBytes.Store(byteN)
	t.Cleanup(func() { limitBytes.Store(old) })
}

// expectedPanic fn'in, code ileti koduyla panik etmesini ve want değerlerinin hepsinin
// iletinin parametrelerinde (metin olarak) bulunmasını ister.
func expectedPanic(t *testing.T, ad string, fn func(), code messages.Code, want ...any) {
	t.Helper()
	err := panicError(fn)
	if err == nil {
		t.Fatalf("%s: hata bekleniyordu, işlem başarılı oldu", ad)
	}
	m, ok := messages.As(err)
	if !ok || m.Code != code {
		t.Fatalf("%s: %s kodu bekleniyordu, alınan: %v", ad, code, err)
	}
	for _, w := range want {
		found := false
		for _, a := range m.Args {
			found = found || fmt.Sprint(a) == fmt.Sprint(w)
		}
		if !found {
			t.Fatalf("%s: %v parametresi yok: %v", ad, w, m.Args)
		}
	}
}

func TestPhysicalMemoryRead(t *testing.T) {
	b, ok := physicalMemory()
	switch runtime.GOOS {
	case "windows", "linux", "darwin":
		if !ok || b < 256<<20 {
			t.Fatalf("fiziksel bellek okunamadı ya da akla yatkın değil: %d bayt (ok=%v)", b, ok)
		}
	}
	if s := allocLimit(); s <= 0 {
		t.Fatalf("ayırma sınırı pozitif olmalı: %d", s)
	}
}

// Güvenlik denetimi Y2: [185000, 1, 1] + [4, 46000] yayınlaması 34 milyar eleman (yaklaşık
// 127 GB) isterdi ve süreç yakalanamaz bir bellek hatasıyla çökerdi. Sınır 16 GB'a
// sabitlenir ki test bellek miktarından bağımsız olsun (daha büyük makinede gerçekten
// ayırmaya çalışmasın).
func TestBroadcastLimit(t *testing.T) {
	setLimit(t, 16<<30)
	a, b := Zeros(185000, 1, 1), Zeros(4, 46000)
	expectedPanic(t, "toplama", func() { Add(a, b) }, messages.H3085,
		isAdd, "[185000, 4, 46000]", "126,8 GB", PlaceHost, "16,0 GB", "broadcast")
	expectedPanic(t, "çarpma", func() { Mul(b, a) }, messages.H3085, isMul, "[185000, 4, 46000]")
	// Yayınlamasız işlemlerin hatasında yayınlama ipucu boştur.
	m, _ := messages.As(panicError(func() { MatMul(Zeros(100000, 1), Zeros(1, 100000)) }))
	if m == nil || m.Code != messages.H3085 || m.Args[0] != isMatMul || m.Args[1] != "[100000, 100000]" || m.Args[len(m.Args)-1] != "" {
		t.Fatalf("matris çarpımı hatası: %v", m)
	}
}

// Taşan şekil sessizce küçük bir sayıya dönmez: [2^40, 2^40] 2^80 eleman ister.
func TestOverflowingShape(t *testing.T) {
	expectedPanic(t, "sıfırlar", func() { Zeros(1<<40, 1<<40) }, messages.H3086,
		isCreate, "[1099511627776, 1099511627776]", "1,2 × 10^24")
	expectedPanic(t, "rastgele", func() { Randn(nil, 1<<40, 1<<40) }, messages.H3086)
	expectedPanic(t, "aralık", func() { Arange(-1) }, messages.H3053)
	// Sıfır uzunluklu eksenler veri tutmaz; öbür eksenler dev olsa da tensör geçerlidir.
	if n := Zeros(1<<40, 0, 1<<40).Numel(); n != 0 {
		t.Fatalf("boş tensörün eleman sayısı 0 olmalı: %d", n)
	}
	// Ama böyle iki boş tensörün çarpımı dev bir çıktı ister: [2^40, 0] @ [0, 2^40].
	expectedPanic(t, "boş matris çarpımı", func() { MatMul(Zeros(1<<40, 0), Zeros(0, 1<<40)) }, messages.H3087,
		isMatMul, "[1099511627776, 1099511627776]")
	// Verisiz bir dosyanın bildirdiği taşan şekil (yükleyici New çağırır) eskiden 0 elemana
	// sarıp kabul ediliyordu.
	expectedPanic(t, "yükleme", func() { New(nil, 1<<62, 4) }, messages.H3086)
	expectedPanic(t, "16 bit yükleme", func() { NewOf(BFloat16, nil, 1<<62, 4) }, messages.H3086)
}

// Sınırın hemen altı geçer, hemen üstü hata verir (tür boyu hesaba katılır).
func TestBothSidesOfLimit(t *testing.T) {
	setLimit(t, 4096)
	if Zeros(1024).Numel() != 1024 || ZerosOf(BFloat16, 2048).Numel() != 2048 || Full(2, 32, 32).Numel() != 1024 {
		t.Fatal("sınırdaki tensörler oluşturulmalı")
	}
	expectedPanic(t, "float32 üstü", func() { Zeros(1025) }, messages.H3084, "[1025]", "4,0 KB", PlaceHost)
	expectedPanic(t, "bf16 üstü", func() { ZerosOf(BFloat16, 2049) }, messages.H3084, "[2049]")
	expectedPanic(t, "birler", func() { Ones(33, 32) }, messages.H3084, "[33, 32]")
	expectedPanic(t, "rastgele", func() { Randn(nil, 1025) }, messages.H3084, "[1025]")
	// 16 bitten float32'ye dönüşüm belleği iki katına çıkarır.
	h := ZerosOf(BFloat16, 2048)
	expectedPanic(t, "tür dönüşümü", func() { h.Cast(Float32) }, messages.H3085, isCast, "[2048]")
	// Arka uç ayırmaları (gradyanlar, iç tamponlar) şekilsiz denetlenir.
	expectedPanic(t, "arka uç", func() { cpuBackend{}.Alloc(1025, Float32) }, messages.H3083, "1025", Float32)
	expectedPanic(t, "konaktan", func() { New(make([]float32, 1025), 1025) }, messages.H3084, "[1025]")
}

// İşlemlerin çıktıları: matris çarpımı, yeniden şekillendirme, kopya, birleştirme,
// indirgeme, gömme ve dikkat aynı denetimden geçer.
func TestOpOutputsBounded(t *testing.T) {
	setLimit(t, 64<<10) // 64 KB: 16384 float32
	a, b := Zeros(200, 1), Zeros(1, 200)
	expectedPanic(t, "matris çarpımı", func() { MatMul(a, b) }, messages.H3085, isMatMul, "[200, 200]")
	expectedPanic(t, "toplu matris çarpımı", func() { MatMul(Zeros(2, 100, 1), Zeros(2, 1, 100)) }, messages.H3085, "[2, 100, 100]")
	x := Zeros(100, 100)
	expectedPanic(t, "birleştirme", func() { Concat([]*Tensor{x, x}, 0) }, messages.H3085, isConcat, "[200, 100]")
	// Verisiz girdilerin eksen toplamı taşıp küçük bir şekle sarmaz (5 × 2^62 ≡ 2^62).
	empty := Zeros(0, 1<<62)
	expectedPanic(t, "birleştirme taşma", func() { Concat([]*Tensor{empty, empty, empty, empty, empty}, 1) }, messages.H3022)
	expectedPanic(t, "yeniden şekil", func() { Zeros(0).Reshape(1<<62, 4) }, messages.H3087, isReshape)
	expectedPanic(t, "yeniden şekil -1", func() { Zeros(0).Reshape(-1, 1<<62, 4) }, messages.H3087, isReshape)
	// Bitişik olmayan görünümün yeniden şekli kopya ister; kopya sınır içinde kalır.
	y := Zeros(64, 64)
	if got := y.T().Reshape(-1).Numel(); got != 4096 {
		t.Fatalf("sınır içindeki kopya: %d", got)
	}
	table := Zeros(2, 5000)
	expectedPanic(t, "gömme", func() { Embedding(table, []int{0, 1, 0, 1}) }, messages.H3085, isEmbedding, "[4, 5000]")
	q, k, v := Zeros(3000, 1, 1), Zeros(1, 1, 1), Zeros(1, 1, 8)
	expectedPanic(t, "dikkat", func() { Attention(q, k, v, AttentionOptions{}) }, messages.H3085, isAttention, "[3000, 1, 8]")
	// Taşan indeks şekli sessizce 0'a sarıp boş indeks listesine uymaz.
	expectedPanic(t, "gömme taşma", func() { Embedding(Zeros(2, 4), nil, 1<<62, 4) }, messages.H3042, 0)
}

// BenchmarkAllocCheck denetimin kendi maliyeti: sıcak yolda her çıktı için bir şekil
// denetimi ve bir arka uç denetimi yapılır.
func BenchmarkAllocCheck(b *testing.B) {
	sh := []int{8, 64, 64}
	n := 0
	for i := 0; i < b.N; i++ {
		n += allocSize(isAdd, sh, Float32)
		n += checkElem(4096, Float32)
	}
	if n == 0 {
		b.Fatal("beklenmeyen sonuç")
	}
}

// Sınır içindeki her işlem sonucu değişmeden çalışır (denetim yalnızca sınırı korur).
func TestWithinLimitUnchanged(t *testing.T) {
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

// Sınırın yeri ileti şablonunda seçilir: konak "bu makinede", GPU "bu GPU'da" der (Go sözcük
// seçmez, yalnızca PlaceHost/PlaceGPU anahtarını verir).
func TestSizeErrorPlaceIsChosenByTemplate(t *testing.T) {
	host := SizeError(1<<20, Float32, 1<<10, PlaceHost).Error()
	gpu := SizeError(1<<20, Float32, 1<<10, PlaceGPU).Error()
	if !strings.Contains(host, "makinede") || !strings.Contains(gpu, "GPU'da") || host == gpu {
		t.Errorf("yer şablonda seçilmeli:\n%s\n%s", host, gpu)
	}
}
