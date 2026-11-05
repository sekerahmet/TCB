package tensor

import (
	"math"
	"runtime"
	"slices"
	"testing"
)

// Sonuç işçi sayısına bağlı değildir: bir satırın toplama sırası yalnız şekle bağlıdır (15.22a).
func TestLogSumExpSameBitsForAnyWorkerCount(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(0))
	once := func() []uint32 { return floatBits(LogSumExp(New(noise(512*300), 512, 300), []int{1}, false)) }
	runtime.GOMAXPROCS(1)
	want := once()
	for _, w := range []int{2, 3, 8} {
		runtime.GOMAXPROCS(w)
		if !slices.Equal(once(), want) {
			t.Fatalf("GOMAXPROCS=%d sonucu seri hesaptan farklı", w)
		}
	}
}

// Özel değerlerin türevi 15.18'dedir: birden çok +sonsuzda türev ilk +sonsuza gider.
func TestLogSumExpRowGradOfSpecialValues(t *testing.T) {
	inf := float32(math.Inf(1))
	dx := make([]float32, 3)
	lseRowGrad(dx, []float32{1, inf, inf}, inf, 2)
	if !slices.Equal(dx, []float32{0, 2, 0}) {
		t.Fatalf("+sonsuz: %v", dx)
	}
	dx = make([]float32, 2)
	lseRowGrad(dx, []float32{-inf, -inf}, -inf, 2)
	if !slices.Equal(dx, []float32{0, 0}) {
		t.Fatalf("-sonsuz: %v", dx)
	}
}
