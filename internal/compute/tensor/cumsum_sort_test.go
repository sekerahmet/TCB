package tensor

import (
	"math"
	"math/rand"
	"slices"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Birikimli toplam ve sıralama: float64 başvuru, eksenler, gradyanlar, eşitlik ve NaN
// kuralları, 16 bitlik türler, hatalar.

// cumsumRef eksen boyunca float64 birikimli toplam.
func cumsumRef(x []float32, shape []int, axis int) []float32 {
	outer, dim, inner := axisOrder(shape, axis)
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
			near(t, "cumsum", got, cumsumRef(x.Data(), sh, axis), 1e-6)
		}
	}
	// Bitişik olmayan görünüm ve negatif eksen.
	x := Randn(rng, 3, 4)
	near(t, "cumsum devrik", x.T().CumSum(-1).Data(), cumsumRef(x.T().Contiguous().Data(), []int{4, 3}, 1), 1e-6)
	checkGrad(t, "cumsum eksen 0", func(z []*Tensor) *Tensor { return z[0].CumSum(0) }, random(rng, -1, 1, 0, 3, 4))
	checkGrad(t, "cumsum eksen 1", func(z []*Tensor) *Tensor { return z[0].CumSum(1) }, random(rng, -1, 1, 0, 3, 4))
	// Gradyan sondan birikimli toplamdır: d/dx_j Σ_i cumsum_i = n − j.
	v := New([]float32{1, 2, 3, 4}, 4).SetRequiresGrad(true)
	v.CumSum(0).SumAll().Backward()
	nanSame(t, "cumsum gradyanı", v.Grad().Data(), []float32{4, 3, 2, 1})
	// 16 bit: float32'de hesaplanıp sonuç türüne yuvarlanır.
	b := NewOf(BFloat16, []float32{1, 2, 3, 0.5}, 4)
	c := b.CumSum(0)
	if c.DType() != BFloat16 {
		t.Fatalf("bf16 cumsum türü %s", c.DType())
	}
	nanSame(t, "bf16 cumsum", c.Data(), []float32{1, 3, 6, 6.5})
}

// sortRef eksen boyunca kararlı sıralama (NaN en büyük, eşitlerde küçük indeks önce).
func sortRef(x []float32, shape []int, axis int, desc bool) []float32 {
	outer, dim, inner := axisOrder(shape, axis)
	out := make([]float32, len(x))
	for o := 0; o < outer; o++ {
		for i := 0; i < inner; i++ {
			idx := make([]int, dim)
			for d := range idx {
				idx[d] = d
			}
			value := func(d int) float32 { return x[(o*dim+d)*inner+i] }
			slices.SortStableFunc(idx, func(p, q int) int {
				a, b := value(p), value(q)
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

func TestArgSortAndTopK(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	for _, sh := range [][]int{{9}, {4, 6}, {2, 5, 3}, {3, 1}} {
		for axis := range sh {
			data := make([]float32, numelOf(sh))
			for i := range data {
				data[i] = float32(rng.Intn(5)) - 2 // bol eşitlik
			}
			data[0], data[len(data)-1] = nan32, float32(math.Copysign(0, -1))
			x := New(data, sh...)
			nanSame(t, "argsort", x.ArgSort(axis).Data(), sortRef(data, sh, axis, false))
			descending := sortRef(data, sh, axis, true)
			for _, k := range []int{0, 1, sh[axis]} {
				vals, idx := x.TopK(k, axis)
				ksh := copyInts(sh)
				ksh[axis] = k
				shapeEqual(t, "topk şekli", idx.Shape(), ksh)
				// İlk k indeks azalan kararlı sıralamanınkilerdir; değerler o indekslerdekiler.
				want := New(descending, sh...).Slice(axis, 0, k).Contiguous().Data()
				nanSame(t, "topk indeks", idx.Data(), want)
				wv := make([]float32, len(want))
				outer, _, inner := axisOrder(ksh, axis)
				for o := 0; o < outer; o++ {
					for j := 0; j < k; j++ {
						for i := 0; i < inner; i++ {
							p := (o*k+j)*inner + i
							wv[p] = data[(o*sh[axis]+int(want[p]))*inner+i]
						}
					}
				}
				nanSame(t, "topk değer", vals.Data(), wv)
			}
		}
	}
	// -0 ile +0 eşittir: kararlı sıralama ilk geleni önce koyar.
	z := New([]float32{0, float32(math.Copysign(0, -1)), -1, 1}, 4)
	nanSame(t, "sıfırlar", z.ArgSort(0).Data(), []float32{2, 0, 1, 3})
	_, ti := New([]float32{3, nan32, 3, 5}, 4).TopK(3, 0)
	nanSame(t, "NaN en büyük", ti.Data(), []float32{1, 3, 0})
}

func TestTopKGrad(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	x := random(rng, -1, 1, 0, 3, 5)
	checkGrad(t, "topk eksen 1", func(z []*Tensor) *Tensor { v, _ := z[0].TopK(2, 1); return v }, x)
	checkGrad(t, "topk eksen 0", func(z []*Tensor) *Tensor { v, _ := z[0].TopK(3, 0); return v }, random(rng, -1, 1, 0, 3, 5))
	// İndeksler türev izlemez; değerlerin gradyanı seçilen konumlara gider.
	y := New([]float32{1, 5, 3}, 3).SetRequiresGrad(true)
	v, idx := y.TopK(2, 0)
	if idx.RequiresGrad() {
		t.Fatal("topk indeksleri türev izlememeli")
	}
	v.Mul(New([]float32{10, 20}, 2)).SumAll().Backward()
	nanSame(t, "topk gradyanı", y.Grad().Data(), []float32{0, 10, 20})
	if y.ArgSort(0).RequiresGrad() {
		t.Fatal("argsort türev izlememeli")
	}
}

func TestSortErrors(t *testing.T) {
	x := New([]float32{1, 2, 3}, 3)
	panicCode(t, "k büyük", messages.H3027, func() { x.TopK(4, 0) })
	panicCode(t, "k negatif", messages.H3027, func() { x.TopK(-1, 0) })
	bf := NewOf(Float16, []float32{3, 1, 2}, 3)
	v, idx := bf.TopK(2, 0)
	if v.DType() != Float16 || idx.DType() != Float32 {
		t.Fatalf("fp16 topk türleri %s, %s", v.DType(), idx.DType())
	}
	nanSame(t, "fp16 topk", v.Data(), []float32{3, 2})
}

// ============================================================================
// Gradyan kapısı: birikimli toplam ve en büyük k
// ============================================================================

func init() {
	cum := func(l []float64) []float64 {
		r := make([]float64, len(l))
		var s float64
		for i, v := range l {
			s += v
			r[i] = s
		}
		return r
	}
	top2 := func(l []float64) []float64 {
		s := append([]float64(nil), l...)
		slices.SortFunc(s, func(a, b float64) int {
			if a > b {
				return -1
			}
			return 1
		})
		return s[:2]
	}
	lane := laneRef
	dis := genDisjoint
	registerGrad("cumsum_sort",
		gradSpec{name: "cumsum eksen 0", ops: []string{"cumsum"}, build: func(xs []*Tensor) *Tensor { return xs[0].CumSum(0) },
			inputs: genInputs(gen(-1, 1, 0, 4, 3)), ref: lane([]int{4, 3}, 0, cum)},
		gradSpec{name: "cumsum eksen 1", ops: []string{"cumsum"}, build: func(xs []*Tensor) *Tensor { return xs[0].CumSum(1) },
			inputs: genInputs(gen(-1, 1, 0, 3, 5)), ref: lane([]int{3, 5}, 1, cum)},
		gradSpec{name: "topk eksen 1", ops: []string{"topk"}, build: func(xs []*Tensor) *Tensor { v, _ := xs[0].TopK(2, 1); return v },
			inputs: genInputs(dis(3, 5)), ref: lane([]int{3, 5}, 1, top2)},
		gradSpec{name: "topk eksen 0", ops: []string{"topk"}, build: func(xs []*Tensor) *Tensor { v, _ := xs[0].TopK(2, 0); return v },
			inputs: genInputs(dis(4, 3)), ref: lane([]int{4, 3}, 0, top2)})
}

func TestGradCumSumSort(t *testing.T) { runGradGroup(t, "cumsum_sort") }
