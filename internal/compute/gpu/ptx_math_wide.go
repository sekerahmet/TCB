package gpu

import (
	"math"
	"math/big"
)

// Genişletilmiş PTX matematik yardımcıları (tanjant, ters trigonometri, hiperbolik,
// kararlı logaritma ve üstel, SiLU). Yöntem: aralık indirgemesi + indirgenmiş aralıkta
// Taylor çok terimlisi (terim sayısı kesme hatası float32 yarım ulp'unun altında kalacak
// biçimde seçildi) + float32 işlemler. Sonuçlar CPU arka ucunun float64 başvurusuyla
// birkaç ulp içinde uyuşur (sınamalar: genis_test.go). Tanjantın aralık indirgemesi π/2'nin
// üç parçalı yazımıyladır; |x| ≲ 1e5 için doğrudur.

// endPart v'yi float32 toplamı v'ye 72 bit kadar yaklaşan üç parçaya böler.
func endPart(v *big.Float) [3]float32 {
	var p [3]float32
	rest := new(big.Float).SetPrec(200).Set(v)
	for i := range p {
		f, _ := rest.Float64()
		p[i] = float32(f)
		rest.Sub(rest, new(big.Float).SetFloat64(float64(p[i])))
	}
	return p
}

var (
	// piHalfPart π/2'nin üç parçası (Cody-Waite indirgemesi için).
	piHalfPart = func() [3]float32 {
		pi, _, _ := big.ParseFloat("3.14159265358979323846264338327950288419716939937510582097494459", 10, 200, big.ToNearestEven)
		return endPart(pi.Quo(pi, big.NewFloat(2)))
	}()
	piHalf32    = float32(math.Pi / 2)
	pi32        = float32(math.Pi)
	piQuarter   = float32(math.Pi / 4)
	twoOverPi   = float32(2 / math.Pi)
	tanPiEighth = float32(0.41421356237309503) // tan(π/8)
	root2       = float32(math.Sqrt2)
	// ln 2'nin iki parçası: üst parça 9 bitlik (e·üst, |e| < 2^15 için tam).
	ln2Hi = float32(0.693359375)
	ln2Lo = float32(math.Ln2 - 0.693359375)
	// 1/ln 2 ve log10(e), log10(2) parçaları (log2 ve log10 için).
	oneOverLn2Hi = float32(1 / math.Ln2)
	oneOverLn2Lo = float32(1/math.Ln2 - float64(float32(1/math.Ln2)))
	log10eHi     = float32(math.Log10E)
	log10eLo     = float32(math.Log10E - float64(float32(math.Log10E)))
	log10TwoHi   = math.Float32frombits(math.Float32bits(float32(math.Log10(2))) &^ 0xFF) // alt 8 bit sıfır
	log10TwoLo   = float32(math.Log10(2) - float64(log10TwoHi))
)

// taylor katsayıları (kesin formüllerle): sin, cos, atan, asin, sinh, log (atanh), expm1.
var (
	// sinCoef: sin r = r + r·z·P(z), P'nin katsayıları z^0'dan (z = r²).
	sinCoef = []float64{-1.0 / 6, 1.0 / 120, -1.0 / 5040, 1.0 / 362880}
	// cosCoef: cos r = 1 + z·Q(z).
	cosCoef = []float64{-0.5, 1.0 / 24, -1.0 / 720, 1.0 / 40320, -1.0 / 3628800}
	// atanCoef: atan u = u + u·z·R(z): (−1)^n/(2n+1), n = 1..8.
	atanCoef = func() []float64 {
		var c []float64
		for n := 1; n <= 8; n++ {
			c = append(c, math.Pow(-1, float64(n))/float64(2*n+1))
		}
		return c
	}()
	// asinCoef: asin s = s + s·z·S(z): (2n)!/(4^n·(n!)²·(2n+1)), n = 1..11.
	asinCoef = func() []float64 {
		var c []float64
		middle := 1.0 // (2n)!/(4^n (n!)^2) = Π (2i−1)/(2i)
		for n := 1; n <= 11; n++ {
			middle *= float64(2*n-1) / float64(2*n)
			c = append(c, middle/float64(2*n+1))
		}
		return c
	}()
	// sinhCoef: sinh x = x + x·z·H(z): 1/(2n+1)!, n = 1..5.
	sinhCoef = []float64{1.0 / 6, 1.0 / 120, 1.0 / 5040, 1.0 / 362880, 1.0 / 39916800}
	// logCoef: log m = 2s + 2s·z·L(z), s = f/(2+f): 1/(2n+1), n = 1..5.
	logCoef = []float64{1.0 / 3, 1.0 / 5, 1.0 / 7, 1.0 / 9, 1.0 / 11}
	// expm1Coef: e^r − 1 = r + r²·E(r): 1/n!, n = 2..9.
	expm1Coef = func() []float64 {
		var c []float64
		f := 1.0
		for n := 2; n <= 9; n++ {
			f *= float64(n)
			c = append(c, 1/f)
		}
		return c
	}()
)

// horner c[0] + z·(c[1] + z·(...)).
func (k *emitter) horner(z reg, c []float64) reg {
	s := k.constant(float32(c[len(c)-1]))
	for i := len(c) - 2; i >= 0; i-- {
		k.e("fma.rn.f32", s, s, z, float32(c[i]))
	}
	return s
}

// oddPoly x + x·z·P(z), z = x².
func (k *emitter) oddPoly(x reg, c []float64) reg {
	z := k.f()
	k.e("mul.rn.f32", z, x, x)
	p := k.horner(z, c)
	xz := k.f()
	k.e("mul.rn.f32", xz, x, z)
	r := k.f()
	k.e("fma.rn.f32", r, xz, p, x)
	return r
}

// copySign |r|'ye x'in işaretini verir (r ≥ 0 varsayılır).
func (k *emitter) copySign(r, x reg) reg {
	rb, xb := k.r(), k.r()
	k.e("mov.b32", rb, r)
	k.e("mov.b32", xb, x)
	k.e("and.b32", xb, xb, uint32(0x80000000))
	k.e("or.b32", rb, rb, xb)
	o := k.f()
	k.e("mov.b32", o, rb)
	return o
}

// signBit x'in işaret biti 1 mi (−0 dahil).
func (k *emitter) signBit(x reg) reg {
	b := k.r()
	k.e("mov.b32", b, x)
	p := k.p()
	k.e("setp.lt.s32", p, b, 0)
	return p
}

// pick p ? a : b.
func (k *emitter) pick(p reg, a, b any) reg {
	r := k.f()
	k.e("selp.f32", r, a, b, p)
	return r
}

// tangent tan x: j = yuvarla(x·2/π), r = x − j·π/2 (üç parça), j çiftse sin r / cos r,
// tekse −cos r / sin r.
func (k *emitter) tangent(x reg) reg {
	j := k.f()
	k.e("mul.rn.f32", j, x, twoOverPi)
	k.e("cvt.rni.f32.f32", j, j)
	nj := k.f()
	k.e("neg.f32", nj, j)
	r := k.f()
	k.e("fma.rn.f32", r, nj, piHalfPart[0], x)
	k.e("fma.rn.f32", r, nj, piHalfPart[1], r)
	k.e("fma.rn.f32", r, nj, piHalfPart[2], r)
	s := k.oddPoly(r, sinCoef)
	z := k.f()
	k.e("mul.rn.f32", z, r, r)
	c := k.horner(z, cosCoef)
	k.e("fma.rn.f32", c, z, c, float32(1))
	ji := k.r()
	k.e("cvt.rzi.s32.f32", ji, j)
	k.e("and.b32", ji, ji, 1)
	single := k.p()
	k.e("setp.ne.u32", single, ji, 0)
	nc := k.f()
	k.e("neg.f32", nc, c)
	share := k.pick(single, nc, s)
	denom := k.pick(single, s, c)
	o := k.f()
	k.e("div.rn.f32", o, share, denom)
	return o
}

// atanUnit atan a, a ∈ [0, 1]: a > tan(π/8) ise π/4 + atan((a−1)/(a+1)).
func (k *emitter) atanUnit(a reg) reg {
	middle := k.p()
	k.e("setp.gt.f32", middle, a, tanPiEighth)
	share, denom := k.f(), k.f()
	k.e("sub.rn.f32", share, a, float32(1))
	k.e("add.rn.f32", denom, a, float32(1))
	u := k.f()
	k.e("div.rn.f32", u, share, denom)
	u = k.pick(middle, u, a)
	p := k.oddPoly(u, atanCoef)
	base2 := k.pick(middle, piQuarter, float32(0))
	r := k.f()
	k.e("add.rn.f32", r, base2, p)
	return r
}

// arctan atan x = işaret(x)·(|x| > 1 ise π/2 − atan(1/|x|), değilse atan|x|).
func (k *emitter) arctan(x reg) reg {
	a := k.f()
	k.e("abs.f32", a, x)
	large := k.p()
	k.e("setp.gt.f32", large, a, float32(1))
	t := k.f()
	k.e("rcp.rn.f32", t, a)
	t = k.pick(large, t, a)
	r := k.atanUnit(t)
	q := k.f()
	k.e("sub.rn.f32", q, piHalf32, r)
	r = k.pick(large, q, r)
	return k.copySign(r, x)
}

// arctan2 atan2(y, x): a = min/max(|x|, |y|) ∈ [0, 1] üzerinden; bölge düzeltmeleri
// işaret bitleriyle (işaretli sıfırlar ve sonsuzlar IEEE kuralıyla), NaN yayılır.
func (k *emitter) arctan2(y, x reg) reg {
	ax, ay := k.f(), k.f()
	k.e("abs.f32", ax, x)
	k.e("abs.f32", ay, y)
	mx, mn := k.f(), k.f()
	k.e("max.f32", mx, ax, ay)
	k.e("min.f32", mn, ax, ay)
	a := k.f()
	k.e("div.rn.f32", a, mn, mx)
	zero, inf2 := k.p(), k.p()
	k.e("setp.eq.f32", zero, mx, float32(0))
	k.e("setp.eq.f32", inf2, mn, float32(math.Inf(1)))
	a = k.pick(zero, float32(0), a)
	a = k.pick(inf2, float32(1), a)
	r := k.atanUnit(a)
	perpendicular := k.p()
	k.e("setp.gt.f32", perpendicular, ay, ax)
	q := k.f()
	k.e("sub.rn.f32", q, piHalf32, r)
	r = k.pick(perpendicular, q, r)
	q2 := k.f()
	k.e("sub.rn.f32", q2, pi32, r)
	r = k.pick(k.signBit(x), q2, r)
	r = k.copySign(r, y)
	nan := k.p()
	k.e("setp.nan.f32", nan, x, y)
	return k.pick(nan, float32(math.NaN()), r)
}

// asinPart |x|'ten asin ve acos'un ortak parçası: büyük (|x| > 0,5) ise p = asin(√((1−|x|)/2)),
// değilse p = asin|x|.
func (k *emitter) asinPart(x reg) (p, large reg) {
	a := k.f()
	k.e("abs.f32", a, x)
	large = k.p()
	k.e("setp.gt.f32", large, a, float32(0.5))
	zb := k.f()
	k.e("sub.rn.f32", zb, float32(1), a)
	k.e("mul.rn.f32", zb, zb, float32(0.5))
	zk := k.f()
	k.e("mul.rn.f32", zk, a, a)
	z := k.pick(large, zb, zk)
	sk := k.f()
	k.e("sqrt.rn.f32", sk, zb)
	s := k.pick(large, sk, a)
	h := k.horner(z, asinCoef)
	sz := k.f()
	k.e("mul.rn.f32", sz, s, z)
	p = k.f()
	k.e("fma.rn.f32", p, sz, h, s)
	return p, large
}

// arcsin asin x = işaret(x)·(büyükse π/2 − 2p, değilse p).
func (k *emitter) arcsin(x reg) reg {
	p, large := k.asinPart(x)
	q := k.f()
	k.e("fma.rn.f32", q, p, float32(-2), piHalf32)
	return k.copySign(k.pick(large, q, p), x)
}

// arccos acos x: büyükse (x > 0: 2p, x < 0: π − 2p), değilse π/2 − işaret(x)·p.
func (k *emitter) arccos(x reg) reg {
	p, large := k.asinPart(x)
	two := k.f()
	k.e("add.rn.f32", two, p, p)
	inverse := k.f()
	k.e("sub.rn.f32", inverse, pi32, two)
	neg := k.p()
	k.e("setp.lt.f32", neg, x, float32(0))
	b := k.pick(neg, inverse, two)
	sp := k.copySign(p, x)
	kc := k.f()
	k.e("sub.rn.f32", kc, piHalf32, sp)
	return k.pick(large, b, kc)
}

// halfExp q = e^|x|/2: |x| < 88'de 0,5·e^|x|, üstünde (e^{|x|/2}·0,5)·e^{|x|/2}
// (e^|x| float32'de taşsa da q taşmadan gösterilebilir).
func (k *emitter) halfExp(x reg) reg {
	a := k.f()
	k.e("abs.f32", a, x)
	e := k.exponential(a)
	k.e("mul.rn.f32", e, e, float32(0.5))
	ay := k.f()
	k.e("mul.rn.f32", ay, a, float32(0.5))
	h := k.exponential(ay)
	hy := k.f()
	k.e("mul.rn.f32", hy, h, float32(0.5))
	k.e("mul.rn.f32", hy, hy, h)
	small := k.p()
	k.e("setp.lt.f32", small, a, float32(88))
	return k.pick(small, e, hy)
}

// hypSin sinh x: |x| < 1'de Taylor, değilse işaret(x)·(q − 1/(4q)).
func (k *emitter) hypSin(x reg) reg {
	q := k.halfExp(x)
	t := k.f()
	k.e("rcp.rn.f32", t, q)
	b := k.f()
	k.e("fma.rn.f32", b, t, float32(-0.25), q)
	b = k.copySign(b, x)
	kp := k.oddPoly(x, sinhCoef)
	a := k.f()
	k.e("abs.f32", a, x)
	small := k.p()
	k.e("setp.lt.f32", small, a, float32(1))
	return k.pick(small, kp, b)
}

// hypCos cosh x = q + 1/(4q).
func (k *emitter) hypCos(x reg) reg {
	q := k.halfExp(x)
	t := k.f()
	k.e("rcp.rn.f32", t, q)
	r := k.f()
	k.e("fma.rn.f32", r, t, float32(0.25), q)
	return r
}

// logPart u = 2^e·m (m ∈ [√½, √2)) ayrıştırması: e (float32 tam sayı) ve log m
// (s = (m−1)/(m+1) ile 2s + 2s·s²·L(s²)). Normal altı u önce 2^23 ile ölçeklenir. Özel
// değerleri (0, negatif, sonsuz, NaN) çağıran ayrıca ele alır.
func (k *emitter) logPart(u reg) (e, lm reg) {
	sub := k.p()
	k.e("setp.lt.f32", sub, u, float32(1.17549435e-38))
	ol := k.f()
	k.e("mul.rn.f32", ol, u, float32(8388608))
	v := k.pick(sub, ol, u)
	b := k.r()
	k.e("mov.b32", b, v)
	ei := k.r()
	k.e("shr.u32", ei, b, 23)
	k.e("and.b32", ei, ei, 0xFF)
	k.e("sub.s32", ei, ei, 127)
	ek := k.r()
	k.e("selp.b32", ek, -23, 0, sub)
	k.e("add.s32", ei, ei, ek)
	mb := k.r()
	k.e("and.b32", mb, b, 0x007FFFFF)
	k.e("or.b32", mb, mb, 0x3F800000)
	m := k.f()
	k.e("mov.b32", m, mb)
	high := k.p()
	k.e("setp.gt.f32", high, m, root2)
	mh := k.f()
	k.e("mul.rn.f32", mh, m, float32(0.5))
	m = k.pick(high, mh, m)
	ea := k.r()
	k.e("selp.b32", ea, 1, 0, high)
	k.e("add.s32", ei, ei, ea)
	e = k.f()
	k.e("cvt.rn.f32.s32", e, ei)
	f := k.f()
	k.e("sub.rn.f32", f, m, float32(1))
	d := k.f()
	k.e("add.rn.f32", d, f, float32(2))
	s := k.f()
	k.e("div.rn.f32", s, f, d)
	z := k.f()
	k.e("mul.rn.f32", z, s, s)
	l := k.horner(z, logCoef)
	s2 := k.f()
	k.e("add.rn.f32", s2, s, s)
	sz := k.f()
	k.e("mul.rn.f32", sz, s2, z)
	lm = k.f()
	k.e("fma.rn.f32", lm, sz, l, s2)
	return e, lm
}

// logSpecial u'nun özel değerlerinde sonucu düzeltir: 0 → −sonsuz, +sonsuz → +sonsuz,
// negatif ya da NaN → NaN.
func (k *emitter) logSpecial(u, r reg) reg {
	zero, inf2, bad := k.p(), k.p(), k.p()
	k.e("setp.eq.f32", zero, u, float32(0))
	k.e("setp.eq.f32", inf2, u, float32(math.Inf(1)))
	k.e("setp.ltu.f32", bad, u, float32(0)) // negatif ya da NaN
	r = k.pick(zero, float32(math.Inf(-1)), r)
	r = k.pick(inf2, float32(math.Inf(1)), r)
	return k.pick(bad, float32(math.NaN()), r)
}

// lnMerge e·ln2 + lm + c (iki parçalı ln 2).
func (k *emitter) lnMerge(e, lm, c reg) reg {
	t := k.f()
	k.e("add.rn.f32", t, lm, c)
	k.e("fma.rn.f32", t, e, ln2Lo, t)
	r := k.f()
	k.e("fma.rn.f32", r, e, ln2Hi, t)
	return r
}

// log1Plus log(1 + x): u = 1 + x; u = 1 ise x; değilse log u + (x − (u − 1))/u (u'nun
// yuvarlama hatasının birinci derece düzeltmesi).
func (k *emitter) log1Plus(x reg) reg {
	u := k.f()
	k.e("add.rn.f32", u, x, float32(1))
	e, lm := k.logPart(u)
	um := k.f()
	k.e("sub.rn.f32", um, u, float32(1))
	c := k.f()
	k.e("sub.rn.f32", c, x, um)
	k.e("div.rn.f32", c, c, u)
	r := k.logSpecial(u, k.lnMerge(e, lm, c))
	one := k.p()
	k.e("setp.eq.f32", one, u, float32(1))
	return k.pick(one, x, r)
}

// log2Precise log2 u = e + lm/ln 2 (ikinin kuvvetlerinde tam).
func (k *emitter) log2Precise(u reg) reg {
	e, lm := k.logPart(u)
	t := k.f()
	k.e("fma.rn.f32", t, lm, oneOverLn2Lo, e)
	r := k.f()
	k.e("fma.rn.f32", r, lm, oneOverLn2Hi, t)
	return k.logSpecial(u, r)
}

// log10Precise log10 u = e·log10(2) + lm·log10(e) (iki parçalı sabitler).
func (k *emitter) log10Precise(u reg) reg {
	e, lm := k.logPart(u)
	t := k.f()
	k.e("mul.rn.f32", t, e, log10TwoLo)
	k.e("fma.rn.f32", t, lm, log10eLo, t)
	k.e("fma.rn.f32", t, lm, log10eHi, t)
	r := k.f()
	k.e("fma.rn.f32", r, e, log10TwoHi, t)
	return k.logSpecial(u, r)
}

// expMinusOne e^x − 1: |x| ≤ ln2/2'de r + r²·E(r); x > 80'de e^x; x < −17'de −1; arada
// x = n·ln2 + r ile 2^n·(e^r − 1) + (2^n − 1).
func (k *emitter) expMinusOne(x reg) reg {
	em := func(r reg) reg {
		r2 := k.f()
		k.e("mul.rn.f32", r2, r, r)
		p := k.horner(r, expm1Coef)
		o := k.f()
		k.e("fma.rn.f32", o, r2, p, r)
		return o
	}
	smallResult := em(x)
	n := k.f()
	k.e("mul.rn.f32", n, x, log2e)
	k.e("cvt.rni.f32.f32", n, n)
	k.e("min.f32", n, n, float32(127))
	k.e("max.f32", n, n, float32(-126))
	nn := k.f()
	k.e("neg.f32", nn, n)
	r := k.f()
	k.e("fma.rn.f32", r, nn, ln2Hi, x)
	k.e("fma.rn.f32", r, nn, ln2Lo, r)
	er := em(r)
	ni := k.r()
	k.e("cvt.rzi.s32.f32", ni, n)
	k.e("add.s32", ni, ni, 127)
	k.e("shl.b32", ni, ni, 23)
	s := k.f()
	k.e("mov.b32", s, ni)
	s1 := k.f()
	k.e("sub.rn.f32", s1, s, float32(1))
	middle := k.f()
	k.e("fma.rn.f32", middle, s, er, s1)
	a := k.f()
	k.e("abs.f32", a, x)
	small, large, minus := k.p(), k.p(), k.p()
	k.e("setp.le.f32", small, a, float32(0.34657359))
	k.e("setp.gt.f32", large, x, float32(80))
	k.e("setp.lt.f32", minus, x, float32(-17))
	o := k.pick(small, smallResult, middle)
	o = k.pick(large, k.exponential(x), o)
	o = k.pick(minus, float32(-1), o)
	nan := k.p()
	k.e("setp.nan.f32", nan, x, x)
	return k.pick(nan, x, o)
}

// silu x·σ(x); σ(x) = 0 iken (x → −sonsuz) 0 (CPU siluIleri ile aynı).
func (k *emitter) silu(x reg) reg {
	s := k.sigmoid(x)
	r := k.f()
	k.e("mul.rn.f32", r, x, s)
	zero := k.p()
	k.e("setp.eq.f32", zero, s, float32(0))
	return k.pick(zero, float32(0), r)
}

// siluGrad s + x·s(1−s); s(1−s) = 0 iken s.
func (k *emitter) siluGrad(x reg) reg {
	s := k.sigmoid(x)
	q := k.f()
	k.e("sub.rn.f32", q, float32(1), s)
	k.e("mul.rn.f32", q, s, q)
	d := k.f()
	k.e("mul.rn.f32", d, x, q)
	k.e("add.rn.f32", d, s, d)
	zero := k.p()
	k.e("setp.eq.f32", zero, q, float32(0))
	return k.pick(zero, s, d)
}

// signF işaret: x > 0 → 1, x < 0 → −1, sıfır ve NaN olduğu gibi.
func (k *emitter) signF(x reg) reg {
	posN, neg := k.p(), k.p()
	k.e("setp.gt.f32", posN, x, float32(0))
	k.e("setp.lt.f32", neg, x, float32(0))
	r := k.pick(posN, float32(1), x)
	return k.pick(neg, float32(-1), r)
}
