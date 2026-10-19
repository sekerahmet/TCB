package tensor

import (
	"math"
	"testing"
)

// Bulgu 12: GELU'nun çok büyük |x|'te ve x**0'ın x=0'da gradyanı NaN değildir.
func TestGeluAndZeroPowerGrad(t *testing.T) {
	x := New([]float32{1e20, -1e20, 3e38, -3e38, 0, 2}, 6).SetRequiresGrad(true)
	x.Gelu().SumAll().Backward()
	near(t, "GELU' uçlarda", x.Grad().Data()[:4], []float32{1, 0, 1, 0}, 0)
	z := New([]float32{0, 2, -1}, 3).SetRequiresGrad(true)
	z.Pow(0).SumAll().Backward()
	near(t, "d/dx x^0", z.Grad().Data(), []float32{0, 0, 0}, 0)
	near(t, "x^0", z.Detach().Pow(0).Data(), []float32{1, 1, 1}, 0)
}

// Bulgu 5: skalere bölme tensörün türünü korur ve gerçek bölmedir (float32'de bölünüp bir
// kez yuvarlanır).
func TestDivScalarKeepsDType(t *testing.T) {
	x := New([]float32{1, 2, 3}, 3)
	for _, dt := range []DType{Float32, BFloat16, Float16} {
		y := x.Cast(dt).DivScalar(3)
		if y.DType() != dt {
			t.Fatalf("%s / 3 türü %s", dt, y.DType())
		}
		near(t, dt.String()+" / 3", y.Data(), Div(x, Scalar(3)).Cast(dt).Data(), 0)
	}
	// 1/s ile çarpmadan farklı olarak tam bölme: 3/3 = 1.
	near(t, "3/3", New([]float32{3}, 1).DivScalar(3).Data(), []float32{1}, 0)
	checkGrad(t, "DivScalar gradyanı", func(xs []*Tensor) *Tensor { return xs[0].DivScalar(4) }, New([]float32{1, -2, 3}, 3))
}

// İkili işlemlerin aynı şekilli girdilerde doğrudan yazılan (sıfırlamasız) ve devredilen
// gradyanları: aynı tensör iki işlenen olduğunda, birikimde ve karışık türde de doğru.
func TestBinaryGradSameShape(t *testing.T) {
	x := New([]float32{1, -2, 3}, 3).SetRequiresGrad(true)
	w := New([]float32{2, 4, -1}, 3).SetRequiresGrad(true)
	cases := []struct {
		name   string
		f      func() *Tensor
		gx, gw []float32
	}{
		{"x+x", func() *Tensor { return x.Add(x) }, []float32{2, 2, 2}, nil},
		{"x-x", func() *Tensor { return x.Sub(x) }, []float32{0, 0, 0}, nil},
		{"x*x", func() *Tensor { return x.Mul(x) }, []float32{2, -4, 6}, nil},
		{"x-w", func() *Tensor { return x.Sub(w) }, []float32{1, 1, 1}, []float32{-1, -1, -1}},
		{"x*w", func() *Tensor { return x.Mul(w) }, []float32{2, 4, -1}, []float32{1, -2, 3}},
		{"x/w", func() *Tensor { return x.Div(w) }, []float32{0.5, 0.25, -1}, []float32{-0.25, 0.125, -3}},
		{"(x+w)*x", func() *Tensor { return x.Add(w).Mul(x) }, []float32{4, 0, 5}, []float32{1, -2, 3}},
	}
	for _, c := range cases {
		x.grad, w.grad = nil, nil
		c.f().SumAll().Backward()
		near(t, c.name+" dx", x.Grad().Data(), c.gx, 1e-6)
		if c.gw != nil {
			near(t, c.name+" dw", w.Grad().Data(), c.gw, 1e-6)
		}
	}
	// Yaprakta birikim: ikinci geri yayılım üzerine ekler.
	x.grad = nil
	x.Mul(w).SumAll().Backward()
	x.Mul(w).SumAll().Backward()
	near(t, "birikim", x.Grad().Data(), []float32{4, 8, -2}, 0)

	// Karışık tür: bf16 + float32 sonucu float32; bf16 ara sonuca gradyan kendi türünde gelir.
	h := x.Cast(BFloat16).Mul(x.Cast(BFloat16))
	x.grad = nil
	h.Add(w).SumAll().Backward()
	near(t, "karışık tür", x.Grad().Data(), []float32{2, -4, 6}, 0)

	// Çarpımın sıfır işareti korunur: -0 · 1 = -0.
	z := New([]float32{1}, 1).SetRequiresGrad(true)
	z.Mul(New([]float32{float32(math.Copysign(0, -1))}, 1)).SumAll().Backward()
	if g := z.Grad().Data()[0]; !math.Signbit(float64(g)) {
		t.Fatalf("-0 çarpanın gradyanı %v, -0 bekleniyordu", g)
	}
}
