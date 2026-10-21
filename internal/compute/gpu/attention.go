package gpu

import (
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// Dikkat: matris çarpımı yolu (birleşik çekirdeğin paylaşımlı belleğine sığmayan bakış
// boyları için; birleşik yol cekirdek_flash*.go ve arkauc_dikkat.go'da)
// ============================================================================
//
// GQA'da (H = G·Hkv) bir anahtar/değer bakışını paylaşan G sorgu bakışı bitişiktir: q,
// [Batch·Hkv, G·Tq, Dh] matrisleri olarak görülür ve her (b, kv) için tek çarpım yapılır.
//
//	S = q·kᵀ            (toplu çarpım, puanlar [Batch·H·Tq, Tk])
//	P = softmax(S·ölçek + maske), lse = m + log Σ  (satır çekirdeği; yerinde)
//	O = P·v             (toplu çarpım)
//
// Geri yönde P, lse'den yeniden hesaplanır: dV += Pᵀ·dO, dP = dO·vᵀ,
// dS = P·(dP − Σ dO·O)·ölçek, dQ += dS·k, dK += dSᵀ·q. Hiçbir anahtara bakamayan satırda
// P = 0 ve lse = +sonsuz'dur (çıktı ve gradyan 0). Bellek Tq·Tk ile büyür.

// attnLimits sorgu satırı i için bakılabilen anahtar aralığı [lo, hi) (CPU
// AttentionParams.Limits ile aynı kural); nedensel ve pencere çalışma anı parametresidir.
func (k *emitter) attnLimits(i, Tq, Tk, causal, window reg) (lo, hi reg) {
	c := k.r()
	k.e("add.s32", c, i, Tk)
	k.e("sub.s32", c, c, Tq)
	lo, hi = k.whole(0), k.r()
	k.e("mov.u32", hi, Tk)
	nd, pv := k.p(), k.p()
	k.e("setp.ne.u32", nd, causal, 0)
	k.e("setp.gt.s32", pv, window, 0)
	c1 := k.r()
	k.e("add.s32", c1, c, 1)
	h1 := k.r()
	k.e("min.s32", h1, hi, c1)
	k.e("selp.b32", hi, h1, hi, nd)
	l1 := k.r()
	k.e("sub.s32", l1, c1, window)
	k.e("max.s32", l1, lo, l1)
	k.e("selp.b32", lo, l1, lo, pv)
	// Nedensel olmayan pencere iki yönlüdür: hi = min(hi, c + W).
	cw := k.r()
	k.e("add.s32", cw, c, window)
	k.e("min.s32", cw, hi, cw)
	twoWay := k.p()
	k.e("not.pred", nd, nd)
	k.e("and.pred", twoWay, pv, nd)
	k.e("selp.b32", hi, cw, hi, twoWay)
	k.e("max.s32", hi, hi, lo)
	return lo, hi
}

// attnVisible anahtar jj'nin görünürlüğü (sınırlar ve ek maske).
func (k *emitter) attnVisible(jj, lo, hi, maskBase reg, md, msK reg, mask2 bool) reg {
	p1, p2 := k.p(), k.p()
	k.e("setp.ge.s32", p1, jj, lo)
	k.e("setp.lt.s32", p2, jj, hi)
	k.e("and.pred", p1, p1, p2)
	if mask2 {
		o := k.r()
		k.e("mad.lo.s32", o, jj, msK, maskBase)
		mv := k.f()
		k.e("mov.f32", mv, float32(0))
		k.ifCase(p1, "ld.global.f32", mv, mem(k.addressS(md, o, 4), 0))
		p3 := k.p()
		k.e("setp.ne.f32", p3, mv, float32(0))
		k.e("and.pred", p1, p1, p3)
	}
	return p1
}

// attnRow puan satırı çekirdeğinin ortak yazmaçları.
type attnRow struct {
	k                  *emitter
	s, lse, md         reg
	tid, R, base2, one reg
	lo, hi, mt, msK    reg
	scaleF             reg
	B                  int
	mask2              bool
}

// score S·ölçek.
func (d *attnRow) score(ofs reg) reg {
	v := d.k.load(tensor.Float32, d.k.rowAddress(d.s, ofs))
	d.k.e("mul.rn.f32", v, v, d.scaleF)
	return v
}

func (d *attnRow) visible(jj reg) reg {
	return d.k.attnVisible(jj, d.lo, d.hi, d.mt, d.md, d.msK, d.mask2)
}

// attnRowKernel puan satırları üzerinde yerinde çalışır. ileri: P = softmax ve
// lse yazılır; değilse (geri) P = görünür ? e^{S·ölçek − lse} : 0.
func attnRowKernel(B int, mask2, forward bool) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("dikkat_satir_%d_%v_%v", B, mask2, forward))
		d := &attnRow{k: k, B: B, mask2: mask2}
		d.s = k.written("s")
		if forward {
			d.lse = k.written("lse")
		} else {
			d.lse = k.pointer("lse")
		}
		d.md = k.pointer("maske")
		mo := k.pointer("maske_ofset")
		Tq, Tk := k.u32p("tq"), k.u32p("tk")
		reason2, pending2 := k.u32p("nedensel"), k.u32p("pencere")
		msQ := k.u32p("msq")
		d.msK = k.u32p("msk")
		d.scaleF = k.f32p("olcek")
		var sm reg
		if forward {
			sm = k.shared("sm", 4*B)
		}
		d.tid, d.R = k.special("%tid.x"), k.special("%ctaid.x")
		i, bh := k.r(), k.r()
		k.e("rem.u32", i, d.R, Tq)
		k.e("div.u32", bh, d.R, Tq)
		d.lo, d.hi = k.attnLimits(i, Tq, Tk, reason2, pending2)
		d.mt = k.whole(0)
		if mask2 {
			k.e("ld.global.u32", d.mt, mem(k.address(mo, bh, 4), 0))
			k.e("mad.lo.s32", d.mt, i, msQ, d.mt)
		}
		d.base2 = k.r()
		k.e("mul.lo.s32", d.base2, d.R, Tk)
		d.one = k.whole(1)
		if forward {
			d.forward(Tk, sm)
		} else {
			d.backward(Tk)
		}
		return k
	}
}

// backward P = görünür ? e^{S·ölçek − lse} : 0.
func (d *attnRow) backward(Tk reg) {
	k := d.k
	l := k.load(tensor.Float32, k.address(d.lse, d.R, 4))
	k.rowLoop(d.tid, Tk, d.one, d.base2, d.B, func(ofs, jj reg) {
		v := d.score(ofs)
		k.e("sub.rn.f32", v, v, l)
		e := k.exponential(v)
		k.e("selp.f32", e, e, float32(0), d.visible(jj))
		k.store(tensor.Float32, k.rowAddress(d.s, ofs), e)
	})
}

// forward maskeli softmax ve lse (boş satırda P = 0, lse = +sonsuz).
func (d *attnRow) forward(Tk, sm reg) {
	k := d.k
	minus := float32(math.Inf(-1))
	m := k.constant(minus)
	k.rowLoop(d.tid, Tk, d.one, d.base2, d.B, func(ofs, jj reg) {
		v := d.score(ofs)
		k.e("selp.f32", v, v, minus, d.visible(jj))
		k.e("mov.f32", m, k.largest(m, v))
	})
	m = k.blockReduce(m, sm, d.tid, d.B, k.largest)
	total2 := k.constant(0)
	k.rowLoop(d.tid, Tk, d.one, d.base2, d.B, func(ofs, jj reg) {
		v := d.score(ofs)
		k.e("sub.rn.f32", v, v, m)
		e := k.exponential(v)
		k.e("selp.f32", e, e, float32(0), d.visible(jj))
		k.e("add.rn.f32", total2, total2, e)
		k.store(tensor.Float32, k.rowAddress(d.s, ofs), e)
	})
	total2 = k.blockReduce(total2, sm, d.tid, d.B, k.sum)
	empty := k.p()
	k.e("setp.eq.f32", empty, m, minus)
	inverse := k.f()
	k.e("rcp.rn.f32", inverse, total2)
	k.e("selp.f32", inverse, float32(0), inverse, empty)
	k.rowLoop(d.tid, Tk, d.one, d.base2, d.B, func(ofs, _ reg) {
		ad := k.rowAddress(d.s, ofs)
		v := k.load(tensor.Float32, ad)
		k.e("mul.rn.f32", v, v, inverse)
		k.store(tensor.Float32, ad, v)
	})
	last := k.label()
	k.jumpIf(k.tidNonZero(d.tid), last)
	l := k.logarithm(total2)
	k.e("add.rn.f32", l, l, m)
	k.e("selp.f32", l, float32(math.Inf(1)), l, empty)
	k.store(tensor.Float32, k.address(d.lse, d.R, 4), l)
	k.put(last)
}

// attnBackwardKernel dS = P·(dP − Σ_d dO·O)·ölçek; dS, dP tamponuna yerinde yazılır.
func attnBackwardKernel(B int) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("dikkat_geri_%d", B))
		dp := k.written("dp")
		pp := k.pointer("p")
		do := k.pointer("do")
		po := k.pointer("o")
		Tk, Dv := k.u32p("tk"), k.u32p("dv")
		scaleF := k.f32p("olcek")
		sm := k.shared("sm", 4*B)
		tid := k.special("%tid.x")
		R := k.special("%ctaid.x")
		one := k.whole(1)
		tv := k.r()
		k.e("mul.lo.s32", tv, R, Dv)
		d := k.constant(0)
		k.rowLoop(tid, Dv, one, tv, B, func(ofs, _ reg) {
			a := k.load(tensor.Float32, k.rowAddress(do, ofs))
			b := k.load(tensor.Float32, k.rowAddress(po, ofs))
			k.e("fma.rn.f32", d, a, b, d)
		})
		d = k.blockReduce(d, sm, tid, B, k.sum)
		ts := k.r()
		k.e("mul.lo.s32", ts, R, Tk)
		k.rowLoop(tid, Tk, one, ts, B, func(ofs, _ reg) {
			ad := k.rowAddress(dp, ofs)
			v := k.load(tensor.Float32, ad)
			k.e("sub.rn.f32", v, v, d)
			k.e("mul.rn.f32", v, v, k.load(tensor.Float32, k.rowAddress(pp, ofs)))
			k.e("mul.rn.f32", v, v, scaleF)
			k.store(tensor.Float32, ad, v)
		})
		return k
	}
}

// attnRows puan satırı çekirdeğini başlatır.
func (g *backend) attnRows(s, lse *storage, mask2 *storage, mo *storage, p tensor.AttentionParams, forward bool) {
	R := p.Batch * p.H * p.Tq
	B := powerOfTwo(p.Tk, 32, 256)
	c := g.takeKernel(fmt.Sprintf("dikkat_satir:%d:%v:%v", B, mask2 != nil, forward), attnRowKernel(B, mask2 != nil, forward))
	pr := g.param().write(s, 0)
	if forward {
		pr.write(lse, 0)
	} else {
		pr.read(lse, 0)
	}
	reason2 := 0
	if p.Causal {
		reason2 = 1
	}
	pr.readFrom(mask2).readFrom(mo).u32(p.Tq).u32(p.Tk).u32(reason2).u32(p.Window).u32(p.MaskStrideQ).u32(p.MaskStrideK).f32(p.Scale)
	g.launch(c, [3]uint32{uint32(R), 1, 1}, [3]uint32{uint32(B), 1, 1}, pr)
}

// prepareAttn ortak denetimler ve maske ofsetlerinin yüklenmesi.
func (g *backend) prepareAttn(gt *temp, mask tensor.Storage, p tensor.AttentionParams) (md, mo *storage) {
	if p.H%max(p.Hkv, 1) != 0 {
		panic(messages.New(messages.H4061, p.H, p.Hkv))
	}
	if mask != nil {
		md = gt.readD(mask)
		checkSize(md.n) // maske dizini 32 bit
		mo = g.constInts(gt, p.MaskOffsets)
	}
	return md, mo
}

// attnMatrix dikkatin matris çarpımı yolu (ileri); lse'yi doldurur.
func (g *backend) attnMatrix(gt *temp, od, lse *storage, q, k, v tensor.View, md, mo *storage, p tensor.AttentionParams) {
	checkAttnSize(p, false)
	R := p.Batch * p.H * p.Tq
	G, Z := p.H/p.Hkv, p.Batch*p.Hkv
	qv, kv, vv := gt.read(q), gt.read(k), gt.read(v)
	s := gt.buffer(R*p.Tk, false)
	sv := tensor.View{S: s}
	g.gemm(Z, G*p.Tq, p.Tk, p.Dh, qv, false, kv, true, s, 0, true)
	g.attnRows(s, lse, md, mo, p, true)
	g.gemm(Z, G*p.Tq, p.Dv, p.Tk, sv, false, vv, false, od, 0, true)
}

// attnMatrixBackward dikkatin matris çarpımı yolu (geri); gradyanlar eklenir.
func (g *backend) attnMatrixBackward(gt *temp, gq, gk, gv, gs tensor.Storage, q, k, v tensor.View, md, mo *storage, o, lse tensor.Storage, p tensor.AttentionParams) {
	checkAttnSize(p, false)
	R := p.Batch * p.H * p.Tq
	Z, Mq := p.Batch*p.Hkv, p.H/p.Hkv*p.Tq
	qv, kv, vv := gt.read(q), gt.read(k), gt.read(v)
	dO := tensor.View{S: gt.readD(gs)}
	s := gt.buffer(R*p.Tk, false)
	g.gemm(Z, Mq, p.Tk, p.Dh, qv, false, kv, true, s, 0, true)
	g.attnRows(s, g.storageOf(lse), md, mo, p, false)
	if gv != nil {
		dv, last := gt.write(gv, true)
		g.gemm(Z, p.Tk, p.Dv, Mq, tensor.View{S: s}, true, dO, false, dv, 0, false)
		last()
	}
	if gq == nil && gk == nil {
		return
	}
	dp := gt.buffer(R*p.Tk, false)
	g.gemm(Z, Mq, p.Tk, p.Dv, dO, false, vv, true, dp, 0, true)
	B := powerOfTwo(max(p.Tk, p.Dv), 32, 256)
	od := gt.readD(o) // zincirden önce: 16 bitlik o açılırken kopya çekirdeği başlatılır
	c := g.takeKernel(fmt.Sprintf("dikkat_geri:%d", B), attnBackwardKernel(B))
	g.launch(c, [3]uint32{uint32(R), 1, 1}, [3]uint32{uint32(B), 1, 1},
		g.param().write(dp, 0).read(s, 0).readV(dO).read(od, 0).u32(p.Tk).u32(p.Dv).f32(p.Scale))
	dsv := tensor.View{S: dp}
	if gq != nil {
		dq, last := gt.write(gq, true)
		g.gemm(Z, Mq, p.Dh, p.Tk, dsv, false, kv, false, dq, 0, false)
		last()
	}
	if gk != nil {
		dk, last := gt.write(gk, true)
		g.gemm(Z, p.Tk, p.Dh, Mq, dsv, true, qv, false, dk, 0, false)
		last()
	}
}
