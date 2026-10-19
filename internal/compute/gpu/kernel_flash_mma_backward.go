package gpu

import (
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Birleşik dikkat, tensör çekirdekli geri yön (16 bitlik girdiler, sm_80+)
// ============================================================================
//
// SIMT geri yönüyle aynı iki geçiş (atomiksiz, belirlenimci); çarpımlar mma.sync ile:
//   - dikkat_geri_kv_tc: blok mmaKeys anahtarı tutar (8 warp × 16). Sorgular mmaQT'lik
//     döşemelerle (o anahtar başını paylaşan bütün sorgu başları için) gezilir. Warp kendi
//     anahtarları için Sᵀ = K·Qᵀ ve dPᵀ = V·dOᵀ'yi kurar; Pᵀ = 2^{Sᵀ·ölçek·log2e − lse·log2e},
//     dSᵀ = Pᵀ·(dPᵀ − D)·ölçek; dV += Pᵀ·dO ve dK += dSᵀ·Q (Pᵀ ve dSᵀ parçaları doğrudan A
//     parçalarına paketlenir). Q ve dO döşemeleri hem satır öncelikli hem devrik tutulur.
//   - dikkat_geri_q_tc: blok mmaBr sorgu satırı tutar; anahtarlar mmaBc'lik döşemelerle
//     gezilir: S = Q·Kᵀ, dP = dO·Vᵀ, dS, dQ += dS·K (K devrik de tutulur).
// dO float32 gelir ve 16 bite yuvarlanır (çarpımın geri yönündeki gradyan yuvarlamasıyla
// aynı). Baş boyları en çok mmaBackMax'tır (yazmaç bütçesi); büyüğünde SIMT geri yön kalır.

const (
	mmaKeys    = 128 // geri dK/dV bloğunun anahtar sayısı (8 warp × 16)
	mmaQT      = 64  // geri dK/dV'de sorgu döşemesi
	mmaBackMax = 64  // tensör çekirdekli geri yönün en büyük baş boyu
)

// fragRows şeridin parça satırları: r[0] = taban + 16w + g, r[1] = r[0] + 8.
func (k *emitter) fragRows(base2 reg, l mmaLanes) [2]reg {
	var r [2]reg
	r[0], r[1] = k.r(), k.r()
	k.e("mad.lo.s32", r[0], l.w, 16, base2)
	k.e("add.s32", r[0], r[0], l.g)
	k.e("add.s32", r[1], r[0], 8)
	return r
}

// fragA16 16 bitlik [T][D] matristen şeridin satırlarının A parçaları (her k adımı için).
func (k *emitter) fragA16(base2 reg, rows [2]reg, t2, lim reg, D int) [][4]reg {
	a := make([][4]reg, D/16)
	for s := range a {
		for i, o := range [4][2]int{{0, 0}, {1, 0}, {0, 8}, {1, 8}} {
			c := k.r()
			k.e("add.s32", c, t2, 16*s+o[1])
			a[s][i] = k.loadPair16(base2, rows[o[0]], c, lim, D)
		}
	}
	return a
}

// fragAF32 float32 [T][D] matristen 16 bite yuvarlanmış A parçaları.
func (k *emitter) fragAF32(base2 reg, rows [2]reg, t2, lim reg, D int, f16 bool) [][4]reg {
	a := make([][4]reg, D/16)
	var ok [2]reg
	for i := range 2 {
		ok[i] = k.p()
		k.e("setp.lt.u32", ok[i], rows[i], lim)
	}
	for s := range a {
		for i, o := range [4][2]int{{0, 0}, {1, 0}, {0, 8}, {1, 8}} {
			c := k.r()
			k.e("add.s32", c, t2, 16*s+o[1])
			idx := k.r()
			k.e("mad.lo.s32", idx, rows[o[0]], D, c)
			ad := k.address(base2, idx, 4)
			x0, x1 := k.constant(0), k.constant(0)
			k.ifCase(ok[o[0]], "ld.global.f32", x0, mem(ad, 0))
			k.ifCase(ok[o[0]], "ld.global.f32", x1, mem(ad, 4))
			a[s][i] = k.pack2(x0, x1, f16)
		}
	}
	return a
}

// fragMul acc[n] += A·B; B parçaları paylaşımlı bellekteki [n][k] düzenli döşemeden (satır adımı
// step eleman, şeridin ilk adresi addr): b0 = (n·8+g, 16s+2t), b1 = (n·8+g, 16s+2t+8).
func (k *emitter) fragMul(acc [][4]reg, a [][4]reg, addr reg, step int, f16 bool) {
	for s := range a {
		for n := range acc {
			var b [2]reg
			b[0], b[1] = k.r(), k.r()
			ofs := 2 * (n*8*step + 16*s)
			k.e("ld.shared.b32", b[0], mem(addr, ofs))
			k.e("ld.shared.b32", b[1], mem(addr, ofs+16))
			k.mma(acc[n], acc[n], a[s], b, f16)
		}
	}
}

// fragToA C parçalarını (16 satır × 8·len sütun) bir sonraki çarpımın A parçalarına (k ekseni
// sütunlar) paketler.
func (k *emitter) fragToA(C [][4]reg, f16 bool) [][4]reg {
	a := make([][4]reg, len(C)/2)
	for s := range a {
		a[s][0] = k.pack2(C[2*s][0], C[2*s][1], f16)
		a[s][1] = k.pack2(C[2*s][2], C[2*s][3], f16)
		a[s][2] = k.pack2(C[2*s+1][0], C[2*s+1][1], f16)
		a[s][3] = k.pack2(C[2*s+1][2], C[2*s+1][3], f16)
	}
	return a
}

// laneAddr paylaşımlı döşemede şeridin ilk okuma adresi: base + (g·step + 2t)·2.
func (k *emitter) laneAddr(sm reg, l mmaLanes, t2 reg, step int) reg {
	r := k.r()
	k.e("mad.lo.s32", r, l.g, step, t2)
	k.e("mad.lo.s32", r, r, 2, sm)
	return r
}

func (k *emitter) zeroFrags(n int) [][4]reg {
	m := make([][4]reg, n)
	for i := range m {
		for c := range 4 {
			m[i][c] = k.constant(0)
		}
	}
	return m
}

func (k *emitter) clearFrags(m [][4]reg) {
	for i := range m {
		for c := range 4 {
			k.e("mov.f32", m[i][c], float32(0))
		}
	}
}

// mmaGradWrite parçaları (satırlar rows, sütunlar 8n + 2t + 0/1) float32 [T][D] gradyana kipine
// göre yazar (ekle ya da üzerine yaz); lim dışındaki satırlar yazılmaz.
func (k *emitter) mmaGradWrite(M [][4]reg, mode int, base2 reg, rows [2]reg, lim, t2 reg, D int) {
	if mode == noGrad {
		return
	}
	for r := range 2 {
		skip := k.label()
		p := k.p()
		k.e("setp.ge.u32", p, rows[r], lim)
		k.jumpIf(p, skip)
		for n := range M {
			for c := 2 * r; c < 2*r+2; c++ {
				idx := k.r()
				k.e("add.s32", idx, t2, 8*n+c%2)
				k.e("mad.lo.s32", idx, rows[r], D, idx)
				ad := k.address(base2, idx, 4)
				v := M[n][c]
				if mode == addGrad {
					old := k.load(tensor.Float32, ad)
					k.e("add.rn.f32", v, old, v)
				}
				k.store(tensor.Float32, ad, v)
			}
		}
		k.put(skip)
	}
}

// probsGrad P ve dS'yi yerinde kurar (S → P, dP → dS): görünmeyende 0. vis(n, c) parçanın
// görünürlüğü, lse2(c), dd(c) parça satırının/sütununun lse·log2e'si ve D'si.
func (f *flashShared) probsGrad(S, dP [][4]reg, sc2 reg, vis func(n, c int) reg, lse2, dd func(n, c int) reg) {
	k := f.k
	for n := range S {
		for c := range 4 {
			x := k.f()
			k.e("fma.rn.f32", x, S[n][c], sc2, k.negate(lse2(n, c)))
			p := k.f()
			k.e("ex2.approx.f32", p, x)
			k.e("selp.f32", p, p, float32(0), vis(n, c))
			k.e("mov.f32", S[n][c], p)
			t := k.f()
			k.e("sub.rn.f32", t, dP[n][c], dd(n, c))
			k.e("mul.rn.f32", t, t, p)
			k.e("mul.rn.f32", dP[n][c], t, f.scaleF)
		}
	}
}

// negate −x.
func (k *emitter) negate(x reg) reg {
	r := k.f()
	k.e("neg.f32", r, x)
	return r
}

// flashMmaBackwardQKernel dQ; ızgara (⌈Tq/mmaBr⌉, H, Batch).
func flashMmaBackwardQKernel(a flashMma, modeQ int) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("%s_%d", a.key("dikkat_geri_q_tc"), modeQ))
		k.target = 80
		f := &flashShared{k: k, a: flashConfig{Br: mmaBr, Bc: mmaBc, Dh: a.Dh, Dv: a.Dv}}
		pq, pk, pv, pdo := k.pointer("q"), k.pointer("k"), k.pointer("v"), k.pointer("do")
		plse, pd := k.pointer("lse"), k.pointer("d")
		pdq := k.written("dq")
		f.sharedParams()
		f.headIndexes(false)
		ks := k.shared("ks", 2*mmaBc*(a.Dh+8))
		kts := k.shared("kts", 2*a.Dh*(mmaBc+8))
		vs := k.shared("vs", 2*mmaBc*(a.Dv+8))
		l := k.lanes(f.tid)
		t2 := k.r()
		k.e("shl.b32", t2, l.t, 1)
		q0 := k.r()
		k.e("mul.lo.s32", q0, k.special("%ctaid.x"), mmaBr)
		rows := k.fragRows(q0, l)
		qa := k.fragA16(k.base16(pq, f.bh, f.Tq, a.Dh), rows, t2, f.Tq, a.Dh)
		doa := k.fragAF32(k.matrixBase(pdo, f.bh, f.Tq, a.Dv), rows, t2, f.Tq, a.Dv, a.f16)
		kb := k.base16(pk, f.bhkv, f.Tk, a.Dh)
		vb := k.base16(pv, f.bhkv, f.Tk, a.Dv)
		var lse2, dd, lo, hi [2]reg
		lb := k.r()
		k.e("mul.lo.s32", lb, f.bh, f.Tq)
		for r := range 2 {
			lo[r], hi[r] = k.attnLimits(rows[r], f.Tq, f.Tk, f.reason2, f.pending2)
			in := k.p()
			k.e("setp.lt.u32", in, rows[r], f.Tq)
			k.e("selp.b32", hi[r], hi[r], lo[r], in)
			li := k.r()
			k.e("add.s32", li, lb, rows[r])
			lse2[r], dd[r] = k.constant(float32(math.Inf(1))), k.constant(0)
			k.ifCase(in, "ld.global.f32", lse2[r], mem(k.address(plse, li, 4), 0))
			k.ifCase(in, "ld.global.f32", dd[r], mem(k.address(pd, li, 4), 0))
			k.e("mul.rn.f32", lse2[r], lse2[r], float32(math.Log2E))
		}
		sc2 := k.f()
		k.e("mul.rn.f32", sc2, f.scaleF, float32(math.Log2E))
		jStart, jEnd := f.keyRange(q0)
		dQ := k.zeroFrags(a.Dh / 8)
		S, dP := k.zeroFrags(mmaBc/8), k.zeroFrags(mmaBc/8)
		ksAddr := k.laneAddr(ks, l, t2, a.Dh+8)
		vsAddr := k.laneAddr(vs, l, t2, a.Dv+8)
		ktsAddr := k.laneAddr(kts, l, t2, mmaBc+8)
		j0 := k.r()
		k.e("mov.u32", j0, jStart)
		loop, done := k.label(), k.label()
		k.put(loop)
		p := k.p()
		k.e("setp.ge.u32", p, j0, jEnd)
		k.jumpIf(p, done)
		k.barrier()
		k.loadTile16(tile16{row: ks, trans: kts, R: mmaBc, D: a.Dh}, kb, j0, f.Tk, f.tid)
		k.loadTile16(tile16{row: vs, R: mmaBc, D: a.Dv}, vb, j0, f.Tk, f.tid)
		k.barrier()
		k.clearFrags(S)
		k.clearFrags(dP)
		k.fragMul(S, qa, ksAddr, a.Dh+8, a.f16)
		k.fragMul(dP, doa, vsAddr, a.Dv+8, a.f16)
		col := k.r()
		k.e("add.s32", col, j0, t2)
		f.probsGrad(S, dP, sc2, func(n, c int) reg {
			j := k.r()
			k.e("add.s32", j, col, 8*n+c%2)
			p1, p2 := k.p(), k.p()
			k.e("setp.ge.s32", p1, j, lo[c/2])
			k.e("setp.lt.s32", p2, j, hi[c/2])
			k.e("and.pred", p1, p1, p2)
			return p1
		}, func(n, c int) reg { return lse2[c/2] }, func(n, c int) reg { return dd[c/2] })
		k.fragMul(dQ, k.fragToA(dP, a.f16), ktsAddr, mmaBc+8, a.f16)
		k.e("add.s32", j0, j0, mmaBc)
		k.jump(loop)
		k.put(done)
		k.mmaGradWrite(dQ, modeQ, k.matrixBase(pdq, f.bh, f.Tq, a.Dh), rows, f.Tq, t2, a.Dh)
		return k
	}
}

// flashMmaBackwardKVKernel dK ve dV; ızgara (⌈Tk/mmaKeys⌉, Hkv, Batch).
func flashMmaBackwardKVKernel(a flashMma, modeK, modeV int) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("%s_%d_%d", a.key("dikkat_geri_kv_tc"), modeK, modeV))
		k.target = 80
		f := &flashBackwardState{flashShared: &flashShared{k: k, a: flashConfig{Br: mmaQT, Bc: mmaKeys, Dh: a.Dh, Dv: a.Dv}}}
		pq, pk, pv, pdo := k.pointer("q"), k.pointer("k"), k.pointer("v"), k.pointer("do")
		plse, pd := k.pointer("lse"), k.pointer("d")
		pdk, pdv := k.written("dk"), k.written("dv")
		f.sharedParams()
		f.headIndexes(true)
		qs := k.shared("qs", 2*mmaQT*(a.Dh+8))
		qts := k.shared("qts", 2*a.Dh*(mmaQT+8))
		dos := k.shared("dos", 2*mmaQT*(a.Dv+8))
		dots := k.shared("dots", 2*a.Dv*(mmaQT+8))
		info := k.shared("satir", 4*4*mmaQT) // sorgu başına lse·log2e, D, alt ve üst sınır
		l := k.lanes(f.tid)
		t2 := k.r()
		k.e("shl.b32", t2, l.t, 1)
		j0 := k.r()
		k.e("mul.lo.s32", j0, k.special("%ctaid.x"), mmaKeys)
		keys := k.fragRows(j0, l)
		ka := k.fragA16(k.base16(pk, f.bhkv, f.Tk, a.Dh), keys, t2, f.Tk, a.Dh)
		va := k.fragA16(k.base16(pv, f.bhkv, f.Tk, a.Dv), keys, t2, f.Tk, a.Dv)
		dK, dV := k.zeroFrags(a.Dh/8), k.zeroFrags(a.Dv/8)
		St, dPt := k.zeroFrags(mmaQT/8), k.zeroFrags(mmaQT/8)
		qsAddr := k.laneAddr(qs, l, t2, a.Dh+8)
		dosAddr := k.laneAddr(dos, l, t2, a.Dv+8)
		qtsAddr := k.laneAddr(qts, l, t2, mmaQT+8)
		dotsAddr := k.laneAddr(dots, l, t2, mmaQT+8)
		sc2 := k.f()
		k.e("mul.rn.f32", sc2, f.scaleF, float32(math.Log2E))
		iStart, iEnd := f.queryRange(j0)
		g := k.whole(0)
		beginH, endH := k.label(), k.label()
		k.put(beginH)
		pg := k.p()
		k.e("setp.ge.u32", pg, g, f.G)
		k.jumpIf(pg, endH)
		bh := k.r()
		k.e("add.s32", bh, f.h, g)
		k.e("mad.lo.s32", bh, f.b, f.H, bh)
		qb := k.base16(pq, bh, f.Tq, a.Dh)
		dob := k.matrixBase(pdo, bh, f.Tq, a.Dv)
		lb := k.r()
		k.e("mul.lo.s32", lb, bh, f.Tq)
		i0 := k.r()
		k.e("mov.u32", i0, iStart)
		loop, done := k.label(), k.label()
		k.put(loop)
		p := k.p()
		k.e("setp.ge.s32", p, i0, iEnd)
		k.jumpIf(p, done)
		k.barrier()
		k.loadTile16(tile16{row: qs, trans: qts, R: mmaQT, D: a.Dh}, qb, i0, f.Tq, f.tid)
		k.loadTileF32(tile16{row: dos, trans: dots, R: mmaQT, D: a.Dv}, dob, i0, f.Tq, f.tid, a.f16)
		f.queryInfo(info, i0, lb, plse, pd)
		k.barrier()
		k.clearFrags(St)
		k.clearFrags(dPt)
		k.fragMul(St, ka, qsAddr, a.Dh+8, a.f16)
		k.fragMul(dPt, va, dosAddr, a.Dv+8, a.f16)
		f.kvProbs(St, dPt, keys, info, t2, sc2)
		k.fragMul(dV, k.fragToA(St, a.f16), dotsAddr, mmaQT+8, a.f16)
		k.fragMul(dK, k.fragToA(dPt, a.f16), qtsAddr, mmaQT+8, a.f16)
		k.e("add.s32", i0, i0, mmaQT)
		k.jump(loop)
		k.put(done)
		k.e("add.s32", g, g, 1)
		k.jump(beginH)
		k.put(endH)
		k.mmaGradWrite(dK, modeK, k.matrixBase(pdk, f.bhkv, f.Tk, a.Dh), keys, f.Tk, t2, a.Dh)
		k.mmaGradWrite(dV, modeV, k.matrixBase(pdv, f.bhkv, f.Tk, a.Dv), keys, f.Tk, t2, a.Dv)
		return k
	}
}

// kvProbs dK/dV çekirdeğinde Pᵀ ve dSᵀ'yi kurar: satırlar anahtar (keys), sütunlar sorgu;
// sorgu bilgileri (lse2, D, görünür anahtar aralığı) paylaşımlı bellekten.
func (f *flashShared) kvProbs(St, dPt [][4]reg, keys [2]reg, info, t2, sc2 reg) {
	k := f.k
	get := k.columnInfo(info, t2)
	f.probsGrad(St, dPt, sc2, func(n, c int) reg {
		v := get(n, c)
		p1, p2 := k.p(), k.p()
		k.e("setp.ge.s32", p1, keys[c/2], v[2])
		k.e("setp.lt.s32", p2, keys[c/2], v[3])
		k.e("and.pred", p1, p1, p2)
		return p1
	}, func(n, c int) reg { return get(n, c)[0] }, func(n, c int) reg { return get(n, c)[1] })
}

// columnInfo parça sütununun (sorgunun) paylaşımlı bellekteki bilgilerini [lse2, D, alt, üst]
// okuyan işlevi verir; her sütun bir kez okunur.
func (k *emitter) columnInfo(info, t2 reg) func(n, c int) [4]reg {
	cache := map[int][4]reg{}
	return func(n, c int) [4]reg {
		key := 8*n + c%2
		if v, ok := cache[key]; ok {
			return v
		}
		ad := k.r()
		k.e("add.s32", ad, t2, key)
		k.e("mad.lo.s32", ad, ad, 4, info)
		var v [4]reg
		v[0], v[1] = k.f(), k.f()
		v[2], v[3] = k.r(), k.r()
		k.e("ld.shared.f32", v[0], mem(ad, 0))
		k.e("ld.shared.f32", v[1], mem(ad, 4*mmaQT))
		k.e("ld.shared.u32", v[2], mem(ad, 8*mmaQT))
		k.e("ld.shared.u32", v[3], mem(ad, 12*mmaQT))
		cache[key] = v
		return v
	}
}

// queryInfo sorgu döşemesinin (i0..i0+mmaQT) satır bilgilerini paylaşımlı belleğe yazar:
// lse·log2e (Tq dışında +∞), D, görünür anahtar aralığının alt ve üst sınırı (Tq dışında boş).
// İlk mmaQT iş parçacığı yazar.
func (f *flashShared) queryInfo(info, i0, lb, plse, pd reg) {
	k := f.k
	skip := k.label()
	p := k.p()
	k.e("setp.ge.u32", p, f.tid, mmaQT)
	k.jumpIf(p, skip)
	i := k.r()
	k.e("add.s32", i, i0, f.tid)
	lo, hi := k.attnLimits(i, f.Tq, f.Tk, f.reason2, f.pending2)
	in := k.p()
	k.e("setp.lt.u32", in, i, f.Tq)
	k.e("selp.b32", hi, hi, lo, in)
	li := k.r()
	k.e("add.s32", li, lb, i)
	lse, dd := k.constant(float32(math.Inf(1))), k.constant(0)
	k.ifCase(in, "ld.global.f32", lse, mem(k.address(plse, li, 4), 0))
	k.ifCase(in, "ld.global.f32", dd, mem(k.address(pd, li, 4), 0))
	k.e("mul.rn.f32", lse, lse, float32(math.Log2E))
	ad := k.r()
	k.e("mad.lo.s32", ad, f.tid, 4, info)
	k.e("st.shared.f32", mem(ad, 0), lse)
	k.e("st.shared.f32", mem(ad, 4*mmaQT), dd)
	k.e("st.shared.u32", mem(ad, 8*mmaQT), lo)
	k.e("st.shared.u32", mem(ad, 12*mmaQT), hi)
	k.put(skip)
}
