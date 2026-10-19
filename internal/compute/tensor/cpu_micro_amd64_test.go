package tensor

import (
	"math"
	"math/rand"
	"testing"
)

// TestMicro6x16GoMatchesAVX2 AVX2'siz işlemcilerin Go çekirdeğinin (micro6x16Go) assembly
// çekirdeğiyle ve float64 başvurusuyla aynı sonucu verdiğini denetler (kaynaştırılmış
// çarpma-toplama yüzünden birkaç ulp fark olabilir). Adımlı A (doğrudan okuma) da sınanır.
func TestMicro6x16GoMatchesAVX2(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	for _, kb := range []int{1, 2, 7, 64, 257} {
		for _, st := range [][2]int{{1, mr}, {kb + 3, 1}} { // paketli ve satır öncelikli A
			rs, ks := st[0], st[1]
			a := make([]float32, (mr-1)*rs+(kb-1)*ks+1)
			bp := make([]float32, kb*nr)
			for i := range a {
				a[i] = float32(rng.NormFloat64())
			}
			for i := range bp {
				bp[i] = float32(rng.NormFloat64())
			}
			const ldc = nr + 5
			got := make([]float32, mr*ldc)
			for i := range got {
				got[i] = 1
			}
			micro6x16Go(kb, a, rs, ks, bp, got, ldc)
			var asm []float32
			if hasAVX2 {
				asm = make([]float32, mr*ldc)
				for i := range asm {
					asm[i] = 1
				}
				sgemm6x16(kb, &a[0], rs, ks, &bp[0], &asm[0], ldc)
			}
			for i := 0; i < mr; i++ {
				for j := 0; j < ldc; j++ {
					want := 1.0
					if j < nr {
						for k := 0; k < kb; k++ {
							want += float64(a[i*rs+k*ks]) * float64(bp[k*nr+j])
						}
					}
					tol := 1e-5 * float64(kb)
					if g := float64(got[i*ldc+j]); math.Abs(g-want) > tol {
						t.Fatalf("kb=%d rs=%d [%d][%d]: Go %v, başvuru %v", kb, rs, i, j, g, want)
					}
					if asm != nil && math.Abs(float64(asm[i*ldc+j]-got[i*ldc+j])) > tol {
						t.Fatalf("kb=%d rs=%d [%d][%d]: Go %v, AVX2 %v", kb, rs, i, j, got[i*ldc+j], asm[i*ldc+j])
					}
				}
			}
		}
	}
}
