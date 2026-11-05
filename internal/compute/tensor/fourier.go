package tensor

import "math"

// Fourier dönüşümü çekirdeği (23.1): CPU'da, karışık tabanlı Cooley-Tukey; büyük asal çarpan
// Bluestein'la. Toplama sırası uzunluktan gelir, satırlar ve çerçeveler birbirinden bağımsızdır:
// sonuç işçi sayısına bağlı değildir (15.22a). Çarpımlar açıkça yuvarlanır: FMA'lı işlemcide de aynı
// bitler çıkar. Çeyrek periyottaki katsayılar (1, −1, i, −i) tamdır: açı çeyreğe indirgenir.

// Trig köklerin sinüs ve kosinüsüdür: çağıran platformdan bağımsız işlevleri verir (detmath; bu
// katman onu içe aktaramaz).
type Trig struct{ Sin, Cos func(float64) float64 }

// floatType çekirdeğin hesap ondalıklarıdır.
type floatType interface{ float32 | float64 }

// bluesteinPrime bu sınırı aşan asal çarpan Bluestein'la çözülür (doğrudan toplam O(p²) olurdu).
const bluesteinPrime = 61

// unitRoot e^(−2πi·k/n)'dir. Açı çeyreğe indirgenir: k/n'nin çeyreklerinde sonuç tamdır.
func unitRoot(tr Trig, k, n int) (re, im float64) {
	k %= n
	q, r := 4*k/n, 4*k%n
	c, s := 1.0, 0.0
	if r != 0 {
		a := math.Pi / 2 * float64(r) / float64(n)
		c, s = tr.Cos(a), tr.Sin(a)
	}
	switch q {
	case 1:
		c, s = -s, c
	case 2:
		c, s = -c, -s
	case 3:
		c, s = s, -c
	}
	return c, -s
}

// fftPlan bir uzunluğun dönüşüm planıdır; yalnız okunur, işçiler paylaşır.
type fftPlan[T floatType] struct {
	n       int
	factors []int
	wr, wi  []T // birimin n'inci kökleri
	blue    *bluestein[T]
	bigP    int // Bluestein'a giden asal çarpan (0: yok)
}

// bluestein bir asal uzunluğun evrişim planıdır.
type bluestein[T floatType] struct {
	p      int
	m      int // evrişimin ikinin kuvveti uzunluğu
	inner  *fftPlan[T]
	cr, ci []T // e^(−πi·k²/p)
	fr, fi []T // evrişim çekirdeğinin dönüşümü
}

// factorize n'i tabanlara ayırır: önce 4 ve 2, sonra tek asallar.
func factorize(n int) []int {
	var f []int
	for n%4 == 0 {
		f = append(f, 4)
		n /= 4
	}
	for p := 2; p*p <= n; p++ {
		for n%p == 0 {
			f = append(f, p)
			n /= p
		}
	}
	if n > 1 {
		f = append(f, n)
	}
	return f
}

// newPlan n ≥ 1 uzunluğunun planını kurar.
func newPlan[T floatType](tr Trig, n int) *fftPlan[T] {
	p := &fftPlan[T]{n: n, factors: factorize(n), wr: make([]T, n), wi: make([]T, n)}
	for k := range n {
		re, im := unitRoot(tr, k, n)
		p.wr[k], p.wi[k] = T(re), T(im)
	}
	if n > 1 {
		if last := p.factors[len(p.factors)-1]; last > bluesteinPrime {
			p.bigP = last
			p.blue = newBluestein[T](tr, last)
		}
	}
	return p
}

// newBluestein p asalının planını kurar: X[k] = c[k]·Σ (x[j]·c[j])·conj(c[k−j]).
func newBluestein[T floatType](tr Trig, p int) *bluestein[T] {
	m := 1
	for m < 2*p-1 {
		m *= 2
	}
	b := &bluestein[T]{p: p, m: m, inner: newPlan[T](tr, m), cr: make([]T, p), ci: make([]T, p), fr: make([]T, m), fi: make([]T, m)}
	for k := range p {
		re, im := unitRoot(tr, k*k%(2*p), 2*p)
		b.cr[k], b.ci[k] = T(re), T(im)
	}
	kr, ki := make([]T, m), make([]T, m)
	kr[0], ki[0] = b.cr[0], -b.ci[0]
	for k := 1; k < p; k++ {
		kr[k], ki[k] = b.cr[k], -b.ci[k]
		kr[m-k], ki[m-k] = b.cr[k], -b.ci[k]
	}
	s := b.inner.newScratch()
	b.inner.transform(kr, ki, b.fr, b.fi, s)
	return b
}

// scratch bir işçinin dönüşüm alanıdır.
type scratch[T floatType] struct {
	tr, ti []T // taban çarpımı için p öğe
	ar, ai []T // Bluestein'ın girdisi
	br, bi []T // Bluestein'ın sonucu
	inner  *scratch[T]
}

func (p *fftPlan[T]) newScratch() *scratch[T] {
	big := 0
	for _, f := range p.factors {
		big = max(big, f)
	}
	s := &scratch[T]{tr: make([]T, big), ti: make([]T, big)}
	if p.blue != nil {
		m := p.blue.m
		s.ar, s.ai, s.br, s.bi = make([]T, m), make([]T, m), make([]T, m), make([]T, m)
		s.inner = p.blue.inner.newScratch()
	}
	return s
}

// transform (xr, xi)'nin dönüşümünü (yr, yi)'ye yazar; uzunluklar p.n'dir.
func (p *fftPlan[T]) transform(xr, xi, yr, yi []T, s *scratch[T]) {
	p.level(xr, xi, 1, yr, yi, 0, p.n, s)
}

// level uzunluğu n, adımı stride olan altdizinin dönüşümüdür: fi çarpan sırasıdır.
func (p *fftPlan[T]) level(xr, xi []T, stride int, yr, yi []T, fi, n int, s *scratch[T]) {
	if n == 1 {
		yr[0], yi[0] = xr[0], xi[0]
		return
	}
	f := p.factors[fi]
	if f == p.bigP && f == n {
		p.prime(xr, xi, stride, yr, yi, s)
		return
	}
	m := n / f
	for q := range f {
		p.level(xr[q*stride:], xi[q*stride:], stride*f, yr[q*m:], yi[q*m:], fi+1, m, s)
	}
	step := p.n / n
	for k := range m {
		p.butterfly(yr, yi, k, m, f, step, s)
	}
}

// butterfly k'ıncı öğelerin f tabanlı toplamıdır: X[k+r·m] = Σ_q W^(q(k+r·m))·Y_q[k].
func (p *fftPlan[T]) butterfly(yr, yi []T, k, m, f, step int, s *scratch[T]) {
	wr, wi, N := p.wr, p.wi, p.n
	tr, ti := s.tr[:f], s.ti[:f]
	for q := range f {
		a, b := yr[q*m+k], yi[q*m+k]
		w := q * k * step % N
		c, d := wr[w], wi[w]
		tr[q] = T(a*c) - T(b*d)
		ti[q] = T(a*d) + T(b*c)
	}
	for r := range f {
		sr, si := tr[0], ti[0]
		idx, inc := 0, r*m*step%N
		for q := 1; q < f; q++ {
			idx += inc
			if idx >= N {
				idx -= N
			}
			c, d := wr[idx], wi[idx]
			sr += T(tr[q]*c) - T(ti[q]*d)
			si += T(tr[q]*d) + T(ti[q]*c)
		}
		yr[r*m+k], yi[r*m+k] = sr, si
	}
}

// prime asal uzunluğun Bluestein dönüşümüdür.
func (p *fftPlan[T]) prime(xr, xi []T, stride int, yr, yi []T, s *scratch[T]) {
	b := p.blue
	clear(s.ar)
	clear(s.ai)
	for j := range b.p {
		a, c := xr[j*stride], xi[j*stride]
		s.ar[j] = T(a*b.cr[j]) - T(c*b.ci[j])
		s.ai[j] = T(a*b.ci[j]) + T(c*b.cr[j])
	}
	b.inner.transform(s.ar, s.ai, s.br, s.bi, s.inner)
	inv := T(1) / T(b.m)
	for i := range b.m {
		a, c := s.br[i], s.bi[i]
		s.ar[i] = T(a*b.fr[i]) - T(c*b.fi[i])
		s.ai[i] = -(T(a*b.fi[i]) + T(c*b.fr[i])) // eşleniği: ters dönüşüm ileriyle yapılır
	}
	b.inner.transform(s.ar, s.ai, s.br, s.bi, s.inner)
	for k := range b.p {
		a, c := T(s.br[k]*inv), -T(s.bi[k]*inv)
		yr[k] = T(a*b.cr[k]) - T(c*b.ci[k])
		yi[k] = T(a*b.ci[k]) + T(c*b.cr[k])
	}
}

// realWork bir işçinin gerçek sinyal alanıdır.
type realWork[T floatType] struct {
	xr, xi, yr, yi []T
	s              *scratch[T]
}

func (p *fftPlan[T]) newRealWork() *realWork[T] {
	return &realWork[T]{xr: make([]T, p.n), xi: make([]T, p.n), yr: make([]T, p.n), yi: make([]T, p.n), s: p.newScratch()}
}

// real w.xr'deki gerçek sinyali dönüştürür; sonuç w.yr, w.yi'nin ilk n/2+1 öğesidir.
func (p *fftPlan[T]) real(w *realWork[T]) {
	p.transform(w.xr, w.xi, w.yr, w.yi, w.s)
}
