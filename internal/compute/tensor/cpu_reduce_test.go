package tensor

import (
	"math"
	"math/rand"
	"runtime"
	"testing"
)

// serialArgmax argmaxCPU'nun sıralı döngüsüdür (paralel yoldan önceki tek döngü): başvuru.
func serialArgmax(x []float32) (int, float32) {
	best, bi := float32(math.Inf(-1)), 0
	for d, v := range x {
		if v > best || (v != v && best == best) || (v == 0 && best == 0 && negZero(best) && !negZero(v)) {
			best, bi = v, d
		}
	}
	return bi, best
}

// TestArgmaxParallelMatchesSerial: tek satırlı uzun eksende (üretimde sözlük boyu) paralel argmax sıralı döngünün
// indeksini ve değerini verir: paralel yolun eşiğinin iki yanında (65 535, 65 536) ve 151 936'da; parça sınırında
// eşitlikte ilk indeks, sonraki parçada NaN'da ilk NaN, parçalar arası −0/+0'da +0, hepsi −∞ satırda 0; işçi sayısı
// 1, 2, 7 ve makinenin sayısıyken aynı.
func TestArgmaxParallelMatchesSerial(t *testing.T) {
	prev := runtime.GOMAXPROCS(0)
	defer runtime.GOMAXPROCS(prev)
	nan, negZ := float32(math.NaN()), float32(math.Copysign(0, -1))
	setAll := func(x []float32, v float32) {
		for i := range x {
			x[i] = v
		}
	}
	for _, n := range []int{2*parallelThreshold - 1, 2 * parallelThreshold, 151936} {
		cases := []struct {
			name string
			fill func(x []float32)
		}{
			{"rastgele", func(x []float32) {}},
			{"parça sınırında eşit", func(x []float32) { x[reducePart-1], x[reducePart] = 9, 9 }},
			{"sonraki parçalarda NaN", func(x []float32) { x[n-1], x[reducePart+3] = nan, nan }},
			{"−0 sonra +0", func(x []float32) { setAll(x, -1); x[5], x[reducePart+7] = negZ, 0 }},
			{"yalnız −0", func(x []float32) { setAll(x, -1); x[reducePart+1], x[n-1] = negZ, negZ }},
			{"hepsi −∞", func(x []float32) { setAll(x, float32(math.Inf(-1))) }},
			{"ilk ve son parça eşit", func(x []float32) { x[2], x[n-2], x[n-1] = 9, 9, 9 }},
		}
		for _, c := range cases {
			x := randomData(rand.New(rand.NewSource(int64(n))), n)
			c.fill(x)
			wi, wv := serialArgmax(x)
			for _, p := range []int{1, 2, 7, max(prev, 8)} {
				runtime.GOMAXPROCS(p)
				gi, gv := -1, float32(0)
				argmaxCPU(x, 1, n, 1, func(_, d int, v float32) { gi, gv = d, v })
				if gi != wi || math.Float32bits(gv) != math.Float32bits(wv) {
					t.Fatalf("n=%d %s, %d işçi: (%d, %v), sıralı (%d, %v)", n, c.name, p, gi, gv, wi, wv)
				}
			}
		}
	}
}
