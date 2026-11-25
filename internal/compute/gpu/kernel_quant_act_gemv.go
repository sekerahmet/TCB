package gpu

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Tek satırlı (M = 1) tamsayı çarpım, tek başlatma (q4, q4_k, q6_k): blok (actWarpsMV warp) satırı önce paylaşımlı
// belleğe nicemler (quantActQuantizeKernel'in formülü), engelden sonra her warp bir çıktıyı hesaplar. Ayrı bir
// nicemleme başlatması konağın payını ve kartın boşluğunu artırıyordu (kart profili, 2026-11-25). Sonlu olmayan
// değer taşıyan bloğun ölçeği +∞ yazılır (sonlu bloğun ölçeği en çok MaxFloat32 / 127'dir); warp onu görürse çıktıyı
// aynı çekirdekte float yoluyla yazar (actFloatRow). Çıktı ezilir.
//
// Paylaşımlı bellek: kodlar (K bayt, q4'te 256'ya yuvarlı), blok ölçekleri (float32) ve q4_k, q6_k'da 32'lik kod
// toplamları (int32).

// actInfBits float32 +∞'un bitleridir: sonlu olmayan bloğun ölçek işareti.
const actInfBits = 0x7F800000

// quantActGemvShared K öğelik satırın paylaşımlı belleğidir (bayt); quantActGemvLimit'i aşan K karolu çekirdeğe
// gider.
func quantActGemvShared(dt tensor.DType, K int) int {
	if dt == tensor.Q4 {
		return (K+actBlk-1)/actBlk*actBlk + K/8
	}
	return K + K/64 + K/8
}

// actGemv tek satırlı çarpımın ortak yazmaçlarıdır.
type actGemv struct {
	k                  *emitter
	dt                 tensor.DType
	pa, pw, pc         reg
	N, K, nb, chunks   reg // chunks: 256 öğelik dilim sayısı
	sq, sd, sb         reg
	lane, warp         reg
	wrow               reg
	acc, bad           reg // bad: satırda sonlu olmayan blok var mı (0, 1)
	blockElems, blockB int
}

// quantActGemvKernel türün tek satırlı çekirdeğidir; ızgara N/actWarpsMV, blok 32·actWarpsMV iş parçacığı. dp yanlışsa
// (sm_61 altı) dp4a yerine dört tamsayı çarpma-toplama.
func quantActGemvKernel(dt tensor.DType, dp bool) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("nicem_etk_gemv_%s_%v", dt.Name(), dp))
		if dp {
			k.target = 61
		}
		k.dynamic = true
		g := &actGemv{k: k, dt: dt}
		g.blockElems, g.blockB = dt.QuantBlock()
		g.setup()
		g.quantize()
		k.barrier()
		n := k.r()
		k.e("mad.lo.s32", n, k.special("%ctaid.x"), actWarpsMV, g.warp)
		end := k.label()
		p := k.p()
		k.e("setp.ge.u32", p, n, g.N) // bütün warp aynı çıktıda: warp birlikte çıkar
		k.jumpIf(p, end)
		rowBytes := k.r()
		g.wrow = k.rd()
		k.e("mul.lo.s32", rowBytes, g.nb, g.blockB)
		k.e("mul.wide.u32", g.wrow, n, rowBytes)
		k.e("add.s64", g.wrow, g.wrow, g.pw)
		g.acc, g.bad = k.constant(0), k.whole(0)
		if dt == tensor.Q4 {
			g.sumQ4()
		} else {
			g.sumK()
		}
		g.finish(n, end)
		k.put(end)
		return k
	}
}

// setup parametreleri okur, paylaşımlı dizilerin yerini kurar (izin 48 KB'tır, başlatma K kadarını ister).
func (g *actGemv) setup() {
	k := g.k
	g.pa, g.pw = k.pointer("a"), k.pointer("w")
	g.pc = k.written("c")
	g.N, g.nb = k.u32p("n"), k.u32p("bloklar")
	g.sq = k.shared("q", staticSharedMemLimit)
	g.K, g.sd, g.sb = k.r(), k.r(), k.r()
	k.e("shl.b32", g.K, g.nb, log2(g.blockElems))
	g.chunks = g.nb
	if g.dt == tensor.Q4 {
		g.chunks = k.r()
		k.e("add.s32", g.chunks, g.nb, 7)
		k.e("shr.u32", g.chunks, g.chunks, 3)
	}
	k.e("mad.lo.s32", g.sd, g.chunks, actBlk, g.sq)
	k.e("mad.lo.s32", g.sb, g.nb, 4, g.sd)
	tid := k.special("%tid.x")
	g.lane, g.warp = k.r(), k.r()
	k.e("and.b32", g.lane, tid, 31)
	k.e("shr.u32", g.warp, tid, 5)
}

// quantize satırı paylaşımlı belleğe nicemler: warp 256'lık dilimleri actWarpsMV adımla gezer, şerit 8 öğe alır;
// blockElems/8 şeritlik öbek bir bloktur. q4'te K'nın son dilimi yarım kalabilir (K'nın üstü okunmaz).
func (g *actGemv) quantize() {
	k := g.k
	b := k.r()
	k.e("mov.u32", b, g.warp)
	k.whileLess("u32", b, g.chunks, func() {
		e, valid := k.r(), reg("") // q4_k ve q6_k'da K 256'nın katıdır: her şerit K'nın içindedir
		k.e("mul.lo.s32", e, b, actBlk)
		k.e("mad.lo.s32", e, g.lane, 8, e)
		if g.dt == tensor.Q4 {
			valid = k.p()
			k.e("setp.lt.u32", valid, e, g.K)
		}
		src := k.address(g.pa, e, 4)
		x := append(k.loadV("global", valid, src, 0, 4), k.loadV("global", valid, src, 16, 4)...)
		amax := k.constant(0)
		bad, p := k.p(), k.p()
		k.e("setp.ne.u32", bad, g.lane, g.lane) // yanlış
		for i := range x {
			a, t := k.f(), k.f()
			k.e("abs.f32", a, x[i])
			k.e("max.f32", amax, amax, a)
			k.e("sub.rn.f32", t, x[i], x[i])
			k.e("setp.neu.f32", p, t, float32(0))
			k.e("or.pred", bad, bad, p)
		}
		inf := k.f()
		k.e("mov.b32", inf, k.whole(actInfBits))
		k.e("selp.f32", amax, inf, amax, bad)
		for m := g.blockElems / 16; m > 0; m /= 2 {
			k.e("max.f32", amax, amax, k.shflXor(amax, m))
		}
		id, d := k.actScale(amax)
		words, sum := k.actCodes(x, id)
		for m := 1; m < 4 && g.dt != tensor.Q4; m *= 2 { // kelebek her şeritte koşar: atlamadan önce
			k.e("add.s32", sum, sum, k.shflXor(sum, m))
		}
		skip := k.label()
		if valid != "" {
			k.jumpUnless(valid, skip)
		}
		qa := k.r()
		k.e("add.s32", qa, g.sq, e)
		k.e("st.shared.v2.u32", mem(qa, 0), vlist(words[:]))
		g.storeScale(e, d, sum, skip)
		k.put(skip)
		k.e("add.s32", b, b, actWarpsMV)
	})
}

// storeScale bloğun ölçeğini (öbeğin ilk şeridi) ve q4_k, q6_k'da 32'lik kod toplamını (dört şeritte bir) yazar.
func (g *actGemv) storeScale(e, d, sum reg, skip string) {
	k := g.k
	first, p := k.r(), k.p()
	if g.dt == tensor.Q4 {
		k.e("and.b32", first, g.lane, g.blockElems/8-1)
		k.e("setp.ne.u32", p, first, 0)
		k.jumpIf(p, skip)
		da := k.r()
		k.e("shr.u32", da, e, 3) // (e / 32)·4
		k.e("add.s32", da, da, g.sd)
		k.e("st.shared.f32", mem(da, 0), d)
		return
	}
	k.e("and.b32", first, g.lane, 3)
	k.e("setp.ne.u32", p, first, 0)
	k.jumpIf(p, skip)
	ba := k.r()
	k.e("shr.u32", ba, e, 3) // (e / 32)·4
	k.e("add.s32", ba, ba, g.sb)
	k.e("st.shared.u32", mem(ba, 0), sum)
	k.e("setp.ne.u32", p, g.lane, 0)
	k.jumpIf(p, skip)
	da := k.r()
	k.e("shr.u32", da, e, 6) // (e / 256)·4
	k.e("add.s32", da, da, g.sd)
	k.e("st.shared.f32", mem(da, 0), d)
}

// lastBlock i + grp'nin bloğudur: geçersizse son blok okunur, katkısı 0 sayılır (valid).
func (g *actGemv) lastBlock(i, grp reg) (bc, valid reg) {
	k := g.k
	bi := k.r()
	bc, valid = k.r(), k.p()
	k.e("add.s32", bi, i, grp)
	k.e("setp.lt.u32", valid, bi, g.nb)
	k.e("sub.s32", bc, g.nb, 1)
	k.e("min.u32", bc, bi, bc)
	return bc, valid
}

// scale bloğun ölçeğidir; +∞ ise bad'i kurar.
func (g *actGemv) scale(bc reg) reg {
	k := g.k
	da, d8 := k.r(), k.f()
	k.e("mad.lo.s32", da, bc, 4, g.sd)
	k.e("ld.shared.f32", d8, mem(da, 0))
	bits, inf := k.r(), k.p()
	k.e("mov.b32", bits, d8)
	k.e("setp.eq.u32", inf, bits, actInfBits)
	k.e("selp.b32", g.bad, 1, g.bad, inf)
	return d8
}

// sumQ4 q4'ün döngüsüdür: şerit bir 32'lik blok alır (warp dilimde 32 blok); blokların katkıları 0. şeritte sırayla
// toplanır (dotQ4Q8Go).
func (g *actGemv) sumQ4() {
	k := g.k
	i := k.whole(0)
	k.whileLess("u32", i, g.nb, func() {
		bc, valid := g.lastBlock(i, g.lane)
		block := k.address(g.wrow, bc, g.blockB)
		dw := k.half(block, 0)
		d8 := g.scale(bc)
		at := k.r()
		k.e("mad.lo.s32", at, bc, 32, g.sq)
		var x [2][]reg
		for h := range x {
			x[h] = []reg{k.r(), k.r(), k.r(), k.r()}
			k.e("ld.shared.v4.u32", vlist(x[h]), mem(at, 16*h))
		}
		isum := k.whole(0)
		for j := range 4 {
			code := k.words16(block, 2+4*j)
			for h := range 2 {
				c := k.r()
				k.e("shr.u32", c, code, 4*h)
				k.e("and.b32", c, c, 0x0F0F0F0F)
				k.dot4(isum, k.minusBytes(c, 0x08080808), x[h][j])
			}
		}
		t := k.f()
		k.e("mul.rn.f32", t, dw, d8)
		k.e("mul.rn.f32", t, t, k.toFloat(isum))
		k.e("selp.f32", t, t, float32(0), valid)
		k.e("add.rn.f32", g.acc, g.acc, t)
		for m := 1; m < 32; m++ {
			k.e("add.rn.f32", g.acc, g.acc, k.shflXor(t, m))
		}
		k.e("add.s32", i, i, 32)
	})
}

// sumK q4_k ve q6_k'nın döngüsüdür: 8 şeritlik dört öbek dört bloğu birlikte okur; öbeğin tamsayı toplamları
// kelebekle kesin indirgenir, blokların float katkıları 0. şeritte sırayla toplanır (dotQ4KQ8Go, dotQ6KQ8Go).
func (g *actGemv) sumK() {
	k := g.k
	grp, L := k.r(), k.r()
	k.e("shr.u32", grp, g.lane, 3)
	k.e("and.b32", L, g.lane, 7)
	i := k.whole(0)
	k.whileLess("u32", i, g.nb, func() {
		bc, valid := g.lastBlock(i, grp)
		block := k.address(g.wrow, bc, g.blockB)
		d8 := g.scale(bc)
		var t reg
		if g.dt == tensor.Q4K {
			t = g.blockQ4K(block, bc, L)
		} else {
			t = g.blockQ6K(block, bc, L)
		}
		k.e("mul.rn.f32", t, t, d8)
		k.e("selp.f32", t, t, float32(0), valid)
		for _, v := range []reg{t, k.shflXor(t, 8), k.shflXor(t, 16), k.shflXor(t, 24)} {
			k.e("add.rn.f32", g.acc, g.acc, v)
		}
		k.e("add.s32", i, i, 4)
	})
}

// blockQ4K bloğun ölçeklenmemiş katkısıdır (öbekte aynı): isum = sc·s, msum = m·Σkod; d·isum − dmin·msum.
func (g *actGemv) blockQ4K(block, bc, L reg) reg {
	k := g.k
	at := k.r()
	k.e("mad.lo.s32", at, bc, actBlk, g.sq)
	k.e("mad.lo.s32", at, L, 32, at)
	w := k.q4kLane(block, L)
	s := k.whole(0)
	for j := range 2 {
		x := []reg{k.r(), k.r(), k.r(), k.r()}
		k.e("ld.shared.v4.u32", vlist(x), mem(at, 16*j))
		for i := range x {
			k.dot4(s, w.c[4*j+i], x[i])
		}
	}
	isum, ba, b0, msum := k.r(), k.r(), k.r(), k.r()
	k.e("mul.lo.s32", isum, w.sc, s)
	k.e("mad.lo.s32", ba, bc, 32, g.sb)
	k.e("mad.lo.s32", ba, L, 4, ba)
	k.e("ld.shared.u32", b0, mem(ba, 0))
	k.e("mul.lo.s32", msum, w.m, b0)
	isum, msum = k.groupSum(isum), k.groupSum(msum)
	x1, y, t := k.f(), k.f(), k.f()
	k.e("mul.rn.f32", x1, w.d, k.toFloat(isum))
	k.e("mul.rn.f32", y, w.dmin, k.toFloat(msum))
	k.e("sub.rn.f32", t, x1, y)
	return t
}

// blockQ6K bloğun ölçeklenmemiş katkısıdır (öbekte aynı): şerit L bir 128'liğin (n = L/4) dört çeyreğinden 8'er öğe
// alır (dotQ6K'nın düzeni: ağırlığın okunan baytı az), I = Σ sc·Σ kod·x, d·I.
func (g *actGemv) blockQ6K(block, bc, L reg) reg {
	k := g.k
	d := k.half(block, 208)
	n, l, si := k.r(), k.r(), k.r()
	k.e("shr.u32", n, L, 2)
	k.e("and.b32", l, L, 3)
	k.e("shr.u32", si, l, 1) // ölçek sc[n·8 + 2·çeyrek + l/16]
	k.e("mad.lo.s32", si, n, 8, si)
	k.e("shl.b32", l, l, 3)
	qlAt, qhAt, xAt := k.r(), k.r(), k.r()
	k.e("mad.lo.s32", qlAt, n, 64, l)
	k.e("mad.lo.s32", qhAt, n, 32, l)
	k.e("mad.lo.s32", xAt, bc, actBlk, g.sq)
	k.e("mad.lo.s32", xAt, n, 128, xAt)
	k.e("add.s32", xAt, xAt, l)
	ql, qh := k.address(block, qlAt, 1), k.address(block, qhAt, 1)
	var lo, hi, top [2]reg
	for j := range 2 {
		lo[j], hi[j], top[j] = k.words16(ql, 4*j), k.words16(ql, 32+4*j), k.words16(qh, 128+4*j)
	}
	I := k.whole(0)
	for q := range 4 {
		x := []reg{k.r(), k.r()}
		k.e("ld.shared.v2.u32", vlist(x), mem(xAt, 32*q))
		s := k.whole(0)
		for j := range 2 {
			src := lo[j]
			if q%2 == 1 {
				src = hi[j]
			}
			c, t := k.r(), k.r()
			k.e("shr.u32", c, src, 4*(q/2))
			k.e("and.b32", c, c, 0x0F0F0F0F)
			k.e("shr.u32", t, top[j], 2*q)
			k.e("and.b32", t, t, 0x03030303)
			k.e("shl.b32", t, t, 4)
			k.e("or.b32", c, c, t)
			k.dot4(s, k.minusBytes(c, 0x20202020), x[j])
		}
		k.e("mad.lo.s32", I, k.signedByte(k.byteAt(block, si, 192+2*q)), s, I)
	}
	I = k.groupSum(I)
	t := k.f()
	k.e("mul.rn.f32", t, d, k.toFloat(I))
	return t
}

// finish çıktıyı yazar (0. şerit): satır sonlu değilse toplam float yoluyla yeniden hesaplanır. 0 eklenir: CPU'nun
// 0 + çarpımı gibi −0 +0 olur.
func (g *actGemv) finish(n reg, end string) {
	k := g.k
	for m := 1; m < 32; m *= 2 {
		k.e("or.b32", g.bad, g.bad, k.shflXor(g.bad, m))
	}
	p := k.p()
	k.e("setp.ne.u32", p, g.lane, 0)
	k.jumpIf(p, end)
	write := k.label()
	k.e("setp.eq.u32", p, g.bad, 0)
	k.jumpIf(p, write)
	k.e("mov.f32", g.acc, k.actFloatRow(g.dt, g.wrow, g.pa, g.K))
	k.put(write)
	k.e("add.rn.f32", g.acc, g.acc, float32(0))
	k.e("st.global.f32", mem(k.address(g.pc, n, 4), 0), g.acc)
}

// minusBytes c'nin baytlarından b'ninkileri çıkarır (c ≤ 63, b ≤ 127; işaretli bayt): ((c | 0x80) − b) ^ 0x80 ödünç
// almaz.
func (k *emitter) minusBytes(c reg, b int) reg {
	r := k.r()
	k.e("or.b32", r, c, -2139062144)
	k.e("sub.s32", r, r, b)
	k.e("xor.b32", r, r, -2139062144)
	return r
}

// groupSum 8 şeritlik öbeğin tamsayı toplamıdır (bütün öbekte aynı).
func (k *emitter) groupSum(v reg) reg {
	for m := 1; m < 8; m *= 2 {
		o := k.shflXor(v, m)
		k.e("add.s32", v, v, o)
	}
	return v
}

// actLane şeridin bir q4_k bloğundan açtıklarıdır: dp4a sözcükleri (0..15), alt bloğun ölçeği ve en küçüğü, d, dmin.
type actLane struct {
	c       [8]reg
	sc, m   reg
	d, dmin reg
}

// q4kLane şerit L alt blok L'yi (32 öğe) alır: sözcükler, ölçek, en küçük, d, dmin.
func (k *emitter) q4kLane(block, L reg) actLane {
	var w actLane
	g, half := k.r(), k.r()
	k.e("shr.u32", g, L, 1)
	k.e("and.b32", half, L, 1)
	wa := k.rd()
	o := k.r()
	k.e("shl.b32", o, g, 5)
	k.e("cvt.u64.u32", wa, o)
	k.e("add.s64", wa, wa, block)
	raw := append(k.r4(wa, 16), k.r4(wa, 32)...)
	sh := k.r()
	k.e("shl.b32", sh, half, 2)
	for j := range 8 {
		c := k.r()
		k.e("shr.u32", c, raw[j], sh)
		k.e("and.b32", c, c, 0x0F0F0F0F)
		w.c[j] = c
	}
	w.sc, w.m = k.q4kScale(block, L)
	w.d, w.dmin = k.half(block, 0), k.half(block, 2)
	return w
}
