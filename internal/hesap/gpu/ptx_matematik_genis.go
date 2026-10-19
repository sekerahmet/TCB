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

// ucParca v'yi float32 toplamı v'ye 72 bit kadar yaklaşan üç parçaya böler.
func ucParca(v *big.Float) [3]float32 {
	var p [3]float32
	kalan := new(big.Float).SetPrec(200).Set(v)
	for i := range p {
		f, _ := kalan.Float64()
		p[i] = float32(f)
		kalan.Sub(kalan, new(big.Float).SetFloat64(float64(p[i])))
	}
	return p
}

var (
	// piYarimParca π/2'nin üç parçası (Cody-Waite indirgemesi için).
	piYarimParca = func() [3]float32 {
		pi, _, _ := big.ParseFloat("3.14159265358979323846264338327950288419716939937510582097494459", 10, 200, big.ToNearestEven)
		return ucParca(pi.Quo(pi, big.NewFloat(2)))
	}()
	piYarim32  = float32(math.Pi / 2)
	pi32       = float32(math.Pi)
	piCeyrek   = float32(math.Pi / 4)
	ikiBoluPi  = float32(2 / math.Pi)
	tanPiSekiz = float32(0.41421356237309503) // tan(π/8)
	kok2       = float32(math.Sqrt2)
	// ln 2'nin iki parçası: üst parça 9 bitlik (e·üst, |e| < 2^15 için tam).
	ln2Ust = float32(0.693359375)
	ln2Alt = float32(math.Ln2 - 0.693359375)
	// 1/ln 2 ve log10(e), log10(2) parçaları (log2 ve log10 için).
	birBoluLn2Ust = float32(1 / math.Ln2)
	birBoluLn2Alt = float32(1/math.Ln2 - float64(float32(1/math.Ln2)))
	log10eUst     = float32(math.Log10E)
	log10eAlt     = float32(math.Log10E - float64(float32(math.Log10E)))
	log10ikiUst   = math.Float32frombits(math.Float32bits(float32(math.Log10(2))) &^ 0xFF) // alt 8 bit sıfır
	log10ikiAlt   = float32(math.Log10(2) - float64(log10ikiUst))
)

// taylor katsayıları (kesin formüllerle): sin, cos, atan, asin, sinh, log (atanh), expm1.
var (
	// sinKat: sin r = r + r·z·P(z), P'nin katsayıları z^0'dan (z = r²).
	sinKat = []float64{-1.0 / 6, 1.0 / 120, -1.0 / 5040, 1.0 / 362880}
	// cosKat: cos r = 1 + z·Q(z).
	cosKat = []float64{-0.5, 1.0 / 24, -1.0 / 720, 1.0 / 40320, -1.0 / 3628800}
	// atanKat: atan u = u + u·z·R(z): (−1)^n/(2n+1), n = 1..8.
	atanKat = func() []float64 {
		var c []float64
		for n := 1; n <= 8; n++ {
			c = append(c, math.Pow(-1, float64(n))/float64(2*n+1))
		}
		return c
	}()
	// asinKat: asin s = s + s·z·S(z): (2n)!/(4^n·(n!)²·(2n+1)), n = 1..11.
	asinKat = func() []float64 {
		var c []float64
		orta := 1.0 // (2n)!/(4^n (n!)^2) = Π (2i−1)/(2i)
		for n := 1; n <= 11; n++ {
			orta *= float64(2*n-1) / float64(2*n)
			c = append(c, orta/float64(2*n+1))
		}
		return c
	}()
	// sinhKat: sinh x = x + x·z·H(z): 1/(2n+1)!, n = 1..5.
	sinhKat = []float64{1.0 / 6, 1.0 / 120, 1.0 / 5040, 1.0 / 362880, 1.0 / 39916800}
	// logKat: log m = 2s + 2s·z·L(z), s = f/(2+f): 1/(2n+1), n = 1..5.
	logKat = []float64{1.0 / 3, 1.0 / 5, 1.0 / 7, 1.0 / 9, 1.0 / 11}
	// expm1Kat: e^r − 1 = r + r²·E(r): 1/n!, n = 2..9.
	expm1Kat = func() []float64 {
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
func (k *yazici) horner(z yazmac, c []float64) yazmac {
	s := k.sabit(float32(c[len(c)-1]))
	for i := len(c) - 2; i >= 0; i-- {
		k.e("fma.rn.f32", s, s, z, float32(c[i]))
	}
	return s
}

// tekPolinom x + x·z·P(z), z = x².
func (k *yazici) tekPolinom(x yazmac, c []float64) yazmac {
	z := k.f()
	k.e("mul.rn.f32", z, x, x)
	p := k.horner(z, c)
	xz := k.f()
	k.e("mul.rn.f32", xz, x, z)
	r := k.f()
	k.e("fma.rn.f32", r, xz, p, x)
	return r
}

// isaretKopyala |r|'ye x'in işaretini verir (r ≥ 0 varsayılır).
func (k *yazici) isaretKopyala(r, x yazmac) yazmac {
	rb, xb := k.r(), k.r()
	k.e("mov.b32", rb, r)
	k.e("mov.b32", xb, x)
	k.e("and.b32", xb, xb, uint32(0x80000000))
	k.e("or.b32", rb, rb, xb)
	o := k.f()
	k.e("mov.b32", o, rb)
	return o
}

// isaretBiti x'in işaret biti 1 mi (−0 dahil).
func (k *yazici) isaretBiti(x yazmac) yazmac {
	b := k.r()
	k.e("mov.b32", b, x)
	p := k.p()
	k.e("setp.lt.s32", p, b, 0)
	return p
}

// sec p ? a : b.
func (k *yazici) sec(p yazmac, a, b any) yazmac {
	r := k.f()
	k.e("selp.f32", r, a, b, p)
	return r
}

// tanjant tan x: j = yuvarla(x·2/π), r = x − j·π/2 (üç parça), j çiftse sin r / cos r,
// tekse −cos r / sin r.
func (k *yazici) tanjant(x yazmac) yazmac {
	j := k.f()
	k.e("mul.rn.f32", j, x, ikiBoluPi)
	k.e("cvt.rni.f32.f32", j, j)
	nj := k.f()
	k.e("neg.f32", nj, j)
	r := k.f()
	k.e("fma.rn.f32", r, nj, piYarimParca[0], x)
	k.e("fma.rn.f32", r, nj, piYarimParca[1], r)
	k.e("fma.rn.f32", r, nj, piYarimParca[2], r)
	s := k.tekPolinom(r, sinKat)
	z := k.f()
	k.e("mul.rn.f32", z, r, r)
	c := k.horner(z, cosKat)
	k.e("fma.rn.f32", c, z, c, float32(1))
	ji := k.r()
	k.e("cvt.rzi.s32.f32", ji, j)
	k.e("and.b32", ji, ji, 1)
	tek := k.p()
	k.e("setp.ne.u32", tek, ji, 0)
	nc := k.f()
	k.e("neg.f32", nc, c)
	pay := k.sec(tek, nc, s)
	payda := k.sec(tek, s, c)
	o := k.f()
	k.e("div.rn.f32", o, pay, payda)
	return o
}

// atanBirim atan a, a ∈ [0, 1]: a > tan(π/8) ise π/4 + atan((a−1)/(a+1)).
func (k *yazici) atanBirim(a yazmac) yazmac {
	orta := k.p()
	k.e("setp.gt.f32", orta, a, tanPiSekiz)
	pay, payda := k.f(), k.f()
	k.e("sub.rn.f32", pay, a, float32(1))
	k.e("add.rn.f32", payda, a, float32(1))
	u := k.f()
	k.e("div.rn.f32", u, pay, payda)
	u = k.sec(orta, u, a)
	p := k.tekPolinom(u, atanKat)
	taban := k.sec(orta, piCeyrek, float32(0))
	r := k.f()
	k.e("add.rn.f32", r, taban, p)
	return r
}

// arktanjant atan x = işaret(x)·(|x| > 1 ise π/2 − atan(1/|x|), değilse atan|x|).
func (k *yazici) arktanjant(x yazmac) yazmac {
	a := k.f()
	k.e("abs.f32", a, x)
	buyuk := k.p()
	k.e("setp.gt.f32", buyuk, a, float32(1))
	t := k.f()
	k.e("rcp.rn.f32", t, a)
	t = k.sec(buyuk, t, a)
	r := k.atanBirim(t)
	q := k.f()
	k.e("sub.rn.f32", q, piYarim32, r)
	r = k.sec(buyuk, q, r)
	return k.isaretKopyala(r, x)
}

// arktanjant2 atan2(y, x): a = min/max(|x|, |y|) ∈ [0, 1] üzerinden; bölge düzeltmeleri
// işaret bitleriyle (işaretli sıfırlar ve sonsuzlar IEEE kuralıyla), NaN yayılır.
func (k *yazici) arktanjant2(y, x yazmac) yazmac {
	ax, ay := k.f(), k.f()
	k.e("abs.f32", ax, x)
	k.e("abs.f32", ay, y)
	mx, mn := k.f(), k.f()
	k.e("max.f32", mx, ax, ay)
	k.e("min.f32", mn, ax, ay)
	a := k.f()
	k.e("div.rn.f32", a, mn, mx)
	sifir, sonsuz := k.p(), k.p()
	k.e("setp.eq.f32", sifir, mx, float32(0))
	k.e("setp.eq.f32", sonsuz, mn, float32(math.Inf(1)))
	a = k.sec(sifir, float32(0), a)
	a = k.sec(sonsuz, float32(1), a)
	r := k.atanBirim(a)
	dik := k.p()
	k.e("setp.gt.f32", dik, ay, ax)
	q := k.f()
	k.e("sub.rn.f32", q, piYarim32, r)
	r = k.sec(dik, q, r)
	q2 := k.f()
	k.e("sub.rn.f32", q2, pi32, r)
	r = k.sec(k.isaretBiti(x), q2, r)
	r = k.isaretKopyala(r, y)
	nan := k.p()
	k.e("setp.nan.f32", nan, x, y)
	return k.sec(nan, float32(math.NaN()), r)
}

// asinParca |x|'ten asin ve acos'un ortak parçası: büyük (|x| > 0,5) ise p = asin(√((1−|x|)/2)),
// değilse p = asin|x|.
func (k *yazici) asinParca(x yazmac) (p, buyuk yazmac) {
	a := k.f()
	k.e("abs.f32", a, x)
	buyuk = k.p()
	k.e("setp.gt.f32", buyuk, a, float32(0.5))
	zb := k.f()
	k.e("sub.rn.f32", zb, float32(1), a)
	k.e("mul.rn.f32", zb, zb, float32(0.5))
	zk := k.f()
	k.e("mul.rn.f32", zk, a, a)
	z := k.sec(buyuk, zb, zk)
	sk := k.f()
	k.e("sqrt.rn.f32", sk, zb)
	s := k.sec(buyuk, sk, a)
	h := k.horner(z, asinKat)
	sz := k.f()
	k.e("mul.rn.f32", sz, s, z)
	p = k.f()
	k.e("fma.rn.f32", p, sz, h, s)
	return p, buyuk
}

// arksinus asin x = işaret(x)·(büyükse π/2 − 2p, değilse p).
func (k *yazici) arksinus(x yazmac) yazmac {
	p, buyuk := k.asinParca(x)
	q := k.f()
	k.e("fma.rn.f32", q, p, float32(-2), piYarim32)
	return k.isaretKopyala(k.sec(buyuk, q, p), x)
}

// arkkosinus acos x: büyükse (x > 0: 2p, x < 0: π − 2p), değilse π/2 − işaret(x)·p.
func (k *yazici) arkkosinus(x yazmac) yazmac {
	p, buyuk := k.asinParca(x)
	iki := k.f()
	k.e("add.rn.f32", iki, p, p)
	ters := k.f()
	k.e("sub.rn.f32", ters, pi32, iki)
	neg := k.p()
	k.e("setp.lt.f32", neg, x, float32(0))
	b := k.sec(neg, ters, iki)
	sp := k.isaretKopyala(p, x)
	kc := k.f()
	k.e("sub.rn.f32", kc, piYarim32, sp)
	return k.sec(buyuk, b, kc)
}

// yarimUstel q = e^|x|/2: |x| < 88'de 0,5·e^|x|, üstünde (e^{|x|/2}·0,5)·e^{|x|/2}
// (e^|x| float32'de taşsa da q taşmadan gösterilebilir).
func (k *yazici) yarimUstel(x yazmac) yazmac {
	a := k.f()
	k.e("abs.f32", a, x)
	e := k.ustel(a)
	k.e("mul.rn.f32", e, e, float32(0.5))
	ay := k.f()
	k.e("mul.rn.f32", ay, a, float32(0.5))
	h := k.ustel(ay)
	hy := k.f()
	k.e("mul.rn.f32", hy, h, float32(0.5))
	k.e("mul.rn.f32", hy, hy, h)
	kucuk := k.p()
	k.e("setp.lt.f32", kucuk, a, float32(88))
	return k.sec(kucuk, e, hy)
}

// hipSinus sinh x: |x| < 1'de Taylor, değilse işaret(x)·(q − 1/(4q)).
func (k *yazici) hipSinus(x yazmac) yazmac {
	q := k.yarimUstel(x)
	t := k.f()
	k.e("rcp.rn.f32", t, q)
	b := k.f()
	k.e("fma.rn.f32", b, t, float32(-0.25), q)
	b = k.isaretKopyala(b, x)
	kp := k.tekPolinom(x, sinhKat)
	a := k.f()
	k.e("abs.f32", a, x)
	kucuk := k.p()
	k.e("setp.lt.f32", kucuk, a, float32(1))
	return k.sec(kucuk, kp, b)
}

// hipKosinus cosh x = q + 1/(4q).
func (k *yazici) hipKosinus(x yazmac) yazmac {
	q := k.yarimUstel(x)
	t := k.f()
	k.e("rcp.rn.f32", t, q)
	r := k.f()
	k.e("fma.rn.f32", r, t, float32(0.25), q)
	return r
}

// logParca u = 2^e·m (m ∈ [√½, √2)) ayrıştırması: e (float32 tam sayı) ve log m
// (s = (m−1)/(m+1) ile 2s + 2s·s²·L(s²)). Normal altı u önce 2^23 ile ölçeklenir. Özel
// değerleri (0, negatif, sonsuz, NaN) çağıran ayrıca ele alır.
func (k *yazici) logParca(u yazmac) (e, lm yazmac) {
	alt := k.p()
	k.e("setp.lt.f32", alt, u, float32(1.17549435e-38))
	ol := k.f()
	k.e("mul.rn.f32", ol, u, float32(8388608))
	v := k.sec(alt, ol, u)
	b := k.r()
	k.e("mov.b32", b, v)
	ei := k.r()
	k.e("shr.u32", ei, b, 23)
	k.e("and.b32", ei, ei, 0xFF)
	k.e("sub.s32", ei, ei, 127)
	ek := k.r()
	k.e("selp.b32", ek, -23, 0, alt)
	k.e("add.s32", ei, ei, ek)
	mb := k.r()
	k.e("and.b32", mb, b, 0x007FFFFF)
	k.e("or.b32", mb, mb, 0x3F800000)
	m := k.f()
	k.e("mov.b32", m, mb)
	yuksek := k.p()
	k.e("setp.gt.f32", yuksek, m, kok2)
	mh := k.f()
	k.e("mul.rn.f32", mh, m, float32(0.5))
	m = k.sec(yuksek, mh, m)
	ea := k.r()
	k.e("selp.b32", ea, 1, 0, yuksek)
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
	l := k.horner(z, logKat)
	s2 := k.f()
	k.e("add.rn.f32", s2, s, s)
	sz := k.f()
	k.e("mul.rn.f32", sz, s2, z)
	lm = k.f()
	k.e("fma.rn.f32", lm, sz, l, s2)
	return e, lm
}

// logOzel u'nun özel değerlerinde sonucu düzeltir: 0 → −sonsuz, +sonsuz → +sonsuz,
// negatif ya da NaN → NaN.
func (k *yazici) logOzel(u, r yazmac) yazmac {
	sifir, sonsuz, kotu := k.p(), k.p(), k.p()
	k.e("setp.eq.f32", sifir, u, float32(0))
	k.e("setp.eq.f32", sonsuz, u, float32(math.Inf(1)))
	k.e("setp.ltu.f32", kotu, u, float32(0)) // negatif ya da NaN
	r = k.sec(sifir, float32(math.Inf(-1)), r)
	r = k.sec(sonsuz, float32(math.Inf(1)), r)
	return k.sec(kotu, float32(math.NaN()), r)
}

// lnBirlestir e·ln2 + lm + c (iki parçalı ln 2).
func (k *yazici) lnBirlestir(e, lm, c yazmac) yazmac {
	t := k.f()
	k.e("add.rn.f32", t, lm, c)
	k.e("fma.rn.f32", t, e, ln2Alt, t)
	r := k.f()
	k.e("fma.rn.f32", r, e, ln2Ust, t)
	return r
}

// log1Arti log(1 + x): u = 1 + x; u = 1 ise x; değilse log u + (x − (u − 1))/u (u'nun
// yuvarlama hatasının birinci derece düzeltmesi).
func (k *yazici) log1Arti(x yazmac) yazmac {
	u := k.f()
	k.e("add.rn.f32", u, x, float32(1))
	e, lm := k.logParca(u)
	um := k.f()
	k.e("sub.rn.f32", um, u, float32(1))
	c := k.f()
	k.e("sub.rn.f32", c, x, um)
	k.e("div.rn.f32", c, c, u)
	r := k.logOzel(u, k.lnBirlestir(e, lm, c))
	bir := k.p()
	k.e("setp.eq.f32", bir, u, float32(1))
	return k.sec(bir, x, r)
}

// log2Hassas log2 u = e + lm/ln 2 (ikinin kuvvetlerinde tam).
func (k *yazici) log2Hassas(u yazmac) yazmac {
	e, lm := k.logParca(u)
	t := k.f()
	k.e("fma.rn.f32", t, lm, birBoluLn2Alt, e)
	r := k.f()
	k.e("fma.rn.f32", r, lm, birBoluLn2Ust, t)
	return k.logOzel(u, r)
}

// log10Hassas log10 u = e·log10(2) + lm·log10(e) (iki parçalı sabitler).
func (k *yazici) log10Hassas(u yazmac) yazmac {
	e, lm := k.logParca(u)
	t := k.f()
	k.e("mul.rn.f32", t, e, log10ikiAlt)
	k.e("fma.rn.f32", t, lm, log10eAlt, t)
	k.e("fma.rn.f32", t, lm, log10eUst, t)
	r := k.f()
	k.e("fma.rn.f32", r, e, log10ikiUst, t)
	return k.logOzel(u, r)
}

// ustelEksiBir e^x − 1: |x| ≤ ln2/2'de r + r²·E(r); x > 80'de e^x; x < −17'de −1; arada
// x = n·ln2 + r ile 2^n·(e^r − 1) + (2^n − 1).
func (k *yazici) ustelEksiBir(x yazmac) yazmac {
	em := func(r yazmac) yazmac {
		r2 := k.f()
		k.e("mul.rn.f32", r2, r, r)
		p := k.horner(r, expm1Kat)
		o := k.f()
		k.e("fma.rn.f32", o, r2, p, r)
		return o
	}
	kucukSonuc := em(x)
	n := k.f()
	k.e("mul.rn.f32", n, x, log2e)
	k.e("cvt.rni.f32.f32", n, n)
	k.e("min.f32", n, n, float32(127))
	k.e("max.f32", n, n, float32(-126))
	nn := k.f()
	k.e("neg.f32", nn, n)
	r := k.f()
	k.e("fma.rn.f32", r, nn, ln2Ust, x)
	k.e("fma.rn.f32", r, nn, ln2Alt, r)
	er := em(r)
	ni := k.r()
	k.e("cvt.rzi.s32.f32", ni, n)
	k.e("add.s32", ni, ni, 127)
	k.e("shl.b32", ni, ni, 23)
	s := k.f()
	k.e("mov.b32", s, ni)
	s1 := k.f()
	k.e("sub.rn.f32", s1, s, float32(1))
	orta := k.f()
	k.e("fma.rn.f32", orta, s, er, s1)
	a := k.f()
	k.e("abs.f32", a, x)
	kucuk, buyuk, eksi := k.p(), k.p(), k.p()
	k.e("setp.le.f32", kucuk, a, float32(0.34657359))
	k.e("setp.gt.f32", buyuk, x, float32(80))
	k.e("setp.lt.f32", eksi, x, float32(-17))
	o := k.sec(kucuk, kucukSonuc, orta)
	o = k.sec(buyuk, k.ustel(x), o)
	o = k.sec(eksi, float32(-1), o)
	nan := k.p()
	k.e("setp.nan.f32", nan, x, x)
	return k.sec(nan, x, o)
}

// siluSigma σ(x) = e^{-|x|} ile sayısal kararlı biçimde (x ≥ 0: 1/(1+e), değilse e/(1+e)).
func (k *yazici) siluSigma(x yazmac) yazmac {
	a := k.f()
	k.e("abs.f32", a, x)
	k.e("neg.f32", a, a)
	e := k.ustel(a)
	b := k.f()
	k.e("add.rn.f32", b, e, float32(1))
	r1, r2 := k.f(), k.f()
	k.e("div.rn.f32", r1, float32(1), b)
	k.e("div.rn.f32", r2, e, b)
	p := k.p()
	k.e("setp.ge.f32", p, x, float32(0))
	return k.sec(p, r1, r2)
}

// silu x·σ(x); σ(x) = 0 iken (x → −sonsuz) 0 (CPU siluIleri ile aynı).
func (k *yazici) silu(x yazmac) yazmac {
	s := k.siluSigma(x)
	r := k.f()
	k.e("mul.rn.f32", r, x, s)
	sifir := k.p()
	k.e("setp.eq.f32", sifir, s, float32(0))
	return k.sec(sifir, float32(0), r)
}

// siluTurev s + x·s(1−s); s(1−s) = 0 iken s.
func (k *yazici) siluTurev(x yazmac) yazmac {
	s := k.siluSigma(x)
	q := k.f()
	k.e("sub.rn.f32", q, float32(1), s)
	k.e("mul.rn.f32", q, s, q)
	d := k.f()
	k.e("mul.rn.f32", d, x, q)
	k.e("add.rn.f32", d, s, d)
	sifir := k.p()
	k.e("setp.eq.f32", sifir, q, float32(0))
	return k.sec(sifir, s, d)
}

// isaretF işaret: x > 0 → 1, x < 0 → −1, sıfır ve NaN olduğu gibi.
func (k *yazici) isaretF(x yazmac) yazmac {
	poz, neg := k.p(), k.p()
	k.e("setp.gt.f32", poz, x, float32(0))
	k.e("setp.lt.f32", neg, x, float32(0))
	r := k.sec(poz, float32(1), x)
	return k.sec(neg, float32(-1), r)
}
