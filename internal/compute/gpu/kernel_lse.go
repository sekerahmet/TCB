package gpu

import (
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// TR_logsumexp'in satır çekirdekleri (15.18). Satır başına bir blok; B yalnız satırın uzunluğundan
// seçilir (powerOfTwo), bitler yalnız şekle bağlıdır (15.22a). İki geçiş: önce NaN'ı koruyan en
// büyük öğe m; m sonlu değilse (−∞, +∞, NaN) sonuç m'dir. Sonluysa iş parçacığı başına telafili
// Σ e^{x−m}, blok içinde sabit sıralı ağaç toplamı; sonuç m + log s. Hesap float32'dedir, 16
// bitlik girdi okunurken açılır, sonuç kendi türüne bir kez yuvarlanır.

// finitePred |v| < +∞ yüklemi (NaN'da yanlış).
func (k *emitter) finitePred(v reg) reg {
	a := k.f()
	k.e("abs.f32", a, v)
	p := k.p()
	k.e("setp.lt.f32", p, a, float32(math.Inf(1)))
	return p
}

// rowExpSum satırın Σ e^{x−m}'idir: iş parçacığı başına telafili, blok içinde ağaç toplamı.
func (k *emitter) rowExpSum(x, m, tid, L, I, base2, sm reg, B int, xt tensor.DType) reg {
	cs := k.newCompensatedSum()
	k.rowLoopE(tid, L, I, base2, B, func(o, _ reg) {
		v := k.load(xt, k.rowAt(x, o, xt))
		k.e("sub.rn.f32", v, v, m)
		k.addCompensated(cs, k.exponential(v))
	})
	return k.blockReduce(k.compensatedTotal(cs), sm, tid, B, k.sum)
}

// lseKernel y[satır] = log Σ e^{x[satır, ·]}; x xt'de, y yt'de.
func lseKernel(B int, xt, yt tensor.DType) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("logsumexp_%d_%s_%s", B, xt, yt))
		x := k.pointer("x")
		y := k.written("y")
		L := k.u32p("l")
		I := k.u32p("i")
		sm := k.shared("sm", 4*B)
		tid := k.special("%tid.x")
		base2 := k.rowStart(L, I)
		m := k.constant(float32(math.Inf(-1)))
		k.rowLoopE(tid, L, I, base2, B, func(o, _ reg) {
			k.e("mov.f32", m, k.largest(m, k.load(xt, k.rowAt(x, o, xt))))
		})
		m = k.blockReduce(m, sm, tid, B, k.largest)
		r := k.f()
		k.e("mov.f32", r, m)
		done := k.label()
		k.jumpIf(k.notPred(k.finitePred(m)), done)
		s := k.rowExpSum(x, m, tid, L, I, base2, sm, B, xt)
		k.e("add.rn.f32", r, m, k.logarithm(s))
		k.put(done)
		last := k.label()
		k.jumpIf(k.tidNonZero(tid), last)
		row := k.special("%ctaid.x")
		k.store(yt, k.rowAt(y, row, yt), r)
		k.put(last)
		return k
	}
}

// lseGradKernel gx[satır, i] += g[satır]·∂y/∂x[satır, i] (15.18): m −∞ ise 0; +∞ ise türevin
// tamamı satırın ilk +∞ öğesine; yoksa g·e^{x−m}/s (NaN'lı satırda NaN). İlk +∞'ın konumu
// float32'de en küçük indekstir: satır 2²⁴ öğeyi geçmez (çağıran denetler).
func lseGradKernel(B int, dt [3]tensor.DType) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("logsumexp_turev_%d_%s_%s_%s", B, dt[0], dt[1], dt[2]))
		gx := k.written("gx")
		g := k.pointer("g")
		x := k.pointer("x")
		L := k.u32p("l")
		I := k.u32p("i")
		sm := k.shared("sm", 4*B)
		tid := k.special("%tid.x")
		base2 := k.rowStart(L, I)
		row := k.special("%ctaid.x")
		gv := k.load(dt[1], k.rowAt(g, row, dt[1]))
		m := k.constant(float32(math.Inf(-1)))
		k.rowLoopE(tid, L, I, base2, B, func(o, _ reg) {
			k.e("mov.f32", m, k.largest(m, k.load(dt[2], k.rowAt(x, o, dt[2]))))
		})
		m = k.blockReduce(m, sm, tid, B, k.largest)
		last, finite := k.label(), k.label()
		empty, inf := k.p(), k.p()
		k.e("setp.eq.f32", empty, m, float32(math.Inf(-1)))
		k.jumpIf(empty, last)
		k.e("setp.eq.f32", inf, m, float32(math.Inf(1)))
		k.jumpIf(k.notPred(inf), finite)
		k.lseGradAtFirstInf(gx, gv, x, tid, L, I, base2, sm, B, dt)
		k.jump(last)
		k.put(finite)
		s := k.rowExpSum(x, m, tid, L, I, base2, sm, B, dt[2])
		k.rowLoopE(tid, L, I, base2, B, func(o, _ reg) {
			v := k.load(dt[2], k.rowAt(x, o, dt[2]))
			k.e("sub.rn.f32", v, v, m)
			t := k.f()
			k.e("div.rn.f32", t, k.exponential(v), s)
			k.e("mul.rn.f32", t, gv, t)
			k.addAt(gx, o, t, dt[0])
		})
		k.put(last)
		return k
	}
}

// lseGradAtFirstInf satırın ilk +∞ öğesine g'yi ekler (blok içinde en küçük indeks).
func (k *emitter) lseGradAtFirstInf(gx, gv, x, tid, L, I, base2, sm reg, B int, dt [3]tensor.DType) {
	first := k.constant(float32(math.Inf(1)))
	k.rowLoopE(tid, L, I, base2, B, func(o, l reg) {
		v := k.load(dt[2], k.rowAt(x, o, dt[2]))
		p := k.p()
		k.e("setp.eq.f32", p, v, float32(math.Inf(1)))
		lf := k.f()
		k.e("cvt.rn.f32.u32", lf, l)
		k.e("min.f32", lf, lf, first)
		k.e("selp.f32", first, lf, first, p)
	})
	first = k.blockReduce(first, sm, tid, B, k.smaller)
	k.rowLoopE(tid, L, I, base2, B, func(o, l reg) {
		lf := k.f()
		k.e("cvt.rn.f32.u32", lf, l)
		p := k.p()
		k.e("setp.eq.f32", p, lf, first)
		skip := k.label()
		k.jumpIf(k.notPred(p), skip)
		k.addAt(gx, o, gv, dt[0])
		k.put(skip)
	})
}

// smaller birleştirme işlevi: min(a, b).
func (k *emitter) smaller(a, b reg) reg {
	r := k.f()
	k.e("min.f32", r, a, b)
	return r
}

// notPred yüklemin değilidir.
func (k *emitter) notPred(p reg) reg {
	q := k.p()
	k.e("not.pred", q, p)
	return q
}

// addAt dt türündeki depolamanın o. öğesine t'yi ekler.
func (k *emitter) addAt(ptr, o, t reg, dt tensor.DType) {
	a := k.rowAt(ptr, o, dt)
	old := k.load(dt, a)
	k.e("add.rn.f32", old, old, t)
	k.store(dt, a, old)
}
