package gpu

import (
	"math"

	"tr_dili/internal/hesap/tensor"
)

// PTX matematik ve tür yardımcıları. Donanımın yaklaşık komutları (ex2.approx,
// lg2.approx, sin.approx) aralık indirgemesi ve düzeltme terimleriyle kullanılır;
// sonuçlar CPU arka ucuyla birkaç ulp içinde uyuşur. Kaynaştırılmamış çarpma ve toplama
// gereken yerlerde (CPU çekirdeğinin işlem sırası) .rn biçimleri yazılır: .rn'siz mul/add
// çiftlerini derleyici fma'ya birleştirebilir.

var (
	log2e   = float32(math.Log2E)
	log2eAl = float32(math.Log2E - float64(float32(math.Log2E)))
	ln2     = float32(math.Ln2)
	ikiPi   = float32(2 * math.Pi)
	ikiPiAl = float32(2*math.Pi - float64(float32(2*math.Pi)))
	birBolu = float32(1 / (2 * math.Pi))
)

// ustel e^x: t = x·log2e, düzeltme t_al = fma(x, log2e, -t) + x·log2e_al ve
// 2^t·(1 + t_al·ln2). |t| > 126'da düzeltme sıfırlanır (taşma, -sonsuz ve NaN için).
func (k *yazici) ustel(x yazmac) yazmac {
	t := k.f()
	k.e("mul.rn.f32", t, x, log2e)
	al := k.f()
	k.e("neg.f32", al, t)
	k.e("fma.rn.f32", al, x, log2e, al)
	k.e("fma.rn.f32", al, x, log2eAl, al)
	a := k.f()
	k.e("abs.f32", a, t)
	p := k.p()
	k.e("setp.gt.f32", p, a, float32(126))
	k.e("selp.f32", al, float32(0), al, p)
	e := k.f()
	k.e("ex2.approx.f32", e, t)
	m := k.f()
	k.e("mul.rn.f32", m, al, ln2)
	d := k.f()
	k.e("fma.rn.f32", d, e, m, e)
	// Taşmada e = sonsuz ve düzeltme 0: fma sonsuz·0 = NaN verirdi; e'nin kendisi alınır.
	k.e("selp.f32", d, e, d, p)
	return d
}

// logaritma doğal logaritma.
func (k *yazici) logaritma(x yazmac) yazmac {
	r := k.f()
	k.e("lg2.approx.f32", r, x)
	k.e("mul.rn.f32", r, r, ln2)
	return r
}

// tanh |x| < 0,3'te tek dereceli çok terimli (x^11'e kadar), aksi halde
// işaret(x)·(1 − 2/(e^{2|x|} + 1)).
func (k *yazici) tanh(x yazmac) yazmac {
	x2 := k.f()
	k.e("mul.rn.f32", x2, x, x)
	s := k.sabit(float32(-1382.0 / 155925.0))
	for _, c := range []float64{62.0 / 2835.0, -17.0 / 315.0, 2.0 / 15.0, -1.0 / 3.0} {
		k.e("fma.rn.f32", s, s, x2, float32(c))
	}
	k.e("mul.rn.f32", s, s, x2)
	k.e("fma.rn.f32", s, s, x, x) // x·(1 + x²·(...))
	a := k.f()
	k.e("abs.f32", a, x)
	a2 := k.f()
	k.e("add.rn.f32", a2, a, a)
	e := k.ustel(a2)
	k.e("add.rn.f32", e, e, float32(1))
	b := k.f()
	k.e("div.rn.f32", b, float32(2), e)
	k.e("sub.rn.f32", b, float32(1), b)
	n := k.p()
	k.e("setp.lt.f32", n, x, float32(0))
	nb := k.f()
	k.e("neg.f32", nb, b)
	k.e("selp.f32", b, nb, b, n)
	kucuk := k.p()
	k.e("setp.lt.f32", kucuk, a, float32(0.3))
	r := k.f()
	k.e("selp.f32", r, s, b, kucuk)
	return r
}

// sigmoid e = e^{-|x|}; x ≥ 0 için 1/(1+e), aksi halde e/(1+e) (NaN ikinci yola gider).
func (k *yazici) sigmoid(x yazmac) yazmac {
	a := k.f()
	k.e("abs.f32", a, x)
	k.e("neg.f32", a, a)
	e := k.ustel(a)
	b := k.f()
	k.e("add.rn.f32", b, e, float32(1))
	r1 := k.f()
	k.e("div.rn.f32", r1, float32(1), b)
	r2 := k.f()
	k.e("div.rn.f32", r2, e, b)
	p := k.p()
	k.e("setp.ge.f32", p, x, float32(0))
	r := k.f()
	k.e("selp.f32", r, r1, r2, p)
	return r
}

// geluU u = (x³·a + x)·(−2c) (CPU geluIleriGo ile aynı sıra).
func (k *yazici) geluU(x yazmac) yazmac {
	x3 := k.f()
	k.e("mul.rn.f32", x3, x, x)
	k.e("mul.rn.f32", x3, x3, x)
	u := k.f()
	k.e("mul.rn.f32", u, x3, float32(0.044715))
	k.e("add.rn.f32", u, u, x)
	k.e("mul.rn.f32", u, u, float32(-2*0.7978845608028654))
	return u
}

// gelu x / (1 + e^{u}).
func (k *yazici) gelu(x yazmac) yazmac {
	e := k.ustel(k.geluU(x))
	k.e("add.rn.f32", e, e, float32(1))
	r := k.f()
	k.e("div.rn.f32", r, x, e)
	return r
}

// geluTurev GELU türevi (CPU geluGeriGo): s = 1/(1+e^u), q = s(1−s),
// d = q = 0 ise s, değilse (2x·q)·u' + s, u' = (x²·3a + 1)·c.
func (k *yazici) geluTurev(x yazmac) yazmac {
	e := k.ustel(k.geluU(x))
	k.e("add.rn.f32", e, e, float32(1))
	s := k.f()
	k.e("div.rn.f32", s, float32(1), e)
	q := k.f()
	k.e("sub.rn.f32", q, float32(1), s)
	k.e("mul.rn.f32", q, s, q)
	x2 := k.f()
	k.e("mul.rn.f32", x2, x, x)
	du := k.f()
	k.e("mul.rn.f32", du, x2, float32(3*0.044715))
	k.e("add.rn.f32", du, du, float32(1))
	k.e("mul.rn.f32", du, du, float32(0.7978845608028654))
	d := k.f()
	k.e("add.rn.f32", d, x, x)
	k.e("mul.rn.f32", d, d, q)
	k.e("mul.rn.f32", d, d, du)
	k.e("add.rn.f32", d, d, s)
	sifir := k.p()
	k.e("setp.eq.f32", sifir, q, float32(0))
	k.e("selp.f32", d, s, d, sifir)
	return d
}

// aciIndirge x − round(x/2π)·2π (iki parçalı 2π ile).
func (k *yazici) aciIndirge(x yazmac) yazmac {
	n := k.f()
	k.e("mul.rn.f32", n, x, birBolu)
	k.e("cvt.rni.f32.f32", n, n)
	r := k.f()
	k.e("neg.f32", r, n)
	k.e("fma.rn.f32", r, r, ikiPi, x)
	nn := k.f()
	k.e("neg.f32", nn, n)
	k.e("fma.rn.f32", r, nn, ikiPiAl, r)
	return r
}

// sinus ve kosinus aralık indirgemeli sin.approx/cos.approx.
func (k *yazici) sinus(x yazmac) yazmac {
	r := k.f()
	k.e("sin.approx.f32", r, k.aciIndirge(x))
	return r
}

func (k *yazici) kosinus(x yazmac) yazmac {
	r := k.f()
	k.e("cos.approx.f32", r, k.aciIndirge(x))
	return r
}

// us |x|^p = 2^{p·log2|x|} ile x^p; negatif x'te p tam sayıysa işaret tek/çiftliğe göre,
// değilse NaN. x = 0'da lg2 = −sonsuz olduğundan p > 0 için 0, p < 0 için sonsuz çıkar.
// p = 0, 1, 2 durumlarını çağıran ayrı ele alır.
func (k *yazici) us(x, p yazmac) yazmac {
	a := k.f()
	k.e("abs.f32", a, x)
	l := k.f()
	k.e("lg2.approx.f32", l, a)
	k.e("mul.rn.f32", l, l, p)
	r := k.f()
	k.e("ex2.approx.f32", r, l)
	// Negatif taban: p tam sayı mı, tek mi?
	pr := k.f()
	k.e("cvt.rni.f32.f32", pr, p)
	tamMi := k.p()
	k.e("setp.eq.f32", tamMi, pr, p)
	yarim := k.f()
	k.e("mul.rn.f32", yarim, p, float32(0.5))
	yr := k.f()
	k.e("cvt.rni.f32.f32", yr, yarim)
	tekMi := k.p()
	k.e("setp.ne.f32", tekMi, yr, yarim)
	nr := k.f()
	k.e("neg.f32", nr, r)
	negR := k.f()
	k.e("selp.f32", negR, nr, r, tekMi)
	nan := k.sabit(float32(math.NaN()))
	k.e("selp.f32", negR, negR, nan, tamMi)
	neg := k.p()
	k.e("setp.lt.f32", neg, x, float32(0))
	k.e("selp.f32", r, negR, r, neg)
	return r
}

// ----------------------------------------------------------------------------
// Veri türü yükleme ve saklama
// ----------------------------------------------------------------------------

// turBoyu depolama türünün eleman baytı (desteklenen kayan noktalı türler).
func turBoyu(dt tensor.DType) int {
	if dt == tensor.Float32 {
		return 4
	}
	return 2
}

// yukle adresteki dt türündeki elemanı float32 olarak yükler.
func (k *yazici) yukle(dt tensor.DType, adr yazmac) yazmac {
	v := k.f()
	switch dt {
	case tensor.Float32:
		k.e("ld.global.f32", v, bel(adr, 0))
	case tensor.Float16:
		h := k.h()
		k.e("ld.global.u16", h, bel(adr, 0))
		k.e("cvt.f32.f16", v, h)
	default: // bfloat16: üst 16 bit
		h := k.h()
		k.e("ld.global.u16", h, bel(adr, 0))
		r := k.r()
		k.e("cvt.u32.u16", r, h)
		k.e("shl.b32", r, r, 16)
		k.e("mov.b32", v, r)
	}
	return v
}

// sakla float32 v'yi dt türüne (en yakına-çifte) yuvarlayıp adrese yazar. bfloat16 NaN
// sessiz NaN olarak (işaret korunarak) kalır; taşma sonsuza gider (CPU f32ToBF16 ile aynı).
func (k *yazici) sakla(dt tensor.DType, adr, v yazmac) {
	switch dt {
	case tensor.Float32:
		k.e("st.global.f32", bel(adr, 0), v)
	case tensor.Float16:
		h := k.h()
		k.e("cvt.rn.f16.f32", h, v)
		k.e("st.global.u16", bel(adr, 0), h)
	default:
		b := k.r()
		k.e("mov.b32", b, v)
		mut := k.r()
		k.e("and.b32", mut, b, 0x7FFFFFFF)
		nan := k.p()
		k.e("setp.gt.u32", nan, mut, 0x7F800000)
		q := k.r()
		k.e("shr.u32", q, b, 16)
		k.e("or.b32", q, q, 0x40)
		t := k.r()
		k.e("shr.u32", t, b, 16)
		k.e("and.b32", t, t, 1)
		k.e("add.s32", t, t, 0x7FFF)
		k.e("add.s32", b, b, t)
		k.e("shr.u32", b, b, 16)
		k.e("selp.b32", b, q, b, nan)
		h := k.h()
		k.e("cvt.u16.u32", h, b)
		k.e("st.global.u16", bel(adr, 0), h)
	}
}
