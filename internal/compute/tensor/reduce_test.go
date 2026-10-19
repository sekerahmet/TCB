package tensor

import (
	"fmt"
	"math"
	"testing"
)

// Bulgu 4: maks/min/relu ve maks_indeks NaN'ı yayar (indirgemede NaN varsa sonuç NaN,
// maks_indeks ilk NaN'ın yeri); 16 bitlik türlerde de aynı.
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
