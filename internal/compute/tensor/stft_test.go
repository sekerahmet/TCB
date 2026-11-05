package tensor

import (
	"math"
	"runtime"
	"slices"
	"testing"
)

var testTrig = Trig{Sin: math.Sin, Cos: math.Cos}

var testCall = FourierCall{Fn: "fourier", Move: "x", Trig: testTrig}

// noise deterministik bir sınav sinyalidir.
func noise(n int) []float32 {
	out := make([]float32, n)
	state := uint32(2463534242)
	for i := range out {
		state ^= state << 13
		state ^= state >> 17
		state ^= state << 5
		out[i] = float32(state%20001)/10000 - 1
	}
	return out
}

// Karışık taban ve Bluestein, float64'te tanımın toplamıyla aynıdır.
func TestPlanMatchesTheDefinition(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 12, 16, 25, 60, 61, 67, 134, 400, 401, 1009} {
		x := make([]float64, n)
		for i, v := range noise(n) {
			x[i] = float64(v)
		}
		plan := newPlan[float64](testTrig, n)
		w := plan.newRealWork()
		copy(w.xr, x)
		plan.real(w)
		for j := 0; j <= n/2; j++ {
			var re, im float64
			for k, v := range x {
				c, s := unitRoot(testTrig, j*k, n)
				re += v * c
				im += v * s
			}
			if math.Abs(re-w.yr[j]) > 1e-9*float64(n) || math.Abs(im-w.yi[j]) > 1e-9*float64(n) {
				t.Fatalf("n=%d j=%d: (%g, %g), beklenen (%g, %g)", n, j, w.yr[j], w.yi[j], re, im)
			}
		}
	}
}

// Çeyrek periyottaki kökler tamdır (23.1).
func TestQuarterRootsAreExact(t *testing.T) {
	for _, n := range []int{4, 8, 12, 400} {
		cases := map[int][2]float64{0: {1, 0}, n / 4: {0, -1}, n / 2: {-1, 0}, 3 * n / 4: {0, 1}}
		for k, want := range cases {
			if re, im := unitRoot(testTrig, k, n); re != want[0] || im != want[1] {
				t.Errorf("n=%d k=%d: (%g, %g), beklenen %v", n, k, re, im, want)
			}
		}
	}
}

// floatBits ondalıkların bit desenidir.
func floatBits(t *Tensor) []uint32 {
	d := t.Data()
	out := make([]uint32, len(d))
	for i, v := range d {
		out[i] = math.Float32bits(v)
	}
	return out
}

// stftOnce sınav sinyalinin STFT'sinin bitleridir.
func stftOnce(rows, n int) ([]uint32, []uint32) {
	x := New(noise(rows*n), rows, n)
	w := HannWindow(400, true, testTrig)
	re, im := Stft(x, w, StftOptions{N: 400, Hop: 160, Center: true, Reflect: true}, testCall)
	return floatBits(re), floatBits(im)
}

// Paralel yol seri hesapla aynı bitleri verir; işçi sayısı sonucu değiştirmez (15.22a, 23.5).
func TestStftSameBitsForAnyWorkerCount(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(0))
	runtime.GOMAXPROCS(1)
	wantRe, wantIm := stftOnce(2, 48000)
	for _, w := range []int{2, 3, 8} {
		runtime.GOMAXPROCS(w)
		re, im := stftOnce(2, 48000)
		if !slices.Equal(re, wantRe) || !slices.Equal(im, wantIm) {
			t.Fatalf("GOMAXPROCS=%d sonucu seri hesaptan farklı", w)
		}
	}
}

// Toplu verilen satırlar tek tek verilenle aynıdır; rfft de işçi sayısından bağımsızdır.
func TestRfftSameBitsForAnyWorkerCount(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(0))
	once := func() []uint32 {
		re, _ := Rfft(New(noise(64*1000), 64, 1000), 1000, testCall)
		return floatBits(re)
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

// Çerçeve sayısı tanımdır: sığmayan son örnekler hiçbir çerçeveye girmez (23.3).
func TestStftFrames(t *testing.T) {
	if p, f := StftFrames(10, StftOptions{N: 4, Hop: 3}); p != 10 || f != 3 {
		t.Errorf("dolgusuz: %d, %d", p, f)
	}
	if p, f := StftFrames(10, StftOptions{N: 4, Hop: 2, Center: true}); p != 14 || f != 6 {
		t.Errorf("ortalı: %d, %d", p, f)
	}
}
