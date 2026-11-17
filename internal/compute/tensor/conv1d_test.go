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
