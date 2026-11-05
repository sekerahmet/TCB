package tensor

import (
	"runtime"
	"slices"
	"testing"
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
