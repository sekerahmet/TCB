package gpu

import "math"

// Tam (erf tabanlı) GELU için PTX yardımcıları. PTX'te erf komutu yoktur; standart normal
// dağılım işlevi Φ(x) = ½·erfc(−x/√2) iki bölgede hesaplanır (t = |x|/√2):
//
//   - t < 1/2: erf(t) = t·P(t²), P erf'in Maclaurin serisinin 7 terimidir (kesme hatası
//     göreli ~4e-10); Φ = ½ ± ½·erf(t). Toplamdaki iptal en çok 2 kat hata büyütür.
//   - t ≥ 1/2: erfc(t) = e^{−t²}·g(t)/(t·√π), g(t) = √π·t·erfcx(t) → 1; g, z = 1/(1+t) için
//     s = 3z−1 ∈ [−1, 1]'de 12. dereceden Chebyshev serisidir (float64'te göreli hata
//     4e-9). Kuyruk q = ½·erfc(t) göreli doğrulukla bulunur (1 − erf farkı yok), böylece
//     x çok negatifken Φ(x) ve x·Φ(x) göreli doğruluğunu korur; x > 0 için Φ = 1 − q.
//
// e^{−t²} = e^{−x²/2} üssü x² = hi + lo olarak bölünür (lo = fma(x,x,−hi)) ve e^{−hi/2}·(1 − lo/2)
// alınır: x²'nin float32 yuvarlama hatası (x = 12'de göreli ~1e-5) ve t = x/√2'nin yuvarlaması
// üsse girmez.

// erfSmallCoef erf(t) = t·P(t²)'nin katsayıları: 2/√π · (−1)^n / (n!·(2n+1)), n = 0..6.
var erfSmallCoef = func() []float64 {
	c := make([]float64, 7)
	fact := 1.0
	for n := range c {
		if n > 0 {
			fact *= float64(n)
		}
		c[n] = 2 / math.Sqrt(math.Pi) * math.Pow(-1, float64(n)) / (fact * float64(2*n+1))
	}
	return c
}()

// erfcxCoef g(z)'nin s = 4z−1 değişkenindeki Chebyshev katsayılarıdır (T0..T12).
var erfcxCoef = []float64{
	0.8373258834099423, -0.22580444569265462, -0.06620334093421042, -0.0013181685758215456,
	0.0017603075264132162, -6.775670679915347e-05, -6.348721004562756e-05,
	1.1908634561373757e-05, 1.214095428332053e-06, -8.872659073277788e-07,
	1.1784307464471269e-07, 2.827312170713883e-08, -1.4913116012946786e-08,
}

const (
	sqrtHalf    = 0.70710678118654752440 // 1/√2
	invSqrt2Pi  = 0.39894228040143267794 // 1/√(2π)
	sqrtPi      = 1.77245385090551602730 // √π
	erfSwitchAt = 0.5                    // t'nin seri ile kuyruk arasındaki eşiği
)

// expNegSquare e^{−scale·a²}; a² hi + lo'ya bölünür. a sonsuzken (hi = ∞, lo = NaN) sonuç 0'dır.
func (k *emitter) expNegSquare(a reg, scale float32) reg {
	hi := k.f()
	k.e("mul.rn.f32", hi, a, a)
	lo := k.f()
	nh := k.f()
	k.e("neg.f32", nh, hi)
	k.e("fma.rn.f32", lo, a, a, nh)
	arg := k.f()
	k.e("mul.rn.f32", arg, hi, -scale)
	ex := k.exponential(arg)
	corr := k.f()
	k.e("mul.rn.f32", corr, lo, -scale)
	fixed := k.f()
	k.e("fma.rn.f32", fixed, ex, corr, ex)
	zero := k.p()
	k.e("setp.eq.f32", zero, ex, float32(0))
	return k.pick(zero, ex, fixed)
}

// normalCdf Φ(x) = ½·(1 + erf(x/√2)); NaN yayılır, Φ(−∞) = 0, Φ(+∞) = 1.
func (k *emitter) normalCdf(x reg) reg {
	a := k.f()
	k.e("abs.f32", a, x)
	t := k.f()
	k.e("mul.rn.f32", t, a, float32(sqrtHalf))
	nonNeg := k.p()
	k.e("setp.ge.f32", nonNeg, x, float32(0))

	// t < 1/2: Φ = ½ + (±½)·erf(t)
	t2 := k.f()
	k.e("mul.rn.f32", t2, t, t)
	poly := k.horner(t2, erfSmallCoef)
	erf := k.f()
	k.e("mul.rn.f32", erf, t, poly)
	half := k.pick(nonNeg, float32(0.5), float32(-0.5))
	near := k.f()
	k.e("fma.rn.f32", near, half, erf, float32(0.5))

	// t ≥ 1/2: q = ½·e^{−t²}·g/(t·√π); Φ = q (x < 0) ya da 1 − q
	z := k.f()
	k.e("add.rn.f32", z, t, float32(1))
	k.e("div.rn.f32", z, float32(1), z)
	s := k.f()
	k.e("fma.rn.f32", s, z, float32(3), float32(-1))
	twoS := k.f()
	k.e("add.rn.f32", twoS, s, s)
	b1 := k.constant(float32(erfcxCoef[12]))
	b2 := k.constant(0)
	for i := 11; i >= 1; i-- {
		b0 := k.f()
		k.e("fma.rn.f32", b0, twoS, b1, float32(erfcxCoef[i]))
		k.e("sub.rn.f32", b0, b0, b2)
		b2, b1 = b1, b0
	}
	g := k.f()
	k.e("fma.rn.f32", g, s, b1, float32(erfcxCoef[0]))
	k.e("sub.rn.f32", g, g, b2)
	ex := k.expNegSquare(x, 0.5) // e^{−t²} = e^{−x²/2}; t'nin yuvarlaması üsse girmez
	den := k.f()
	k.e("mul.rn.f32", den, t, float32(sqrtPi))
	q := k.f()
	k.e("mul.rn.f32", q, ex, g)
	k.e("div.rn.f32", q, q, den)
	k.e("mul.rn.f32", q, q, float32(0.5))
	oneMinus := k.f()
	k.e("sub.rn.f32", oneMinus, float32(1), q)
	far := k.pick(nonNeg, oneMinus, q)

	small := k.p()
	k.e("setp.lt.f32", small, t, float32(erfSwitchAt))
	return k.pick(small, near, far)
}

// geluExact x·Φ(x); Φ(x) = 0 olan uçta 0 (−∞·0 NaN üretirdi; CPU ile aynı).
func (k *emitter) geluExact(x reg) reg {
	phi := k.normalCdf(x)
	r := k.f()
	k.e("mul.rn.f32", r, x, phi)
	zero := k.p()
	k.e("setp.eq.f32", zero, phi, float32(0))
	return k.pick(zero, float32(0), r)
}

// geluExactGrad Φ(x) + x·φ(x); φ(x) = 0 iken Φ(x) (CPU ile aynı).
func (k *emitter) geluExactGrad(x reg) reg {
	phi := k.normalCdf(x)
	density := k.expNegSquare(x, 0.5)
	k.e("mul.rn.f32", density, density, float32(invSqrt2Pi))
	d := k.f()
	k.e("fma.rn.f32", d, x, density, phi)
	zero := k.p()
	k.e("setp.eq.f32", zero, density, float32(0))
	return k.pick(zero, phi, d)
}
