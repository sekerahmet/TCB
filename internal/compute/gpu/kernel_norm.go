package gpu

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Katman normu ve RMS normu (ileri ve geri)
// ============================================================================
//
// Satır başına bir blok. İleri (CPU lnIleriBlokGo ile aynı formül):
//
//	mu = Σx / D (RMS: 0),  v = Σ(x − mu)² / D,  rs = 1 / √(v + eps),
//	y  = ((x − mu)·rs)·γ + β.
//
// Geri: h = (x − mu)·rs, dh = g·γ, m1 = Σdh / D (RMS: 0), m2 = Σdh·h / D;
// gx += rs·((dh − m1) − h·m2). γ'nın gradyanı için satır başına g·h terimi geçici bir
// tampona yazılır ve satırlar boyunca (belirlenimci) indirgenir; β'nınki g'nin satır
// toplamıdır. x, y, g, gx, γ ve β kendi türlerinde (float32 ya da 16 bit) okunup yazılır;
// ortalama, rstd, γ teriminin tamponu ve bütün hesap float32'dedir.

// normConfig norm çekirdeğinin üretim ayarları.
type normConfig struct {
	B                  int
	layer, gamma, beta bool
	xGrad, gammaGrad   bool // geri yön
	// dt: x, y (ileri) ya da g (geri), γ, β, gx (geri) türleri.
	dt [5]tensor.DType
}

func (a normConfig) key(ad string) string {
	return fmt.Sprintf("%s:%d:%v:%v:%v:%v:%v:%v", ad, a.B, a.layer, a.gamma, a.beta, a.xGrad, a.gammaGrad, a.dt)
}

func normForwardKernel(a normConfig) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("norm_ileri_%d_%v_%v_%v_%s_%s_%s_%s", a.B, a.layer, a.gamma, a.beta, a.dt[0], a.dt[1], a.dt[2], a.dt[3]))
		xt, yt, gt, bt2 := a.dt[0], a.dt[1], a.dt[2], a.dt[3]
		x := k.pointer("x")
		gm := k.pointer("gamma")
		bt := k.pointer("beta")
		y := k.written("y")
		avg := k.written("ortalama")
		rstd := k.written("rstd")
		D := k.u32p("d")
		eps := k.f32p("eps")
		sm := k.shared("sm", 4*a.B)
		tid := k.special("%tid.x")
		r := k.special("%ctaid.x")
		one := k.whole(1)
		base2 := k.r()
		k.e("mul.lo.s32", base2, r, D)
		df := k.f()
		k.e("cvt.rn.f32.u32", df, D)
		mu := k.constant(0)
		if a.layer {
			k.rowLoopE(tid, D, one, base2, a.B, func(o, _ reg) {
				k.e("add.rn.f32", mu, mu, k.load(xt, k.rowAt(x, o, xt)))
			})
			mu = k.blockReduce(mu, sm, tid, a.B, k.sum)
			k.e("div.rn.f32", mu, mu, df)
		}
		v := k.constant(0)
		k.rowLoopE(tid, D, one, base2, a.B, func(o, _ reg) {
			d := k.load(xt, k.rowAt(x, o, xt))
			k.e("sub.rn.f32", d, d, mu)
			k.e("fma.rn.f32", v, d, d, v)
		})
		v = k.blockReduce(v, sm, tid, a.B, k.sum)
		k.e("div.rn.f32", v, v, df)
		k.e("add.rn.f32", v, v, eps)
		k.e("sqrt.rn.f32", v, v)
		rs := k.f()
		k.e("rcp.rn.f32", rs, v)
		k.rowLoopE(tid, D, one, base2, a.B, func(o, j reg) {
			h := k.load(xt, k.rowAt(x, o, xt))
			k.e("sub.rn.f32", h, h, mu)
			k.e("mul.rn.f32", h, h, rs)
			if a.gamma {
				k.e("mul.rn.f32", h, h, k.load(gt, k.rowAt(gm, j, gt)))
			}
			if a.beta {
				k.e("add.rn.f32", h, h, k.load(bt2, k.rowAt(bt, j, bt2)))
			}
			k.store(yt, k.rowAt(y, o, yt), h)
		})
		last := k.label()
		k.jumpIf(k.tidNonZero(tid), last)
		if a.layer {
			k.store(tensor.Float32, k.address(avg, r, 4), mu)
		}
		k.store(tensor.Float32, k.address(rstd, r, 4), rs)
		k.put(last)
		return k
	}
}

func normBackwardKernel(a normConfig) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("norm_geri_%d_%v_%v_%v_%v_%s_%s_%s_%s", a.B, a.layer, a.gamma, a.xGrad, a.gammaGrad, a.dt[0], a.dt[1], a.dt[2], a.dt[4]))
		xt, ggt, gt, gxt := a.dt[0], a.dt[1], a.dt[2], a.dt[4]
		gx := k.written("gx")
		term := k.written("terim")
		g := k.pointer("g")
		x := k.pointer("x")
		gm := k.pointer("gamma")
		avg := k.pointer("ortalama")
		rstd := k.pointer("rstd")
		D := k.u32p("d")
		sm := k.shared("sm", 4*a.B)
		tid := k.special("%tid.x")
		r := k.special("%ctaid.x")
		one := k.whole(1)
		base2 := k.r()
		k.e("mul.lo.s32", base2, r, D)
		df := k.f()
		k.e("cvt.rn.f32.u32", df, D)
		m := k.constant(0)
		if a.layer {
			m = k.load(tensor.Float32, k.address(avg, r, 4))
		}
		rs := k.load(tensor.Float32, k.address(rstd, r, 4))
		h := func(o reg) reg {
			v := k.load(xt, k.rowAt(x, o, xt))
			k.e("sub.rn.f32", v, v, m)
			k.e("mul.rn.f32", v, v, rs)
			return v
		}
		dh := func(o, j reg) reg {
			v := k.load(ggt, k.rowAt(g, o, ggt))
			if a.gamma {
				k.e("mul.rn.f32", v, v, k.load(gt, k.rowAt(gm, j, gt)))
			}
			return v
		}
		if a.gammaGrad {
			k.rowLoopE(tid, D, one, base2, a.B, func(o, _ reg) {
				t := k.load(ggt, k.rowAt(g, o, ggt))
				k.e("mul.rn.f32", t, t, h(o))
				k.store(tensor.Float32, k.rowAt(term, o, tensor.Float32), t)
			})
		}
		if a.xGrad {
			a1, a2 := k.constant(0), k.constant(0)
			k.rowLoopE(tid, D, one, base2, a.B, func(o, j reg) {
				d := dh(o, j)
				k.e("add.rn.f32", a1, a1, d)
				k.e("fma.rn.f32", a2, d, h(o), a2)
			})
			m1 := k.constant(0)
			if a.layer {
				m1 = k.blockReduce(a1, sm, tid, a.B, k.sum)
				k.e("div.rn.f32", m1, m1, df)
			}
			m2 := k.blockReduce(a2, sm, tid, a.B, k.sum)
			k.e("div.rn.f32", m2, m2, df)
			k.rowLoopE(tid, D, one, base2, a.B, func(o, j reg) {
				t := dh(o, j)
				k.e("sub.rn.f32", t, t, m1)
				hm := k.f()
				k.e("mul.rn.f32", hm, h(o), m2)
				k.e("sub.rn.f32", t, t, hm)
				k.e("mul.rn.f32", t, t, rs)
				ha := k.rowAt(gx, o, gxt)
				old := k.load(gxt, ha)
				k.e("add.rn.f32", old, old, t)
				k.store(gxt, ha, old)
			})
		}
		return k
	}
}
