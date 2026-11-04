package tensor

import (
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"testing"
)

// Üretimin çarpım boyları (batch, N, K): Qwen3-0.6B ve SmolLM2-135M'in katmanları ile paylaşımlı
// gömmenin son çarpımı (satırlar bağımsızdır, sözlük satır sayısı küçültüldü), kısa son şerit,
// 32'ye bölünmeyen K ve toplu çarpım.
var gemv16Shapes = [][3]int{
	{1, 2048, 1024}, {1, 1024, 2048}, {1, 3072, 1024}, {1, 1024, 3072}, {1, 4096, 1024},
	{1, 576, 576}, {1, 192, 576}, {1, 1536, 576}, {1, 576, 1536}, {1, 4096, 576},
	{1, 65, 70}, {1, 577, 33}, {1, 63, 200}, {1, 64, 31}, {1, 100, 7}, {3, 130, 33}, {2, 192, 256},
}

// random16 n öğelik 16 bitlik dizi; special doğruysa içinde normal altı, sıfır, sonsuz ve NaN da
// vardır.
func random16(r *rand.Rand, n int, dt DType, special bool) []uint16 {
	odd := []float32{0, float32(math.Copysign(0, -1)), 1e-40, 6e-8, float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.NaN())}
	h := make([]uint16, n)
	for i := range h {
		v := float32(r.NormFloat64())
		if special && r.Intn(16) == 0 {
			v = odd[r.Intn(len(odd))]
		}
		h[i] = narrowOne(v, dt)
	}
	return h
}

func randomData(r *rand.Rand, n int) []float32 {
	d := make([]float32, n)
	for i := range d {
		d[i] = float32(r.NormFloat64())
	}
	return d
}

func sameFloatBits(t *testing.T, what string, got, want []float32) {
	t.Helper()
	for i, w := range want {
		if math.Float32bits(got[i]) != math.Float32bits(w) {
			t.Fatalf("%s: [%d] = %v (%#08x), beklenen %v (%#08x)", what, i, got[i], math.Float32bits(got[i]), w, math.Float32bits(w))
		}
	}
}

// 16 bitlik yol "aç + float32 gemv" ile iki B düzeninde, her iki türde, AVX2'li ve Go yolunda,
// kısa şeritte ve toplu çarpımda aynı bitleri verir; c'ye ekler. Özel değerlerde de aynıdır.
func TestGemv16MatchesWidenedGemv(t *testing.T) {
	old := hasAVX2
	defer func() { hasAVX2 = old }()
	r := rand.New(rand.NewSource(1))
	for _, avx := range []bool{old, false} {
		hasAVX2 = avx
		for _, dt := range []DType{BFloat16, Float16} {
			if !direct16(dt) {
				continue
			}
			for _, s := range gemv16Shapes {
				batch, N, K := s[0], s[1], s[2]
				for _, transB := range []bool{false, true} {
					for _, special := range []bool{false, true} {
						a := randomData(r, batch*K)
						b := random16(r, batch*K*N, dt, special)
						wide := make([]float32, len(b))
						widen16(wide, b, dt)
						want, got := make([]float32, batch*N), make([]float32, batch*N)
						for i := range want {
							want[i] = float32(i%5) - 2
							got[i] = want[i]
						}
						gemv(batch, N, K, a, wide, transB, want)
						gemv16(batch, N, K, a, b, dt, transB, got)
						sameFloatBits(t, fmt.Sprintf("avx2=%v %s b=%d N=%d K=%d transB=%v özel=%v", avx, dt, batch, N, K, transB, special), got, want)
					}
				}
			}
		}
	}
}

// İş parçacığı sayısı sonucu değiştirmez (15.22a): her çıktı tek parçada hesaplanır.
func TestGemv16SameBitsAcrossThreads(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(0))
	r := rand.New(rand.NewSource(2))
	N, K := 2048, 1024
	a := randomData(r, K)
	for _, dt := range []DType{BFloat16, Float16} {
		b := random16(r, K*N, dt, false)
		for _, transB := range []bool{false, true} {
			var want []float32
			for _, g := range []int{1, 2, 8} {
				runtime.GOMAXPROCS(g)
				got := make([]float32, N)
				gemv16(1, N, K, a, b, dt, transB, got)
				if want == nil {
					want = got
				}
				sameFloatBits(t, fmt.Sprintf("%s transB=%v g=%d", dt, transB, g), got, want)
			}
		}
	}
}

// Arka ucun çarpımı eski yolla (bütün matrisi açan f32Gemm) aynı bitleri verir: tek satır,
// paketli çarpıma giden küçük ve çok satırlı çarpım, 16 bitlik ya da float32 A ve C, uzaklıklı
// ağırlık görünümü; AVX2'li ama F16C'siz işlemcide float16 açılan yoldan geçer.
func TestGemm16MatchesWidenedGemm(t *testing.T) {
	oldF16C := hasF16C
	defer func() { hasF16C = oldF16C }()
	r := rand.New(rand.NewSource(3))
	be := cpuBackend{}
	for _, f16c := range []bool{oldF16C, false} {
		hasF16C = f16c
		for _, wt := range []DType{BFloat16, Float16} {
			for _, at := range []DType{Float32, wt} {
				for _, s := range [][3]int{{1, 256, 64}, {1, 200, 70}, {1, 16, 16}, {3, 256, 64}} {
					M, N, K := s[0], s[1], s[2]
					for _, transB := range []bool{false, true} {
						bv := NewOf(wt, randomData(r, (K+3)*N), K+3, N).Slice(0, 3, K+3).view()
						a := NewOf(at, randomData(r, M*K), M, K).view()
						got, want := be.Alloc(M*N, at), be.Alloc(M*N, at)
						be.Gemm(1, M, N, K, a, false, bv, transB, got)
						var gt temp
						o, last := gt.write(want, true)
						be.f32Gemm(1, M, N, K, gt.read(a), false, gt.read(bv), transB, o)
						last()
						gt.finish()
						name := fmt.Sprintf("f16c=%v %s·%s %v transB=%v", f16c, at, wt, s, transB)
						sameFloatBits(t, name, wideOf(got), wideOf(want))
					}
				}
			}
		}
	}
}

// wideOf depolamanın float32 karşılığı.
func wideOf(s Storage) []float32 {
	c := cpuStore(s)
	if c.dtyp == Float32 {
		return c.d
	}
	d := make([]float32, len(c.h))
	widen16(d, c.h, c.dtyp)
	return d
}
