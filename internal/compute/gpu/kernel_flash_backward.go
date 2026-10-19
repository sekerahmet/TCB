package gpu

import (
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Birleşik dikkat: geri yön (atomiksiz, belirlenimci)
// ============================================================================
//
// D_i = Σ_d dO[i,d]·O[i,d] önce satır başına hesaplanır (dikkat_d). Ardından iki çekirdek
// puanları lse'den yeniden kurar (P = e^{S·ölçek − lse}, dP = dO·Vᵀ, dS = P·(dP − D)·ölçek):
//   - dikkat_geri_kv: her blok bir anahtar döşemesinin dK += dSᵀ·Q ve dV += Pᵀ·dO'sunu,
//     o anahtar başını paylaşan bütün sorgu başları ve görünür sorgu döşemeleri üzerinden,
//   - dikkat_geri_q: her blok bir sorgu döşemesinin dQ += dS·K'sını görünür anahtar
//     döşemeleri üzerinden
// biriktirir. Her çıktı tek blokta, sabit sırayla toplanır (atomik yok). Çıktı kipleri:
// 0 yok, 1 eskisine ekle, 2 üzerine yaz.

const (
	noGrad = iota
	addGrad
	gradOver
)

// flashDKernel D[r] = Σ_d dO[r,d]·O[r,d]; satır başına iş parçacığı.
func flashDKernel() *emitter {
	k := newEmitter("dikkat_d")
	do, o := k.pointer("do"), k.pointer("o")
	d := k.written("d")
	R, Dv := k.u32p("r"), k.u32p("dv")
	r := k.globalIndex()
	last := k.bounded(r, R)
	base2 := k.r()
	k.e("mul.lo.s32", base2, r, Dv)
	acc := k.constant(0)
	j := k.whole(0)
	loop, done := k.label(), k.label()
	k.put(loop)
	p := k.p()
	k.e("setp.ge.u32", p, j, Dv)
	k.jumpIf(p, done)
	i := k.r()
	k.e("add.s32", i, base2, j)
	a := k.load(tensor.Float32, k.address(do, i, 4))
	b := k.load(tensor.Float32, k.address(o, i, 4))
	k.e("fma.rn.f32", acc, a, b, acc)
	k.e("add.s32", j, j, 1)
	k.jump(loop)
	k.put(done)
	k.store(tensor.Float32, k.address(d, r, 4), acc)
	k.put(last)
	return k
}

// flashBackwardState geri çekirdeklerin döşemeleri ve satır bilgileri.
type flashBackwardState struct {
	*flashShared
	qs, tilesDone, ks, vs, ps smTile
	dhp, dvp                  int
}

func (f *flashBackwardState) tiles() {
	k, a := f.k, f.a
	f.dhp, f.dvp = round16(a.Dh), round16(a.Dv)
	f.qs = smTile{k.shared("qs", 4*a.Br*(f.dhp+1)), a.Br, f.dhp + 1}
	f.tilesDone = smTile{k.shared("dos", 4*a.Br*(f.dvp+1)), a.Br, f.dvp + 1}
	f.ks = smTile{k.shared("ks", 4*a.Bc*(f.dhp+1)), a.Bc, f.dhp + 1}
	f.vs = smTile{k.shared("vs", 4*a.Bc*(f.dvp+1)), a.Bc, f.dvp + 1}
	f.ps = smTile{k.shared("ps", 4*a.Br*(a.Bc+1)), a.Br, a.Bc + 1}
}

// rowInfo sorgu döşemesinin (q0) iş parçacığına düşen satırları: indeksleri, sınırları,
// lse'leri (Tq dışında +∞) ve D'leri.
func (f *flashBackwardState) rowInfo(q0, row0, bh, plse, pd reg) ([]reg, [][3]reg, []reg, []reg) {
	k := f.k
	rpt := f.a.Br / 16
	rows, limit := make([]reg, rpt), make([][3]reg, rpt)
	lse, dd := make([]reg, rpt), make([]reg, rpt)
	lb := k.r()
	k.e("mul.lo.s32", lb, bh, f.Tq)
	for i := range rows {
		rows[i] = k.r()
		k.e("add.s32", rows[i], q0, row0)
		k.e("add.s32", rows[i], rows[i], i)
		lo, hi, mt := f.visibility(rows[i], bh)
		limit[i] = [3]reg{lo, hi, mt}
		skipOver := k.p()
		k.e("setp.lt.u32", skipOver, rows[i], f.Tq)
		li := k.r()
		k.e("add.s32", li, lb, rows[i])
		lse[i], dd[i] = k.constant(float32(math.Inf(1))), k.constant(0)
		k.ifCase(skipOver, "ld.global.f32", lse[i], mem(k.address(plse, li, 4), 0))
		k.ifCase(skipOver, "ld.global.f32", dd[i], mem(k.address(pd, li, 4), 0))
	}
	return rows, limit, lse, dd
}

// probs S ve dP'yi kurar ve P ile dS'ye çevirir (yerinde: S → P, dP → dS).
func (f *flashBackwardState) probs(row0 reg, rows []reg, j0 reg, limit [][3]reg, lse, dd []reg) ([][]reg, [][]reg) {
	k, a := f.k, f.a
	rpt, cpt := a.Br/16, a.Bc/16
	S := k.zeroMatrix(rpt, cpt)
	column0 := k.mul(f.tx, cpt)
	k.microMul(S, k.thAddr(f.qs.base2, row0, f.qs.step), f.qs.step, 1, k.thAddr(f.ks.base2, column0, f.ks.step), f.ks.step, 1, a.Dh)
	dP := k.zeroMatrix(rpt, cpt)
	k.microMul(dP, k.thAddr(f.tilesDone.base2, row0, f.tilesDone.step), f.tilesDone.step, 1, k.thAddr(f.vs.base2, column0, f.vs.step), f.vs.step, 1, a.Dv)
	f.scores(S, rows, j0, limit)
	for i := range S {
		for j := range S[i] {
			x := k.f()
			k.e("sub.rn.f32", x, S[i][j], lse[i])
			k.e("mov.f32", S[i][j], k.exponential(x))
			t := k.f()
			k.e("sub.rn.f32", t, dP[i][j], dd[i])
			k.e("mul.rn.f32", t, t, S[i][j])
			k.e("mul.rn.f32", dP[i][j], t, f.scaleF)
		}
	}
	return S, dP
}

// psWrite P ya da dS'yi ps döşemesine yazar ([sorgu][anahtar]).
func (f *flashBackwardState) psWrite(M [][]reg, row0 reg) { f.writeTile(M, row0, f.ps) }

// gradWriteMode biriktirilen döşemeyi (satırlar r0 + ty·rpt + i < lim, sütunlar < D) yazar.
func (f *flashBackwardState) gradWriteMode(M [][]reg, mode2 int, ptr, r0, lim reg, D int) {
	if mode2 == noGrad {
		return
	}
	k := f.k
	for i := range M {
		row := k.r()
		k.e("mad.lo.s32", row, f.ty, len(M), r0)
		k.e("add.s32", row, row, i)
		skipS := k.label()
		p := k.p()
		k.e("setp.ge.u32", p, row, lim)
		k.jumpIf(p, skipS)
		for j, v := range M[i] {
			column := k.r()
			k.e("mad.lo.s32", column, f.tx, len(M[i]), j)
			skip := k.label()
			q := k.p()
			k.e("setp.ge.u32", q, column, D)
			k.jumpIf(q, skip)
			idx := k.r()
			k.e("mad.lo.s32", idx, row, D, column)
			ha := k.address(ptr, idx, 4)
			if mode2 == addGrad {
				old := k.load(tensor.Float32, ha)
				k.e("add.rn.f32", v, old, v)
			}
			k.store(tensor.Float32, ha, v)
			k.put(skip)
		}
		k.put(skipS)
	}
}

// queryRange anahtar döşemesini [j0, j0+Bc) görebilecek sorgu satırlarını kapsayan aralık
// [iBas, iSon) (iBas Br'nin katı; sınırlar satırla azalmadığından iki uç yeter).
func (f *flashBackwardState) queryRange(j0 reg) (iStart, iEnd reg) {
	k := f.k
	diff := k.r() // Tq − Tk
	k.e("sub.s32", diff, f.Tq, f.Tk)
	iStart, iEnd = k.whole(0), k.r()
	k.e("mov.u32", iEnd, f.Tq)
	reason2, pending2 := k.p(), k.p()
	k.e("setp.ne.u32", reason2, f.reason2, 0)
	k.e("setp.gt.s32", pending2, f.pending2, 0)
	// Nedensel: c(i) ≥ j0 ⟺ i ≥ j0 + Tq − Tk.
	b1 := k.r()
	k.e("add.s32", b1, j0, diff)
	k.e("selp.b32", iStart, b1, iStart, reason2)
	// Nedensel olmayan pencere: c + W > j0 ⟺ i ≥ j0 − W + Tq − Tk + 1.
	b2 := k.r()
	k.e("sub.s32", b2, b1, f.pending2)
	k.e("add.s32", b2, b2, 1)
	two := k.p()
	k.e("not.pred", two, reason2)
	k.e("and.pred", two, two, pending2)
	k.e("selp.b32", iStart, b2, iStart, two)
	// Pencere: c − W + 1 < j0 + Bc ⟺ i < j0 + Bc + W − 1 + Tq − Tk.
	s := k.r()
	k.e("add.s32", s, b1, f.a.Bc-1)
	k.e("add.s32", s, s, f.pending2)
	k.e("min.s32", s, s, iEnd)
	k.e("selp.b32", iEnd, s, iEnd, pending2)
	k.e("max.s32", iStart, iStart, 0)
	k.e("max.s32", iEnd, iEnd, 0)
	k.e("div.u32", iStart, iStart, f.a.Br)
	k.e("mul.lo.s32", iStart, iStart, f.a.Br)
	return iStart, iEnd
}

// flashBackwardKVKernel dK ve dV; ızgara (anahtar döşemeleri, Hkv, Batch).
func flashBackwardKVKernel(a flashConfig, modeK, modeV int) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("%s_%d_%d", a.key("dikkat_geri_kv"), modeK, modeV))
		k.dynamic = a.isDynamic(true)
		f := &flashBackwardState{flashShared: &flashShared{k: k, a: a}}
		pq, pk, pv, pdo := k.pointer("q"), k.pointer("k"), k.pointer("v"), k.pointer("do")
		plse, pd := k.pointer("lse"), k.pointer("d")
		pdk, pdv := k.written("dk"), k.written("dv")
		f.sharedParams()
		f.headIndexes(true)
		f.tiles()
		j0 := k.r()
		k.e("mul.lo.s32", j0, k.special("%ctaid.x"), a.Bc)
		kb := k.matrixBase(pk, f.bhkv, f.Tk, a.Dh)
		vb := k.matrixBase(pv, f.bhkv, f.Tk, a.Dv)
		k.loadTile(f.ks, a.Dh, f.dhp, kb, j0, f.Tk, f.tid)
		k.loadTile(f.vs, a.Dv, f.dvp, vb, j0, f.Tk, f.tid)
		rptK := a.Bc / 16
		dK, dV := k.zeroMatrix(rptK, f.dhp/16), k.zeroMatrix(rptK, f.dvp/16)
		row0 := k.mul(f.ty, a.Br/16)
		key0 := k.mul(f.ty, rptK)
		iStart, iEnd := f.queryRange(j0)
		g := k.whole(0)
		beginD, beginB := k.label(), k.label()
		k.put(beginD)
		pg := k.p()
		k.e("setp.ge.u32", pg, g, f.G)
		k.jumpIf(pg, beginB)
		bh := k.r()
		k.e("add.s32", bh, f.h, g)
		k.e("mad.lo.s32", bh, f.b, f.H, bh)
		qb := k.matrixBase(pq, bh, f.Tq, a.Dh)
		dob := k.matrixBase(pdo, bh, f.Tq, a.Dv)
		i0 := k.r()
		k.e("mov.u32", i0, iStart)
		loop, done := k.label(), k.label()
		k.put(loop)
		p := k.p()
		k.e("setp.ge.s32", p, i0, iEnd)
		k.jumpIf(p, done)
		k.loadTile(f.qs, a.Dh, f.dhp, qb, i0, f.Tq, f.tid)
		k.loadTile(f.tilesDone, a.Dv, f.dvp, dob, i0, f.Tq, f.tid)
		rows, limit, lse, dd := f.rowInfo(i0, row0, bh, plse, pd)
		k.barrier()
		P, dS := f.probs(row0, rows, j0, limit, lse, dd)
		f.psWrite(P, row0)
		k.barrier()
		// dV += Pᵀ·dO ve dK += dSᵀ·Q: satırlar anahtar, iç eksen sorgu.
		k.microMul(dV, k.thAddr(f.ps.base2, key0, 1), 1, f.ps.step, k.thAddr(f.tilesDone.base2, k.mul(f.tx, f.dvp/16), 1), 1, f.tilesDone.step, a.Br)
		k.barrier()
		f.psWrite(dS, row0)
		k.barrier()
		k.microMul(dK, k.thAddr(f.ps.base2, key0, 1), 1, f.ps.step, k.thAddr(f.qs.base2, k.mul(f.tx, f.dhp/16), 1), 1, f.qs.step, a.Br)
		k.barrier()
		k.e("add.s32", i0, i0, a.Br)
		k.jump(loop)
		k.put(done)
		k.e("add.s32", g, g, 1)
		k.jump(beginD)
		k.put(beginB)
		f.gradWriteMode(dK, modeK, k.matrixBase(pdk, f.bhkv, f.Tk, a.Dh), j0, f.Tk, a.Dh)
		f.gradWriteMode(dV, modeV, k.matrixBase(pdv, f.bhkv, f.Tk, a.Dv), j0, f.Tk, a.Dv)
		return k
	}
}

// flashBackwardQKernel dQ; ızgara (sorgu döşemeleri, H, Batch).
func flashBackwardQKernel(a flashConfig, modeQ int) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("%s_%d", a.key("dikkat_geri_q"), modeQ))
		k.dynamic = a.isDynamic(true)
		f := &flashBackwardState{flashShared: &flashShared{k: k, a: a}}
		pq, pk, pv, pdo := k.pointer("q"), k.pointer("k"), k.pointer("v"), k.pointer("do")
		plse, pd := k.pointer("lse"), k.pointer("d")
		pdq := k.written("dq")
		f.sharedParams()
		f.headIndexes(false)
		f.tiles()
		q0 := k.r()
		k.e("mul.lo.s32", q0, k.special("%ctaid.x"), a.Br)
		qb := k.matrixBase(pq, f.bh, f.Tq, a.Dh)
		dob := k.matrixBase(pdo, f.bh, f.Tq, a.Dv)
		kb := k.matrixBase(pk, f.bhkv, f.Tk, a.Dh)
		vb := k.matrixBase(pv, f.bhkv, f.Tk, a.Dv)
		k.loadTile(f.qs, a.Dh, f.dhp, qb, q0, f.Tq, f.tid)
		k.loadTile(f.tilesDone, a.Dv, f.dvp, dob, q0, f.Tq, f.tid)
		row0 := k.mul(f.ty, a.Br/16)
		rows, limit, lse, dd := f.rowInfo(q0, row0, f.bh, plse, pd)
		jStart, jEnd := f.keyRange(q0)
		dQ := k.zeroMatrix(a.Br/16, f.dhp/16)
		j0 := k.r()
		k.e("mov.u32", j0, jStart)
		loop, done := k.label(), k.label()
		k.put(loop)
		p := k.p()
		k.e("setp.ge.u32", p, j0, jEnd)
		k.jumpIf(p, done)
		k.loadTile(f.ks, a.Dh, f.dhp, kb, j0, f.Tk, f.tid)
		k.loadTile(f.vs, a.Dv, f.dvp, vb, j0, f.Tk, f.tid)
		k.barrier()
		_, dS := f.probs(row0, rows, j0, limit, lse, dd)
		f.psWrite(dS, row0)
		k.barrier()
		k.microMul(dQ, k.thAddr(f.ps.base2, row0, f.ps.step), f.ps.step, 1, k.thAddr(f.ks.base2, k.mul(f.tx, f.dhp/16), 1), 1, f.ks.step, a.Bc)
		k.barrier()
		k.e("add.s32", j0, j0, a.Bc)
		k.jump(loop)
		k.put(done)
		f.gradWriteMode(dQ, modeQ, k.matrixBase(pdq, f.bh, f.Tq, a.Dh), q0, f.Tq, a.Dh)
		return k
	}
}
