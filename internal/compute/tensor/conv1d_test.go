package tensor

import (
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

// İşlemcide türevsiz çağrının sütun düzeni (openColumns) türevli çağrının satır düzeniyle aynı sonucu
// verir: adım, genişleme, dolgu, grup, toplu, sapmalı ve sapmasız, dolgudan geniş çekirdek, boş toplu.
func TestConv1dColumnsMatchRows(t *testing.T) {
	for _, c := range []struct {
		o                      Conv1dOptions
		batch, cin, l, cout, k int
		bias                   bool
	}{
		{Conv1dOptions{Stride: 1, Padding: 0, Dilation: 1, Groups: 1}, 1, 3, 20, 4, 3, false},
		{Conv1dOptions{Stride: 1, Padding: 3, Dilation: 1, Groups: 1}, 1, 8, 50, 6, 7, true},
		{Conv1dOptions{Stride: 1, Padding: 10, Dilation: 5, Groups: 1}, 1, 4, 30, 4, 5, true},
		{Conv1dOptions{Stride: 2, Padding: 2, Dilation: 2, Groups: 2}, 2, 4, 9, 6, 3, true},
		{Conv1dOptions{Stride: 3, Padding: 1, Dilation: 1, Groups: 4}, 3, 8, 41, 8, 4, false},
		{Conv1dOptions{Stride: 1, Padding: 6, Dilation: 1, Groups: 1}, 1, 2, 4, 3, 11, true},
		{Conv1dOptions{Stride: 1, Padding: 1, Dilation: 1, Groups: 1}, 0, 2, 6, 3, 3, true},
	} {
		x := noise(c.batch * c.cin * c.l)
		w := New(noise(c.cout*c.cin/c.o.Groups*c.k), c.cout, c.cin/c.o.Groups, c.k)
		var b *Tensor
		if c.bias {
			b = New(noise(c.cout), c.cout)
		}
		got := Conv1d(New(x, c.batch, c.cin, c.l), w, b, c.o, MatMul)
		want := Conv1d(New(x, c.batch, c.cin, c.l).SetRequiresGrad(true), w, b, c.o, MatMul)
		if !slices.Equal(got.Shape(), want.Shape()) {
			t.Fatalf("%+v: şekil %v, beklenen %v", c, got.Shape(), want.Shape())
		}
		g, h := got.Data(), want.Detach().Data()
		for i := range g {
			if d := g[i] - h[i]; d > 1e-6 || d < -1e-6 {
				t.Fatalf("%+v: %d. öğe %v, satır düzeninde %v", c, i, g[i], h[i])
			}
		}
	}
}

// Türevsiz çağrının sonucu da işçi sayısına bağlı değildir (15.22a).
func TestConv1dColumnsSameBitsForAnyWorkerCount(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(0))
	o := Conv1dOptions{Stride: 1, Padding: 4, Dilation: 2, Groups: 1}
	once := func() []uint32 {
		return floatBits(Conv1d(New(noise(1*16*300), 1, 16, 300), New(noise(12*16*5), 12, 16, 5), New(noise(12), 12), o, MatMul))
	}
	runtime.GOMAXPROCS(1)
	want := once()
	for _, n := range []int{2, 3, 8} {
		runtime.GOMAXPROCS(n)
		if !slices.Equal(once(), want) {
			t.Fatalf("GOMAXPROCS=%d sonucu seri hesaptan farklı", n)
		}
	}
}
