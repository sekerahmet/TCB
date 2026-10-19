package tensor

import (
	"math"
	"math/rand"
	"testing"
)

// Matris çarpımı çekirdeğinin (derlemeye göre AVX2/FMA ya da taşınabilir Go)
// rastgele ve kenar boyutlarla doğruluk testleri.

// gemmRef float64 birikimli C += op(A)·op(B) ve hata sınırı için Σ|a·b|.
func gemmRef(batch, M, N, K int, a []float32, transA bool, b []float32, transB bool, c []float32) ([]float64, []float64) {
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

func randomSlice(rng *rand.Rand, n int) []float32 {
	d := make([]float32, n)
	for i := range d {
		d[i] = float32(rng.NormFloat64())
	}
	return d
}

func TestGemmEdgeSizes(t *testing.T) {
	rng := rand.New(rand.NewSource(31))
	size := []int{1, 3, 7, 16, 17, 63, 65}
	counter := 0
	for _, M := range size {
		for _, N := range size {
			for _, K := range size {
				counter++
				// Her üçlüde devrik bayrakları ve toplu boyut dönüşümlü seçilir.
				transA, transB := counter%2 == 1, counter%3 == 1
				batch := 1 + counter%3
				checkGemm(t, rng, batch, M, N, K, transA, transB)
			}
		}
	}
	// Büyük, paralel ve K bloklu (K > kcBlok) durumlar.
	for _, d := range [][3]int{{1024, 64, 64}, {64, 256, 1024}, {300, 130, 600}, {97, 33, 257}} {
		for _, tr := range [][2]bool{{false, false}, {true, false}, {false, true}} {
			checkGemm(t, rng, 1, d[0], d[1], d[2], tr[0], tr[1])
		}
	}
	// Dikkat boyutları: 64 × (64×16)·(16×64) ve 64 × (64×64)·(64×16).
	checkGemm(t, rng, 64, 64, 64, 16, false, true)
	checkGemm(t, rng, 64, 64, 16, 64, false, false)
}

func checkGemm(t *testing.T, rng *rand.Rand, batch, M, N, K int, transA, transB bool) {
	t.Helper()
	a := randomSlice(rng, batch*M*K)
	b := randomSlice(rng, batch*K*N)
	c := randomSlice(rng, batch*M*N) // C += olduğu için sıfırdan farklı başlangıç
	want, abs := gemmRef(batch, M, N, K, a, transA, b, transB, c)
	gemm(batch, M, N, K, a, transA, b, transB, c)
	for i := range c {
		// float32 birikiminin hata sınırı: ~K·ε·Σ|a·b| (çok gevşek değil).
		limit := 4 * float64(K+2) * 6e-8 * abs[i]
		if d := math.Abs(float64(c[i]) - want[i]); d > limit+1e-6 || math.IsNaN(float64(c[i])) {
			t.Fatalf("%s gemm b=%d M=%d N=%d K=%d tA=%v tB=%v: [%d] = %v, beklenen %v (sınır %g)",
				microKernelName, batch, M, N, K, transA, transB, i, c[i], want[i], limit)
		}
	}
}

// Mikro çekirdek kenar blokları: mm < mr, nn < nr ve küçük kb.
func TestMicroKernelEdges(t *testing.T) {
	rng := rand.New(rand.NewSource(32))
	for _, kb := range []int{1, 2, 3, 16, 17} {
		for mm := 1; mm <= mr; mm++ {
			for nn := 1; nn <= nr; nn++ {
				ap := randomSlice(rng, kb*mr)
				bp := randomSlice(rng, kb*nr)
				ldc := nr + 3
				c := randomSlice(rng, mr*ldc)
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
						t.Fatalf("%s kb=%d mm=%d nn=%d: [%d] = %v, beklenen %v", microKernelName, kb, mm, nn, i, c[i], want[i])
					}
				}
			}
		}
	}
}

// Adımlı A okuyan mikro çekirdek (satır öncelikli, devrik ve paketli düzenler).
func TestMicroAStrided(t *testing.T) {
	rng := rand.New(rand.NewSource(34))
	for _, kb := range []int{1, 5, 16, 64} {
		for _, d := range [][2]int{{kb + 3, 1}, {1, mr + 2}, {1, mr}} { // (rs, ks)
			rs, ks := d[0], d[1]
			a := randomSlice(rng, (mr-1)*rs+(kb-1)*ks+1)
			bp := randomSlice(rng, kb*nr)
			for _, mn := range [][2]int{{mr, nr}, {mr - 1, nr}, {mr, min(3, nr)}, {1, 1}} {
				mm, nn := mn[0], mn[1]
				ldc := nr + 1
				c := randomSlice(rng, mr*ldc)
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
				microA(kb, a, rs, ks, bp, c, ldc, mm, nn)
				for i := range c {
					if math.Abs(float64(c[i])-want[i]) > 1e-4*(1+math.Abs(want[i])) {
						t.Fatalf("%s mikroA kb=%d rs=%d ks=%d mm=%d nn=%d: [%d] = %v, beklenen %v",
							microKernelName, kb, rs, ks, mm, nn, i, c[i], want[i])
					}
				}
			}
		}
	}
}

// Bulgu 18: büyük toplu çarpımda B paketlemesi toplu parçalara bölünür (tampon sınırlı ve
// havuzdan); sonuç her toplu elemanın tek başına çarpımıyla bit düzeyinde aynıdır.
func TestGemmBatchPart(t *testing.T) {
	rng := rand.New(rand.NewSource(9))
	cases := []struct{ batch, M, N, K int }{{3, 13, 17, 300}, {70, 64, 1100, 64}, {300, 9, 300, 64}}
	for _, d := range cases {
		if d.batch*((d.N+nr-1)/nr*d.K*nr) <= bufMaxB && d.batch > 3 {
			t.Fatalf("%v parçalanmıyor; durum yanlış seçilmiş", d)
		}
		for _, tr := range [][2]bool{{false, false}, {true, false}, {false, true}, {true, true}} {
			a := Randn(rng, d.batch*d.M*d.K).Data()
			b := Randn(rng, d.batch*d.K*d.N).Data()
			c1 := make([]float32, d.batch*d.M*d.N)
			c2 := make([]float32, d.batch*d.M*d.N)
			gemm(d.batch, d.M, d.N, d.K, a, tr[0], b, tr[1], c1)
			for bi := 0; bi < d.batch; bi++ {
				gemm(1, d.M, d.N, d.K, a[bi*d.M*d.K:(bi+1)*d.M*d.K], tr[0], b[bi*d.K*d.N:(bi+1)*d.K*d.N], tr[1], c2[bi*d.M*d.N:(bi+1)*d.M*d.N])
			}
			for i := range c1 {
				if c1[i] != c2[i] {
					t.Fatalf("%v devrik=%v: [%d] %v != %v", d, tr, i, c1[i], c2[i])
				}
			}
		}
	}
}

// Eşdeğerlik: çarpım, float64 ile yapılan düz üçlü döngüyle aynı sonucu vermeli
// (float32 toleransı).
func TestMatMulMatchesFloat64(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	n := 70
	x, y := make([]float64, n*n), make([]float64, n*n)
	for i := range x {
		x[i], y[i] = rng.NormFloat64(), rng.NormFloat64()
	}
	want := make([]float32, n*n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			s := 0.0
			for k := 0; k < n; k++ {
				s += x[i*n+k] * y[k*n+j]
			}
			want[i*n+j] = float32(s)
		}
	}
	got := MatMul(FromFloat64(x, n, n), FromFloat64(y, n, n)).Data()
	near(t, "float64 eşdeğerlik", got, want, 1e-4)
}
