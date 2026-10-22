package gpu

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Norm çekirdeklerinin warp biçimi: warp başına satır (dar satırlar)
// ============================================================================
//
// Satır başına blok (kernel_norm.go) geniş satırda verimlidir; D = 64 gibi dar satırda blok 64
// iş parçacığıdır, her biri tek eleman alır ve toplamlar engelli bir ağaçla indirgenir. Warp
// biçiminde her warp bir satırdır: şerit satırın D/32 elemanını yazmaçta tutar (x bir kez
// okunur), toplamlar warp içinde kelebek karıştırmayla (shfl.bfly, engelsiz; gemv ve flash ile
// aynı) indirgenir; blok warpRows satırdır. Formül kernel_norm.go'nun ortak adımlarıdır
// (normRstd, normAffine, normGradTerm, normGradX); toplama sırası farklıdır (şerit içinde
// sıralı, sonra kelebek), sonuç belirlenimcidir. Hedef modelde (bf16, 32768 token): q_norm
// [·, 8, 64] ileri 150 → 31,5 µs, k_norm (adımlı) 101 → 16,8 µs; torch.compile 84 ve 64 µs.
//
// Satırlar iki düzeyli adımla gezilir (rowsAddress): ön eksenleri iki düzeye inen adımlı bir
// görünüm (ör. K ile V'nin tek çarpımdan bölünmüş K yarısı) kopyalanmadan okunur (rowMap).

// warpRows warp çekirdeğinde bir bloğun işlediği satır (warp) sayısıdır.
const warpRows = 8

// normWarpD D'nin warp ileri çekirdeğine uygun olduğudur: 32'nin katı ve en çok 256 (şerit başına
// en çok 8 eleman yazmaçta). Daha geniş satırlar satır başına blokla kalır (orada blok verimli).
func normWarpD(D int) bool { return D > 0 && D%32 == 0 && D <= 256 }

// normWarpGradD D'nin warp geri çekirdeğine uygun olduğudur: 32'nin katı ve en çok 512 (şerit
// başına en çok 16 sütun; γ teriminin blok sonundaki birleştirmesi 8·D float32 paylaşımlı bellek).
func normWarpGradD(D int) bool { return D > 0 && D%32 == 0 && D <= 512 }

// rowsAddress satırın girdideki ilk elemanının ofsetidir: (row / H)·S1 + (row % H)·S0 (bitişik
// girdide H = 1, S1 = D).
func (k *emitter) rowsAddress(row, H, S0, S1 reg) reg {
	q, r, o := k.r(), k.r(), k.r()
	k.e("div.u32", q, row, H)
	k.e("rem.u32", r, row, H)
	k.e("mul.lo.s32", o, q, S1)
	k.e("mad.lo.s32", o, r, S0, o)
	return o
}

// warpSum warp içindeki 32 şeridin değerini kelebek karıştırmayla (sabit sıra) toplar; sonuç
// bütün şeritlerdedir.
func (k *emitter) warpSum(v reg) reg {
	for m := 16; m > 0; m /= 2 {
		v = k.sum(v, k.shflXor(v, m))
	}
	return v
}

// warpLanes iş parçacığının şeridi ve warp'ı (tid % 32, tid / 32).
func (k *emitter) warpLanes() (tid, lane, w reg) {
	tid = k.special("%tid.x")
	lane, w = k.r(), k.r()
	k.e("and.b32", lane, tid, 31)
	k.e("shr.u32", w, tid, 5)
	return tid, lane, w
}

// laneOffset o + 32·e: şeridin e. elemanı (warp'ın 32 şeridi ardışık elemanları okur).
func (k *emitter) laneOffset(o reg, e int) reg {
	r := k.r()
	k.e("add.s32", r, o, 32*e)
	return r
}

// normForwardWarpKernel normForwardKernel'in warp biçimidir (a.D > 0).
func normForwardWarpKernel(a normConfig) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("norm_ileri_w%d_%v_%v_%v_%s_%s_%s_%s", a.D, a.layer, a.gamma, a.beta, a.dt[0], a.dt[1], a.dt[2], a.dt[3]))
		xt, yt := a.dt[0], a.dt[1]
		x, gm, bt := k.pointer("x"), k.pointer("gamma"), k.pointer("beta")
		y, avg, rstd := k.written("y"), k.written("ortalama"), k.written("rstd")
		R, H, S0, S1 := k.u32p("satir"), k.u32p("h"), k.u32p("s0"), k.u32p("s1")
		eps := k.f32p("eps")
		_, lane, w := k.warpLanes()
		row := k.r()
		k.e("mad.lo.s32", row, k.special("%ctaid.x"), warpRows, w)
		last := k.label()
		outside := k.p()
		k.e("setp.ge.u32", outside, row, R)
		k.jumpIf(outside, last)
		xo := k.rowsAddress(row, H, S0, S1)
		k.e("add.s32", xo, xo, lane)
		yo := k.r()
		k.e("mad.lo.s32", yo, row, a.D, lane)
		v := make([]reg, a.D/32)
		for e := range v {
			v[e] = k.load(xt, k.rowAt(x, k.laneOffset(xo, e), xt))
		}
		df := k.constant(float32(a.D))
		mu := k.constant(0)
		if a.layer {
			for _, h := range v {
				k.e("add.rn.f32", mu, mu, h)
			}
			mu = k.warpSum(mu)
			k.e("div.rn.f32", mu, mu, df)
		}
		q := k.constant(0)
		for _, h := range v {
			d := k.f()
			k.e("sub.rn.f32", d, h, mu)
			k.e("fma.rn.f32", q, d, d, q)
		}
		rs := k.normRstd(k.warpSum(q), df, eps)
		for e, h := range v {
			k.normAffine(a, h, mu, rs, gm, bt, k.laneOffset(lane, e))
			k.store(yt, k.rowAt(y, k.laneOffset(yo, e), yt), h)
		}
		k.jumpIf(k.tidNonZero(lane), last)
		if a.layer {
			k.store(tensor.Float32, k.address(avg, row, 4), mu)
		}
		k.store(tensor.Float32, k.address(rstd, row, 4), rs)
		k.put(last)
		return k
	}
}

// warpGrad warp geri çekirdeğinin yazmaçları ve göstericileri.
type warpGrad struct {
	a                              normConfig
	gx, g, x, avg, rstd, H, S0, S1 reg
	lane, df, zero                 reg
	gam, acc                       []reg
}

// normBackwardWarpKernel γ'nın gradyanı istenen geri yönün warp biçimidir (a.D > 0):
// normBackwardFusedKernel gibi blok b ardışık satır parçasını işler, parçanın satırlarını warp'lar
// paylaşır (warp w: w, w+8, ...). γ teriminin toplamı şerit yazmacında satır sırasıyla birikir;
// blok sonunda warp'ların toplamları sabit sırayla (warp 0, 1, ...) birleştirilip blok başına
// kısmi toplam [b, D] yazılır (satırlar boyunca son toplam ayrı, sabit sıralı indirgemededir).
func normBackwardWarpKernel(a normConfig) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("norm_geri_w%d_%v_%v_%v_%s_%s_%s_%s%s", a.D, a.layer, a.gamma, a.xGrad, a.dt[0], a.dt[1], a.dt[2], a.dt[4], setSuffix(a.set)))
		f := &warpGrad{a: a, gx: k.written("gx")}
		part := k.written("kismi")
		f.g, f.x = k.pointer("g"), k.pointer("x")
		gm := k.pointer("gamma")
		f.avg, f.rstd = k.pointer("ortalama"), k.pointer("rstd")
		R, chunk := k.u32p("r"), k.u32p("parca")
		f.H, f.S0, f.S1 = k.u32p("h"), k.u32p("s0"), k.u32p("s1")
		sm := k.shared("sm", 4*warpRows*a.D)
		tid, lane, w := k.warpLanes()
		f.lane = lane
		b := k.special("%ctaid.x")
		f.df, f.zero = k.constant(float32(a.D)), k.constant(0)
		f.gam, f.acc = make([]reg, a.D/32), make([]reg, a.D/32)
		for e := range f.gam {
			f.gam[e] = k.constant(1)
			if a.gamma {
				f.gam[e] = k.load(a.dt[2], k.rowAt(gm, k.laneOffset(lane, e), a.dt[2]))
			}
			f.acc[e] = k.constant(0)
		}
		r, r1 := k.r(), k.r()
		k.e("mul.lo.s32", r1, b, chunk)
		k.e("add.s32", r, r1, w)
		k.e("add.s32", r1, r1, chunk)
		k.e("min.u32", r1, r1, R)
		loop, done := k.label(), k.label()
		k.put(loop)
		p := k.p()
		k.e("setp.ge.u32", p, r, r1)
		k.jumpIf(p, done)
		k.warpGradRow(f, r)
		k.e("add.s32", r, r, warpRows)
		k.jump(loop)
		k.put(done)
		k.warpGammaPartials(f, sm, tid, w, b, part)
		return k
	}
}

// warpGradRow bir satırın geri yönü: g ve x bir kez okunur, γ terimi acc'de birikir, x'in
// gradyanı (istenirse) warp içi toplamlarla gx'e eklenir.
func (k *emitter) warpGradRow(f *warpGrad, r reg) {
	a := f.a
	xt, ggt, gxt := a.dt[0], a.dt[1], a.dt[4]
	xo := k.rowsAddress(r, f.H, f.S0, f.S1)
	k.e("add.s32", xo, xo, f.lane)
	gofs := k.r()
	k.e("mad.lo.s32", gofs, r, a.D, f.lane)
	m := f.zero
	if a.layer {
		m = k.load(tensor.Float32, k.address(f.avg, r, 4))
	}
	rs := k.load(tensor.Float32, k.address(f.rstd, r, 4))
	hs, dhs := make([]reg, len(f.acc)), make([]reg, len(f.acc))
	a1, a2 := k.constant(0), k.constant(0)
	for e := range hs {
		gv := k.load(ggt, k.rowAt(f.g, k.laneOffset(gofs, e), ggt))
		hs[e], dhs[e] = k.normGradTerm(gv, k.load(xt, k.rowAt(f.x, k.laneOffset(xo, e), xt)), m, rs, f.gam[e], f.acc[e], a1, a2)
	}
	if !a.xGrad {
		return
	}
	m1 := f.zero
	if a.layer {
		m1 = k.warpSum(a1)
		k.e("div.rn.f32", m1, m1, f.df)
	}
	m2 := k.warpSum(a2)
	k.e("div.rn.f32", m2, m2, f.df)
	for e := range hs {
		k.normGradX(dhs[e], hs[e], m1, m2, rs, gxt, a.set, func() reg { return k.rowAt(f.gx, k.laneOffset(gofs, e), gxt) })
	}
}

// warpGammaPartials warp'ların γ toplamlarını paylaşımlı bellekte (sm[w·D + sütun]) birleştirir:
// sütun başına warp 0..7 sırasıyla toplanıp blok b'nin kısmi toplamı part[b·D + sütun]'a yazılır.
func (k *emitter) warpGammaPartials(f *warpGrad, sm, tid, w, b, part reg) {
	D := f.a.D
	for e := range f.acc {
		o := k.r()
		k.e("mad.lo.s32", o, w, D, f.lane)
		k.e("st.shared.f32", mem(k.sharedAt(sm, k.laneOffset(o, e)), 0), f.acc[e])
	}
	k.barrier()
	for c := 0; c < D; c += 32 * warpRows {
		j, skip := k.r(), k.label()
		k.e("add.s32", j, tid, c)
		out := k.p()
		k.e("setp.ge.u32", out, j, D)
		k.jumpIf(out, skip)
		total := k.constant(0)
		for v := 0; v < warpRows; v++ {
			o := k.r()
			k.e("add.s32", o, j, v*D)
			t := k.f()
			k.e("ld.shared.f32", t, mem(k.sharedAt(sm, o), 0))
			k.e("add.rn.f32", total, total, t)
		}
		pb := k.r()
		k.e("mad.lo.s32", pb, b, D, j)
		k.store(tensor.Float32, k.rowAt(part, pb, tensor.Float32), total)
		k.put(skip)
	}
}

// sharedAt paylaşımlı bellekte 4 baytlık o. elemanın adresidir.
func (k *emitter) sharedAt(sm, o reg) reg {
	r := k.r()
	k.e("mad.lo.s32", r, o, 4, sm)
	return r
}
