package tensor

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

// İşçi havuzu: her parça tam bir kez yürütülmeli, katılımcı numarası [0, w)
// içinde kalmalı ve aynı numara aynı anda iki goroutine'de çalışmamalı.
func TestDagitParcalarTamBirKez(t *testing.T) {
	eski := runtime.GOMAXPROCS(8)
	defer runtime.GOMAXPROCS(eski)
	for _, w := range []int{1, 2, 3, 4, 8, 20} {
		for _, parca := range []int{1, 2, 7, 64, 1000} {
			sayac := make([]atomic.Int32, parca)
			mesgul := make([]atomic.Int32, w)
			var hata atomic.Value
			dagit(w, parca, func(k, c int) {
				if k < 0 || k >= w {
					hata.Store("katılımcı numarası aralık dışında")
					return
				}
				if mesgul[k].Add(1) != 1 {
					hata.Store("aynı katılımcı numarası eşzamanlı kullanıldı")
				}
				sayac[c].Add(1)
				mesgul[k].Add(-1)
			})
			if h := hata.Load(); h != nil {
				t.Fatalf("w=%d parça=%d: %v", w, parca, h)
			}
			for c := range sayac {
				if n := sayac[c].Load(); n != 1 {
					t.Fatalf("w=%d parça=%d: parça %d %d kez yürütüldü", w, parca, c, n)
				}
			}
		}
	}
}

// Eşzamanlı ve iç içe çağrılar (havuz meşgulken geçici goroutine'lere düşer)
// ve çalışırken değişen GOMAXPROCS doğru sonuç vermeli.
func TestDagitEszamanliVeIcIce(t *testing.T) {
	eski := runtime.GOMAXPROCS(0)
	defer runtime.GOMAXPROCS(eski)
	const n = 1 << 18
	topla := func() int64 {
		var s atomic.Int64
		parallelFor(n, 1, func(lo, hi int) {
			// İç içe: her parça kendi aralığını yine paralel toplar.
			var ic atomic.Int64
			parallelFor(hi-lo, 4, func(a, b int) {
				var y int64
				for i := lo + a; i < lo+b; i++ {
					y += int64(i)
				}
				ic.Add(y)
			})
			s.Add(ic.Load())
		})
		return s.Load()
	}
	beklenen := int64(n) * (n - 1) / 2
	for _, p := range []int{8, 2, 20, 1, 4} {
		runtime.GOMAXPROCS(p)
		var wg sync.WaitGroup
		hatalar := make(chan int64, 8)
		for g := 0; g < 8; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for k := 0; k < 20; k++ {
					if s := topla(); s != beklenen {
						hatalar <- s
						return
					}
				}
			}()
		}
		wg.Wait()
		close(hatalar)
		for s := range hatalar {
			t.Fatalf("GOMAXPROCS=%d: toplam %d, beklenen %d", p, s, beklenen)
		}
	}
}

// İşçi sayısı seçimi: küçük işler dönen işçilerle sınırlı, büyük işler bütün
// çekirdekleri kullanabilir, her katılımcıya en az enAz birim düşer.
func TestIsciSayisi(t *testing.T) {
	eski := runtime.GOMAXPROCS(20)
	defer runtime.GOMAXPROCS(eski)
	durumlar := []struct{ is, enAz, buyuk, want int }{
		{100, 1000, 1 << 20, 1},
		{2500, 1000, 1 << 20, 2},
		{1 << 19, 1000, 1 << 20, donenIsci + 1},
		{1 << 20, 1000, 1 << 20, 20},
		{1 << 20, 1 << 17, 1 << 20, 8},
	}
	for _, d := range durumlar {
		if got := isciSayisi(d.is, d.enAz, d.buyuk); got != d.want {
			t.Errorf("isciSayisi(%d, %d, %d) = %d, beklenen %d", d.is, d.enAz, d.buyuk, got, d.want)
		}
	}
	runtime.GOMAXPROCS(2)
	if got := isciSayisi(1<<30, 1, 1); got != 2 {
		t.Errorf("GOMAXPROCS=2'de %d işçi seçildi", got)
	}
}

// Bulgu 10: işçi goroutine'inde (ya da çağıranın parçasında) oluşan panik kurtarılıp
// çağırana taşınır; işçiler yarım işi sonraki çağrılarla eşzamanlı yürütmeye devam etmez ve
// havuz kullanılabilir kalır. İç içe (geçici goroutine'li) yolda da aynı.
func TestIsciPanigiCagiranaTasinir(t *testing.T) {
	if workers() < 2 {
		t.Skip("tek işlemci")
	}
	for _, icIce := range []bool{false, true} {
		calistir := func(fn func()) {
			if icIce {
				dagit(2, 2, func(k, c int) {
					if c == 0 {
						fn()
					}
				})
				return
			}
			fn()
		}
		var yurutulen atomic.Int64
		m := panikMesaji(func() {
			calistir(func() {
				dagit(4, 64, func(_, c int) {
					yurutulen.Add(1)
					if c == 5 {
						panic("tensor: işçide hata")
					}
				})
			})
		})
		if m != "tensor: işçide hata" {
			t.Fatalf("iç içe=%v: panik çağırana taşınmadı: %q", icIce, m)
		}
		// Havuz hâlâ çalışır ve bütün parçaları yürütür.
		var n atomic.Int64
		dagit(4, 100, func(_, c int) { n.Add(1) })
		if n.Load() != 100 {
			t.Fatalf("panik sonrası havuz %d/100 parça yürüttü", n.Load())
		}
	}
}
