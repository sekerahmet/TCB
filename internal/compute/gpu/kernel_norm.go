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
	// D > 0 warp çekirdeğidir (normWarpD): her warp bir satır, D derleme anında bilinir;
	// blok warpRows satır işler. Sıfır: satır başına blok (B iş parçacığı).
	D int
}

func (a normConfig) key(ad string) string {
	if a.D > 0 {
		return fmt.Sprintf("%s:w%d:%v:%v:%v:%v:%v:%v", ad, a.D, a.layer, a.gamma, a.beta, a.xGrad, a.gammaGrad, a.dt)
	}
	return fmt.Sprintf("%s:%d:%v:%v:%v:%v:%v:%v", ad, a.B, a.layer, a.gamma, a.beta, a.xGrad, a.gammaGrad, a.dt)
}

// normRstd q = Σ(x − mu)² iken rs = 1 / √(q/D + eps) (q yerinde değişir); bütün norm
// çekirdeklerinin ortak adımı.
func (k *emitter) normRstd(q, df, eps reg) reg {
	k.e("div.rn.f32", q, q, df)
	k.e("add.rn.f32", q, q, eps)
	k.e("sqrt.rn.f32", q, q)
	rs := k.f()
	k.e("rcp.rn.f32", rs, q)
	return rs
}

// normAffine h ← ((h − mu)·rs)·γ[j] + β[j] (yerinde; γ ve β varsa): ileri yönün çıktısı.
func (k *emitter) normAffine(a normConfig, h, mu, rs, gm, bt, j reg) {
	k.e("sub.rn.f32", h, h, mu)
	k.e("mul.rn.f32", h, h, rs)
	if a.gamma {
		k.e("mul.rn.f32", h, h, k.load(a.dt[2], k.rowAt(gm, j, a.dt[2])))
	}
	if a.beta {
		k.e("add.rn.f32", h, h, k.load(a.dt[3], k.rowAt(bt, j, a.dt[3])))
	}
}

// normGradTerm bir sütunun geri yön terimleri (yüklenmiş g ve x ile): h = (x − m)·rs, γ terimi
// acc += g·h, dh = g·γ, a1 += dh, a2 += dh·h; h ve dh döner.
func (k *emitter) normGradTerm(gv, h, m, rs, gam, acc, a1, a2 reg) (reg, reg) {
	k.e("sub.rn.f32", h, h, m)
	k.e("mul.rn.f32", h, h, rs)
	k.e("fma.rn.f32", acc, gv, h, acc)
	dh := k.f()
	k.e("mul.rn.f32", dh, gv, gam)
	k.e("add.rn.f32", a1, a1, dh)
	k.e("fma.rn.f32", a2, dh, h, a2)
	return h, dh
}

// normGradX x'in gradyanını ekler: gx += rs·((dh − m1) − h·m2); adres hesaptan sonra kurulur.
func (k *emitter) normGradX(dh, h, m1, m2, rs reg, gxt tensor.DType, addr func() reg) {
	t, hm := k.f(), k.f()
	k.e("sub.rn.f32", t, dh, m1)
	k.e("mul.rn.f32", hm, h, m2)
	k.e("sub.rn.f32", t, t, hm)
	k.e("mul.rn.f32", t, t, rs)
	ha := addr()
	old := k.load(gxt, ha)
	k.e("add.rn.f32", old, old, t)
	k.store(gxt, ha, old)
}

func normForwardKernel(a normConfig) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("norm_ileri_%d_%v_%v_%v_%s_%s_%s_%s", a.B, a.layer, a.gamma, a.beta, a.dt[0], a.dt[1], a.dt[2], a.dt[3]))
		xt, yt := a.dt[0], a.dt[1]
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
		rs := k.normRstd(v, df, eps)
		k.rowLoopE(tid, D, one, base2, a.B, func(o, j reg) {
			h := k.load(xt, k.rowAt(x, o, xt))
			k.normAffine(a, h, mu, rs, gm, bt, j)
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

// normGradCols kaynaşık geri yönde iş parçacığı başına sütun sayısının sınırı: D ≤ B·8 (B = 256
// ile 2048) kaynaşık çekirdekle, daha genişi satır başına bloklu çekirdek ve geçici tamponla.
const normGradCols = 8

// normBackwardFusedKernel γ'nın gradyanı istenen geri yöndür (Apex, Liger ve Triton layer-norm
// geri yönündeki yöntem): blok b ardışık satır parçasını [b·parça, min((b+1)·parça, R)) sırayla
// işler; iş parçacığı sütunları j = tid + c·B (c < C) yazmaçlarda tutar. Her satırda x ve g bir
// kez okunur, h ve dh yazmaçta kalır; γ teriminin (g·h) toplamı sütun başına yazmaçta, satır
// sırasıyla birikir ve sonda blok başına kısmi toplam [b, D] yazılır (satırlar boyunca son toplam
// ayrı, sabit sıralı indirgemededir: backend_row.go). R×D'lik terim tamponu yoktur. x'in gradyanı
// satır başına iki blok indirgemesiyle normBackwardKernel'deki formülle aynıdır.
func normBackwardFusedKernel(a normConfig, C int) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("norm_geri_%d_%d_%v_%v_%v_parcali_%s_%s_%s_%s", a.B, C, a.layer, a.gamma, a.xGrad, a.dt[0], a.dt[1], a.dt[2], a.dt[4]))
		f := &fusedNorm{a: a, gx: k.written("gx"), part: k.written("kismi"), g: k.pointer("g"), x: k.pointer("x")}
		gm := k.pointer("gamma")
		f.avg, f.rstd = k.pointer("ortalama"), k.pointer("rstd")
		f.D = k.u32p("d")
		R, chunk := k.u32p("r"), k.u32p("parca")
		f.sm = k.shared("sm", 4*a.B)
		f.tid = k.special("%tid.x")
		b := k.special("%ctaid.x")
		f.df = k.f()
		k.e("cvt.rn.f32.u32", f.df, f.D)
		f.zero = k.constant(0)
		k.fusedNormColumns(f, C, gm)
		r, r1 := k.r(), k.r()
		k.e("mul.lo.s32", r, b, chunk)
		k.e("add.s32", r1, r, chunk)
		k.e("min.u32", r1, r1, R)
		loop, done := k.label(), k.label()
		k.put(loop)
		p := k.p()
		k.e("setp.ge.u32", p, r, r1)
		k.jumpIf(p, done)
		k.fusedNormRow(f, r)
		k.e("add.s32", r, r, 1)
		k.jump(loop)
		k.put(done)
		pb := k.r()
		k.e("mul.lo.s32", pb, b, f.D)
		for c := range f.cols {
			skip := k.label()
			k.jumpUnless(f.valid[c], skip)
			o := k.r()
			k.e("add.s32", o, pb, f.cols[c])
			k.store(tensor.Float32, k.rowAt(f.part, o, tensor.Float32), f.acc[c])
			k.put(skip)
		}
		return k
	}
}

// fusedNorm kaynaşık norm geri yönünün yazmaçları: göstericiler, sütunlar (geçersiz sütun son
// sütunu okur, katkısı 0), sütun başına γ ve γ teriminin toplayıcısı.
type fusedNorm struct {
	a                                normConfig
	gx, part, g, x, avg, rstd, D, sm reg
	tid, df, zero                    reg
	cols, valid, gam, acc            []reg
}

// fusedNormColumns iş parçacığının sütunlarını, geçerliliklerini, γ değerlerini (bir kez okunur)
// ve sıfır toplayıcıları kurar.
func (k *emitter) fusedNormColumns(f *fusedNorm, C int, gm reg) {
	last := k.r()
	k.e("add.s32", last, f.D, -1)
	f.cols, f.valid, f.gam, f.acc = make([]reg, C), make([]reg, C), make([]reg, C), make([]reg, C)
	for c := range f.cols {
		f.cols[c], f.valid[c] = k.r(), k.p()
		k.e("add.s32", f.cols[c], f.tid, c*f.a.B)
		k.e("setp.lt.u32", f.valid[c], f.cols[c], f.D)
		k.e("min.u32", f.cols[c], f.cols[c], last)
		f.gam[c] = k.constant(1)
		if f.a.gamma {
			f.gam[c] = k.load(f.a.dt[2], k.rowAt(gm, f.cols[c], f.a.dt[2]))
		}
		f.acc[c] = k.constant(0)
	}
}

// fusedNormRow r. satırı işler: x ve g bir kez okunur, γ terimi toplayıcılara eklenir, x'in
// gradyanı (isteniyorsa) iki blok indirgemesiyle yazılır.
func (k *emitter) fusedNormRow(f *fusedNorm, r reg) {
	a := f.a
	xt, ggt, gxt := a.dt[0], a.dt[1], a.dt[4]
	base2 := k.r()
	k.e("mul.lo.s32", base2, r, f.D)
	m := f.zero
	if a.layer {
		m = k.load(tensor.Float32, k.address(f.avg, r, 4))
	}
	rs := k.load(tensor.Float32, k.address(f.rstd, r, 4))
	hs, dhs := make([]reg, len(f.cols)), make([]reg, len(f.cols))
	a1, a2 := k.constant(0), k.constant(0)
	for c := range f.cols {
		o := k.r()
		k.e("add.s32", o, base2, f.cols[c])
		gv := k.load(ggt, k.rowAt(f.g, o, ggt))
		k.e("selp.f32", gv, gv, f.zero, f.valid[c])
		hs[c], dhs[c] = k.normGradTerm(gv, k.load(xt, k.rowAt(f.x, o, xt)), m, rs, f.gam[c], f.acc[c], a1, a2)
	}
	if !a.xGrad {
		return
	}
	m1 := f.zero
	if a.layer {
		m1 = k.blockReduce(a1, f.sm, f.tid, a.B, k.sum)
		k.e("div.rn.f32", m1, m1, f.df)
	}
	m2 := k.blockReduce(a2, f.sm, f.tid, a.B, k.sum)
	k.e("div.rn.f32", m2, m2, f.df)
	for c := range f.cols {
		skip := k.label()
		k.jumpUnless(f.valid[c], skip)
		k.normGradX(dhs[c], hs[c], m1, m2, rs, gxt, func() reg {
			o := k.r()
			k.e("add.s32", o, base2, f.cols[c])
			return k.rowAt(f.gx, o, gxt)
		})
		k.put(skip)
	}
}
