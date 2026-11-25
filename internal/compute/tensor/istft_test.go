package tensor

import (
	"math"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit"
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

// Kural (23.8): aynı n, adım, pencere ve ortalamayla Istft, Stft'in tersidir; ortalıda iki dolgu kipinin
// izgesinden de aynı sinyal çıkar. Hann'da adım en çok n/4 (üst uçta yuvarlama büyür, 23.8).
func TestIstftRoundTrip(t *testing.T) {
	for _, c := range []struct {
		n, hop, signal  int
		center, reflect bool
	}{
		{16, 4, 480, true, true},      // HiFT'in sonu
		{2048, 320, 6400, true, true}, // PerTh
		{400, 100, 1673, true, false},
		{9, 2, 50, true, true}, // tek n
		{8, 2, 5, true, true},  // sinyal n/2 + 1
		{8, 3, 50, false, false},
		{1, 1, 7, true, false},
		{8, 8, 8, false, false}, // tek çerçeve
	} {
		w := HannWindow(c.n, true, testTrig)
		if !c.center {
			w = New(noise(c.n), c.n) // ortalısızda ilk öğe 0 olamaz
		}
		so := StftOptions{N: c.n, Hop: c.hop, Center: c.center, Reflect: c.reflect}
		_, frames := StftFrames(c.signal, so)
		o := IstftOptions{N: c.n, Hop: c.hop, Center: c.center, Length: c.signal}
		if !c.center { // sığmayan son örnekler hiçbir çerçevede yoktur
			_, o.Length, _ = IstftLengths(frames, o)
		}
		data := noise(2 * c.signal)
		want := append(slices.Clone(data[:o.Length]), data[c.signal:c.signal+o.Length]...)
		re, im := Stft(New(data, 2, c.signal), w, so, testCall)
		got := Istft(re, im, w, o, testCall)
		if !slices.Equal(got.Shape(), []int{2, o.Length}) {
			t.Fatalf("%+v: şekil %v", c, got.Shape())
		}
		tol := 2e-6
		if !c.center {
			tol = 1e-4 // rastgele pencerenin küçük öğeleri yuvarlamayı büyütür
		}
		scaledClose(t, "istft", got.Data(), want, tol)
		wide := func(x *Tensor) *Tensor { return Float64sOn(CPU, x.Float64s(), x.Shape()...) }
		wr, wi := Stft(wide(New(data, 2, c.signal)), w, so, testCall)
		for i, v := range Istft(wr, wi, w, o, testCall).Float64s() {
			if math.Abs(v-float64(want[i])) > 1e-9 {
				t.Fatalf("%+v float64[%d]: %g, beklenen %g", c, i, v, want[i])
			}
		}
	}
}

// Kural (23.8): IstftLengths, Stft'in aynı ayarla tam frames çerçeve verdiği ve her örneğine bir çerçeve
// değen boyların ta kendisidir (kaba arama); alt uç doğal boydur. Adım n'den büyükse (birden çok çerçeve)
// hiçbir boy yoktur: IstftGap onu yakalar. Boş aralık yalnız tek çerçeve, çift n, adım 1'dedir.
func TestIstftLengthsMatchStft(t *testing.T) {
	cases, empty := 0, 0
	for n := 1; n <= 12; n++ {
		for hop := 1; hop <= 14; hop++ {
			for frames := 1; frames <= 5; frames++ {
				for _, center := range []bool{true, false} {
					cases++
					o := IstftOptions{N: n, Hop: hop, Center: center}
					var valid []int
					for u := 1; u <= n+(frames+2)*hop; u++ {
						if stftCovers(u, frames, o) {
							valid = append(valid, u)
						}
					}
					if _, gap := IstftGap(frames, o); gap {
						if len(valid) > 0 {
							t.Fatalf("%+v %d çerçeve: boşlukta geçerli boy %v", o, frames, valid)
						}
						continue
					}
					natural, lo, hi := IstftLengths(frames, o)
					if len(valid) == 0 {
						empty++
						if hi >= lo || frames != 1 || n%2 != 0 || hop != 1 || !center || natural != 0 {
							t.Fatalf("%+v %d çerçeve: aralık %d..=%d, geçerli boy yok", o, frames, lo, hi)
						}
						continue
					}
					if valid[0] != lo || valid[len(valid)-1] != hi || len(valid) != hi-lo+1 || max(natural, 1) != lo {
						t.Fatalf("%+v %d çerçeve: aralık %d..=%d (doğal %d), kaba arama %v", o, frames, lo, hi, natural, valid)
					}
				}
			}
		}
	}
	if cases != 12*14*5*2 || empty != 6 {
		t.Fatalf("%d durum, %d boş aralık", cases, empty)
	}
	// Dev adımda taşma yok: tek çerçevenin aralığı n'den çıkar, birden çok çerçeve boşluktur.
	o := IstftOptions{N: 8, Hop: math.MaxInt, Center: true}
	if natural, lo, hi := IstftLengths(1, o); natural != 0 || lo != 1 || hi != 4 {
		t.Fatalf("dev adım: %d, %d..=%d", natural, lo, hi)
	}
	if index, gap := IstftGap(2, o); !gap || index != 4 {
		t.Fatalf("dev adımın boşluğu: %d, %v", index, gap)
	}
}

// stftCovers u uzunluklu sinyalin tam frames çerçeve verdiğini ve her örneğine bir çerçeve değdiğini söyler.
func stftCovers(u, frames int, o IstftOptions) bool {
	padded, f := StftFrames(u, StftOptions{N: o.N, Hop: o.Hop, Center: o.Center})
	if padded < o.N || f != frames {
		return false
	}
	for p := range u {
		at, hit := p+o.start(), false
		for k := range frames {
			hit = hit || at >= k*o.Hop && at < k*o.Hop+o.N
		}
		if !hit {
			return false
		}
	}
	return true
}

// Çerçeveler paralel dönüşür, üst üste ekleme sırası sabittir: işçi sayısı bitleri değiştirmez (15.22a).
func TestIstftSameBitsForAnyWorkerCount(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(0))
	w := HannWindow(400, true, testTrig)
	re, im := Stft(New(noise(2*16000), 2, 16000), w, StftOptions{N: 400, Hop: 160, Center: true, Reflect: true}, testCall)
	once := func() []uint32 {
		a, b, win := re.Detach().SetRequiresGrad(true), im.Detach().SetRequiresGrad(true), w.Detach().SetRequiresGrad(true)
		y := Istft(a, b, win, IstftOptions{N: 400, Hop: 160, Center: true, Length: 16000}, testCall)
		Mul(y, y).SumAll().Backward()
		return slices.Concat(floatBits(y), floatBits(a.Grad()), floatBits(b.Grad()), floatBits(win.Grad()))
	}
	runtime.GOMAXPROCS(1)
	want := once()
	for _, workers := range []int{2, 3, 8} {
		runtime.GOMAXPROCS(workers)
		if !slices.Equal(once(), want) {
			t.Fatalf("GOMAXPROCS=%d sonucu seri hesaptan farklı", workers)
		}
	}
}

// Kural (23.8): kurulan bir örnekte pencere karelerinin toplamı en çok 1e-11 ise H7921; ileti örneği söyler,
// Hann'ın cümlesini yalnız ortalısız kipin 0. örneğinde. Ortalıda baştaki n/2 örnek kesilir: ilk çerçevenin
// sıfır öğesi sonuca girmez. Penceredeki NaN sonuca yayılır (15.50).
func TestIstftRejectsZeroEnvelope(t *testing.T) {
	const hann = "TR_center: TR_false"
	for _, c := range []struct {
		window []float32
		o      IstftOptions
		frames int
		index  string // boşsa hata yok
		hann   bool
	}{
		{[]float32{0, 1, 1, 0}, IstftOptions{N: 4, Hop: 4, Length: 8}, 2, " 0.", true},
		{[]float32{0, 1, 1, 1}, IstftOptions{N: 4, Hop: 4, Length: 8}, 2, " 0.", true},
		{[]float32{1, 1, 1, 0}, IstftOptions{N: 4, Hop: 4, Length: 8}, 2, " 3.", false},
		{[]float32{0, 1, 1, 0}, IstftOptions{N: 4, Hop: 2, Center: true, Length: 2}, 2, "", false},
		{[]float32{0, 1, 1, 0}, IstftOptions{N: 4, Hop: 4, Center: true, Length: 4}, 2, " 1.", false},
		{[]float32{0, 1, 1, 0}, IstftOptions{N: 4, Hop: 2, Center: true, Length: 3}, 2, "", false},
		{[]float32{3e-6, 3e-6}, IstftOptions{N: 2, Hop: 2, Length: 2}, 1, " 0.", true},
		{[]float32{4e-6, 4e-6}, IstftOptions{N: 2, Hop: 2, Length: 2}, 1, "", false},
	} {
		bins := c.o.N/2 + 1
		re, im := New(make([]float32, bins*c.frames), bins, c.frames), New(make([]float32, bins*c.frames), bins, c.frames)
		err := testkit.PanicError(func() { Istft(re, im, New(c.window, c.o.N), c.o, testCall) })
		if c.index == "" {
			if err != nil {
				t.Errorf("%v %+v: %v, beklenen hata yok", c.window, c.o, err)
			}
			continue
		}
		if !messages.Has(err, messages.H7921) || !strings.Contains(err.Error(), c.index) || strings.Contains(err.Error(), hann) != c.hann {
			t.Errorf("%v %+v: %v, beklenen H7921 (%s örnek, Hann cümlesi %v)", c.window, c.o, err, c.index, c.hann)
		}
	}
	nan := float32(math.NaN())
	y := Istft(New([]float32{1, 1, 1}, 3, 1), New(make([]float32, 3), 3, 1), New([]float32{nan, 3, 3, 3}, 4), IstftOptions{N: 4, Hop: 4, Length: 4}, testCall)
	if d := y.Data(); d[0] == d[0] || d[1] != d[1] {
		t.Errorf("NaN pencere: %v, beklenen yalnız ilk örnek NaN", d)
	}
}
