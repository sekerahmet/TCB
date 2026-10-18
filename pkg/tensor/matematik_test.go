package tensor

import (
	"math"
	"math/rand"
	"testing"
)

// ulpFark iki float32 arasındaki ulp uzaklığı (aynı işaretli sonlu değerler).
func ulpFark(a, b float32) int64 {
	if a == b {
		return 0
	}
	ia, ib := int64(math.Float32bits(a)), int64(math.Float32bits(b))
	if (ia < 0x80000000) != (ib < 0x80000000) {
		return math.MaxInt32
	}
	d := ia - ib
	if d < 0 {
		d = -d
	}
	return d
}

func TestExpf32(t *testing.T) {
	rng := rand.New(rand.NewSource(41))
	denetle := func(x float32, enCok int64) {
		t.Helper()
		got, want := expf32(x), float32(math.Exp(float64(x)))
		if math.IsNaN(float64(want)) {
			if !math.IsNaN(float64(got)) {
				t.Fatalf("expf32(%v) = %v, NaN beklenen", x, got)
			}
			return
		}
		if d := ulpFark(got, want); d > enCok {
			t.Fatalf("expf32(%v) = %v, beklenen %v (%d ulp)", x, got, want, d)
		}
	}
	for i := 0; i < 2000000; i++ {
		denetle(float32(rng.Float64()*200-100), 2)
	}
	for x := float32(-20); x < 20; x += 0.001 {
		denetle(x, 2)
	}
	// Uçlar: taşma sınırı, normal altı sonuçlar, alt taşma, sonsuzlar, NaN.
	for _, x := range []float32{0, -0.0, 1e-8, -1e-8, 1, -1, 88.7, 88.72283, 88.7229, 89, 1000,
		-87.3, -87.33654, -87.34, -95, -103.9, -103.97, -104, -1000,
		float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.NaN())} {
		denetle(x, 2)
	}
	if expf32(89) != float32(math.Inf(1)) || expf32(-200) != 0 || expf32(float32(math.Inf(-1))) != 0 {
		t.Fatal("expf32 uç değerleri yanlış")
	}
}

func TestTanhf32(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	denetle := func(x float32) {
		t.Helper()
		got, want := tanhf32(x), float32(math.Tanh(float64(x)))
		if math.IsNaN(float64(want)) {
			if !math.IsNaN(float64(got)) {
				t.Fatalf("tanhf32(%v) = %v, NaN beklenen", x, got)
			}
			return
		}
		if d := ulpFark(got, want); d > 2 {
			t.Fatalf("tanhf32(%v) = %v, beklenen %v (%d ulp)", x, got, want, d)
		}
	}
	for i := 0; i < 2000000; i++ {
		denetle(float32(rng.NormFloat64() * 3))
	}
	for x := float32(-10); x < 10; x += 0.0007 {
		denetle(x)
	}
	for _, x := range []float32{0, 1e-30, -1e-30, 1e-4, 0.000244, 0.000245, 0.624999, 0.625, 0.625001, -0.625,
		8.9, 9.0, 9.01, 9.02, 20, -20, 1e30, float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.NaN())} {
		denetle(x)
	}
	if math.Signbit(float64(tanhf32(float32(math.Copysign(0, -1))))) != true {
		t.Fatal("tanhf32(-0) işareti korunmalı")
	}
}

// Softmax ve çapraz entropi uç girdilerle sonlu ve doğru kalmalı.
func TestSoftmaxUcDegerler(t *testing.T) {
	x := New([]float32{1000, 0, -1000, float32(math.Inf(-1)), 88, 89}, 1, 6)
	s := x.Softmax(-1).Data()
	var top float64
	for _, v := range s {
		if math.IsNaN(float64(v)) {
			t.Fatalf("softmax NaN: %v", s)
		}
		top += float64(v)
	}
	if math.Abs(top-1) > 1e-6 || s[0] != 1 {
		t.Fatalf("softmax %v", s)
	}
	l := CrossEntropy(New([]float32{50, -50, 0}, 1, 3), []int{1}).Item()
	if math.Abs(float64(l)-100) > 1e-4 {
		t.Fatalf("çapraz entropi %v, beklenen ~100", l)
	}
}

func BenchmarkUstel(b *testing.B) {
	xs := make([]float32, 4096)
	for i := range xs {
		xs[i] = float32(i)/512 - 4
	}
	ys := make([]float32, len(xs))
	b.Run("expf32", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for j, x := range xs {
				ys[j] = expf32(x)
			}
		}
	})
	b.Run("math.Exp", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for j, x := range xs {
				ys[j] = float32(math.Exp(float64(x)))
			}
		}
	})
	b.Run("tanhf32", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for j, x := range xs {
				ys[j] = tanhf32(x)
			}
		}
	})
	b.Run("math.Tanh", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for j, x := range xs {
				ys[j] = float32(math.Tanh(float64(x)))
			}
		}
	})
}
