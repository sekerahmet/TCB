package gpu

import (
	"math"
	"math/rand"
	"slices"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// sumBoth int64 toplamını işlemcide ve kartta yapar; değer ya da hata (çeşit, konum, A, Sum)
// aynı olmalı.
func sumBoth(t *testing.T, name string, vals []int64, shape []int, axis int) *tensor.ElemFault {
	t.Helper()
	c, cf := tensor.ReduceTyped(int64On(tensor.CPU, vals, shape...), tensor.ReduceSum, axis)
	g, gf := tensor.ReduceTyped(int64On(gpuDevice, vals, shape...), tensor.ReduceSum, axis)
	if !sameFault(cf, gf) || cf != nil && gf.Sum != cf.Sum {
		t.Fatalf("%s: kart %+v, işlemci %+v", name, gf, cf)
	}
	if cf == nil && (g.Device() != gpuDevice || !slices.Equal(g.Int64s(), c.Int64s()) || !slices.Equal(g.Shape(), c.Shape())) {
		t.Fatalf("%s: kart %v %v, işlemci %v %v", name, g.Int64s(), g.Shape(), c.Int64s(), c.Shape())
	}
	return gf
}

// Kural (15.18, 15.6; T1 dilim 2b): kartta "int64" TR_sum işlemcinin değerini ve hatasını verir:
// toplam 128 bitte kesin (ara toplamın taşması sayılmaz), yalnız sonuç int64'e sığmalı; hata
// kesin değeri (Sum) ve int64'ün ucunu (A) taşır. Her eksen, eksensiz, boş eksen, tek öğe,
// iki uçta taşma; birden çok taşan çıktıda düz konumu en küçük olanı.
func TestSumInt64MatchesCPU(t *testing.T) {
	const big, small = math.MaxInt64, math.MinInt64
	vals := []int64{big, 1, -1, 5, small, -3, 2, big, 7, -7, 0, 1}
	for _, shape := range [][]int{{12}, {3, 4}, {2, 3, 2}} {
		for axis := -1; axis < len(shape); axis++ {
			sumBoth(t, "karışık", vals, shape, axis)
		}
	}
	for _, c := range []struct {
		name  string
		vals  []int64
		shape []int
		axis  int
		index int
	}{
		{"üstten taşma", []int64{big, 1}, []int{2}, -1, 0},
		{"alttan taşma", []int64{small, -1}, []int{2}, -1, 0},
		{"birden çok çıktı", []int64{1, big, big, 0, 3, 1}, []int{2, 3}, 0, 1},
		{"satırlar", []int64{0, 0, big, big, small, small}, []int{3, 2}, 1, 1},
	} {
		if f := sumBoth(t, c.name, c.vals, c.shape, c.axis); f == nil || f.Index != c.index || f.Kind != tensor.FaultOverflow {
			t.Fatalf("%s: konum %d'de taşma bekleniyordu: %+v", c.name, c.index, f)
		}
	}
	for _, c := range []struct {
		vals  []int64
		shape []int
		axis  int
	}{
		{[]int64{big, 1, -1, -big}, []int{4}, -1}, // ara toplam taşar, sonuç 0
		{nil, []int{2, 0}, 1},                     // boş eksen: 0
		{nil, []int{0, 3}, 1},                     // boş çıktı
		{[]int64{42}, []int{}, -1},                // 0 boyutlu
	} {
		if f := sumBoth(t, "sınır", c.vals, c.shape, c.axis); f != nil {
			t.Fatalf("%v: hata beklenmiyordu: %+v", c.vals, f)
		}
	}
}

// Kural (15.6; T1 dilim 2b): birçok çıktı taşarsa kart her koşuda düz konumu en küçük olanı
// söyler (atomların sırası sonucu değiştirmez). Benzeticide 2¹², kartta 2²⁰ çıktı.
func TestSumInt64FirstFaultIsSmallestPosition(t *testing.T) {
	n := 1 << 20
	if inSim() {
		n = 1 << 12
	}
	rng := rand.New(rand.NewSource(17))
	for run := 0; run < 3; run++ {
		x := make([]int64, 2*n)
		want := n
		for k := 0; k < 64; k++ {
			at := rng.Intn(n)
			x[2*at], x[2*at+1] = math.MaxInt64, 1
			want = min(want, at)
		}
		_, f := tensor.ReduceTyped(int64On(gpuDevice, x, n, 2), tensor.ReduceSum, 1)
		if f == nil || f.Index != want || f.Kind != tensor.FaultOverflow || f.Sum != "9223372036854775808" {
			t.Fatalf("koşu %d: %+v; beklenen konum %d", run, f, want)
		}
	}
}
