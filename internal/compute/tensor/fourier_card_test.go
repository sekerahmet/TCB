package tensor

import (
	"fmt"
	"math"
	"slices"
	"testing"
)

// Kural (23.1, 23.3, 23.5): kartın yolu (taban matrisli DFT; satır seçme, pencere, MatMul) tanımın
// toplamını verir: aynı girdide CPU'nun hızlı dönüşümüyle 15.41'in 3. basamağında aynı, türevi de.
// Kart yolu işlemcide de koşar: matematiği burada sınanır, kartta koşusu gpu paketinde
// (behavior_fourier_test.go). Sınır girdileri: n = 1, tek çerçeve, yansıtmada sinyal n/2 + 1,
// kısa sinyal (sıfır dolgu), toplu girdi.

// scaledClose a ile b'nin |a−b| ≤ tol·(1 + en büyük |b|) olmasını ister.
func scaledClose(t *testing.T, ad string, a, b []float32, tol float64) {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("%s: uzunluk %d, beklenen %d", ad, len(a), len(b))
	}
	scale := 0.0
	for _, v := range b {
		scale = math.Max(scale, math.Abs(float64(v)))
	}
	for i := range a {
		if d := math.Abs(float64(a[i]) - float64(b[i])); d > tol*(1+scale) {
			t.Fatalf("%s[%d]: %v, beklenen %v", ad, i, a[i], b[i])
		}
	}
}

func TestFourierCardPathMatchesCPU(t *testing.T) {
	for _, c := range []struct {
		shape []int
		n     int
	}{{[]int{1}, 1}, {[]int{5}, 8}, {[]int{2, 3, 7}, 12}, {[]int{2, 400}, 400}} {
		data := noise(numel(c.shape))
		ad := fmt.Sprintf("rfft %v n %d", c.shape, c.n)
		x1, x2 := New(data, c.shape...).SetRequiresGrad(true), New(data, c.shape...).SetRequiresGrad(true)
		r1, i1 := Rfft(x1, c.n, testCall)
		r2, i2 := rfftOnCard(x2, c.n, testTrig)
		scaledClose(t, ad+" gerçek", r2.Data(), r1.Data(), 1e-5)
		scaledClose(t, ad+" sanal", i2.Data(), i1.Data(), 1e-5)
		Add(r1.SumAll(), Mul(i1, i1).SumAll()).Backward()
		Add(r2.SumAll(), Mul(i2, i2).SumAll()).Backward()
		scaledClose(t, ad+" türev", x2.Grad().Data(), x1.Grad().Data(), 1e-4)
	}
	for _, c := range []struct {
		rows, signal int
		o            StftOptions
	}{
		{2, 50, StftOptions{N: 8, Hop: 3, Center: true, Reflect: true}},
		{1, 50, StftOptions{N: 8, Hop: 3, Center: true}},
		{1, 20, StftOptions{N: 8, Hop: 5}},
		{1, 5, StftOptions{N: 8, Hop: 2, Center: true, Reflect: true}}, // sinyal n/2 + 1
		{1, 6, StftOptions{N: 1, Hop: 1}},
		{1, 8, StftOptions{N: 8, Hop: 4}}, // tek çerçeve
	} {
		data := noise(c.rows * c.signal)
		w := noise(c.o.N)
		ad := fmt.Sprintf("stft %d×%d %+v", c.rows, c.signal, c.o)
		x1, x2 := New(data, c.rows, c.signal).SetRequiresGrad(true), New(data, c.rows, c.signal).SetRequiresGrad(true)
		w1, w2 := New(w, c.o.N).SetRequiresGrad(true), New(w, c.o.N).SetRequiresGrad(true)
		r1, i1 := Stft(x1, w1, c.o, testCall)
		r2, i2 := stftOnCard(x2, w2, c.o, testTrig)
		if !slices.Equal(r1.Shape(), r2.Shape()) {
			t.Fatalf("%s: şekil %v, beklenen %v", ad, r2.Shape(), r1.Shape())
		}
		scaledClose(t, ad+" gerçek", r2.Data(), r1.Data(), 1e-5)
		scaledClose(t, ad+" sanal", i2.Data(), i1.Data(), 1e-5)
		Add(Mul(r1, r1).SumAll(), i1.SumAll()).Backward()
		Add(Mul(r2, r2).SumAll(), i2.SumAll()).Backward()
		scaledClose(t, ad+" x türevi", x2.Grad().Data(), x1.Grad().Data(), 1e-4)
		scaledClose(t, ad+" pencere türevi", w2.Grad().Data(), w1.Grad().Data(), 1e-4)
	}
}

// numel şeklin öğe sayısıdır.
func numel(shape []int) int {
	n := 1
	for _, d := range shape {
		n *= d
	}
	return n
}

// Önbellek en çok fourierCacheBytes taban tutar; dolunca en eski taban bırakılır, kullanılan
// taban yeniden kurulmaz (G6.2).
func TestFourierBasisCacheEvictsOldest(t *testing.T) {
	defer func(old int) { fourierCacheBytes = old }(fourierCacheBytes)
	fourierBases.Lock()
	fourierBases.m, fourierBases.order, fourierBases.bytes = map[basisKey]*Tensor{}, nil, 0
	fourierBases.Unlock()
	size := func(n int) int { return 4 * n * 2 * (n/2 + 1) }
	fourierCacheBytes = size(64) + size(32)
	a := fourierBasis(64, CPU, testTrig)
	fourierBasis(32, CPU, testTrig)
	if fourierBasis(64, CPU, testTrig) != a {
		t.Fatal("aynı taban yeniden kuruldu")
	}
	fourierBasis(16, CPU, testTrig) // 64 en eskisidir, bırakılır
	var ns []int
	for _, k := range fourierBases.order {
		ns = append(ns, k.n)
	}
	if !slices.Equal(ns, []int{32, 16}) || fourierBases.bytes != size(32)+size(16) {
		t.Fatalf("önbellek %v (%d bayt), beklenen [32 16]", ns, fourierBases.bytes)
	}
	if b := fourierBasis(4, CPU, testTrig); b.RequiresGrad() || !slices.Equal(b.Shape(), []int{4, 6}) {
		t.Fatalf("taban türev izlememeli, şekli [4 6]: %v", b.Shape())
	}
}
