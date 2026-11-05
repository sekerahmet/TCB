package gpu

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Nicemlenmiş blokları açmak (15.53: açılmış değerler cihazlar arasında kesin aynıdır)
// ============================================================================
//
// Her iş parçacığı bir öğe açar. Çarpımlar ve çıkarma CPU'nun dequantQ8, dequantQ4K ve
// dequantQ6K'sıyla aynı sırada, aynı yuvarlamayla yazılır (.rn komutları kaynaştırılmaz). Kartın
// NaN kuralı CPU'nunkinden ayrıdır: cvt ve aritmetik her NaN'ı tek desene çevirir; x86 ise birinci
// NaN işleneni susturarak taşır, geçersiz işlem işaretli varsayılan NaN'ı verir. Açılan değer bit
// bit aynı kalsın diye bu kural burada elle kurulur.

// Float32 NaN'ın bitleri: sessizlik biti ve x86'nın geçersiz işlem sonucu.
const (
	nanQuietBit = 0x00400000
	nanDefault  = 0xFFC00000
)

// Blok boyları (ggml-common.h; tensor.DType.QuantBlock'la aynı).
const (
	q8BlockBytes  = 34
	q4kBlockBytes = 144
	q6kBlockBytes = 210
)

// quantOpenKernel cikti[i] = aç(w)[i]; w bitişik bloklardır.
func quantOpenKernel(dt tensor.DType) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("nicem_ac_%s", dt.Name()))
		w := k.pointer("w")
		out := k.written("cikti")
		n := k.u32p("n")
		i := k.globalIndex()
		end := k.label()
		p := k.p()
		k.e("setp.ge.u32", p, i, n)
		k.jumpIf(p, end)
		k.e("st.global.f32", mem(k.address(out, i, 4), 0), k.quantElem(dt, w, i))
		k.put(end)
		return k
	}
}

// quantElem w'nin e. öğesini kesin açar.
func (k *emitter) quantElem(dt tensor.DType, w, e reg) reg {
	elems, size := dt.QuantBlock()
	b, j := k.r(), k.r()
	k.e("shr.u32", b, e, log2(elems))
	k.e("and.b32", j, e, elems-1)
	block := k.address(w, b, size)
	switch dt {
	case tensor.Q8:
		return k.openQ8(block, j)
	case tensor.Q4K:
		return k.openQ4K(block, j)
	}
	return k.openQ6K(block, j)
}

// byteAt adres + i'deki baytı okur (i işaretsiz 32 bit).
func (k *emitter) byteAt(addr, i reg, offset int) reg {
	a, v := k.rd(), k.r()
	k.e("cvt.u64.u32", a, i)
	k.e("add.s64", a, a, addr)
	k.e("ld.global.u8", v, mem(a, offset))
	return v
}

// signedByte baytın işaretli değeridir.
func (k *emitter) signedByte(b reg) reg {
	r := k.r()
	k.e("shl.b32", r, b, 24)
	k.e("shr.s32", r, r, 24)
	return r
}

// toFloat tam sayının float32 değeridir (bu işlerde kesindir).
func (k *emitter) toFloat(v reg) reg {
	f := k.f()
	k.e("cvt.rn.f32.s32", f, v)
	return f
}

// halfExact adresteki fp16 ölçeği CPU'nun f16ToF32'siyle aynı bitlere açar: alt normal ve sonlu
// değerde cvt kesindir; üs bitlerinin hepsi 1'se sonsuz ve NaN'ın bitleri elle kurulur.
func (k *emitter) halfExact(addr reg, offset int) reg {
	h := k.h()
	k.e("ld.global.u16", h, mem(addr, offset))
	v, b := k.f(), k.r()
	k.e("cvt.f32.f16", v, h)
	k.e("cvt.u32.u16", b, h)
	exp, sign, frac, wide := k.r(), k.r(), k.r(), k.r()
	k.e("and.b32", exp, b, 0x7C00)
	k.e("and.b32", sign, b, 0x8000)
	k.e("shl.b32", sign, sign, 16)
	k.e("and.b32", frac, b, 0x3FF)
	k.e("shl.b32", frac, frac, 13)
	k.e("or.b32", wide, sign, frac)
	k.e("or.b32", wide, wide, 0x7F800000)
	special := k.p()
	k.e("setp.eq.u32", special, exp, 0x7C00)
	bits := k.r()
	k.e("mov.b32", bits, v)
	k.e("selp.b32", bits, wide, bits, special)
	out := k.f()
	k.e("mov.b32", out, bits)
	return out
}

// nanRule r NaN ise x86'nın sonucunu koyar: NaN olan ilk işlenen susturulur, işlenenlerde NaN yoksa
// sonuç varsayılan NaN'dır.
func (k *emitter) nanRule(r, a, b reg) reg {
	ab, bb, rb, q := k.r(), k.r(), k.r(), k.r()
	k.e("mov.b32", ab, a)
	k.e("mov.b32", bb, b)
	k.e("mov.b32", rb, r)
	k.e("or.b32", ab, ab, nanQuietBit)
	k.e("or.b32", bb, bb, nanQuietBit)
	pa, pb, pr := k.p(), k.p(), k.p()
	k.e("setp.nan.f32", pa, a, a)
	k.e("setp.nan.f32", pb, b, b)
	k.e("setp.nan.f32", pr, r, r)
	k.e("mov.b32", q, uint32(nanDefault))
	k.e("selp.b32", q, bb, q, pb)
	k.e("selp.b32", q, ab, q, pa)
	k.e("selp.b32", rb, q, rb, pr)
	out := k.f()
	k.e("mov.b32", out, rb)
	return out
}

// mulExact a·b'dir, kuralı nanRule'dur.
func (k *emitter) mulExact(a, b reg) reg {
	r := k.f()
	k.e("mul.rn.f32", r, a, b)
	return k.nanRule(r, a, b)
}

// subExact a − b'dir, kuralı nanRule'dur.
func (k *emitter) subExact(a, b reg) reg {
	r := k.f()
	k.e("sub.rn.f32", r, a, b)
	return k.nanRule(r, a, b)
}

// openQ8 bloğun j. öğesi: dequantQ8.
func (k *emitter) openQ8(block, j reg) reg {
	code := k.toFloat(k.signedByte(k.byteAt(block, j, 2)))
	return k.mulExact(code, k.halfExact(block, 0))
}

// q4kScale s. alt bloğun 6 bitlik ölçeği ve minimumu (quant.go: q4kScale); s yazmaçtadır.
func (k *emitter) q4kScale(block, s reg) (sc, m reg) {
	low := k.byteAt(block, s, 4)  // q[s]
	high := k.byteAt(block, s, 8) // q[s+4]
	back := k.r()
	k.e("and.b32", back, s, 3)
	prev := k.byteAt(block, back, 4) // s ≥ 4'te q[s-4]
	sc, m = k.r(), k.r()
	k.e("and.b32", sc, low, 63)
	k.e("and.b32", m, high, 63)
	wideS, wideM, t := k.r(), k.r(), k.r()
	k.e("and.b32", wideS, high, 0xF)
	k.e("shr.u32", t, prev, 6)
	k.e("shl.b32", t, t, 4)
	k.e("or.b32", wideS, wideS, t)
	k.e("shr.u32", wideM, high, 4)
	k.e("shr.u32", t, low, 6)
	k.e("shl.b32", t, t, 4)
	k.e("or.b32", wideM, wideM, t)
	p := k.p()
	k.e("setp.lt.u32", p, s, 4)
	k.e("selp.b32", sc, sc, wideS, p)
	k.e("selp.b32", m, m, wideM, p)
	return sc, m
}

// openQ4K bloğun j. öğesi: dequantQ4K. j = g·64 + yarı·32 + l.
func (k *emitter) openQ4K(block, j reg) reg {
	s, l, g := k.r(), k.r(), k.r()
	k.e("shr.u32", s, j, 5) // 2g + yarı
	k.e("and.b32", l, j, 31)
	k.e("shr.u32", g, j, 6)
	sc, m := k.q4kScale(block, s)
	d1 := k.mulExact(k.halfExact(block, 0), k.toFloat(sc))
	m1 := k.mulExact(k.halfExact(block, 2), k.toFloat(m))
	at, shift := k.r(), k.r()
	k.e("mad.lo.s32", at, g, 32, l)
	k.e("and.b32", shift, s, 1)
	k.e("shl.b32", shift, shift, 2)
	code := k.r()
	k.e("shr.u32", code, k.byteAt(block, at, 16), shift)
	k.e("and.b32", code, code, 0xF)
	return k.subExact(k.mulExact(d1, k.toFloat(code)), m1)
}

// openQ6K bloğun j. öğesi: dequantQ6K. j = n·128 + çeyrek·32 + l.
func (k *emitter) openQ6K(block, j reg) reg {
	n, quarter, l := k.r(), k.r(), k.r()
	k.e("shr.u32", n, j, 7)
	k.e("shr.u32", quarter, j, 5)
	k.e("and.b32", quarter, quarter, 3)
	k.e("and.b32", l, j, 31)
	// ql: çeyreğin tek olanı 32 sonraki bayt, 2 ve 3 üst yarım bayt.
	at, t := k.r(), k.r()
	k.e("mad.lo.s32", at, n, 64, l)
	k.e("and.b32", t, quarter, 1)
	k.e("mad.lo.s32", at, t, 32, at)
	shift := k.r()
	k.e("shr.u32", shift, quarter, 1)
	k.e("shl.b32", shift, shift, 2)
	low := k.r()
	k.e("shr.u32", low, k.byteAt(block, at, 0), shift)
	k.e("and.b32", low, low, 0xF)
	// qh: çeyrek başına iki bit.
	hAt := k.r()
	k.e("mad.lo.s32", hAt, n, 32, l)
	k.e("shl.b32", shift, quarter, 1)
	high := k.r()
	k.e("shr.u32", high, k.byteAt(block, hAt, 128), shift)
	k.e("and.b32", high, high, 3)
	k.e("shl.b32", high, high, 4)
	code := k.r()
	k.e("or.b32", code, low, high)
	k.e("add.s32", code, code, -32)
	// Ölçek: sc[n·8 + l/16 + 2·çeyrek].
	sAt := k.r()
	k.e("shr.u32", sAt, l, 4)
	k.e("mad.lo.s32", sAt, n, 8, sAt)
	k.e("mad.lo.s32", sAt, quarter, 2, sAt)
	scale := k.toFloat(k.signedByte(k.byteAt(block, sAt, 192)))
	return k.mulExact(k.mulExact(k.halfExact(block, 208), scale), k.toFloat(code))
}
