package gpu

import (
	"math"
	"slices"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural (15.5, 15.6, 15.15; 2a adım 8): GPU'da karşılaştırma TR_Bool verir (ondalıkta IEEE 754:
// NaN'lı karşılaştırma yanlış, != doğru; "int64"te kesin), maskeler &, |, ^, ~ ile birleşir ve
// TR_where TR_Bool koşulla seçer; sonuçlar CPU'nunkiyle aynıdır.
func TestTypedMaskPathsOnGPU(t *testing.T) {
	nan := float32(math.NaN())
	fa := []float32{1, nan, float32(math.Copysign(0, -1)), 3}
	fb := []float32{1, 2, 0, 2}
	ia := []int64{16777217, 5, -3, 0}
	ib := []int64{16777216, 5, 4, 0}
	for _, op := range []tensor.CmpOp{tensor.CmpEq, tensor.CmpNe, tensor.CmpLt, tensor.CmpGt, tensor.CmpLe, tensor.CmpGe} {
		cf := tensor.CompareTyped(tensor.NewOn(tensor.CPU, fa, 4), tensor.NewOn(tensor.CPU, fb, 4), op).Int64s()
		gf := tensor.CompareTyped(tensor.NewOn(gpuDevice, fa, 4), tensor.NewOn(gpuDevice, fb, 4), op)
		if gf.DType() != tensor.Bool || !slices.Equal(gf.Int64s(), cf) {
			t.Fatalf("ondalık kıyas %d: GPU %v, CPU %v", op, gf.Int64s(), cf)
		}
		ci := tensor.CompareTyped(tensor.Int64sOn(tensor.CPU, tensor.Int64, ia, 4), tensor.Int64sOn(tensor.CPU, tensor.Int64, ib, 4), op).Int64s()
		gi := tensor.CompareTyped(tensor.Int64sOn(gpuDevice, tensor.Int64, ia, 4), tensor.Int64sOn(gpuDevice, tensor.Int64, ib, 4), op).Int64s()
		if !slices.Equal(gi, ci) {
			t.Fatalf("int64 kıyas %d: GPU %v, CPU %v", op, gi, ci)
		}
	}
	// Yayın: [T, 1] <= [1] (KV önbelleğinin maskesi).
	pos := tensor.Int64sOn(gpuDevice, tensor.Int64, []int64{0, 1, 2, 3}, 4)
	step := tensor.Int64sOn(gpuDevice, tensor.Int64, []int64{2}, 1)
	m := tensor.CompareTyped(pos, step, tensor.CmpLe)
	if got := m.Int64s(); !slices.Equal(got, []int64{1, 1, 1, 0}) {
		t.Fatalf("konum maskesi: %v", got)
	}
	w := tensor.CompareTyped(pos, tensor.Int64sOn(gpuDevice, tensor.Int64, []int64{0}, 1), tensor.CmpGt)
	and, _ := tensor.Logic(m, w, tensor.LogicAnd, tensor.Bool)
	if got := and.Int64s(); !slices.Equal(got, []int64{0, 1, 1, 0}) {
		t.Fatalf("maske &: %v", got)
	}
	if got := tensor.Not(m).Int64s(); !slices.Equal(got, []int64{0, 0, 0, 1}) {
		t.Fatalf("maske ~: %v", got)
	}
	x := tensor.NewOn(gpuDevice, []float32{1.5, 2.5, 3.5, 4.5}, 4)
	zero := tensor.NewOn(gpuDevice, []float32{0}, 1)
	if got := tensor.WhereTyped(m, x, zero).Data(); !slices.Equal(got, []float32{1.5, 2.5, 3.5, 0}) {
		t.Fatalf("where float32: %v", got)
	}
	if got := tensor.WhereTyped(m, pos, step).Int64s(); !slices.Equal(got, []int64{0, 1, 2, 2}) {
		t.Fatalf("where int64: %v", got)
	}
	f, _ := tensor.Convert(m, tensor.Float32)
	if got := f.Data(); !slices.Equal(got, []float32{1, 1, 1, 0}) {
		t.Fatalf("maskeden float32: %v", got)
	}
}

// Kural (15.48; 2a): GPU'da adım 8'in dışındaki cinsli işlem hassasiyet hatasıdır. T1 dilim
// 2b'den beri "int64" aritmetiği kartta (typed_arith_test.go); dar tam sayıların aritmetiği
// kapsam dışında kaldığı için hata yolu int32 + ile sınanır (A16, TCB 11:28: test istisnası).
func TestTypedArithOnGPUIsPrecisionError(t *testing.T) {
	a := tensor.Int64sOn(gpuDevice, tensor.Int32, []int64{1, 2}, 2)
	err := testkit.PanicError(func() { tensor.Arith(a, a, tensor.ArithAdd, tensor.Int32) })
	if !messages.Has(err, messages.H6969) {
		t.Fatalf("hassasiyet hatası bekleniyordu: %v", err)
	}
}
