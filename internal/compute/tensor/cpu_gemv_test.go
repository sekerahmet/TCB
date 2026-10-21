package tensor

import (
	"math"
	"testing"
)

// gemv (M = 1) gemm'in yerine geçer: c'ye ekler (c += a·op(B)), 64 sütunluk şerit sınırlarında,
// kısa son şeritte, toplu çarpımda ve iki B düzeninde float64 başvuruyla aynıdır; AVX2'li ve
// Go yolları aynı sonucu verir.
func TestGemvMatchesReference(t *testing.T) {
	for _, avx := range []bool{hasAVX2, false} {
		old := hasAVX2
		hasAVX2 = avx
		for _, s := range [][3]int{{1, 64, 64}, {1, 65, 70}, {2, 130, 33}, {1, 577, 1536}, {3, 63, 200}, {1, 1536, 576}} {
			batch, N, K := s[0], s[1], s[2]
			for _, transB := range []bool{false, true} {
				a := make([]float32, batch*K)
				b := make([]float32, batch*K*N)
				c := make([]float32, batch*N)
				for i := range a {
					a[i] = float32(math.Sin(float64(i) * 0.37))
				}
				for i := range b {
					b[i] = float32(math.Cos(float64(i) * 0.11))
				}
				for i := range c {
					c[i] = float32(i%5) - 2 // c'ye eklenir
				}
				want := make([]float64, batch*N)
				for bi := 0; bi < batch; bi++ {
					for n := 0; n < N; n++ {
						s := float64(c[bi*N+n])
						for k := 0; k < K; k++ {
							bv := b[bi*K*N+k*N+n]
							if transB {
								bv = b[bi*N*K+n*K+k]
							}
							s += float64(a[bi*K+k]) * float64(bv)
						}
						want[bi*N+n] = s
					}
				}
				gemv(batch, N, K, a, b, transB, c)
				for i, w := range want {
					if d := math.Abs(float64(c[i]) - w); d > 1e-4*float64(K) {
						t.Fatalf("avx2=%v b=%d N=%d K=%d transB=%v: [%d] = %v, beklenen %v", avx, batch, N, K, transB, i, c[i], w)
					}
				}
			}
		}
		hasAVX2 = old
	}
}
