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

// Kural (23.5, 23.7, 23.8): kartın ters yolu (ters taban matrisi, satır seçerek üst üste ekleme) CPU'nun hızlı
// yoluyla aynıdır, türevi de. Sınır girdileri: n = 1, n = 2, tek ve çift n, toplu girdi, tek çerçeve, adım n,
// boyun iki ucu.
func TestInverseFourierCardPathMatchesCPU(t *testing.T) {
	for _, c := range []struct {
		shape []int
		n     int
	}{{[]int{1}, 1}, {[]int{2}, 2}, {[]int{5}, 8}, {[]int{2, 3, 7}, 13}, {[]int{2, 7}, 12}, {[]int{2, 201}, 400}} {
		rd, id := noise(numel(c.shape)), noise(numel(c.shape) + 3)[3:]
		ad := fmt.Sprintf("irfft %v n %d", c.shape, c.n)
		r1, i1 := New(rd, c.shape...).SetRequiresGrad(true), New(id, c.shape...).SetRequiresGrad(true)
		r2, i2 := New(rd, c.shape...).SetRequiresGrad(true), New(id, c.shape...).SetRequiresGrad(true)
		y1, y2 := Irfft(r1, i1, c.n, testCall), irfftOnCard(r2, i2, c.n, testTrig)
		if !slices.Equal(y1.Shape(), y2.Shape()) {
			t.Fatalf("%s: şekil %v, beklenen %v", ad, y2.Shape(), y1.Shape())
		}
		scaledClose(t, ad, y2.Data(), y1.Data(), 1e-5)
		Mul(y1, y1).SumAll().Backward()
		Mul(y2, y2).SumAll().Backward()
		scaledClose(t, ad+" gerçek türevi", r2.Grad().Data(), r1.Grad().Data(), 1e-5)
		scaledClose(t, ad+" sanal türevi", i2.Grad().Data(), i1.Grad().Data(), 1e-5)
	}
	for _, c := range []struct {
		rows, frames int
		o            IstftOptions
	}{
		{2, 8, IstftOptions{N: 8, Hop: 2, Center: true, Length: 14}},
		{2, 8, IstftOptions{N: 8, Hop: 2, Center: true, Length: 15}},
		{1, 6, IstftOptions{N: 9, Hop: 4, Center: true, Length: 21}},
		{1, 4, IstftOptions{N: 8, Hop: 3, Length: 17}},
		{1, 1, IstftOptions{N: 8, Hop: 5, Center: true, Length: 4}}, // tek çerçeve
		{3, 2, IstftOptions{N: 5, Hop: 5, Length: 10}},              // adım n
		{1, 3, IstftOptions{N: 1, Hop: 1, Center: true, Length: 3}},
	} {
		F := c.o.N/2 + 1
		size := c.rows * F * c.frames
		rd, id, wd := noise(size), noise(size + 5)[5:], noise(c.o.N)
		for k := range wd {
			wd[k] = 0.75 + wd[k]/4
		}
		ad := fmt.Sprintf("istft %d×%d %+v", c.rows, c.frames, c.o)
		var out [2]*Tensor
		var in [2][3]*Tensor
		for k := range 2 {
			in[k] = [3]*Tensor{New(rd, c.rows, F, c.frames).SetRequiresGrad(true), New(id, c.rows, F, c.frames).SetRequiresGrad(true), New(wd, c.o.N).SetRequiresGrad(true)}
		}
		out[0] = Istft(in[0][0], in[0][1], in[0][2], c.o, testCall)
		out[1] = istftOnCard(in[1][0], in[1][1], in[1][2], c.o, testCall)
		if !slices.Equal(out[0].Shape(), out[1].Shape()) || !slices.Equal(out[0].Shape(), []int{c.rows, c.o.Length}) {
			t.Fatalf("%s: şekil %v, beklenen %v", ad, out[1].Shape(), out[0].Shape())
		}
		scaledClose(t, ad, out[1].Data(), out[0].Data(), 1e-5)
		for _, y := range out {
			Mul(y, y).SumAll().Backward()
		}
		for k, part := range []string{" gerçek türevi", " sanal türevi", " pencere türevi"} {
			scaledClose(t, ad+part, in[1][k].Grad().Data(), in[0][k].Grad().Data(), 1e-5)
		}
	}
}

// Kural (23.7, 23.8): sıfırıncı ve çift n'de son kutunun sanal kısmı kartın yolunda da sonuca girmez: NaN ve
// sonsuz olsa da sonuç temiz izgenin bitleridir, o kutuların türevi 0'dır (gelen gradyan NaN olsa da).
func TestInverseFourierCardPathDropsUnusedBins(t *testing.T) {
	nan, inf := float32(math.NaN()), float32(math.Inf(1))
	for _, n := range []int{1, 2, 3, 5, 8, 13, 16} {
		F, frames := n/2+1, 3
		o := IstftOptions{N: n, Hop: 1, Center: true, Length: 2 + n%2}
		rd, clean := noise(F*frames), noise(F*frames + 7)[7:]
		dirty := slices.Clone(clean)
		for f := range frames {
			dirty[f] = []float32{nan, inf, -inf}[f]
			if n%2 == 0 {
				dirty[(F-1)*frames+f] = []float32{-inf, nan, inf}[f]
			}
		}
		for _, back := range []float32{1, nan} {
			run := func(id []float32) (*Tensor, []*Tensor) {
				re, im := New(rd, F, frames).SetRequiresGrad(true), New(id, F, frames).SetRequiresGrad(true)
				w := New(slices.Repeat([]float32{1}, n), n)
				y := istftOnCard(re, im, w, o, testCall)
				r2, i2 := New(transposed(rd, F, frames), frames, F).SetRequiresGrad(true), New(transposed(id, F, frames), frames, F).SetRequiresGrad(true)
				y2 := irfftOnCard(r2, i2, n, testTrig)
				Add(y.MulScalar(back).SumAll(), y2.MulScalar(back).SumAll()).Backward()
				return Concat([]*Tensor{y.Detach().Reshape(o.Length), y2.Detach().Reshape(frames * n)}, 0), []*Tensor{im, i2}
			}
			want, _ := run(clean)
			got, ims := run(dirty)
			if !slices.Equal(floatBits(got), floatBits(want)) {
				t.Fatalf("n=%d: NaN ve sonsuz sanal kutular sonuca girdi: %v, beklenen %v", n, got.Data(), want.Data())
			}
			gi, g2 := ims[0].Grad().Data(), ims[1].Grad().Data()
			for f := range frames {
				for _, v := range []float32{gi[f], gi[(F-1)*frames+f], g2[f*F], g2[f*F+F-1]} {
					if v != 0 && n%2 == 0 {
						t.Fatalf("n=%d: kullanılmayan kutunun türevi %v, beklenen 0", n, v)
					}
				}
				if gi[f] != 0 || g2[f*F] != 0 {
					t.Fatalf("n=%d: sıfırıncı kutunun türevi %v, %v, beklenen 0", n, gi[f], g2[f*F])
				}
			}
		}
	}
}

// transposed [a, b] verinin [b, a] devriğidir.
func transposed(d []float32, a, b int) []float32 {
	out := make([]float32, len(d))
	for i := range a {
		for j := range b {
			out[j*a+i] = d[i*b+j]
		}
	}
	return out
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
