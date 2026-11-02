package gpu

import (
	"math"
	"slices"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural (15.1, 15.39): TR_Int ve TR_Bool tensörü GPU'ya kayıpsız taşınır; görünümler ve adımlı
// kopyalar cinsi korur (2²⁴'ün üstündeki tam sayı bozulmaz).
func TestTypedStorageOnGPU(t *testing.T) {
	vals := []int64{16777217, -9007199254740993, math.MaxInt64, math.MinInt64, 5, 6}
	c := tensor.Int64sOn(tensor.CPU, tensor.Int64, vals, 2, 3)
	g := c.To(gpuDevice)
	if g.DType() != tensor.Int64 || !slices.Equal(g.Int64s(), vals) {
		t.Fatalf("int64 GPU'da: %v", g.Int64s())
	}
	want := c.Transpose(0, 1).Contiguous().Int64s()
	if got := g.Transpose(0, 1).Contiguous().Int64s(); !slices.Equal(got, want) {
		t.Fatalf("devrik kopya: GPU %v, CPU %v", got, want)
	}
	if got := g.Slice(1, 2, 3).Squeeze(1).Int64s(); !slices.Equal(got, []int64{math.MaxInt64, 6}) {
		t.Fatalf("seçim: %v", got)
	}
	b := tensor.Int64sOn(gpuDevice, tensor.Bool, []int64{1, 0, 1}, 3)
	if got := b.To(tensor.CPU).Int64s(); !slices.Equal(got, []int64{1, 0, 1}) {
		t.Fatalf("bool: %v", got)
	}
	if got := b.Slice(0, 1, 3).Contiguous().Int64s(); !slices.Equal(got, []int64{0, 1}) {
		t.Fatalf("bool dilim: %v", got)
	}
	if z := tensor.ZerosOf(tensor.Bool, 5).To(gpuDevice); !slices.Equal(z.Int64s(), []int64{0, 0, 0, 0, 0}) {
		t.Fatalf("sıfır bool: %v", z.Int64s())
	}
}

// Kural (15.48): GPU'nun ondalık çekirdeği ham bayt türünü sessizce float32 saymaz.
func TestTypedInGPUFloatKernelIsPrecisionError(t *testing.T) {
	x := tensor.Int64sOn(gpuDevice, tensor.Int64, []int64{1, 2}, 2)
	err := testkit.PanicError(func() { x.Softmax(0) })
	if !messages.Has(err, messages.H6969) {
		t.Fatalf("hassasiyet hatası bekleniyordu: %v", err)
	}
}
