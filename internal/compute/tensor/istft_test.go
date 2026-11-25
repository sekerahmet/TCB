package tensor

import (
	"math"
	"runtime"
	"slices"
	"testing"
)

// Kural (23.7): Irfft, Rfft'in tersidir: her uzunlukta (1, 2, tek, çift, asal, Bluestein) sinyali geri
// verir; float64'te 1e-12 içinde. Sıfırıncı ve çift n'de son kutunun sanal kısmı sonuca girmez.
func TestIrfftRoundTripAllLengths(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 12, 15, 16, 25, 60, 61, 67, 127, 134, 400, 401, 512, 1920} {
		data := noise(3 * n)
		x := New(data, 3, n)
		re, im := Rfft(x, n, testCall)
		y := Irfft(re, im, n, testCall)
		if !slices.Equal(y.Shape(), []int{3, n}) {
			t.Fatalf("n=%d: şekil %v", n, y.Shape())
		}
		scaledClose(t, "irfft", y.Data(), data, 1e-6)
		wide := Float64sOn(CPU, x.Float64s(), 3, n)
		wr, wi := Rfft(wide, n, testCall)
		for k, v := range Irfft(wr, wi, n, testCall).Float64s() {
			if math.Abs(v-float64(data[k])) > 1e-12 {
				t.Fatalf("n=%d float64[%d]: %g, beklenen %g", n, k, v, data[k])
			}
		}
		dirty := im.Data()
		for r := range 3 {
			dirty[r*(n/2+1)] = 5
			if n%2 == 0 {
				dirty[r*(n/2+1)+n/2] = -7
			}
		}
		if !slices.Equal(floatBits(Irfft(re, New(dirty, 3, n/2+1), n, testCall)), floatBits(y)) {
			t.Fatalf("n=%d: sıfırıncı ya da son kutunun sanal kısmı sonuca girdi", n)
		}
	}
}

// Ters dönüşüm de işçi sayısından bağımsızdır (15.22a, 23.5).
func TestIrfftSameBitsForAnyWorkerCount(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(0))
	once := func() []uint32 {
		re, im := New(noise(64*501), 64, 501), New(noise(64*501 + 1)[1:], 64, 501)
		return floatBits(Irfft(re, im, 1000, testCall))
	}
	runtime.GOMAXPROCS(1)
	want := once()
	for _, w := range []int{2, 3, 8} {
		runtime.GOMAXPROCS(w)
		if !slices.Equal(once(), want) {
			t.Fatalf("GOMAXPROCS=%d sonucu seri hesaptan farklı", w)
		}
	}
}
