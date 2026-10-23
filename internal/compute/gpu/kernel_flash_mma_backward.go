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
//   - dikkat_geri_kv_tc: blok mmaKeys anahtarı tutar (8 warp × 16). Sorgular mmaBackTile'lık
//     döşemelerle (o anahtar bakışını paylaşan bütün sorgu bakışları için) gezilir. Warp kendi
//     anahtarları için Sᵀ = K·Qᵀ ve dPᵀ = V·dOᵀ'yi kurar; Pᵀ = 2^{Sᵀ·ölçek·log2e − lse·log2e},
//     dSᵀ = Pᵀ·(dPᵀ − D)·ölçek; dV += Pᵀ·dO ve dK += dSᵀ·Q (Pᵀ ve dSᵀ parçaları doğrudan A
//     parçalarına paketlenir). Q ve dO döşemeleri hem satır öncelikli hem devrik tutulur.
//   - dikkat_geri_q_tc: blok mmaBr sorgu satırı tutar; anahtarlar mmaBackTile'lık döşemelerle
//     gezilir: S = Q·Kᵀ, dP = dO·Vᵀ, dS, dQ += dS·K (K devrik de tutulur).
// dO float32 gelir ve 16 bite yuvarlanır (çarpımın geri yönündeki gradyan yuvarlamasıyla
// aynı). Bakış boyları en çok mmaBackMax'tır (yazmaç bütçesi); büyüğünde SIMT geri yön kalır.

const (
	mmaKeys    = 128 // geri dK/dV bloğunun anahtar sayısı (8 warp × 16)
	mmaBackMax = 128 // tensör çekirdekli geri yönün en büyük bakış boyu
)

// mmaBackTile geri çekirdeklerin döşemesi (dQ'da anahtar, dK/dV'de sorgu döşemesi): bakış boyu
// 64'e kadar 64, üstünde 32. Bakış boyu 128'de döşeme 64 olsa paylaşımlı bellek dQ'da 53.248,
// dK/dV'de 72.704 bayt olurdu (statik sınır 48 KB); 32 ile 27.648 ve 38.400 bayttır, parça
// yazmaçları da yarıya iner. Emsal: FlashAttention-2'nin hdim128 geri yönü paylaşımlı bellek
// azken döşemeyi daraltır (run_mha_bwd_hdim128); Triton'un fused-attention örneği geri yönde
// 32'lik döşeme kullanır (BLOCK_M1 = BLOCK_N2 = 32).
func mmaBackTile(a flashMma) int {
	if max(a.Dh, a.Dv) > 64 {
		return 32
	}
	return 64
}

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
func (k *emitter) fragA16(m hmat, rows [2]reg, t2, lim reg, D int) [][4]reg {
	a := make([][4]reg, D/16)
	for s := range a {
		for i, o := range [4][2]int{{0, 0}, {1, 0}, {0, 8}, {1, 8}} {
			c := k.r()
			k.e("add.s32", c, t2, 16*s+o[1])
			a[s][i] = k.loadPair16(m, rows[o[0]], c, lim)
		}
	}
	return a
}

// fragAF32 float32 [T][D] matristen 16 bite yuvarlanmış A parçaları.
func (k *emitter) fragAF32(m hmat, rows [2]reg, t2, lim reg, D int, f16 bool) [][4]reg {
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
			k.e("mad.lo.s32", idx, rows[o[0]], m.stride, c)
			ad := k.address(m.base, idx, 4)
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
func (k *emitter) mmaGradWrite(M [][4]reg, mode int, m hmat, rows [2]reg, lim, t2 reg, D int) {
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
				k.e("mad.lo.s32", idx, rows[r], m.stride, idx)
				ad := k.address(m.base, idx, 4)
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
			// Görünmeyen konumun dS'si 0'dır; çarpımla bulunmaz: satırın D'si NaN ise (dP − D)·0 NaN
			// olur ve bakılmayan anahtara sızar (15.24).
			k.e("selp.f32", dP[n][c], dP[n][c], float32(0), vis(n, c))
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
		bc := mmaBackTile(a)
		f := &flashShared{k: k, a: flashConfig{Br: mmaBr, Bc: bc, Dh: a.Dh, Dv: a.Dv}}
		pq, pk, pv, pdo := k.pointer("q"), k.pointer("k"), k.pointer("v"), k.pointer("do")
		plse, pd := k.pointer("lse"), k.pointer("d")
		pdq := k.written("dq")
		f.sharedParams()
		f.headIndexes(false)
		ks := k.shared("ks", 2*bc*(a.Dh+8))
		kts := k.shared("kts", 2*a.Dh*(bc+8))
		vs := k.shared("vs", 2*bc*(a.Dv+8))
		l := k.lanes(f.tid)
		t2 := k.r()
		k.e("shl.b32", t2, l.t, 1)
		q0 := k.r()
		k.e("mul.lo.s32", q0, k.special("%ctaid.x"), mmaBr)
		rows := k.fragRows(q0, l)
		qa := k.fragA16(k.seqBase(pq, f.bh, f.Tq, f.H, a.Dh, 2), rows, t2, f.Tq, a.Dh)
		doa := k.fragAF32(k.seqBase(pdo, f.bh, f.Tq, f.H, a.Dv, 4), rows, t2, f.Tq, a.Dv, a.f16)
		kb := k.seqBase(pk, f.bhkv, f.Tk, f.Hkv, a.Dh, 2)
		vb := k.seqBase(pv, f.bhkv, f.Tk, f.Hkv, a.Dv, 2)
		var lse2, dd, lo, hi [2]reg
		lb := k.r()
		k.e("mul.lo.s32", lb, f.bh, f.Tq)
		for r := range 2 {
			lo[r], hi[r], lse2[r], dd[r] = f.rowStats(rows[r], lb, plse, pd)
		}
		sc2 := k.f()
		k.e("mul.rn.f32", sc2, f.scaleF, float32(math.Log2E))
		jStart, jEnd := f.keyRange(q0)
		dQ := k.zeroFrags(a.Dh / 8)
		S, dP := k.zeroFrags(bc/8), k.zeroFrags(bc/8)
		ksAddr := k.laneAddr(ks, l, t2, a.Dh+8)
		vsAddr := k.laneAddr(vs, l, t2, a.Dv+8)
		ktsAddr := k.laneAddr(kts, l, t2, bc+8)
		j0 := k.r()
		k.e("mov.u32", j0, jStart)
		k.whileLess("u32", j0, jEnd, func() {
			k.barrier()
			k.loadTile16(tile16{row: ks, trans: kts, R: bc, D: a.Dh}, kb, j0, f.Tk, f.tid)
			k.loadTile16(tile16{row: vs, R: bc, D: a.Dv}, vb, j0, f.Tk, f.tid)
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
			k.fragMul(dQ, k.fragToA(dP, a.f16), ktsAddr, bc+8, a.f16)
			k.e("add.s32", j0, j0, bc)
		})
		k.mmaGradWrite(dQ, modeQ, k.seqBase(pdq, f.bh, f.Tq, f.H, a.Dh, 4), rows, f.Tq, t2, a.Dh)
		return k
	}
}

// flashMmaBackwardKVKernel dK ve dV; ızgara (⌈Tk/mmaKeys⌉, Hkv, Batch).
func flashMmaBackwardKVKernel(a flashMma, modeK, modeV int) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("%s_%d_%d", a.key("dikkat_geri_kv_tc"), modeK, modeV))
		k.target = 80
		qt := mmaBackTile(a)
		f := &flashBackwardState{flashShared: &flashShared{k: k, a: flashConfig{Br: qt, Bc: mmaKeys, Dh: a.Dh, Dv: a.Dv}}}
		pq, pk, pv, pdo := k.pointer("q"), k.pointer("k"), k.pointer("v"), k.pointer("do")
		plse, pd := k.pointer("lse"), k.pointer("d")
		pdk, pdv := k.written("dk"), k.written("dv")
		f.sharedParams()
		f.headIndexes(true)
		qs := k.shared("qs", 2*qt*(a.Dh+8))
		qts := k.shared("qts", 2*a.Dh*(qt+8))
		dos := k.shared("dos", 2*qt*(a.Dv+8))
		dots := k.shared("dots", 2*a.Dv*(qt+8))
		info := k.shared("satir", 4*4*qt) // sorgu başına lse·log2e, D, alt ve üst sınır
		l := k.lanes(f.tid)
		t2 := k.r()
		k.e("shl.b32", t2, l.t, 1)
		j0 := k.r()
		k.e("mul.lo.s32", j0, k.special("%ctaid.x"), mmaKeys)
		keys := k.fragRows(j0, l)
		ka := k.fragA16(k.seqBase(pk, f.bhkv, f.Tk, f.Hkv, a.Dh, 2), keys, t2, f.Tk, a.Dh)
		va := k.fragA16(k.seqBase(pv, f.bhkv, f.Tk, f.Hkv, a.Dv, 2), keys, t2, f.Tk, a.Dv)
		dK, dV := k.zeroFrags(a.Dh/8), k.zeroFrags(a.Dv/8)
		St, dPt := k.zeroFrags(qt/8), k.zeroFrags(qt/8)
		qsAddr := k.laneAddr(qs, l, t2, a.Dh+8)
		dosAddr := k.laneAddr(dos, l, t2, a.Dv+8)
		qtsAddr := k.laneAddr(qts, l, t2, qt+8)
		dotsAddr := k.laneAddr(dots, l, t2, qt+8)
		sc2 := k.f()
		k.e("mul.rn.f32", sc2, f.scaleF, float32(math.Log2E))
		iStart, iEnd := f.queryRange(j0)
		g := k.whole(0)
		k.whileLess("u32", g, f.G, func() {
			bh := k.r()
			k.e("add.s32", bh, f.h, g)
			k.e("mad.lo.s32", bh, f.b, f.H, bh)
			qb := k.seqBase(pq, bh, f.Tq, f.H, a.Dh, 2)
			dob := k.seqBase(pdo, bh, f.Tq, f.H, a.Dv, 4)
			lb := k.r()
			k.e("mul.lo.s32", lb, bh, f.Tq)
			i0 := k.r()
			k.e("mov.u32", i0, iStart)
			k.whileLess("s32", i0, iEnd, func() {
				k.barrier()
				k.loadTile16(tile16{row: qs, trans: qts, R: qt, D: a.Dh}, qb, i0, f.Tq, f.tid)
				k.loadTileF32(tile16{row: dos, trans: dots, R: qt, D: a.Dv}, dob, i0, f.Tq, f.tid, a.f16)
				f.queryInfo(info, i0, lb, plse, pd)
				k.barrier()
				k.clearFrags(St)
				k.clearFrags(dPt)
				k.fragMul(St, ka, qsAddr, a.Dh+8, a.f16)
				k.fragMul(dPt, va, dosAddr, a.Dv+8, a.f16)
				f.kvProbs(St, dPt, keys, info, t2, sc2)
				k.fragMul(dV, k.fragToA(St, a.f16), dotsAddr, qt+8, a.f16)
				f.zeroNaNQueryColumns(qts, info, qt, a.Dh)
				k.barrier()
				k.fragMul(dK, k.fragToA(dPt, a.f16), qtsAddr, qt+8, a.f16)
				k.e("add.s32", i0, i0, qt)
			})
			k.e("add.s32", g, g, 1)
		})
		k.mmaGradWrite(dK, modeK, k.seqBase(pdk, f.bhkv, f.Tk, f.Hkv, a.Dh, 4), keys, f.Tk, t2, a.Dh)
		k.mmaGradWrite(dV, modeV, k.seqBase(pdv, f.bhkv, f.Tk, f.Hkv, a.Dv, 4), keys, f.Tk, t2, a.Dv)
		return k
	}
}

// zeroNaNQueryColumns logsumexp'i NaN olan (puanı NaN) sorguların sütununu aktarılmış sorgu
// döşemesinde (trans: [D][qt+8], 16 bit) 0'lar: dK = dSᵀ·Q çarpımında bakılmayan anahtarın dS'si 0'dır
// ama 0·NaN NaN verirdi; bakılan anahtarlar dS'den zaten NaN alır (15.24). İlk qt/2 iş parçacığı
// birer sorgu çiftini (bir 32 bitlik sözcük) işler; bilgi dizisi paylaşımlı bellekten okunur.
// Sonrasında döşemeyi okumadan önce engel gerekir.
func (f *flashShared) zeroNaNQueryColumns(trans, info reg, qt, D int) {
	k := f.k
	skip := k.label()
	out := k.p()
	k.e("setp.ge.u32", out, f.tid, qt/2)
	k.jumpIf(out, skip)
	i := k.mul(f.tid, 2)
	ad := k.r()
	k.e("mad.lo.s32", ad, i, 4, info)
	var v, nan [2]reg
	for j := range 2 {
		v[j], nan[j] = k.f(), k.p()
		k.e("ld.shared.f32", v[j], mem(ad, 4*j))
		k.e("setp.nan.f32", nan[j], v[j], v[j])
	}
	either := k.p()
	k.e("or.pred", either, nan[0], nan[1])
	none := k.p()
	k.e("not.pred", none, either)
	k.jumpIf(none, skip)
	m0, m1 := k.r(), k.r()
	k.e("selp.b32", m0, -65536, -1, nan[0])
	k.e("selp.b32", m1, 0xFFFF, -1, nan[1])
	k.e("and.b32", m0, m0, m1)
	so := k.r()
	k.e("mad.lo.s32", so, i, 2, trans)
	for d := range D {
		w := k.r()
		k.e("ld.shared.b32", w, mem(so, 2*d*(qt+8)))
		k.e("and.b32", w, w, m0)
		k.e("st.shared.b32", mem(so, 2*d*(qt+8)), w)
	}
	k.put(skip)
}

// kvProbs dK/dV çekirdeğinde Pᵀ ve dSᵀ'yi kurar: satırlar anahtar (keys), sütunlar sorgu;
// sorgu bilgileri (lse2, D, görünür anahtar aralığı) paylaşımlı bellekten.
func (f *flashShared) kvProbs(St, dPt [][4]reg, keys [2]reg, info, t2, sc2 reg) {
	k := f.k
	get := k.columnInfo(info, t2, f.a.Br)
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
// okuyan işlevi verir; her sütun bir kez okunur. qt sorgu döşemesidir (bilgi dizilerinin adımı).
func (k *emitter) columnInfo(info, t2 reg, qt int) func(n, c int) [4]reg {
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
		k.e("ld.shared.f32", v[1], mem(ad, 4*qt))
		k.e("ld.shared.u32", v[2], mem(ad, 8*qt))
		k.e("ld.shared.u32", v[3], mem(ad, 12*qt))
		cache[key] = v
		return v
	}
}

// rowStats bir sorgu satırının görünür anahtar aralığını (lo, hi; Tq dışındaki satırda boş), lse·log2e'yi
// (Tq dışında +∞) ve D'yi (Tq dışında 0) verir. lb = bh·Tq (lse ve D dizilerinin satır tabanı).
func (f *flashShared) rowStats(row, lb, plse, pd reg) (lo, hi, lse, dd reg) {
	k := f.k
	lo, hi = k.attnLimits(row, f.Tq, f.Tk, f.reason2, f.pending2)
	in := k.p()
	k.e("setp.lt.u32", in, row, f.Tq)
	k.e("selp.b32", hi, hi, lo, in)
	li := k.r()
	k.e("add.s32", li, lb, row)
	lse, dd = k.constant(float32(math.Inf(1))), k.constant(0)
	k.ifCase(in, "ld.global.f32", lse, mem(k.address(plse, li, 4), 0))
	k.ifCase(in, "ld.global.f32", dd, mem(k.address(pd, li, 4), 0))
	k.e("mul.rn.f32", lse, lse, float32(math.Log2E))
	return lo, hi, lse, dd
}

// queryInfo sorgu döşemesinin (i0..i0+f.a.Br) satır bilgilerini paylaşımlı belleğe yazar:
// lse·log2e (Tq dışında +∞), D, görünür anahtar aralığının alt ve üst sınırı (Tq dışında boş).
// İlk f.a.Br iş parçacığı yazar.
func (f *flashShared) queryInfo(info, i0, lb, plse, pd reg) {
	k := f.k
	skip := k.label()
	p := k.p()
	k.e("setp.ge.u32", p, f.tid, f.a.Br)
	k.jumpIf(p, skip)
	i := k.r()
	k.e("add.s32", i, i0, f.tid)
	lo, hi, lse, dd := f.rowStats(i, lb, plse, pd)
	ad := k.r()
	k.e("mad.lo.s32", ad, f.tid, 4, info)
	k.e("st.shared.f32", mem(ad, 0), lse)
	k.e("st.shared.f32", mem(ad, 4*f.a.Br), dd)
	k.e("st.shared.u32", mem(ad, 8*f.a.Br), lo)
	k.e("st.shared.u32", mem(ad, 12*f.a.Br), hi)
	k.put(skip)
}
