package tensor

import "github.com/sekerahmet/TCB/internal/lang/messages"

// Ters Fourier dönüşümü ve ters STFT (23.7, 23.8). CPU'da kutular eşlenik simetrik sinyale açılır ve ileri planla
// dönüştürülür (x[k] = Re Σ conj(X[j])·e^(−iθ_jk) / n): katsayılar ve toplama sırası ileri
// dönüşümünkidir. Ters STFT'de çerçeveler paralel dönüştürülür, bir satırın çerçeveleri sırayla üst üste
// eklenir: sonuç işçi sayısına bağlı değildir (15.22a). Kartta ters taban matrisiyle hesaplanır
// (fourier_card.go), CPU'ya düşmez (23.5).

// signalDType T'nin tensör türüdür (bellek denetimi için).
func signalDType[T floatType]() DType {
	var zero T
	if _, wide := any(zero).(float64); wide {
		return Float64
	}
	return Float32
}

// hermitianFrame n/2+1 kutuyu (base'den step adımla) eşlenik simetrik n uzunluğa açar, sanal kısmı ters
// işaretle: ileri dönüşümün gerçek kısmı (w.yr) n kat ters dönüşümdür. Sıfırıncı kutunun ve çift n'de
// son kutunun sanal kısmı okunmaz: gerçek sinyalde sıfırdır (23.7).
func hermitianFrame[T floatType](p *fftPlan[T], w *realWork[T], re, im []T, base, step int) {
	n := p.n
	clear(w.xi)
	w.xr[0] = re[base]
	for j := 1; 2*j < n; j++ {
		r, m := re[base+j*step], im[base+j*step]
		w.xr[j], w.xi[j] = r, -m
		w.xr[n-j], w.xi[n-j] = r, m
	}
	if n%2 == 0 {
		w.xr[n/2] = re[base+n/2*step]
	}
	p.transform(w.xr, w.xi, w.yr, w.yi, w.s)
}

// wideSpectrum ters dönüşümün float64'te hesaplanacağını söyler: kutulardan biri float64'se. O zaman
// girdilerden biri türev izliyorsa H6040'tır (15.46).
func wideSpectrum(re, im *Tensor, others ...*Tensor) bool {
	if re.dtyp != Float64 && im.dtyp != Float64 {
		return false
	}
	if err := float64RiseError(append([]*Tensor{re, im}, others...)...); err != nil {
		panic(err)
	}
	return true
}

// Irfft (re, im) kutularının son eksen boyunca n uzunluklu gerçek ters dönüşümüdür (23.7); çağıran
// şekilleri ve kutu sayısını denetler.
func Irfft(re, im *Tensor, n int, c FourierCall) *Tensor {
	if c.onCard(re, n) {
		c.onCard(im, n)
		return irfftOnCard(re, im, n, c.Trig)
	}
	c.checkDevice(im)
	if wideSpectrum(re, im) {
		return irfft[float64](re, im, n, c.Trig)
	}
	return irfft[float32](re, im, n, c.Trig)
}

func irfft[T floatType](re, im *Tensor, n int, tr Trig) *Tensor {
	bins, rows := re.shape[len(re.shape)-1], rowsOf(re)
	shape := append(copyInts(re.shape[:len(re.shape)-1]), n)
	checkAllocOn(re.be, isCreate, shape, signalDType[T]())
	xr, xi := signalOf[T](re), signalOf[T](im)
	out := make([]T, rows*n)
	plan := newPlan[T](tr, n)
	parallelFor(rows, n*fourierCost, func(lo, hi int) {
		w := plan.newRealWork()
		for r := lo; r < hi; r++ {
			hermitianFrame(plan, w, xr, xi, r*bins, 1)
			for k := range n {
				out[r*n+k] = w.yr[k] / T(n)
			}
		}
	})
	return trackIrfft(re, im, fromSignal(out, shape), n, tr)
}

// IstftOptions TR_istft'nin ayarlarıdır (23.8). Length kurulacak boydur; IstftLengths'in aralığındadır.
type IstftOptions struct {
	N, Hop int
	Center bool
	Length int
}

// start kurulan sinyalin ilk örneğinin üst üste eklenen toplamdaki yeridir: ortalıda baştaki n/2 örnek
// kesilir.
func (o IstftOptions) start() int {
	if o.Center {
		return o.N / 2
	}
	return 0
}

// overlap f'inci çerçevenin kurulan sinyaldeki yeri ve sinyale düşen [lo, hi) örnekleridir.
func (o IstftOptions) overlap(f int) (at, lo, hi int) {
	at = f*o.Hop - o.start()
	return at, max(0, -at), min(o.N, o.Length-at)
}

// IstftLengths frames çerçevenin kurabileceği boylardır (23.8): natural TR_length: verilmeyince çıkan
// boy (0 olabilir), [lo, hi] aynı n, adım ve ortalamayla Stft'nin tam frames çerçeve vereceği ve her
// örneğine bir çerçeve değen boylar. Boş aralıkta hi < lo'dur (tek çerçeve, çift n, adım 1).
func IstftLengths(frames int, o IstftOptions) (natural, lo, hi int) {
	base := (frames - 1) * o.Hop
	if !o.Center {
		natural = o.N + base
		return natural, natural, natural
	}
	odd := o.N % 2
	natural = base + odd
	return natural, max(natural, 1), natural + min(o.Hop-1, (o.N+1)/2-odd)
}

// IstftGap birden çok çerçevede adım n'den büyükse kurulan sinyalde kaçınılmaz ilk boşluğun indeksidir
// (23.8). Bu denetim sinyal ayrılmadan ve IstftLengths'ten önce yapılır: dev adımda (frames−1)·adım taşar.
func IstftGap(frames int, o IstftOptions) (index int, gap bool) {
	return o.N - o.start(), frames > 1 && o.Hop > o.N
}

// Istft [..., n/2+1, çerçeve] şekilli (re, im) izgeden sinyali kurar (23.8): her çerçeve Irfft ile
// kurulur, pencereyle çarpılır, adımla üst üste eklenir; toplam pencere karelerinin aynı toplamına bölünür.
// Çağıran şekilleri, adımı (IstftGap) ve boyu (IstftLengths) denetler.
func Istft(re, im, window *Tensor, o IstftOptions, c FourierCall) *Tensor {
	frames := re.shape[len(re.shape)-1]
	if _, gap := IstftGap(frames, o); gap || frames < 1 {
		panic(messages.Internal("istft: frames and hop are not checked"))
	}
	if _, lo, hi := IstftLengths(frames, o); o.Length < lo || o.Length > hi {
		panic(messages.Internal("istft: length is not checked"))
	}
	if c.onCard(re, o.N) {
		c.onCard(im, o.N)
		c.onCard(window, o.N)
		return istftOnCard(re, im, window, o, c)
	}
	c.checkDevice(im)
	c.checkDevice(window)
	if wideSpectrum(re, im, window) {
		return istft[float64](re, im, window, o, c)
	}
	return istft[float32](re, im, window, o, c)
}

// istftEnvelope pencere karelerinin kurulan sinyaldeki toplamıdır; toplamı en çok 1e-11 olan örnek
// H7921'dir (NaN yayılır, 15.50).
func istftEnvelope[T floatType](win []T, frames int, o IstftOptions, fn any) []T {
	env := make([]T, o.Length)
	for f := range frames {
		at, lo, hi := o.overlap(f)
		for i := lo; i < hi; i++ {
			env[at+i] += T(win[i] * win[i])
		}
	}
	for p, v := range env {
		if v <= 1e-11 {
			reason := "centered"
			if !o.Center && p == 0 { // pencerenin ilk öğesi: Hann'ın cümlesi yalnız burada doğrudur
				reason = "plain"
			}
			panic(messages.New(messages.H7921, fn, p, o.Hop, reason))
		}
	}
	return env
}

// istftShape toplu satır sayısı, kutu, çerçeve ve sonucun şeklidir.
func istftShape(re *Tensor, o IstftOptions) (rows, bins, frames int, shape []int) {
	d := len(re.shape)
	rows = 1
	for _, n := range re.shape[:d-2] {
		rows *= n
	}
	return rows, re.shape[d-2], re.shape[d-1], append(copyInts(re.shape[:d-2]), o.Length)
}

func istft[T floatType](re, im, window *Tensor, o IstftOptions, c FourierCall) *Tensor {
	rows, bins, frames, shape := istftShape(re, o)
	n, dt := o.N, signalDType[T]()
	for _, size := range [][]int{shape, {o.Length}, {rows * frames, n}} { // sonuç, zarf, çerçeveler
		checkAllocOn(re.be, isCreate, size, dt)
	}
	xr, xi, win := signalOf[T](re), signalOf[T](im), signalOf[T](window)
	if rows == 0 {
		return trackIstft(re, im, window, fromSignal([]T{}, shape), o, c)
	}
	env := istftEnvelope(win, frames, o, c.Fn)
	buf := make([]T, rows*frames*n)
	plan := newPlan[T](c.Trig, n)
	parallelFor(rows*frames, n*fourierCost, func(from, to int) {
		w := plan.newRealWork()
		for u := from; u < to; u++ {
			hermitianFrame(plan, w, xr, xi, u/frames*bins*frames+u%frames, frames)
			dst := buf[u*n : (u+1)*n]
			for i := range dst {
				dst[i] = T(w.yr[i]/T(n)) * win[i]
			}
		}
	})
	out := make([]T, rows*o.Length)
	parallelFor(rows, frames*n, func(from, to int) {
		for r := from; r < to; r++ {
			dst := out[r*o.Length : (r+1)*o.Length]
			for f := range frames {
				src := buf[(r*frames+f)*n : (r*frames+f+1)*n]
				at, lo, hi := o.overlap(f)
				for i := lo; i < hi; i++ {
					dst[at+i] += src[i]
				}
			}
			for p := range dst {
				dst[p] /= env[p]
			}
		}
	})
	return trackIstft(re, im, window, fromSignal(out, shape), o, c)
}
