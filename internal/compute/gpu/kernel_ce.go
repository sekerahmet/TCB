package gpu

import (
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Birleşik çapraz entropi: tek geçişli logsumexp ve doğrudan gradyan
// ============================================================================
//
// İleri: satır başına bir blok; her iş parçacığı satırı adımla gezerken çevrimiçi
// logsumexp tutar (m en büyük, s = Σ e^{x−m}; yeni en büyükte s ölçeklenir), çiftler blok
// içinde sabit sırayla birleştirilir. Logitler bir kez okunur; softmax ara tensörü yoktur.
// Geri: gx = k·e^{x−lse} − k·[c = hedef] (k = g/geçerli) doğrudan yazılır (üzerine ya da
// eskisine eklenerek). vek doğruysa satır 4'lü vektörlerle okunur/yazılır (C % 4 = 0 ve
// hizalı görünüm).

const ceBlock = 256

// ceGez satırın elemanlarını (vek ise 4'lülerle) iş parçacığı adımıyla gezer; govde her
// elemanın değerini ve sütununu alır. adr satır tabanının bayt adresi (x için), ofs
// işlevi diğer göstericiler için aynı bayt ofsetini verir.
func (k *emitter) ceWalk(tid, C reg, vec bool, body func(ofs reg, c reg, n int)) {
	step, n := ceBlock, 1
	if vec {
		step, n = 4*ceBlock, 4
	}
	c := k.r()
	k.e("mul.lo.s32", c, tid, n)
	begin, last := k.label(), k.label()
	k.put(begin)
	p := k.p()
	k.e("setp.ge.u32", p, c, C)
	k.jumpIf(p, last)
	ofs := k.rd()
	k.e("mul.wide.u32", ofs, c, 4)
	body(ofs, c, n)
	k.e("add.s32", c, c, step)
	k.jump(begin)
	k.put(last)
}

// ceDegerler adresteki 1 ya da 4 float32.
func (k *emitter) ceValues(addr reg, n int) []reg {
	if n == 1 {
		return []reg{k.load(tensor.Float32, addr)}
	}
	return k.loadV("global", "", addr, 0, 4)
}

// cevrimiciEkle (m, s) çiftine v'yi ekler: v > m ise s = s·e^{m−v} + 1, m = v; değilse
// s += e^{v−m}. v = −∞ katkı vermez (m = −∞ iken −∞ − (−∞) = NaN olurdu; CPU'da e^{−∞} = 0).
// NaN, s'yi NaN yapar (CPU ile aynı).
func (k *emitter) onlineAdd(m, s, v reg) {
	large := k.p()
	k.e("setp.gt.f32", large, v, m)
	d1, d2 := k.f(), k.f()
	k.e("sub.rn.f32", d1, m, v)
	k.e("sub.rn.f32", d2, v, m)
	k.e("selp.f32", d1, d1, d2, large)
	negInf := k.p()
	k.e("setp.eq.f32", negInf, v, float32(math.Inf(-1)))
	k.e("selp.f32", d1, float32(math.Inf(-1)), d1, negInf)
	e := k.exponential(d1)
	s1 := k.f()
	k.e("fma.rn.f32", s1, s, e, float32(1))
	k.e("add.rn.f32", s, s, e)
	k.e("selp.f32", s, s1, s, large)
	k.e("selp.f32", m, v, m, large)
}

// ciftBirlestir iki (m, s) çiftinin birleşimi; s = 0 olan (boş) çift katkı vermez.
func (k *emitter) pairJoin(m1, s1, m2, s2 reg) (reg, reg) {
	m := k.f()
	k.e("max.f32", m, m1, m2)
	term := func(mi, si reg) reg {
		d := k.f()
		k.e("sub.rn.f32", d, mi, m)
		t := k.f()
		k.e("mul.rn.f32", t, si, k.exponential(d))
		empty := k.p()
		k.e("setp.eq.f32", empty, si, float32(0))
		k.e("selp.f32", t, float32(0), t, empty)
		return t
	}
	s := k.f()
	k.e("add.rn.f32", s, term(m1, s1), term(m2, s2))
	return m, s
}

// ciftIndirge blokta (m, s) çiftlerini ağaç biçiminde birleştirir; sonuç bütün iş
// parçacıklarına döner. smM, smS en az 4·B baytlık paylaşımlı bellek.
func (k *emitter) pairReduce(m, s, smM, smS, tid reg, B int) (reg, reg) {
	am, as := k.r(), k.r()
	k.e("mad.lo.s32", am, tid, 4, smM)
	k.e("mad.lo.s32", as, tid, 4, smS)
	k.e("st.shared.f32", mem(am, 0), m)
	k.e("st.shared.f32", mem(as, 0), s)
	k.barrier()
	for step := B / 2; step > 0; step /= 2 {
		skip := k.label()
		p := k.p()
		k.e("setp.ge.u32", p, tid, step)
		k.jumpIf(p, skip)
		m1, s1, m2, s2 := k.f(), k.f(), k.f(), k.f()
		k.e("ld.shared.f32", m1, mem(am, 0))
		k.e("ld.shared.f32", s1, mem(as, 0))
		k.e("ld.shared.f32", m2, mem(am, 4*step))
		k.e("ld.shared.f32", s2, mem(as, 4*step))
		mm, ss := k.pairJoin(m1, s1, m2, s2)
		k.e("st.shared.f32", mem(am, 0), mm)
		k.e("st.shared.f32", mem(as, 0), ss)
		k.put(skip)
		k.barrier()
	}
	rm, rs := k.f(), k.f()
	k.e("ld.shared.f32", rm, mem(smM, 0))
	k.e("ld.shared.f32", rs, mem(smS, 0))
	k.barrier()
	return rm, rs
}

// ceIleriCekirdegi satır başına lse = m + log s ve kayıp = lse − x[hedef]; hedefi negatif
// satırlarda ikisi de 0.
func ceForwardKernel(vec bool) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("ce_ileri_%v", vec))
		x := k.pointer("x")
		target := k.pointer("hedef")
		lse := k.written("lse")
		loss2 := k.written("kayip")
		C := k.u32p("c")
		smM, smS := k.shared("sm_m", 4*ceBlock), k.shared("sm_s", 4*ceBlock)
		tid := k.special("%tid.x")
		r := k.special("%ctaid.x")
		base2 := k.rd()
		k.e("mul.wide.u32", base2, r, C)
		k.e("shl.b64", base2, base2, 2)
		k.e("add.s64", base2, base2, x)
		t := k.r()
		k.e("ld.global.s32", t, mem(k.address(target, r, 4), 0))
		last, valid := k.label(), k.label()
		p := k.p()
		k.e("setp.ge.s32", p, t, 0)
		k.jumpIf(p, valid)
		k.jumpIf(k.tidNonZero(tid), last)
		zero := k.constant(0)
		k.store(tensor.Float32, k.address(lse, r, 4), zero)
		k.store(tensor.Float32, k.address(loss2, r, 4), zero)
		k.jump(last)
		k.put(valid)
		m, s := k.constant(float32(math.Inf(-1))), k.constant(0)
		k.ceWalk(tid, C, vec, func(ofs, _ reg, n int) {
			for _, v := range k.ceValues(k.rowAddress(base2, ofs), n) {
				k.onlineAdd(m, s, v)
			}
		})
		m, s = k.pairReduce(m, s, smM, smS, tid, ceBlock)
		k.jumpIf(k.tidNonZero(tid), last)
		l := k.logarithm(s)
		k.e("add.rn.f32", l, l, m)
		k.store(tensor.Float32, k.address(lse, r, 4), l)
		xv := k.load(tensor.Float32, k.addressS(base2, t, 4))
		kv := k.f()
		k.e("sub.rn.f32", kv, l, xv)
		k.store(tensor.Float32, k.address(loss2, r, 4), kv)
		k.put(last)
		return k
	}
}

// ceGeriCekirdegi gx[r, c] (+)= k·e^{x−lse} − (c = hedef ? k : 0), k = g·ters; satır başına
// blok. uzerine doğruysa eski içerik okunmaz ve hedefi negatif satırlar 0 yazılır.
func ceBackwardKernel(vec, over bool) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("ce_geri_%v_%v", vec, over))
		gx := k.written("gx")
		g := k.pointer("g")
		x := k.pointer("x")
		lse := k.pointer("lse")
		target := k.pointer("hedef")
		C := k.u32p("c")
		inverse := k.f32p("ters")
		r := k.special("%ctaid.x")
		tid := k.special("%tid.x")
		row := k.rd()
		k.e("mul.wide.u32", row, r, C)
		k.e("shl.b64", row, row, 2)
		xt, gt := k.rd(), k.rd()
		k.e("add.s64", xt, row, x)
		k.e("add.s64", gt, row, gx)
		t := k.r()
		k.e("ld.global.s32", t, mem(k.address(target, r, 4), 0))
		last := k.label()
		skipOver := k.p()
		k.e("setp.ge.s32", skipOver, t, 0)
		if !over {
			k.jumpUnless(skipOver, last)
		}
		coef := k.load(tensor.Float32, g)
		k.e("mul.rn.f32", coef, coef, inverse)
		lv := k.load(tensor.Float32, k.address(lse, r, 4))
		k.ceWalk(tid, C, vec, func(ofs, c reg, n int) {
			xs := k.ceValues(k.rowAddress(xt, ofs), n)
			ha := k.rowAddress(gt, ofs)
			var old []reg
			if !over {
				old = k.ceValues(ha, n)
			}
			out := make([]reg, n)
			for i, xv := range xs {
				d := k.f()
				k.e("sub.rn.f32", d, xv, lv)
				v := k.f()
				k.e("mul.rn.f32", v, coef, k.exponential(d))
				ci := k.r()
				k.e("add.s32", ci, c, i)
				equal := k.p()
				k.e("setp.eq.s32", equal, ci, t)
				vk := k.f()
				k.e("sub.rn.f32", vk, v, coef)
				k.e("selp.f32", v, vk, v, equal)
				if over {
					k.e("selp.f32", v, v, float32(0), skipOver) // yok sayılan satır: 0
				} else {
					k.e("add.rn.f32", v, old[i], v)
				}
				out[i] = v
			}
			if n == 1 {
				k.store(tensor.Float32, ha, out[0])
			} else {
				k.storeV("global", "", ha, 0, out)
			}
		})
		k.put(last)
		return k
	}
}
