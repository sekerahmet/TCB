package tensor

import (
	"math"
	"math/rand"
	"testing"
)

// ulpDiff iki float32 arasındaki ulp uzaklığı (aynı işaretli sonlu değerler).
func ulpDiff(a, b float32) int64 {
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
	check := func(x float32, maxN int64) {
		t.Helper()
		got, want := expf32(x), float32(math.Exp(float64(x)))
		if math.IsNaN(float64(want)) {
			if !math.IsNaN(float64(got)) {
				t.Fatalf("expf32(%v) = %v, NaN beklenen", x, got)
			}
			return
		}
		if d := ulpDiff(got, want); d > maxN {
			t.Fatalf("expf32(%v) = %v, beklenen %v (%d ulp)", x, got, want, d)
		}
	}
	for i := 0; i < 2000000; i++ {
		check(float32(rng.Float64()*200-100), 2)
	}
	for x := float32(-20); x < 20; x += 0.001 {
		check(x, 2)
	}
	// Uçlar: taşma sınırı, normal altı sonuçlar, alt taşma, sonsuzlar, NaN.
	for _, x := range []float32{0, -0.0, 1e-8, -1e-8, 1, -1, 88.7, 88.72283, 88.7229, 89, 1000,
		-87.3, -87.33654, -87.34, -95, -103.9, -103.97, -104, -1000,
		float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.NaN())} {
		check(x, 2)
	}
	if expf32(89) != float32(math.Inf(1)) || expf32(-200) != 0 || expf32(float32(math.Inf(-1))) != 0 {
		t.Fatal("expf32 uç değerleri yanlış")
	}
}

func TestTanhf32(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	check := func(x float32) {
		t.Helper()
		got, want := tanhf32(x), float32(math.Tanh(float64(x)))
		if math.IsNaN(float64(want)) {
			if !math.IsNaN(float64(got)) {
				t.Fatalf("tanhf32(%v) = %v, NaN beklenen", x, got)
			}
			return
		}
		if d := ulpDiff(got, want); d > 2 {
			t.Fatalf("tanhf32(%v) = %v, beklenen %v (%d ulp)", x, got, want, d)
		}
	}
	for i := 0; i < 2000000; i++ {
		check(float32(rng.NormFloat64() * 3))
	}
	for x := float32(-10); x < 10; x += 0.0007 {
		check(x)
	}
	for _, x := range []float32{0, 1e-30, -1e-30, 1e-4, 0.000244, 0.000245, 0.624999, 0.625, 0.625001, -0.625,
		8.9, 9.0, 9.01, 9.02, 20, -20, 1e30, float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.NaN())} {
		check(x)
	}
	if math.Signbit(float64(tanhf32(float32(math.Copysign(0, -1))))) != true {
		t.Fatal("tanhf32(-0) işareti korunmalı")
	}
}

// expVek (GOAMD64=v3'te AVX2 çekirdeği) skalerle aynı doğrulukta olmalı; normal
// altı sonuçlar, taşma, -sonsuz ve NaN dahil.
func TestExpVec(t *testing.T) {
	rng := rand.New(rand.NewSource(43))
	var xs []float32
	for i := 0; i < 1<<20; i++ {
		xs = append(xs, float32(rng.Float64()*220-115))
	}
	xs = append(xs, 0, -0.0, 88.7, 88.72283, 88.7229, 89, 1000, -87.33654, -87.34, -95, -100, -103.9, -103.97,
		-104, -150, float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.NaN()), 1e-8, -1e-8)
	for k := 0; k < 9; k++ { // farklı kuyruk uzunlukları
		xs = append(xs, float32(k)-4)
		ys := make([]float32, len(xs))
		expVec(ys, xs)
		for i, x := range xs {
			want := float32(math.Exp(float64(x)))
			if math.IsNaN(float64(want)) {
				if !math.IsNaN(float64(ys[i])) {
					t.Fatalf("expVek(%v) = %v, NaN beklenen", x, ys[i])
				}
				continue
			}
			if d := ulpDiff(ys[i], want); d > 2 {
				t.Fatalf("%s expVek(%v) = %v, beklenen %v (%d ulp)", microKernelName, x, ys[i], want, d)
			}
		}
	}
	// Yerinde kullanım.
	z := []float32{0, 1, 2, 3, 4, 5, 6, 7, 8}
	expVec(z, z)
	if ulpDiff(z[8], float32(math.Exp(8))) > 2 || z[0] != 1 {
		t.Fatalf("yerinde expVek %v", z)
	}
}

// Softmax ve çapraz entropi uç girdilerle sonlu ve doğru kalmalı.
func TestSoftmaxExtremeValues(t *testing.T) {
	x := New([]float32{1000, 0, -1000, float32(math.Inf(-1)), 88, 89}, 1, 6)
	s := x.Softmax(-1).Data()
	var totalSum float64
	for _, v := range s {
		if math.IsNaN(float64(v)) {
			t.Fatalf("softmax NaN: %v", s)
		}
		totalSum += float64(v)
	}
	if math.Abs(totalSum-1) > 1e-6 || s[0] != 1 {
		t.Fatalf("softmax %v", s)
	}
	l := CrossEntropy(New([]float32{50, -50, 0}, 1, 3), []int{1}).Item()
	if math.Abs(float64(l)-100) > 1e-4 {
		t.Fatalf("çapraz entropi %v, beklenen ~100", l)
	}
}

func BenchmarkExp(b *testing.B) {
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

// Bulgu 11: üstelin sonucu elemanın konumuna (8'li gövde ya da kuyruk, blok sınırı) ve
// softmax'ın eksenine bağlı değildir.
func TestExpPositionIndependent(t *testing.T) {
	rng := rand.New(rand.NewSource(13))
	for _, n := range []int{1, 7, 9, 15, 17, 255, 257, 300, 1000} {
		x := make([]float32, n)
		for i := range x {
			x[i] = float32(rng.NormFloat64() * 30)
		}
		y := make([]float32, n)
		expVec(y, x)
		for i := range x {
			var single [1]float32
			expVec(single[:], x[i:i+1])
			if y[i] != single[0] && !(y[i] != y[i] && single[0] != single[0]) {
				t.Fatalf("n=%d: e^x[%d] gövdede %v, tek başına %v", n, i, y[i], single[0])
			}
		}
	}
	// Eşit elemanlı tensörde (hakem kanıtı: 9 eleman, e[0] != e[8]) hepsi aynı.
	e := Full(-48.528, 9).Exp().Data()
	for i := range e {
		if e[i] != e[0] {
			t.Fatalf("üstel(t)[%d] = %v, [0] = %v", i, e[i], e[0])
		}
	}
	// Softmax son eksende ve diğer eksende aynı değerleri verir.
	x := Randn(rng, 37, 11).MulScalar(5)
	for _, logMode := range []bool{false, true} {
		var a, b *Tensor
		if logMode {
			a, b = x.LogSoftmax(1), x.T().Contiguous().LogSoftmax(0).T()
		} else {
			a, b = x.Softmax(1), x.T().Contiguous().Softmax(0).T()
		}
		ad, bd := a.Data(), b.Data()
		for i := range ad {
			if ad[i] != bd[i] {
				t.Fatalf("log=%v: softmax son eksen %v, eksen 0 %v ([%d])", logMode, ad[i], bd[i], i)
			}
		}
	}
}
