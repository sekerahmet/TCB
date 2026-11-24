package gpu

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// actRowsMV dp4a'lı çarpımda warp başına satır sayısıdır (M > 1, sm_80 altı): ağırlık bloğu bir kez
// açılır, bu kadar satırda kullanılır.
const actRowsMV = 8

// quantActRowsKernel dp4a'lı çarpım: her warp bir sütunun R satırını hesaplar (ızgara (N/8, M/R)).
// 8 şeritlik dört öbek dört bloğu birlikte okur (şerit 32 öğe); şeridin ağırlık sözcükleri bir kez
// açılır, R satırın etkinleştirmesiyle dp4a'lanır. Öbeğin tamsayı toplamları kelebekle kesin indirgenir;
// blokların float katkıları 0. şeritte blok sırasıyla toplanır (dotQ4KQ8Go ve dotQ6KQ8Go'nun sırası).
// R = 1 tek satırlı çarpımdır (üretim). dp yanlışsa (sm_61 altı) dp4a yerine dört tamsayı çarpma-toplama.
func quantActRowsKernel(dt tensor.DType, R int, dp bool) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("nicem_etk_dp4a_%s_%d_%v", dt.Name(), R, dp))
		if dp {
			k.target = 61
		}
		pq, pd, pbs, pw := k.pointer("q"), k.pointer("d"), k.pointer("bs"), k.pointer("w")
		pc, pfin := k.written("c"), k.pointer("fin")
		M, N, nb := k.u32p("m"), k.u32p("n"), k.u32p("bloklar")
		K := k.r()
		k.e("shl.b32", K, nb, 8)
		tid := k.special("%tid.x")
		lane, n := k.r(), k.r()
		k.e("and.b32", lane, tid, 31)
		k.e("shr.u32", n, tid, 5)
		k.e("mad.lo.s32", n, k.special("%ctaid.x"), actWarpsMV, n)
		end := k.label()
		p := k.p()
		k.e("setp.ge.u32", p, n, N)
		k.jumpIf(p, end)
		rows, rowOK := k.actRows(pfin, M, R)
		_, size := dt.QuantBlock()
		rowBytes, wrow := k.r(), k.rd()
		k.e("mul.lo.s32", rowBytes, nb, size)
		k.e("mul.wide.u32", wrow, n, rowBytes)
		k.e("add.s64", wrow, wrow, pw)
		grp, L := k.r(), k.r()
		k.e("shr.u32", grp, lane, 3)
		k.e("and.b32", L, lane, 7)
		acc := make([]reg, R)
		for r := range acc {
			acc[r] = k.constant(0)
		}
		i := k.whole(0)
		k.whileLess("u32", i, nb, func() {
			bi := k.r()
			k.e("add.s32", bi, i, grp)
			valid := k.p()
			k.e("setp.lt.u32", valid, bi, nb)
			bc := k.r() // geçersiz blok: son blok okunur, katkısı 0 sayılır
			k.e("sub.s32", bc, nb, 1)
			k.e("min.u32", bc, bi, bc)
			block := k.address(wrow, bc, size)
			var row func(r reg) reg
			if dt == tensor.Q4K {
				w := k.q4kLane(block, L)
				row = func(r reg) reg { return k.q4kRow(w, pq, pd, pbs, r, K, nb, bc, L) }
			} else {
				w := k.q6kLane(block, L)
				row = func(r reg) reg { return k.q6kRow(w, pq, pd, r, K, nb, bc, L) }
			}
			for r := range R {
				u := row(rows[r])
				k.e("selp.f32", u, u, float32(0), valid)
				parts := []reg{u, k.shflXor(u, 8), k.shflXor(u, 16), k.shflXor(u, 24)}
				for _, v := range parts {
					k.e("add.rn.f32", acc[r], acc[r], v)
				}
			}
			k.e("add.s32", i, i, 4)
		})
		k.e("setp.ne.u32", p, lane, 0)
		k.jumpIf(p, end)
		for r := range R {
			k.addOut(pc, rows[r], N, n, acc[r], rowOK[r])
		}
		k.put(end)
		return k
	}
}

// actRows bloğun R satırıdır (M − 1'e kırpılmış) ve satırın yazılıp yazılmayacağı: M'nin içinde ve sonlu
// (sonlu olmayan satır float yoluyla yazılır, kernel_quant_act_float.go).
func (k *emitter) actRows(pfin, M reg, R int) (rows, ok []reg) {
	rows, ok = make([]reg, R), make([]reg, R)
	r0, mLast := k.r(), k.r()
	k.e("mul.lo.s32", r0, k.special("%ctaid.y"), R)
	k.e("sub.s32", mLast, M, 1)
	for r := range R {
		rows[r] = k.r()
		k.e("add.s32", rows[r], r0, r)
		ok[r] = k.p()
		k.e("setp.lt.u32", ok[r], rows[r], M)
		k.e("min.u32", rows[r], rows[r], mLast)
		f, finite := k.r(), k.p()
		k.e("ld.global.u32", f, mem(k.address(pfin, rows[r], 4), 0))
		k.e("setp.eq.u32", finite, f, 0)
		k.e("and.pred", ok[r], ok[r], finite)
	}
	return rows, ok
}

// addOut ok doğruysa c[row·N + n] += v'dir (CPU'nun c += dot'u).
func (k *emitter) addOut(pc, row, N, n, v, ok reg) {
	skip := k.label()
	k.jumpUnless(ok, skip)
	idx := k.r()
	k.e("mad.lo.s32", idx, row, N, n)
	ha := k.address(pc, idx, 4)
	old := k.f()
	k.e("ld.global.f32", old, mem(ha, 0))
	k.e("add.rn.f32", old, old, v)
	k.e("st.global.f32", mem(ha, 0), old)
	k.put(skip)
}

// groupSum 8 şeritlik öbeğin tamsayı toplamıdır (bütün öbekte aynı).
func (k *emitter) groupSum(v reg) reg {
	for m := 1; m < 8; m *= 2 {
		o := k.shflXor(v, m)
		k.e("add.s32", v, v, o)
	}
	return v
}

// actLane şeridin bir ağırlık bloğundan açtıklarıdır: dp4a sözcükleri, ölçekler.
type actLane struct {
	c       [8]reg // dört koddan oluşan sözcükler (q4_k: 0..15; q6_k: q6 − 32, işaretli)
	sc, m   reg    // q4_k: alt bloğun ölçeği ve en küçüğü; q6_k: iki 16'lığın işaretli ölçeği
	d, dmin reg
}

// actRowCodes satırın şeride düşen 32 kodudur (sekiz sözcük): q[row·K + bi·256 + L·32].
func (k *emitter) actRowCodes(pq, row, K, bi, L reg) []reg {
	at := k.r()
	k.e("mul.lo.s32", at, row, K)
	k.e("mad.lo.s32", at, bi, actBlk, at)
	k.e("mad.lo.s32", at, L, 32, at)
	xa := k.address(pq, at, 1)
	x := make([]reg, 8)
	for j := range 2 {
		v := []reg{k.r(), k.r(), k.r(), k.r()}
		k.e("ld.global.v4.u32", vlist(v), mem(xa, 16*j))
		copy(x[4*j:], v)
	}
	return x
}

// actRowScale satırın bloğunun ölçeğidir: d[row·nb + bi].
func (k *emitter) actRowScale(pd, row, nb, bi reg) reg {
	at := k.r()
	k.e("mad.lo.s32", at, row, nb, bi)
	d8 := k.f()
	k.e("ld.global.f32", d8, mem(k.address(pd, at, 4), 0))
	return d8
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

// q4kRow satırın blok katkısıdır (öbekte aynı): isum = sc·s, msum = m·(bs[2L] + bs[2L+1]);
// t = d·isum − dmin·msum, t·d8.
func (k *emitter) q4kRow(w actLane, pq, pd, pbs, row, K, nb, bi, L reg) reg {
	x := k.actRowCodes(pq, row, K, bi, L)
	s := k.whole(0)
	for j := range 8 {
		k.dot4(s, w.c[j], x[j])
	}
	isum := k.r()
	k.e("mul.lo.s32", isum, w.sc, s)
	bsAt := k.r()
	k.e("mad.lo.s32", bsAt, row, nb, bi)
	k.e("shl.b32", bsAt, bsAt, 4)
	k.e("mad.lo.s32", bsAt, L, 2, bsAt)
	ba := k.address(pbs, bsAt, 4)
	b0, b1 := k.r(), k.r()
	k.e("ld.global.v2.u32", vlist([]reg{b0, b1}), mem(ba, 0))
	k.e("add.s32", b0, b0, b1)
	msum := k.r()
	k.e("mul.lo.s32", msum, w.m, b0)
	isum, msum = k.groupSum(isum), k.groupSum(msum)
	x1, y, t := k.f(), k.f(), k.f()
	k.e("mul.rn.f32", x1, w.d, k.toFloat(isum))
	k.e("mul.rn.f32", y, w.dmin, k.toFloat(msum))
	k.e("sub.rn.f32", t, x1, y)
	k.e("mul.rn.f32", t, t, k.actRowScale(pd, row, nb, bi))
	return t
}

// q6kLane şerit L bir 128'liğin (n = L/4) bir çeyreğini (L%4) alır: 32 öğe, iki 16'lık alt blok.
func (k *emitter) q6kLane(block, L reg) actLane {
	var w actLane
	n, quarter := k.r(), k.r()
	k.e("shr.u32", n, L, 2)
	k.e("and.b32", quarter, L, 3)
	qlOf, qhOf, t := k.r(), k.r(), k.r()
	k.e("and.b32", t, quarter, 1)
	k.e("shl.b32", qlOf, t, 5)
	k.e("mad.lo.s32", qlOf, n, 64, qlOf)
	k.e("shl.b32", qhOf, n, 5)
	k.e("add.s32", qhOf, qhOf, 128)
	qla, qha := k.rd(), k.rd()
	k.e("cvt.u64.u32", qla, qlOf)
	k.e("add.s64", qla, qla, block)
	k.e("cvt.u64.u32", qha, qhOf)
	k.e("add.s64", qha, qha, block)
	shL, shH := k.r(), k.r()
	k.e("shr.u32", shL, quarter, 1)
	k.e("shl.b32", shL, shL, 2)
	k.e("shl.b32", shH, quarter, 1)
	for j := range 8 {
		q, h := k.words16(qla, 4*j), k.words16(qha, 4*j)
		k.e("shr.u32", q, q, shL)
		k.e("and.b32", q, q, 0x0F0F0F0F)
		k.e("shr.u32", h, h, shH)
		k.e("and.b32", h, h, 0x03030303)
		k.e("shl.b32", h, h, 4)
		k.e("or.b32", q, q, h)
		// Bayt bayt q − 32: ((q | 0x80) − 0x20) ^ 0x80 ödünç almaz.
		k.e("or.b32", q, q, -2139062144)
		k.e("sub.s32", q, q, 0x20202020)
		k.e("xor.b32", q, q, -2139062144)
		w.c[j] = q
	}
	// Ölçek: sc[n·8 + çeyrek·2 + {0, 1}] (işaretli).
	si := k.r()
	k.e("shl.b32", si, n, 3)
	k.e("mad.lo.s32", si, quarter, 2, si)
	w.sc = k.signedByte(k.byteAt(block, si, 192))
	w.m = k.signedByte(k.byteAt(block, si, 193))
	w.d = k.half(block, 208)
	return w
}

// q6kRow satırın blok katkısıdır (öbekte aynı): I = sc0·Σ₀ + sc1·Σ₁ (tamsayıda kesin), t = d·I, t·d8.
func (k *emitter) q6kRow(w actLane, pq, pd, row, K, nb, bi, L reg) reg {
	x := k.actRowCodes(pq, row, K, bi, L)
	dots := [2]reg{k.whole(0), k.whole(0)}
	for j := range 8 {
		k.dot4(dots[j/4], w.c[j], x[j])
	}
	I := k.r()
	k.e("mul.lo.s32", I, w.sc, dots[0])
	k.e("mad.lo.s32", I, w.m, dots[1], I)
	I = k.groupSum(I)
	tt := k.f()
	k.e("mul.rn.f32", tt, w.d, k.toFloat(I))
	k.e("mul.rn.f32", tt, tt, k.actRowScale(pd, row, nb, bi))
	return tt
}
