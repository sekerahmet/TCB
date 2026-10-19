package gpu

import (
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Satır çekirdekleri: softmax, log_softmax (çapraz entropi: cekirdek_ce.go)
// ============================================================================
//
// Her satıra bir blok düşer; iş parçacıkları satırın elemanlarını B adımla gezer, satır
// toplamları ve en büyükleri blok içinde ağaç biçiminde (sabit sıra) indirgenir.
// Satır (dış, iç) çiftidir: eleman (o, l, i) bitişik düzende (o·L + l)·I + i'dedir; son
// eksen için I = 1.

// enBuyuk birlestir işlevi: max.f32 (NaN'ı yok sayar; satır en büyükleri NaN olmaz).
func (k *emitter) largest(a, b reg) reg {
	r := k.f()
	k.e("max.f32", r, a, b)
	return r
}

// satirDongu l = tid; l < L; l += B döngüsünü kurar; govde, elemanın bayt ofsetini
// ((taban + l·I)·4) alır ve kendi göstericisine ekler (satirAdresi).
func (k *emitter) rowLoop(tid, L, I, base2 reg, B int, body func(addr reg, l reg)) {
	l := k.r()
	k.e("mov.u32", l, tid)
	begin, last := k.label(), k.label()
	k.put(begin)
	p := k.p()
	k.e("setp.ge.u32", p, l, L)
	k.jumpIf(p, last)
	o := k.r()
	k.e("mad.lo.s32", o, l, I, base2)
	b := k.rd()
	k.e("mul.wide.u32", b, o, 4)
	body(b, l)
	k.e("add.s32", l, l, B)
	k.jump(begin)
	k.put(last)
}

// satirAdresi göstericiye satirDongu'nun verdiği bayt ofsetini ekler.
func (k *emitter) rowAddress(ptr, ofs reg) reg {
	r := k.rd()
	k.e("add.s64", r, ptr, ofs)
	return r
}

// satirBasi blok başına satırın taban eleman ofsetini verir: o·L·I + i.
func (k *emitter) rowStart(L, I reg) reg {
	row := k.special("%ctaid.x")
	o, i := k.r(), k.r()
	k.e("div.u32", o, row, I)
	k.e("rem.u32", i, row, I)
	t := k.r()
	k.e("mul.lo.s32", t, o, L)
	k.e("mad.lo.s32", t, t, I, i)
	return t
}

func softmaxKernel(B int, logMode bool) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("softmax_%d_%v", B, logMode))
		x := k.pointer("x")
		y := k.written("y")
		L := k.u32p("l")
		I := k.u32p("i")
		sm := k.shared("sm", 4*B)
		tid := k.special("%tid.x")
		base2 := k.rowStart(L, I)
		m := k.constant(float32(math.Inf(-1)))
		k.rowLoop(tid, L, I, base2, B, func(ofs, _ reg) {
			v := k.load(tensor.Float32, k.rowAddress(x, ofs))
			b := k.p()
			k.e("setp.gt.f32", b, v, m)
			k.e("selp.f32", m, v, m, b)
		})
		m = k.blockReduce(m, sm, tid, B, k.largest)
		last, full := k.label(), k.label()
		empty := k.p()
		k.e("setp.ne.f32", empty, m, float32(math.Inf(-1)))
		k.jumpIf(empty, full)
		// Tamamen −sonsuz satır: softmax 0, log_softmax −sonsuz.
		padding := float32(0)
		if logMode {
			padding = float32(math.Inf(-1))
		}
		dv := k.constant(padding)
		k.rowLoop(tid, L, I, base2, B, func(ofs, _ reg) {
			k.store(tensor.Float32, k.rowAddress(y, ofs), dv)
		})
		k.jump(last)
		k.put(full)
		s := k.constant(0)
		k.rowLoop(tid, L, I, base2, B, func(ofs, _ reg) {
			v := k.load(tensor.Float32, k.rowAddress(x, ofs))
			k.e("sub.rn.f32", v, v, m)
			k.e("add.rn.f32", s, s, k.exponential(v))
		})
		s = k.blockReduce(s, sm, tid, B, k.sum)
		multiplier := k.f()
		if logMode {
			multiplier = k.logarithm(s)
		} else {
			k.e("rcp.rn.f32", multiplier, s)
		}
		k.rowLoop(tid, L, I, base2, B, func(ofs, _ reg) {
			v := k.load(tensor.Float32, k.rowAddress(x, ofs))
			k.e("sub.rn.f32", v, v, m)
			r := k.f()
			if logMode {
				k.e("sub.rn.f32", r, v, multiplier)
			} else {
				k.e("mul.rn.f32", r, k.exponential(v), multiplier)
			}
			k.store(tensor.Float32, k.rowAddress(y, ofs), r)
		})
		k.put(last)
		return k
	}
}

func softmaxGradKernel(B int, logMode bool) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("softmax_turev_%d_%v", B, logMode))
		gx := k.written("gx")
		g := k.pointer("g")
		y := k.pointer("y")
		L := k.u32p("l")
		I := k.u32p("i")
		sm := k.shared("sm", 4*B)
		tid := k.special("%tid.x")
		base2 := k.rowStart(L, I)
		last := k.label()
		if logMode {
			// Tamamen maskeli satır (bütün çıktılar −sonsuz): gradyan 0.
			full := k.constant(0)
			k.rowLoop(tid, L, I, base2, B, func(ofs, _ reg) {
				v := k.load(tensor.Float32, k.rowAddress(y, ofs))
				b := k.p()
				k.e("setp.ne.f32", b, v, float32(math.Inf(-1)))
				k.e("selp.f32", full, float32(1), full, b)
			})
			full = k.blockReduce(full, sm, tid, B, k.largest)
			b := k.p()
			k.e("setp.eq.f32", b, full, float32(0))
			k.jumpIf(b, last)
		}
		s := k.constant(0)
		k.rowLoop(tid, L, I, base2, B, func(ofs, _ reg) {
			gv := k.load(tensor.Float32, k.rowAddress(g, ofs))
			if logMode {
				k.e("add.rn.f32", s, s, gv)
				return
			}
			yv := k.load(tensor.Float32, k.rowAddress(y, ofs))
			k.e("fma.rn.f32", s, gv, yv, s)
		})
		s = k.blockReduce(s, sm, tid, B, k.sum)
		k.rowLoop(tid, L, I, base2, B, func(ofs, _ reg) {
			gv := k.load(tensor.Float32, k.rowAddress(g, ofs))
			yv := k.load(tensor.Float32, k.rowAddress(y, ofs))
			t := k.f()
			if logMode {
				k.e("mul.rn.f32", t, k.exponential(yv), s)
				k.e("sub.rn.f32", t, gv, t)
			} else {
				k.e("sub.rn.f32", t, gv, s)
				k.e("mul.rn.f32", t, yv, t)
			}
			ha := k.rowAddress(gx, ofs)
			old := k.load(tensor.Float32, ha)
			k.e("add.rn.f32", old, old, t)
			k.store(tensor.Float32, ha, old)
		})
		k.put(last)
		return k
	}
}

// tidSifirDegil tid ≠ 0 yüklemi.
func (k *emitter) tidNonZero(tid reg) reg {
	p := k.p()
	k.e("setp.ne.u32", p, tid, 0)
	return p
}
