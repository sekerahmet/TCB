package tensor

import (
	"math"
	"slices"
	"testing"
)

// Kural (15.6; T1 dilim 2b): tam sayı birikimli toplamda birden çok öğe taşarsa hata ilk
// hatalı öğeyi söyler: düz (satır öncelikli) konumu en küçük olanı. [3, 2] şeklinde eksen 0'da
// 0. sütun k = 2'de (konum 4), 1. sütun k = 1'de (konum 3) taşar: beklenen konum 3 (eski
// (o, i, k) döngüsü 4'ü söylerdi). Aynı değerler eksen 1'de ([2, 3]) konum 2. Taşmasız
// girdide değerler eksen sırasının birikimli toplamıdır.
func TestCumSumFirstFaultIsSmallestFlatIndex(t *testing.T) {
	const big = math.MaxInt64
	for _, c := range []struct {
		vals   []int64
		shape  []int
		axis   int
		index  int
		a, b   int64
		reason string
	}{
		{[]int64{1, big, big - 1, 1, 5, 0}, []int{3, 2}, 0, 3, big, 1, "eksen 0"},
		{[]int64{1, big - 1, 5, big, 1, 0}, []int{2, 3}, 1, 2, big, 5, "eksen 1"},
		{[]int64{-1, math.MinInt64}, []int{2}, 0, 1, -1, math.MinInt64, "eksi uç"},
	} {
		_, f := ReduceTyped(Int64sOn(CPU, Int64, c.vals, c.shape...), ReduceCumSum, c.axis)
		if f == nil || f.Kind != FaultOverflow || f.Index != c.index || f.A != c.a || f.B != c.b {
			t.Fatalf("%s: konum %d'de %d + %d taşması bekleniyordu: %+v", c.reason, c.index, c.a, c.b, f)
		}
	}
	for _, c := range []struct {
		vals, want []int64
		shape      []int
		axis       int
	}{
		{[]int64{1, 2, 3, 4, 5, 6}, []int64{1, 2, 4, 6, 9, 12}, []int{3, 2}, 0},
		{[]int64{1, 2, 3, 4, 5, 6}, []int64{1, 3, 6, 4, 9, 15}, []int{2, 3}, 1},
		{[]int64{1, 2, 3, 4, 5, 6, 7, 8}, []int64{1, 2, 4, 6, 5, 6, 12, 14}, []int{2, 2, 2}, 1},
		{[]int64{big, -big}, []int64{big, 0}, []int{2}, -1},
		{[]int64{}, []int64{}, []int{0, 2}, 0},
		{[]int64{7}, []int64{7}, []int{1}, 0},
	} {
		r, f := ReduceTyped(Int64sOn(CPU, Int64, c.vals, c.shape...), ReduceCumSum, c.axis)
		if f != nil || !slices.Equal(r.Int64s(), c.want) || !slices.Equal(r.Shape(), c.shape) {
			t.Fatalf("%v eksen %d: %v %v, beklenen %v", c.vals, c.axis, r, f, c.want)
		}
	}
}
