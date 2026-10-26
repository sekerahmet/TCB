package tensor

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
)

// Bulgu 4: max/min/relu ve argmax NaN'ı yayar (indirgemede NaN varsa sonuç NaN,
// argmax ilk NaN'ın yeri); 16 bitlik türlerde de aynı.
func TestNaNPropagates(t *testing.T) {
	nan := float32(math.NaN())
	for _, dt := range []DType{Float32, BFloat16, Float16} {
		x := New([]float32{1, nan, 3, nan, nan, nan, 0, 5, 2}, 3, 3).Cast(dt)
		ad := dt.String()
		nanSame(t, ad+" Max(1)", x.Max(1, false).Data(), []float32{nan, nan, 5})
		nanSame(t, ad+" Max(0)", x.Max(0, false).Data(), []float32{nan, nan, nan})
		nanSame(t, ad+" -Max(-x)", x.Neg().Max(1, false).Neg().Data(), []float32{nan, nan, 0})
		if got := x.ArgMax(1); got[0] != 1 || got[1] != 0 || got[2] != 1 {
			t.Fatalf("%s ArgMax(1) = %v, beklenen [1 0 1]", ad, got)
		}
		if got := x.ArgMax(0); got[0] != 1 || got[1] != 0 || got[2] != 1 {
			t.Fatalf("%s ArgMax(0) = %v, beklenen [1 0 1]", ad, got)
		}
		nanSame(t, ad+" Relu", New([]float32{-1, nan, 2}, 3).Cast(dt).Relu().Data(), []float32{0, nan, 2})
	}
	// Gradyan ilk NaN'ın yerine gider.
	x := New([]float32{1, nan, 3}, 3).SetRequiresGrad(true)
	x.Max(0, false).Backward()
	nanSame(t, "Max gradyanı", x.Grad().Data(), []float32{0, 1, 0})
}

// Bulgu 6: tamamen maskeli (-sonsuz) softmax satırı 0 (log_softmax'ta -sonsuz) verir ve
// gradyanı 0'dır; birleşik dikkatteki boş satır davranışıyla tutarlı. Son eksen (vektörel)
// ve diğer eksen yolları ile 16 bitlik türler aynı.
func TestFullyMaskedSoftmax(t *testing.T) {
	ni := float32(math.Inf(-1))
	m := New([]float32{ni, ni, ni, 0, ni, 0}, 2, 3)
	for _, dt := range []DType{Float32, BFloat16} {
		for _, transposed := range []bool{false, true} {
			x := m.Cast(dt)
			axis := 1
			if transposed {
				x = x.T().Contiguous() // [3, 2]; satırlar eksen 0'da
				axis = 0
			}
			ad := fmt.Sprintf("%s eksen %d", dt, axis)
			s := x.Softmax(axis)
			ls := x.LogSoftmax(axis)
			if transposed {
				s, ls = s.T(), ls.T()
			}
			nanSame(t, ad+" softmax", s.Data(), []float32{0, 0, 0, 0.5, 0, 0.5})
			l := float32(math.Log(0.5))
			nanSame(t, ad+" log_softmax", ls.Data(), New([]float32{ni, ni, ni, l, ni, l}, 6).Cast(dt).Data())
		}
	}
	// Gradyan: maskeli satırda 0, diğer satırda sonlu.
	for _, logMode := range []bool{false, true} {
		w := New([]float32{1, 2, 3, 1, 2, 3}, 2, 3).SetRequiresGrad(true)
		z := Add(w, m)
		var y *Tensor
		if logMode {
			y = z.LogSoftmax(1)
		} else {
			y = z.Softmax(1)
		}
		// Gelen gradyan maskeli satırda da sıfırdan farklı (ileri değer -sonsuz olsa da
		// geri yayılım tanımlıdır).
		y = Mul(y, New([]float32{1, 2, 3, 1, 2, 3}, 2, 3)).SumAll()
		y.Backward()
		g := w.Grad().Data()
		for i, v := range g {
			if v != v || (i < 3 && v != 0) {
				t.Fatalf("log=%v gradyan %v: maskeli satır 0, hepsi sonlu olmalı", logMode, g)
			}
		}
	}
}

// ============================================================================
// Gradyan kapısı: indirgemeler, softmax, log_softmax (float64 başvuru + bf16)
// ============================================================================

func sum64(l []float64) float64 {
	var s float64
	for _, v := range l {
		s += v
	}
	return s
}

func max64(l []float64) float64 {
	m := math.Inf(-1)
	for _, v := range l {
		m = math.Max(m, v)
	}
	return m
}

func logSumExp64(l []float64) float64 {
	m := max64(l)
	var s float64
	for _, v := range l {
		s += math.Exp(v - m)
	}
	return m + math.Log(s)
}

func init() {
	one := func(f func([]float64) float64) func([]float64) []float64 {
		return func(l []float64) []float64 { return []float64{f(l)} }
	}
	lane := laneRef
	softmax := func(l []float64) []float64 {
		z := logSumExp64(l)
		r := make([]float64, len(l))
		for i, v := range l {
			r[i] = math.Exp(v - z)
		}
		return r
	}
	logSoftmax := func(l []float64) []float64 {
		z := logSumExp64(l)
		r := make([]float64, len(l))
		for i, v := range l {
			r[i] = v - z
		}
		return r
	}
	mean := func(l []float64) float64 { return sum64(l) / float64(len(l)) }
	spec := func(name, op string, f func(*Tensor) *Tensor, ref refFn, in func(*rand.Rand) *Tensor) gradSpec {
		return gradSpec{name: name, ops: []string{op}, build: func(xs []*Tensor) *Tensor { return f(xs[0]) },
			inputs: genInputs(in), ref: ref}
	}
	nrm := func(shape ...int) func(*rand.Rand) *Tensor { return gen(-1, 1, 0, shape...) }
	dis := genDisjoint
	all := func(f func([]float64) float64) refFn {
		return func(in [][]float64) []float64 { return []float64{f(in[0])} }
	}
	registerGrad("reduce",
		spec("sum eksen 0", "sum", func(x *Tensor) *Tensor { return x.Sum(0, false) }, lane([]int{3, 4}, 0, one(sum64)), nrm(3, 4)),
		spec("sum orta eksen keepdim", "sum", func(x *Tensor) *Tensor { return x.Sum(1, true) }, lane([]int{2, 3, 4}, 1, one(sum64)), nrm(2, 3, 4)),
		spec("sum son eksen", "sum", func(x *Tensor) *Tensor { return x.Sum(-1, false) }, lane([]int{2, 3, 4}, 2, one(sum64)), nrm(2, 3, 4)),
		spec("mean eksen 1", "mean", func(x *Tensor) *Tensor { return x.Mean(1, false) }, lane([]int{2, 3, 4}, 1, one(mean)), nrm(2, 3, 4)),
		spec("mean eksen 0 keepdim", "mean", func(x *Tensor) *Tensor { return x.Mean(0, true) }, lane([]int{3, 2}, 0, one(mean)), nrm(3, 2)),
		spec("sum_all", "sum_all", (*Tensor).SumAll, all(sum64), nrm(2, 3)),
		spec("mean_all", "mean_all", (*Tensor).MeanAll, all(mean), nrm(2, 3)),
		spec("max son eksen", "max", func(x *Tensor) *Tensor { return x.Max(-1, false) }, lane([]int{3, 4}, 1, one(max64)), dis(3, 4)),
		spec("max eksen 0 keepdim", "max", func(x *Tensor) *Tensor { return x.Max(0, true) }, lane([]int{3, 4}, 0, one(max64)), dis(3, 4)),
		spec("max orta eksen", "max", func(x *Tensor) *Tensor { return x.Max(1, false) }, lane([]int{2, 3, 2}, 1, one(max64)), dis(2, 3, 2)),
		spec("softmax son eksen", "softmax", func(x *Tensor) *Tensor { return x.Softmax(-1) }, lane([]int{3, 5}, 1, softmax), nrm(3, 5)),
		spec("softmax eksen 0", "softmax", func(x *Tensor) *Tensor { return x.Softmax(0) }, lane([]int{3, 2, 2}, 0, softmax), nrm(3, 2, 2)),
		spec("log_softmax son eksen", "log_softmax", func(x *Tensor) *Tensor { return x.LogSoftmax(-1) }, lane([]int{3, 5}, 1, logSoftmax), nrm(3, 5)),
		spec("log_softmax orta eksen", "log_softmax", func(x *Tensor) *Tensor { return x.LogSoftmax(1) }, lane([]int{2, 4, 3}, 1, logSoftmax), nrm(2, 4, 3)))
}

func TestGradReduce(t *testing.T) { runGradGroup(t, "reduce") }
