package tensor

import (
	"math"
	"math/rand"
	"testing"
)

// Matris çarpımı çekirdeğinin (derlemeye göre AVX2/FMA ya da taşınabilir Go)
// rastgele ve kenar boyutlarla doğruluk testleri.

// gemmBasvuru float64 birikimli C += op(A)·op(B) ve hata sınırı için Σ|a·b|.
func gemmBasvuru(batch, M, N, K int, a []float32, transA bool, b []float32, transB bool, c []float32) ([]float64, []float64) {
	out := make([]float64, batch*M*N)
	abs := make([]float64, batch*M*N)
	for bi := 0; bi < batch; bi++ {
		for i := 0; i < M; i++ {
			for j := 0; j < N; j++ {
				s, sa := float64(c[(bi*M+i)*N+j]), 0.0
				for k := 0; k < K; k++ {
					var av, bv float32
					if transA {
						av = a[bi*M*K+k*M+i]
					} else {
						av = a[bi*M*K+i*K+k]
					}
					if transB {
						bv = b[bi*K*N+j*K+k]
					} else {
						bv = b[bi*K*N+k*N+j]
					}
					s += float64(av) * float64(bv)
					sa += math.Abs(float64(av) * float64(bv))
				}
				out[(bi*M+i)*N+j] = s
				abs[(bi*M+i)*N+j] = sa + math.Abs(float64(c[(bi*M+i)*N+j]))
			}
		}
	}
	return out, abs
}

func rastgeleDilim(rng *rand.Rand, n int) []float32 {
	d := make([]float32, n)
	for i := range d {
		d[i] = float32(rng.NormFloat64())
	}
	return d
}

func TestGemmKenarBoyutlar(t *testing.T) {
	rng := rand.New(rand.NewSource(31))
	boy := []int{1, 3, 7, 16, 17, 63, 65}
	sayac := 0
	for _, M := range boy {
		for _, N := range boy {
			for _, K := range boy {
				sayac++
				// Her üçlüde devrik bayrakları ve toplu boyut dönüşümlü seçilir.
				transA, transB := sayac%2 == 1, sayac%3 == 1
				batch := 1 + sayac%3
				gemmDenetle(t, rng, batch, M, N, K, transA, transB)
			}
		}
	}
	// Büyük, paralel ve K bloklu (K > kcBlok) durumlar.
	for _, d := range [][3]int{{1024, 64, 64}, {64, 256, 1024}, {300, 130, 600}, {97, 33, 257}} {
		for _, tr := range [][2]bool{{false, false}, {true, false}, {false, true}} {
			gemmDenetle(t, rng, 1, d[0], d[1], d[2], tr[0], tr[1])
		}
	}
	// Dikkat boyutları: 64 × (64×16)·(16×64) ve 64 × (64×64)·(64×16).
	gemmDenetle(t, rng, 64, 64, 64, 16, false, true)
	gemmDenetle(t, rng, 64, 64, 16, 64, false, false)
}

func gemmDenetle(t *testing.T, rng *rand.Rand, batch, M, N, K int, transA, transB bool) {
	t.Helper()
	a := rastgeleDilim(rng, batch*M*K)
	b := rastgeleDilim(rng, batch*K*N)
	c := rastgeleDilim(rng, batch*M*N) // C += olduğu için sıfırdan farklı başlangıç
	want, abs := gemmBasvuru(batch, M, N, K, a, transA, b, transB, c)
	gemm(batch, M, N, K, a, transA, b, transB, c)
	for i := range c {
		// float32 birikiminin hata sınırı: ~K·ε·Σ|a·b| (çok gevşek değil).
		sinir := 4 * float64(K+2) * 6e-8 * abs[i]
		if d := math.Abs(float64(c[i]) - want[i]); d > sinir+1e-6 || math.IsNaN(float64(c[i])) {
			t.Fatalf("%s gemm b=%d M=%d N=%d K=%d tA=%v tB=%v: [%d] = %v, beklenen %v (sınır %g)",
				mikroCekirdekAdi, batch, M, N, K, transA, transB, i, c[i], want[i], sinir)
		}
	}
}

// Mikro çekirdek kenar blokları: mm < mr, nn < nr ve küçük kb.
func TestMikroCekirdekKenar(t *testing.T) {
	rng := rand.New(rand.NewSource(32))
	for _, kb := range []int{1, 2, 3, 16, 17} {
		for mm := 1; mm <= mr; mm++ {
			for nn := 1; nn <= nr; nn++ {
				ap := rastgeleDilim(rng, kb*mr)
				bp := rastgeleDilim(rng, kb*nr)
				ldc := nr + 3
				c := rastgeleDilim(rng, mr*ldc)
				want := make([]float64, len(c))
				for i := range c {
					want[i] = float64(c[i])
				}
				for i := 0; i < mm; i++ {
					for j := 0; j < nn; j++ {
						for k := 0; k < kb; k++ {
							want[i*ldc+j] += float64(ap[k*mr+i]) * float64(bp[k*nr+j])
						}
					}
				}
				microKernel(kb, ap, bp, c, ldc, mm, nn)
				for i := range c {
					if math.Abs(float64(c[i])-want[i]) > 1e-4*(1+math.Abs(want[i])) {
						t.Fatalf("%s kb=%d mm=%d nn=%d: [%d] = %v, beklenen %v", mikroCekirdekAdi, kb, mm, nn, i, c[i], want[i])
					}
				}
			}
		}
	}
}

// Adımlı A okuyan mikro çekirdek (satır öncelikli, devrik ve paketli düzenler).
func TestMikroAAdimli(t *testing.T) {
	rng := rand.New(rand.NewSource(34))
	for _, kb := range []int{1, 5, 16, 64} {
		for _, d := range [][2]int{{kb + 3, 1}, {1, mr + 2}, {1, mr}} { // (rs, ks)
			rs, ks := d[0], d[1]
			a := rastgeleDilim(rng, (mr-1)*rs+(kb-1)*ks+1)
			bp := rastgeleDilim(rng, kb*nr)
			for _, mn := range [][2]int{{mr, nr}, {mr - 1, nr}, {mr, min(3, nr)}, {1, 1}} {
				mm, nn := mn[0], mn[1]
				ldc := nr + 1
				c := rastgeleDilim(rng, mr*ldc)
				want := make([]float64, len(c))
				for i := range c {
					want[i] = float64(c[i])
				}
				for i := 0; i < mm; i++ {
					for j := 0; j < nn; j++ {
						for k := 0; k < kb; k++ {
							want[i*ldc+j] += float64(a[i*rs+k*ks]) * float64(bp[k*nr+j])
						}
					}
				}
				mikroA(kb, a, rs, ks, bp, c, ldc, mm, nn)
				for i := range c {
					if math.Abs(float64(c[i])-want[i]) > 1e-4*(1+math.Abs(want[i])) {
						t.Fatalf("%s mikroA kb=%d rs=%d ks=%d mm=%d nn=%d: [%d] = %v, beklenen %v",
							mikroCekirdekAdi, kb, rs, ks, mm, nn, i, c[i], want[i])
					}
				}
			}
		}
	}
}
