package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Kural (15.18, 15.41, 25.9): TR_logsumexp kartta çalışır; değeri ve türevi işlemcininkiyle 15.41'in
// 3. basamağında aynıdır (ondalık indirgeme), özel değerler (−∞, +∞, NaN; boş satır) aynen. Kartın
// bitleri yalnız satırın uzunluğuna bağlıdır: aynı satır tek başına ve başka satırlarla aynı
// bitleri verir (15.22a). Sınır girdileri: boş eksen, tek öğe, 0 boyutlu tensör, iki +∞ (türev
// ilkine), +∞ ile NaN, 16 bitlik girdi, uzun satır, birden çok ve karışık eksen.

var nan32, inf32 = float32(math.NaN()), float32(math.Inf(1))

func TestLogSumExpMatchesCPU(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	cases := []struct {
		shape []int
		axes  []int
		keep  bool
	}{
		{[]int{3, 300}, []int{1}, false},
		{[]int{2, 3, 4}, []int{0, 2}, true},
		{[]int{2, 3, 4}, nil, false},
		{[]int{1, 70000}, []int{1}, false}, // Whisper'ın satırı gibi uzun
		{[]int{4, 1}, []int{1}, false},
		{[]int{}, nil, false},
		{[]int{3, 0}, []int{1}, false}, // boş satır: −∞
		{[]int{0, 5}, []int{1}, false},
	}
	for _, c := range cases {
		n := 1
		for _, d := range c.shape {
			n *= d
		}
		data := random(rng, n, 4)
		for _, dt := range []tensor.DType{tensor.Float32, tensor.BFloat16} {
			ad := fmt.Sprintf("%v eksen %v %s", c.shape, c.axes, dt.Name())
			compare(t, ad, 2e-5+bf16Tol(dt), func(dev int) (*tensor.Tensor, []*tensor.Tensor) {
				x := twin(data, false, c.shape...)[dev].Cast(dt).SetRequiresGrad(true)
				return tensor.LogSumExp(x, c.axes, c.keep).Cast(tensor.Float32), []*tensor.Tensor{x}
			})
		}
	}
}

// bf16Tol 16 bitlik sonucun bir kez yuvarlanmasının payıdır.
func bf16Tol(dt tensor.DType) float64 {
	if dt == tensor.BFloat16 {
		return 8e-3
	}
	return 0
}

func TestLogSumExpSpecialValuesOnTheCard(t *testing.T) {
	ninf := -inf32
	rows := [][]float32{
		{ninf, ninf, ninf},
		{1, inf32, 2},
		{inf32, 3, inf32},
		{1, nan32, 2},
		{inf32, nan32, 1},
		{5, ninf, 5},
		{-1e30, 1e30, 0},
	}
	for i, r := range rows {
		compare(t, fmt.Sprintf("satır %d %v", i, r), 2e-5, func(dev int) (*tensor.Tensor, []*tensor.Tensor) {
			x := twin(r, true, 1, len(r))[dev]
			return tensor.LogSumExp(x, []int{1}, false), []*tensor.Tensor{x}
		})
	}
	// Türev birden çok +∞'da ilk +∞'a gider (15.18).
	x := tensor.NewOn(gpuDevice, []float32{inf32, 3, inf32}, 3).SetRequiresGrad(true)
	tensor.LogSumExp(x, nil, false).Backward()
	if got := x.Grad().Data(); got[0] != 1 || got[1] != 0 || got[2] != 0 {
		t.Fatalf("iki +∞'da türev %v, beklenen [1 0 0]", got)
	}
}

func TestLogSumExpBitsDependOnlyOnShape(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	n := 1000
	all := random(rng, 5*n, 3)
	batch := tensor.LogSumExp(tensor.NewOn(gpuDevice, all, 5, n), []int{1}, false).Data()
	again := tensor.LogSumExp(tensor.NewOn(gpuDevice, all, 5, n), []int{1}, false).Data()
	for r := range 5 {
		one := tensor.LogSumExp(tensor.NewOn(gpuDevice, all[r*n:(r+1)*n], 1, n), []int{1}, false).Data()
		if math.Float32bits(one[0]) != math.Float32bits(batch[r]) || math.Float32bits(again[r]) != math.Float32bits(batch[r]) {
			t.Fatalf("satır %d: tek başına %v, toplu %v, ikinci koşu %v", r, one[0], batch[r], again[r])
		}
	}
	if d := tensor.LogSumExp(tensor.NewOn(gpuDevice, all, 5, n), []int{1}, false).Device(); d != gpuDevice {
		t.Fatalf("sonuç %s'de, kartta değil", d)
	}
}
