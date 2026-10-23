package gpu

import (
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// PTX matematik ve tür yardımcıları. Donanımın yaklaşık komutları (ex2.approx,
// lg2.approx, sin.approx) aralık indirgemesi ve düzeltme terimleriyle kullanılır;
// sonuçlar CPU arka ucuyla birkaç ulp içinde uyuşur. Kaynaştırılmamış çarpma ve toplama
// gereken yerlerde (CPU çekirdeğinin işlem sırası) .rn biçimleri yazılır: .rn'siz mul/add
// çiftlerini derleyici fma'ya birleştirebilir.

var (
	log2e     = float32(math.Log2E)
	takeLog2e = float32(math.Log2E - float64(float32(math.Log2E)))
	ln2       = float32(math.Ln2)
	twoPi     = float32(2 * math.Pi)
	takeTwoPi = float32(2*math.Pi - float64(float32(2*math.Pi)))
	oneOver   = float32(1 / (2 * math.Pi))
)

// exponential e^x: t = x·log2e, düzeltme t_al = fma(x, log2e, -t) + x·log2e_al ve
// 2^t·(1 + t_al·ln2). |t| > 126'da düzeltme sıfırlanır (taşma, -sonsuz ve NaN için).
func (k *emitter) exponential(x reg) reg {
	t := k.f()
	k.e("mul.rn.f32", t, x, log2e)
	al := k.f()
	k.e("neg.f32", al, t)
	k.e("fma.rn.f32", al, x, log2e, al)
	k.e("fma.rn.f32", al, x, takeLog2e, al)
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

// logarithm doğal logaritma.
func (k *emitter) logarithm(x reg) reg {
	r := k.f()
	k.e("lg2.approx.f32", r, x)
	k.e("mul.rn.f32", r, r, ln2)
	return r
}

// tanh |x| < 0,3'te tek dereceli çok terimli (x^11'e kadar), aksi halde
// işaret(x)·(1 − 2/(e^{2|x|} + 1)).
func (k *emitter) tanh(x reg) reg {
	x2 := k.f()
	k.e("mul.rn.f32", x2, x, x)
	s := k.constant(float32(-1382.0 / 155925.0))
	for _, c := range []float64{62.0 / 2835.0, -17.0 / 315.0, 2.0 / 15.0, -1.0 / 3.0} {
		k.e("fma.rn.f32", s, s, x2, float32(c))
	}
	k.e("mul.rn.f32", s, s, x2)
	k.e("fma.rn.f32", s, s, x, x) // x·(1 + x²·(...))
	a := k.f()
	k.e("abs.f32", a, x)
	a2 := k.f()
	k.e("add.rn.f32", a2, a, a)
	e := k.exponential(a2)
	k.e("add.rn.f32", e, e, float32(1))
	b := k.f()
	k.e("div.rn.f32", b, float32(2), e)
	k.e("sub.rn.f32", b, float32(1), b)
	n := k.p()
	k.e("setp.lt.f32", n, x, float32(0))
	nb := k.f()
	k.e("neg.f32", nb, b)
	k.e("selp.f32", b, nb, b, n)
	small := k.p()
	k.e("setp.lt.f32", small, a, float32(0.3))
	r := k.f()
	k.e("selp.f32", r, s, b, small)
	return r
}

// sigmoid e = e^{-|x|}; x ≥ 0 için 1/(1+e), aksi halde e/(1+e) (NaN ikinci yola gider).
func (k *emitter) sigmoid(x reg) reg {
	a := k.f()
	k.e("abs.f32", a, x)
	k.e("neg.f32", a, a)
	e := k.exponential(a)
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
func (k *emitter) geluU(x reg) reg {
	x3 := k.f()
	k.e("mul.rn.f32", x3, x, x)
	k.e("mul.rn.f32", x3, x3, x)
	u := k.f()
	k.e("mul.rn.f32", u, x3, float32(0.044715))
	k.e("add.rn.f32", u, u, x)
	k.e("mul.rn.f32", u, u, float32(-2*0.7978845608028654))
	return u
}

// gelu x / (1 + e^{u}); pay −∞'da en küçük sonlu sayıdır: −∞/∞ NaN yerine −0 (limit, CPU
// geluForwardGo ile aynı).
func (k *emitter) gelu(x reg) reg {
	e := k.exponential(k.geluU(x))
	k.e("add.rn.f32", e, e, float32(1))
	ni := k.p()
	k.e("setp.eq.f32", ni, x, float32(math.Inf(-1)))
	n := k.f()
	k.e("selp.f32", n, float32(-math.MaxFloat32), x, ni)
	r := k.f()
	k.e("div.rn.f32", r, n, e)
	return r
}

// geluGrad GELU türevi (CPU geluGeriGo): s = 1/(1+e^u), q = s(1−s),
// d = q = 0 ise s, değilse (2x·q)·u' + s, u' = (x²·3a + 1)·c.
func (k *emitter) geluGrad(x reg) reg {
	e := k.exponential(k.geluU(x))
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
	zero := k.p()
	k.e("setp.eq.f32", zero, q, float32(0))
	k.e("selp.f32", d, s, d, zero)
	return d
}

// angleReduce x − round(x/2π)·2π (iki parçalı 2π ile).
func (k *emitter) angleReduce(x reg) reg {
	n := k.f()
	k.e("mul.rn.f32", n, x, oneOver)
	k.e("cvt.rni.f32.f32", n, n)
	r := k.f()
	k.e("neg.f32", r, n)
	k.e("fma.rn.f32", r, r, twoPi, x)
	nn := k.f()
	k.e("neg.f32", nn, n)
	k.e("fma.rn.f32", r, nn, takeTwoPi, r)
	return r
}

// sine ve kosinus aralık indirgemeli sin.approx/cos.approx.
func (k *emitter) sine(x reg) reg {
	r := k.f()
	k.e("sin.approx.f32", r, k.angleReduce(x))
	return r
}

func (k *emitter) cosine(x reg) reg {
	r := k.f()
	k.e("cos.approx.f32", r, k.angleReduce(x))
	return r
}

// us x^p: büyüklük |x|^p = 2^{p·log2|x|}, özel durumlar C99 Ek F'deki gibi (CPU math.Pow ile
// aynı): p tek tam sayıysa işaret x'inkidir (−0 ve −∞ dahil); x sonlu negatif ve p tam sayı
// değilse NaN; x = 1'de (p NaN olsa da) ve x = −1, p = ±∞'da 1. |x| = 0'da lg2 = −∞: p > 0
// için 0, p < 0 için ∞; |x| = ∞'da tersi. p = 0, 1, 2 durumlarını çağıran ayrı ele alır.
func (k *emitter) us(x, p reg) reg {
	a := k.f()
	k.e("abs.f32", a, x)
	l := k.f()
	k.e("lg2.approx.f32", l, a)
	k.e("mul.rn.f32", l, l, p)
	r := k.f()
	k.e("ex2.approx.f32", r, l)
	// |x| = 1: l = 0·p, p = ±∞ ya da NaN'da NaN olurdu.
	one := k.p()
	k.e("setp.eq.f32", one, a, float32(1))
	k.e("selp.f32", r, float32(1), r, one)
	// p tam sayı mı, tek mi? (±∞ ve 2²⁴'ten büyükler çift tam sayıdır, NaN tam sayı değildir.)
	pr := k.f()
	k.e("cvt.rni.f32.f32", pr, p)
	isWhole := k.p()
	k.e("setp.eq.f32", isWhole, pr, p)
	half := k.f()
	k.e("mul.rn.f32", half, p, float32(0.5))
	yr := k.f()
	k.e("cvt.rni.f32.f32", yr, half)
	isSingle := k.p()
	k.e("setp.ne.f32", isSingle, yr, half)
	k.e("and.pred", isSingle, isSingle, isWhole)
	// İşaret biti (−0 ve −∞ dahil) ve tek p: sonuç negatif.
	xb := k.r()
	k.e("mov.b32", xb, x)
	neg := k.p()
	k.e("setp.lt.s32", neg, xb, 0)
	k.e("and.pred", neg, neg, isSingle)
	nr := k.f()
	k.e("neg.f32", nr, r)
	k.e("selp.f32", r, nr, r, neg)
	// Sonlu negatif taban, tam sayı olmayan üs: NaN.
	bad := k.p()
	k.e("setp.lt.f32", bad, x, float32(0))
	fin := k.p()
	k.e("setp.lt.f32", fin, a, float32(math.Inf(1)))
	k.e("and.pred", bad, bad, fin)
	whole := k.p()
	k.e("not.pred", whole, isWhole)
	k.e("and.pred", bad, bad, whole)
	k.e("selp.f32", r, k.constant(float32(math.NaN())), r, bad)
	return r
}

// ----------------------------------------------------------------------------
// Veri türü yükleme ve saklama
// ----------------------------------------------------------------------------

// dtypeSize depolama türünün eleman baytı (desteklenen kayan noktalı türler).
func dtypeSize(dt tensor.DType) int {
	if dt == tensor.Float32 {
		return 4
	}
	return 2
}

// load adresteki dt türündeki elemanı float32 olarak yükler.
func (k *emitter) load(dt tensor.DType, addr reg) reg {
	v := k.f()
	switch dt {
	case tensor.Float32:
		k.e("ld.global.f32", v, mem(addr, 0))
	case tensor.Float16:
		h := k.h()
		k.e("ld.global.u16", h, mem(addr, 0))
		k.e("cvt.f32.f16", v, h)
	default: // bfloat16: üst 16 bit
		h := k.h()
		k.e("ld.global.u16", h, mem(addr, 0))
		r := k.r()
		k.e("cvt.u32.u16", r, h)
		k.e("shl.b32", r, r, 16)
		k.e("mov.b32", v, r)
	}
	return v
}

// store float32 v'yi dt türüne (en yakına-çifte) yuvarlayıp adrese yazar. bfloat16 NaN
// sessiz NaN olarak (işaret korunarak) kalır; taşma sonsuza gider (CPU f32ToBF16 ile aynı).
func (k *emitter) store(dt tensor.DType, addr, v reg) {
	switch dt {
	case tensor.Float32:
		k.e("st.global.f32", mem(addr, 0), v)
	case tensor.Float16:
		h := k.h()
		k.e("cvt.rn.f16.f32", h, v)
		k.e("st.global.u16", mem(addr, 0), h)
	default:
		b := k.bf16Bits(v)
		h := k.h()
		k.e("cvt.u16.u32", h, b)
		k.e("st.global.u16", mem(addr, 0), h)
	}
}

// bf16Bits float32 v'nin en yakına-çifte yuvarlanmış bfloat16 bitlerini 32 bitlik yazmacın
// alt yarısında verir (üst yarı 0). NaN sessiz NaN olarak (işaret korunarak) kalır.
func (k *emitter) bf16Bits(v reg) reg {
	b := k.r()
	k.e("mov.b32", b, v)
	abs2 := k.r()
	k.e("and.b32", abs2, b, 0x7FFFFFFF)
	nan := k.p()
	k.e("setp.gt.u32", nan, abs2, 0x7F800000)
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
	return b
}
