package tensor

// Ters Fourier dönüşümü (23.7). CPU'da kutular eşlenik simetrik sinyale açılır ve ileri planla
// dönüştürülür (x[k] = Re Σ conj(X[j])·e^(−iθ_jk) / n): katsayılar ve toplama sırası ileri
// dönüşümünkidir, sonuç işçi sayısına bağlı değildir (15.22a). Kartta ters taban matrisiyle hesaplanır
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
