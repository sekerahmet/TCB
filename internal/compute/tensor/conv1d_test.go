package tensor

import (
	"math"
	"runtime"
	"slices"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Sonuç ve girdinin gradyanı işçi sayısına bağlı değildir (15.22a).
func TestConv1dSameBitsForAnyWorkerCount(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(0))
	o := Conv1dOptions{Stride: 2, Padding: 3, Dilation: 2, Groups: 2}
	once := func() ([]uint32, []uint32) {
		x := New(noise(2*8*200), 2, 8, 200).SetRequiresGrad(true)
		w := New(noise(6*4*5), 6, 4, 5)
		y := Conv1d(x, w, nil, o, MatMul)
		y.Sum(2, false).SumAll().Backward()
		return floatBits(y), floatBits(x.Grad())
	}
	runtime.GOMAXPROCS(1)
	wantY, wantG := once()
	for _, n := range []int{2, 3, 8} {
		runtime.GOMAXPROCS(n)
		y, g := once()
		if !slices.Equal(y, wantY) || !slices.Equal(g, wantG) {
			t.Fatalf("GOMAXPROCS=%d sonucu seri hesaptan farklı", n)
		}
	}
}

// Kural (15.41): evrişimin açması ve katlaması tensörün cihazındadır; Conv1dFolder'ı sağlamayan
// işlemci dışı bir arka uçta (fakeGPU) iç hatadır, sessizce işlemcide kurulmaz.
func TestConv1dWithoutFolderOffTheCPUIsInternal(t *testing.T) {
	x, w := New(noise(1*2*6), 1, 2, 6), New(noise(1*2*3), 1, 2, 3)
	x.be, w.be = fakeGPU{}, fakeGPU{}
	panicCode(t, "açma", messages.H9000, func() { Conv1d(x, w, nil, Conv1dOptions{Stride: 1, Dilation: 1, Groups: 1}, MatMul) })
	f := foldShape{batch: 1, cin: 2, l: 6, k: 3, frames: 4, o: Conv1dOptions{Stride: 1, Dilation: 1, Groups: 1}}
	panicCode(t, "katlama", messages.H9000, func() { foldOn(fakeGPU{}, New(noise(1*6*4), 1, 6, 4), f) })
}

// Dolgu sıfırdır; çıkış uzunluğu PyTorch'un formülüdür.
func TestConv1dLength(t *testing.T) {
	for _, c := range []struct {
		o          Conv1dOptions
		l, k, want int
	}{
		{Conv1dOptions{Stride: 1, Dilation: 1}, 5, 3, 3},
		{Conv1dOptions{Stride: 2, Padding: 1, Dilation: 1}, 3000, 3, 1500},
		{Conv1dOptions{Stride: 2, Padding: 2, Dilation: 2}, 9, 3, 5},
		{Conv1dOptions{Stride: 5, Dilation: 4}, 9, 2, 1},
	} {
		if got := c.o.Length(c.l, c.k); got != c.want {
			t.Errorf("%+v l=%d k=%d: %d, beklenen %d", c.o, c.l, c.k, got, c.want)
		}
	}
}

// openRowsRef openRows'un iç konum ayrımından önceki hâlidir (her öğe sınır denetimli).
func openRowsRef(src []float32, f foldShape) []float32 {
	cg, width := f.cg(), f.cg()*f.k
	rows := f.o.Groups * f.batch * f.frames
	cols := make([]float32, rows*width)
	for u := range rows {
		gi, rest := u/(f.batch*f.frames), u%(f.batch*f.frames)
		b, start := rest/f.frames, rest%f.frames*f.o.Stride-f.o.Padding
		for c := range cg {
			row := src[(b*f.cin+gi*cg+c)*f.l : (b*f.cin+gi*cg+c+1)*f.l]
			for j := range f.k {
				if p := start + j*f.o.Dilation; f.inside(p) {
					cols[u*width+c*f.k+j] = row[p]
				}
			}
		}
	}
	return cols
}

// openRows iç konumları denetimsiz kopyalar; sonuç öğe öğe denetimli yazımla bit bit aynıdır: adım,
// genişleme, dolgu (çekirdekten geniş dolgu), grup, toplu, çekirdekten kısa girdi, tek çıkış ve özel
// değerler (NaN, sonsuz, eksi sıfır kopyalanır, dönüştürülmez).
func TestOpenRowsInteriorMatchesCheckedRows(t *testing.T) {
	for _, c := range []struct {
		o                Conv1dOptions
		batch, cin, l, k int
	}{
		{Conv1dOptions{Stride: 1, Padding: 0, Dilation: 1, Groups: 1}, 1, 3, 20, 3},
		{Conv1dOptions{Stride: 1, Padding: 5, Dilation: 1, Groups: 1}, 1, 4, 40, 11},
		{Conv1dOptions{Stride: 1, Padding: 15, Dilation: 3, Groups: 1}, 2, 4, 50, 11},
		{Conv1dOptions{Stride: 2, Padding: 2, Dilation: 2, Groups: 2}, 2, 4, 9, 3},
		{Conv1dOptions{Stride: 3, Padding: 1, Dilation: 1, Groups: 4}, 3, 8, 41, 4},
		{Conv1dOptions{Stride: 1, Padding: 6, Dilation: 1, Groups: 1}, 1, 2, 4, 11},
		{Conv1dOptions{Stride: 1, Padding: 0, Dilation: 1, Groups: 1}, 1, 2, 7, 7},
		{Conv1dOptions{Stride: 1, Padding: 9, Dilation: 2, Groups: 1}, 1, 1, 3, 5},
	} {
		f := foldShape{batch: c.batch, cin: c.cin, l: c.l, k: c.k, o: c.o}
		f.frames = c.o.Length(c.l, c.k)
		src := noise(c.batch * c.cin * c.l)
		special := []float32{float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.Copysign(0, -1))}
		for i := range special {
			if i*7 < len(src) {
				src[i*7] = special[i]
			}
		}
		got, want := openRows(src, f), openRowsRef(src, f)
		if len(got) != len(want) {
			t.Fatalf("%+v: %d öğe, beklenen %d", c, len(got), len(want))
		}
		for i := range got {
			if math.Float32bits(got[i]) != math.Float32bits(want[i]) {
				t.Fatalf("%+v: %d. öğe %v, denetimli yazımda %v", c, i, got[i], want[i])
			}
		}
	}
}

// conv1dCase işlemcinin türevsiz çekirdeğinin bir sınamasıdır.
type conv1dCase struct {
	o                      Conv1dOptions
	batch, cin, cout, l, k int
	bias                   bool
}

// operands durumun girdisini (özel değerlerle), ağırlığını ve sapmasını kurar.
func (c conv1dCase) operands() (x, w, b *Tensor) {
	src := noise(c.batch * c.cin * c.l)
	special := []float32{float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.Copysign(0, -1))}
	for i := range special {
		if j := i*97 + 5; j < len(src) {
			src[j] = special[i]
		}
	}
	x = New(src, c.batch, c.cin, c.l)
	w = New(noise(c.cout*c.cin/c.o.Groups*c.k), c.cout, c.cin/c.o.Groups, c.k)
	if c.bias {
		b = New(noise(c.cout), c.cout)
	}
	return x, w, b
}

// Kural (15.22a): işlemcinin türevsiz çekirdeği (Conv1dKernel) eski yolla (Conv1d: açma, çarpım,
// devrik kopya, sapma toplaması) bit bit aynıdır: adım, genişleme, çekirdekten geniş dolgu, grup,
// toplu, sapmalı ve sapmasız, 16'nın ve 128'in katı olmayan çıkış, birden çok K bloğu (Cin / g · K >
// 256), Cout 1 ve 6'nın katı olmayan Cout, çıkış boyu 1 (küçük iç boyut), boş çıkış, boş iç boyut
// (Cin 0: çıkış yalnız sapma); girdide NaN, sonsuz ve eksi sıfır.
func TestConv1dKernelMatchesOldPathBits(t *testing.T) {
	one := Conv1dOptions{Stride: 1, Dilation: 1, Groups: 1}
	for _, c := range []conv1dCase{
		{one, 1, 3, 4, 20, 3, true},
		{Conv1dOptions{Stride: 1, Padding: 3, Dilation: 1, Groups: 1}, 1, 64, 64, 300, 7, true},
		{Conv1dOptions{Stride: 1, Padding: 12, Dilation: 6, Groups: 1}, 1, 32, 7, 1000, 5, false},
		{Conv1dOptions{Stride: 2, Padding: 9, Dilation: 2, Groups: 2}, 2, 8, 6, 77, 5, true},
		{Conv1dOptions{Stride: 3, Padding: 1, Dilation: 1, Groups: 4}, 3, 8, 12, 41, 4, true},
		{Conv1dOptions{Stride: 1, Padding: 3, Dilation: 1, Groups: 1}, 1, 32, 1, 2000, 7, false},
		{one, 2, 192, 13, 129, 7, true},
		{one, 1, 2, 3, 3, 3, true},
		{one, 0, 2, 3, 10, 3, true},
		{one, 2, 0, 3, 10, 3, true},
	} {
		x, w, b := c.operands()
		if !Conv1dDirectOK(x, w, b, c.o) {
			t.Fatalf("%+v: çekirdeğin kapsamında değil", c)
		}
		want, got := floatBits(Conv1d(x, w, b, c.o, MatMul)), floatBits(Conv1dDirect(x, w, b, c.o))
		if len(got) != len(want) {
			t.Fatalf("%+v: %d öğe, eski yolda %d", c, len(got), len(want))
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("%+v: %d. öğe %08x, eski yolda %08x", c, i, got[i], want[i])
			}
		}
	}
}

// Kural (15.22a): çekirdeğin sonucu işçi sayısına bağlı değildir (satır blokları işçi sayısıyla
// bölünür, bir öğenin K toplamı tek işin içindedir).
func TestConv1dKernelSameBitsForAnyWorkerCount(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(0))
	c := conv1dCase{Conv1dOptions{Stride: 1, Padding: 2, Dilation: 1, Groups: 1}, 1, 64, 64, 300, 5, true}
	x, w, b := c.operands()
	runtime.GOMAXPROCS(1)
	want := floatBits(Conv1dDirect(x, w, b, c.o))
	for _, n := range []int{2, 3, 8, 20} {
		runtime.GOMAXPROCS(n)
		if !slices.Equal(floatBits(Conv1dDirect(x, w, b, c.o)), want) {
			t.Fatalf("GOMAXPROCS=%d sonucu seri hesaptan farklı", n)
		}
	}
}

// Kural (15.22a): eski yolun çarpımı B · çıkış = 1 ve N · K ≥ 4096'da gemv'dir (K tek nokta çarpımı,
// toplama sırası mikro çekirdekten farklı); o çağrı çekirdeğin kapsamında değildir. Toplu 2'de ya da
// küçük iç boyutta kapsamdadır.
func TestConv1dDirectLeavesTheGemvCall(t *testing.T) {
	one := Conv1dOptions{Stride: 1, Dilation: 1, Groups: 1}
	for _, c := range []struct {
		c    conv1dCase
		want bool
	}{
		{conv1dCase{one, 1, 64, 64, 5, 5, true}, false},
		{conv1dCase{one, 2, 64, 64, 5, 5, true}, true},
		{conv1dCase{one, 1, 2, 3, 3, 3, true}, true},
	} {
		x, w, b := c.c.operands()
		if got := Conv1dDirectOK(x, w, b, c.c.o); got != c.want {
			t.Errorf("%+v: kapsamda %v, beklenen %v", c.c, got, c.want)
		}
	}
}
