package tensor

import (
	"math"
	"math/rand"
	"runtime"
	"testing"
)

// Paralel çekirdek yollarının doğruluk ve belirlenimcilik testleri. Boyutlar,
// paralelleştirme eşiklerini aşacak şekilde seçilmiştir.

// naifSum float64 birikimli başvuru eksen toplamı.
func naifSum(xd []float32, shape []int, axis int) []float32 {
	outer, dim, inner := eksenDuzeni(shape, axis)
	out := make([]float32, outer*inner)
	for o := 0; o < outer; o++ {
		for i := 0; i < inner; i++ {
			var s float64
			for d := 0; d < dim; d++ {
				s += float64(xd[(o*dim+d)*inner+i])
			}
			out[o*inner+i] = float32(s)
		}
	}
	return out
}

func TestSumParalelYollar(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	durumlar := []struct {
		shape []int
		axis  int
	}{
		{[]int{3, 100000}, 1},    // son eksen, birkaç satır
		{[]int{1, 1 << 17}, 1},   // son eksen, tek satır (parçalı toplam)
		{[]int{100, 7, 300}, 1},  // çok dış satır
		{[]int{2, 5, 4096}, 1},   // az dış satır, geniş iç eksen (sütun blokları)
		{[]int{1, 5000, 10}, 1},  // az dış satır, dar iç eksen (eksen parçaları)
		{[]int{5000, 10}, 0},     // eksen 0
		{[]int{2048, 512}, 0},    // eksen 0, orta genişlik
		{[]int{3, 4}, 0},         // küçük
		{[]int{0, 4}, 0},         // boş
		{[]int{4, 0, 3}, 1},      // boş eksen
		{[]int{7, 33, 1, 5}, -2}, // uzunluğu 1 olan eksen
	}
	for _, d := range durumlar {
		x := Randn(rng, d.shape...)
		got := x.Sum(d.axis, false).Data()
		want := naifSum(x.Data(), x.shape, normAxis(d.axis, len(d.shape)))
		yakin(t, "sum", got, want, 1e-5)
		// Belirlenimcilik: tekrar ve farklı işçi sayısıyla bit düzeyinde aynı.
		eski := runtime.GOMAXPROCS(3)
		tekrar := x.Sum(d.axis, false).Data()
		runtime.GOMAXPROCS(eski)
		for i := range got {
			if math.Float32bits(got[i]) != math.Float32bits(tekrar[i]) {
				t.Fatalf("sum %v eksen %d belirlenimci değil: [%d] %v != %v", d.shape, d.axis, i, got[i], tekrar[i])
			}
		}
	}
}

func TestSumAllParalel(t *testing.T) {
	rng := rand.New(rand.NewSource(12))
	x := Randn(rng, 3, 100003)
	var s float64
	for _, v := range x.Data() {
		s += float64(v)
	}
	got := x.SumAll().Item()
	if math.Abs(float64(got)-s) > 1e-3 {
		t.Fatalf("SumAll %v, beklenen %v", got, s)
	}
	eski := runtime.GOMAXPROCS(2)
	tekrar := x.SumAll().Item()
	runtime.GOMAXPROCS(eski)
	if got != tekrar {
		t.Fatalf("SumAll belirlenimci değil: %v != %v", got, tekrar)
	}
	// Devrik (bitişik olmayan) görünüm.
	yakin(t, "devrik SumAll", []float32{x.T().SumAll().Item()}, []float32{float32(s)}, 1e-5)
}
