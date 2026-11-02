package tensor

import (
	"math"
	"slices"
	"testing"
)

// Kural (15.1): TR_Int, TR_Bool ve float64 tensörlerinin depolaması kayıpsızdır; 2²⁴'ün
// üstündeki tam sayı ve float64'ün kesri bozulmaz, görünümler ve kopyalar cinsi korur.
func TestTypedStorageRoundTrip(t *testing.T) {
	big := []int64{16777217, -9007199254740993, math.MaxInt64, math.MinInt64}
	x := Int64sOn(CPU, Int64, big, 2, 2)
	if got := x.Int64s(); !slices.Equal(got, big) {
		t.Fatalf("int64: %v", got)
	}
	if got := x.Transpose(0, 1).Contiguous().Int64s(); !slices.Equal(got, []int64{16777217, math.MaxInt64, -9007199254740993, math.MinInt64}) {
		t.Fatalf("devrik kopya: %v", got)
	}
	if got := x.Clone().Int64s(); !slices.Equal(got, big) || x.Clone().DType() != Int64 {
		t.Fatalf("kopya: %v", got)
	}
	u := Int64sOn(CPU, UInt8, []int64{0, 255, 7}, 3)
	if got := u.Slice(0, 1, 3).Contiguous().Int64s(); !slices.Equal(got, []int64{255, 7}) {
		t.Fatalf("uint8 dilim: %v", got)
	}
	f := Float64sOn(CPU, []float64{0.1, 1e300}, 2)
	if got := f.Float64s(); got[0] != 0.1 || got[1] != 1e300 {
		t.Fatalf("float64: %v", got)
	}
	b := Int64sOn(CPU, Bool, []int64{1, 0, 1}, 3)
	if got := b.Int64s(); !slices.Equal(got, []int64{1, 0, 1}) || b.Elem() != ElemBool {
		t.Fatalf("bool: %v", got)
	}
}

// Kural (15.48): ondalık hesap çekirdeği ham bayt türünü sessizce float32 saymaz.
func TestTypedInFloatKernelIsPrecisionError(t *testing.T) {
	x := Int64sOn(CPU, Int64, []int64{1, 2}, 2)
	expectPanic(t, "int64 tensörde softmax", func() { x.Softmax(0) })
	expectPanic(t, "int64 tensörün Data'sı", func() { x.Data() })
}

// Kural (15.5, 15.6): karşılaştırma TR_Bool verir; ondalıkta IEEE 754, tam sayıda kesin.
func TestTypedCompare(t *testing.T) {
	a := Int64sOn(CPU, Int64, []int64{16777217, 2}, 2)
	b := Int64sOn(CPU, Int64, []int64{16777216}, 1)
	if got := CompareTyped(a, b, CmpGt).Int64s(); !slices.Equal(got, []int64{1, 0}) {
		t.Fatalf("int64 >: %v", got)
	}
	n := New([]float32{float32(math.NaN()), 1}, 2)
	one := New([]float32{1}, 1)
	if got := CompareTyped(n, one, CmpNe).Int64s(); !slices.Equal(got, []int64{1, 0}) {
		t.Fatalf("NaN !=: %v", got)
	}
	if got := CompareTyped(n, one, CmpLe).Int64s(); !slices.Equal(got, []int64{0, 1}) || CompareTyped(n, one, CmpLe).DType() != Bool {
		t.Fatalf("NaN <=: %v", got)
	}
}

// Kural (15.3, 4.2a, 4.3, 4.12): tam sayı aritmetiği taşmada, sıfıra bölmede ve eksi üste
// ilk konumla hata verir; // aşağı yuvarlar, % bölenin işaretini alır; kaydırma taşan bitleri atar.
func TestTypedIntArith(t *testing.T) {
	a := Int64sOn(CPU, Int64, []int64{-7, 7}, 2)
	three := Int64sOn(CPU, Int64, []int64{3}, 1)
	if r, f := Arith(a, three, ArithFloorDiv, Int64); f != nil || !slices.Equal(r.Int64s(), []int64{-3, 2}) {
		t.Fatalf("//: %v %v", r, f)
	}
	if r, f := Arith(a, three, ArithMod, Int64); f != nil || !slices.Equal(r.Int64s(), []int64{2, 1}) {
		t.Fatalf("%%: %v %v", r, f)
	}
	u := Int64sOn(CPU, UInt8, []int64{200, 1}, 2)
	hundred := Int64sOn(CPU, UInt8, []int64{100}, 1)
	if _, f := Arith(u, hundred, ArithAdd, UInt8); f == nil || f.Kind != FaultOverflow || f.Index != 0 || f.A != 200 {
		t.Fatalf("uint8 taşması: %+v", f)
	}
	zero := Int64sOn(CPU, Int64, []int64{1, 0}, 2)
	if _, f := Arith(a, zero, ArithMod, Int64); f == nil || f.Kind != FaultDivZero || f.Index != 1 {
		t.Fatalf("sıfıra bölme: %+v", f)
	}
	if _, f := Arith(three, Int64sOn(CPU, Int64, []int64{-1}, 1), ArithPow, Int64); f == nil || f.Kind != FaultNegExp {
		t.Fatalf("eksi üs: %+v", f)
	}
	if r, f := Arith(Int64sOn(CPU, Int64, []int64{math.MaxInt64}, 1), Int64sOn(CPU, Int64, []int64{1}, 1), ArithAdd, Int64); f == nil {
		t.Fatalf("int64 taşması yakalanmadı: %v", r.Int64s())
	}
	i8 := Int64sOn(CPU, Int8, []int64{1}, 1)
	if r, f := Logic(i8, Int64sOn(CPU, Int8, []int64{7}, 1), LogicShl, Int8); f != nil || r.Int64s()[0] != -128 {
		t.Fatalf("int8 1 << 7: %v %v", r, f)
	}
	if _, f := Logic(i8, Int64sOn(CPU, Int8, []int64{8}, 1), LogicShl, Int8); f == nil || f.Kind != FaultShift {
		t.Fatalf("kaydırma aralığı: %+v", f)
	}
	if _, f := Neg(Int64sOn(CPU, Int8, []int64{-128}, 1)); f == nil {
		t.Fatal("int8 -(-128) taşmalı")
	}
	if _, f := AbsTyped(Int64sOn(CPU, Int64, []int64{math.MinInt64}, 1)); f == nil {
		t.Fatal("int64 en küçüğün mutlak değeri taşmalı")
	}
}

// Kural (15.3): cins dönüşümü ondalığı sıfıra doğru keser, NaN ve sığmayan değer hata; tam
// sayıdan ondalığa en yakına (eşitlikte çifte) yuvarlar, çift yuvarlama yapmaz.
func TestTypedConvert(t *testing.T) {
	f := New([]float32{2.7, -2.7}, 2)
	if r, e := Convert(f, Int64); e != nil || !slices.Equal(r.Int64s(), []int64{2, -2}) {
		t.Fatalf("keser: %v %v", r, e)
	}
	n := New([]float32{1, float32(math.NaN())}, 2)
	if _, e := Convert(n, Int32); e == nil || e.Kind != FaultNaN || e.Index != 1 {
		t.Fatalf("NaN: %+v", e)
	}
	if _, e := Convert(New([]float32{300}, 1), UInt8); e == nil || e.Kind != FaultNotFit {
		t.Fatalf("sığmaz: %+v", e)
	}
	i := Int64sOn(CPU, Int64, []int64{16777217, 16777219}, 2)
	if r, _ := Convert(i, Float32); !slices.Equal(r.Data(), []float32{16777216, 16777220}) {
		t.Fatalf("float32'ye çifte yuvarlama: %v", r.Data())
	}
	// 2^24 + 2^16 + 1 bf16'ya: tek adımda en yakın 2^24 + 2^17'dir; float32 üzerinden çift
	// yuvarlama 2^24 verirdi.
	j := Int64sOn(CPU, Int64, []int64{1<<24 + 1<<16 + 1}, 1)
	if r, _ := Convert(j, BFloat16); r.Data()[0] != 1<<24+1<<17 {
		t.Fatalf("bf16 çift yuvarlama: %v", r.Data())
	}
	if r, _ := Convert(i, Float64); !slices.Equal(r.Float64s(), []float64{16777217, 16777219}) {
		t.Fatalf("float64: %v", r.Float64s())
	}
	if r, e := Convert(Int64sOn(CPU, Int64, []int64{-1}, 1), UInt8); e == nil {
		t.Fatalf("-1 uint8'e sığmaz: %v", r.Int64s())
	}
}

// Kural (15.15): TR_where TR_Bool koşulla seçer, üçü yayınlanır; dallar her türde olabilir.
func TestTypedWhereAndLogic(t *testing.T) {
	c := Int64sOn(CPU, Bool, []int64{1, 0, 1}, 3)
	a := Int64sOn(CPU, Int64, []int64{10, 20, 30}, 3)
	b := Int64sOn(CPU, Int64, []int64{-1}, 1)
	if got := WhereTyped(c, a, b).Int64s(); !slices.Equal(got, []int64{10, -1, 30}) {
		t.Fatalf("where int: %v", got)
	}
	x := New([]float32{1.5, 2.5, 3.5}, 3)
	y := Scalar(0)
	if got := WhereTyped(c, x, y.Reshape(1)).Data(); !slices.Equal(got, []float32{1.5, 0, 3.5}) {
		t.Fatalf("where float32: %v", got)
	}
	d := Int64sOn(CPU, Bool, []int64{1, 1, 0}, 3)
	and, _ := Logic(c, d, LogicAnd, Bool)
	if got := and.Int64s(); !slices.Equal(got, []int64{1, 0, 0}) {
		t.Fatalf("&: %v", got)
	}
	if got := Not(c).Int64s(); !slices.Equal(got, []int64{0, 1, 0}) {
		t.Fatalf("~: %v", got)
	}
}
