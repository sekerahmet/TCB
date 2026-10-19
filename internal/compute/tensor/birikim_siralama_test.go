package tensor

import (
	"math"
	"math/rand"
	"slices"
	"testing"
)

// Birikimli toplam ve sıralama: float64 başvuru, eksenler, gradyanlar, eşitlik ve NaN
// kuralları, 16 bitlik türler, hatalar.

// cumsumBasvuru eksen boyunca float64 birikimli toplam.
func cumsumBasvuru(x []float32, shape []int, axis int) []float32 {
	outer, dim, inner := eksenDuzeni(shape, axis)
	out := make([]float32, len(x))
	for o := 0; o < outer; o++ {
		for i := 0; i < inner; i++ {
			var s float64
			for d := 0; d < dim; d++ {
				j := (o*dim+d)*inner + i
				s += float64(x[j])
				out[j] = float32(s)
			}
		}
	}
	return out
}

func TestCumSum(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, sh := range [][]int{{7}, {3, 5}, {2, 3, 4}, {1, 1}, {4, 0}, {2, 1000}} {
		for axis := range sh {
			x := Randn(rng, sh...)
			got := x.CumSum(axis).Data()
			yakin(t, "cumsum", got, cumsumBasvuru(x.Data(), sh, axis), 1e-6)
		}
	}
	// Bitişik olmayan görünüm ve negatif eksen.
	x := Randn(rng, 3, 4)
	yakin(t, "cumsum devrik", x.T().CumSum(-1).Data(), cumsumBasvuru(x.T().Contiguous().Data(), []int{4, 3}, 1), 1e-6)
	gradDenetle(t, "cumsum eksen 0", func(z []*Tensor) *Tensor { return z[0].CumSum(0) }, rastgele(rng, -1, 1, 0, 3, 4))
	gradDenetle(t, "cumsum eksen 1", func(z []*Tensor) *Tensor { return z[0].CumSum(1) }, rastgele(rng, -1, 1, 0, 3, 4))
	// Gradyan sondan birikimli toplamdır: d/dx_j Σ_i cumsum_i = n − j.
	v := New([]float32{1, 2, 3, 4}, 4).SetRequiresGrad(true)
	v.CumSum(0).SumAll().Backward()
	nanAyni(t, "cumsum gradyanı", v.Grad().Data(), []float32{4, 3, 2, 1})
	// 16 bit: float32'de hesaplanıp sonuç türüne yuvarlanır.
	b := NewOf(BFloat16, []float32{1, 2, 3, 0.5}, 4)
	c := b.CumSum(0)
	if c.DType() != BFloat16 {
		t.Fatalf("bf16 cumsum türü %s", c.DType())
	}
	nanAyni(t, "bf16 cumsum", c.Data(), []float32{1, 3, 6, 6.5})
}

// siralamaBasvuru eksen boyunca kararlı sıralama (NaN en büyük, eşitlerde küçük indeks önce).
func siralamaBasvuru(x []float32, shape []int, axis int, desc bool) []float32 {
	outer, dim, inner := eksenDuzeni(shape, axis)
	out := make([]float32, len(x))
	for o := 0; o < outer; o++ {
		for i := 0; i < inner; i++ {
			idx := make([]int, dim)
			for d := range idx {
				idx[d] = d
			}
			deger := func(d int) float32 { return x[(o*dim+d)*inner+i] }
			slices.SortStableFunc(idx, func(p, q int) int {
				a, b := deger(p), deger(q)
				an, bn := a != a, b != b
				var c int
				switch {
				case an && bn, a == b:
					c = 0
				case an:
					c = 1
				case bn:
					c = -1
				case a < b:
					c = -1
				default:
					c = 1
				}
				if desc {
					c = -c
				}
				return c
			})
			for d, j := range idx {
				out[(o*dim+d)*inner+i] = float32(j)
			}
		}
	}
	return out
}

func TestArgSortVeTopK(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	for _, sh := range [][]int{{9}, {4, 6}, {2, 5, 3}, {3, 1}} {
		for axis := range sh {
			veri := make([]float32, numelOf(sh))
			for i := range veri {
				veri[i] = float32(rng.Intn(5)) - 2 // bol eşitlik
			}
			veri[0], veri[len(veri)-1] = nan32, float32(math.Copysign(0, -1))
			x := New(veri, sh...)
			nanAyni(t, "argsort", x.ArgSort(axis).Data(), siralamaBasvuru(veri, sh, axis, false))
			azalan := siralamaBasvuru(veri, sh, axis, true)
			for _, k := range []int{0, 1, sh[axis]} {
				vals, idx := x.TopK(k, axis)
				ksh := copyInts(sh)
				ksh[axis] = k
				sekilEsit(t, "topk şekli", idx.Shape(), ksh)
				// İlk k indeks azalan kararlı sıralamanınkilerdir; değerler o indekslerdekiler.
				want := New(azalan, sh...).Slice(axis, 0, k).Contiguous().Data()
				nanAyni(t, "topk indeks", idx.Data(), want)
				wv := make([]float32, len(want))
				outer, _, inner := eksenDuzeni(ksh, axis)
				for o := 0; o < outer; o++ {
					for j := 0; j < k; j++ {
						for i := 0; i < inner; i++ {
							p := (o*k+j)*inner + i
							wv[p] = veri[(o*sh[axis]+int(want[p]))*inner+i]
						}
					}
				}
				nanAyni(t, "topk değer", vals.Data(), wv)
			}
		}
	}
	// -0 ile +0 eşittir: kararlı sıralama ilk geleni önce koyar.
	z := New([]float32{0, float32(math.Copysign(0, -1)), -1, 1}, 4)
	nanAyni(t, "sıfırlar", z.ArgSort(0).Data(), []float32{2, 0, 1, 3})
	_, ti := New([]float32{3, nan32, 3, 5}, 4).TopK(3, 0)
	nanAyni(t, "NaN en büyük", ti.Data(), []float32{1, 3, 0})
}

func TestTopKGradyan(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	x := rastgele(rng, -1, 1, 0, 3, 5)
	gradDenetle(t, "topk eksen 1", func(z []*Tensor) *Tensor { v, _ := z[0].TopK(2, 1); return v }, x)
	gradDenetle(t, "topk eksen 0", func(z []*Tensor) *Tensor { v, _ := z[0].TopK(3, 0); return v }, rastgele(rng, -1, 1, 0, 3, 5))
	// İndeksler türev izlemez; değerlerin gradyanı seçilen konumlara gider.
	y := New([]float32{1, 5, 3}, 3).SetRequiresGrad(true)
	v, idx := y.TopK(2, 0)
	if idx.RequiresGrad() {
		t.Fatal("topk indeksleri türev izlememeli")
	}
	v.Mul(New([]float32{10, 20}, 2)).SumAll().Backward()
	nanAyni(t, "topk gradyanı", y.Grad().Data(), []float32{0, 10, 20})
	if y.ArgSort(0).RequiresGrad() {
		t.Fatal("argsort türev izlememeli")
	}
}

func TestSiralamaHatalari(t *testing.T) {
	x := New([]float32{1, 2, 3}, 3)
	panikIcerir(t, "k büyük", "k 0 ile 3 arasında", func() { x.TopK(4, 0) })
	panikIcerir(t, "k negatif", "k 0 ile 3 arasında", func() { x.TopK(-1, 0) })
	bf := NewOf(Float16, []float32{3, 1, 2}, 3)
	v, idx := bf.TopK(2, 0)
	if v.DType() != Float16 || idx.DType() != Float32 {
		t.Fatalf("fp16 topk türleri %s, %s", v.DType(), idx.DType())
	}
	nanAyni(t, "fp16 topk", v.Data(), []float32{3, 2})
}
