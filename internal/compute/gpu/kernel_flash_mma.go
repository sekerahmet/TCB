package gpu

import (
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Birleşik dikkat, tensör çekirdekli yol (16 bitlik girdiler, sm_80+)
// ============================================================================
//
// q, k, v aynı 16 bitlik türdeyse (bf16 ya da f16) çarpımlar mma.sync.m16n8k16 ile yapılır
// (FlashAttention-2 düzeni). Blok bir (toplu, baş) çiftinin mmaBr sorgu satırını tutar; 8
// warp'ın her biri 16 satırdan sorumludur ve satırlarının bütün hesabını (S, çevrimiçi softmax,
// O) yazmaçlarda yapar, warp'lar arası indirgeme yoktur. Q parçaları başta bir kez yazmaçlara
// yüklenir; anahtarlar mmaBc'lik döşemelerle gezilir: K paylaşımlı bellekte [anahtar][Dh+8],
// V devrik [Dv][mmaBc+8] (8 elemanlık dolgu bank çakışmasını önler). S = Q·Kᵀ parçaları
// (float32) softmax'tan sonra doğrudan P·V çarpımının A parçalarına (16 bit) paketlenir;
// P paylaşımlı belleğe yazılmaz. Softmax 2 tabanında yapılır (ölçek·log2e), lse doğal
// logaritmayla yazılır (birleşik SIMT yoluyla aynı çıktılar). Ek maske (maske tensörü)
// desteklenmez: o zaman SIMT yolu kullanılır.

const (
	mmaBr = 128 // blok başına sorgu satırı (8 warp × 16)
	mmaBc = 64  // anahtar döşemesi
)

// flashMma tensör çekirdekli dikkatin üretim ayarları.
type flashMma struct {
	Dh, Dv int
	f16    bool // girdiler f16 (değilse bf16)
}

func (a flashMma) key(ad string) string { return fmt.Sprintf("%s_%d_%d_%v", ad, a.Dh, a.Dv, a.f16) }

// mmaFits tensör çekirdekli yolun bu girdilerle kullanılıp kullanılamayacağı: aynı 16 bitlik
// tür, bitişik ve çift ofsetli görünümler (32 bitlik çift erişimi), 16'nın katı ve en çok 128
// baş boyları, ek maske yok, sm_80+ (TR_GPU_DIKKAT=tensor benzetimde sm denetimini atlar).
func (g *backend) mmaFits(q, k, v tensor.View, mask2 bool, p tensor.AttentionParams) (flashMma, bool) {
	if g.attnPath == "carpim" || g.attnPath == "birlesik" || mask2 || p.H > 65535 || p.Batch > 65535 {
		return flashMma{}, false
	}
	if cc, _ := g.sr.capability(); cc < 80 && g.attnPath != "tensor" {
		return flashMma{}, false
	}
	if p.Dh == 0 || p.Dv == 0 || p.Dh%16 != 0 || p.Dv%16 != 0 || p.Dh > 128 || p.Dv > 128 {
		return flashMma{}, false
	}
	dt := g.storageOf(q.S).dtype
	if dt != tensor.BFloat16 && dt != tensor.Float16 {
		return flashMma{}, false
	}
	for _, w := range []tensor.View{q, k, v} {
		if g.storageOf(w.S).dtype != dt || !w.IsContiguous() || w.Offset%2 != 0 {
			return flashMma{}, false
		}
	}
	checkAttnSize(p, true)
	return flashMma{Dh: p.Dh, Dv: p.Dv, f16: dt == tensor.Float16}, true
}

// base16 ptr + (sıra·T·D)·2: 16 bitlik [T][D] matrisin başı.
func (k *emitter) base16(ptr, order, T reg, D int) reg {
	e := k.r()
	k.e("mul.lo.s32", e, order, T)
	r := k.rd()
	k.e("mul.wide.u32", r, e, 2*D)
	k.e("add.s64", r, r, ptr)
	return r
}

// mmaLanes warp içi konum: w warp, g = şerit/4, t = şerit%4.
type mmaLanes struct{ w, g, t reg }

func (k *emitter) lanes(tid reg) mmaLanes {
	var l mmaLanes
	l.w, l.g, l.t = k.r(), k.r(), k.r()
	lane := k.r()
	k.e("shr.u32", l.w, tid, 5)
	k.e("and.b32", lane, tid, 31)
	k.e("shr.u32", l.g, lane, 2)
	k.e("and.b32", l.t, lane, 3)
	return l
}

// loadPair16 genel bellekteki 16 bitlik [T][D] matrisin (satır, sütun) ve (satır, sütun+1)
// elemanlarını bir 32 bitlik yazmaca yükler; satır lim'in dışındaysa 0.
func (k *emitter) loadPair16(base2, row, col, lim reg, D int) reg {
	v := k.whole(0)
	p := k.p()
	k.e("setp.lt.u32", p, row, lim)
	idx := k.r()
	k.e("mad.lo.s32", idx, row, D, col)
	k.ifCase(p, "ld.global.b32", v, mem(k.address(base2, idx, 2), 0))
	return v
}

// tile16 paylaşımlı bellekte 16 bitlik bir döşeme hedefi: satır öncelikli [R][D+8] (row) ve/veya
// devrik [D][R+8] (trans); boş yazmaç o düzenin yazılmayacağını gösterir.
type tile16 struct {
	row, trans reg
	R, D       int
}

// store2x2 iki satırın (r, r+1) aynı sütun çiftini (c, c+1) döşemeye yazar: satır öncelikli
// düzende v0 = (r, c..c+1), v1 = (r+1, c..c+1); devrik düzende (c, r..r+1) ve (c+1, r..r+1)
// çiftleri 32 bitlik yazmalarla.
func (k *emitter) store2x2(t tile16, r, c, v0, v1 reg) {
	if t.row != "" {
		so := k.r()
		k.e("mad.lo.s32", so, r, t.D+8, c)
		k.e("mad.lo.s32", so, so, 2, t.row)
		k.e("st.shared.b32", mem(so, 0), v0)
		k.e("st.shared.b32", mem(so, 2*(t.D+8)), v1)
	}
	if t.trans != "" {
		lo, hi, x := k.r(), k.r(), k.r()
		k.e("and.b32", lo, v0, 0xFFFF)
		k.e("shl.b32", x, v1, 16)
		k.e("or.b32", lo, lo, x)
		k.e("shr.u32", hi, v0, 16)
		y := k.r()
		k.e("and.b32", y, v1, -65536)
		k.e("or.b32", hi, hi, y)
		so := k.r()
		k.e("mad.lo.s32", so, c, t.R+8, r)
		k.e("mad.lo.s32", so, so, 2, t.trans)
		k.e("st.shared.b32", mem(so, 0), lo)
		k.e("st.shared.b32", mem(so, 2*(t.R+8)), hi)
	}
}

// tileWork döşemenin 2×2'lik parçalarını iş parçacıklarına dağıtır: f(satır, sütun) her parça
// için (satır ve sütun çift) çağrılır.
func (k *emitter) tileWork(tid reg, R, D int, f func(r, c reg)) {
	half := D / 2
	for q := 0; q*flashJob < (R/2)*half; q++ {
		e := k.r()
		k.e("add.s32", e, tid, q*flashJob)
		skip := k.label()
		if (q+1)*flashJob > (R/2)*half {
			p := k.p()
			k.e("setp.ge.u32", p, e, (R/2)*half)
			k.jumpIf(p, skip)
		}
		rp, pc := k.r(), k.r()
		k.e("div.u32", rp, e, half)
		k.e("rem.u32", pc, e, half)
		r, c := k.r(), k.r()
		k.e("shl.b32", r, rp, 1)
		k.e("shl.b32", c, pc, 1)
		f(r, c)
		k.put(skip)
	}
}

// loadTile16 genel bellekteki 16 bitlik [T][D] matrisin r0'dan başlayan R satırını döşemeye
// yükler; lim'in dışındaki satırlar 0.
func (k *emitter) loadTile16(t tile16, base2, r0, lim, tid reg) {
	k.tileWork(tid, t.R, t.D, func(r, c reg) {
		g0, g1 := k.r(), k.r()
		k.e("add.s32", g0, r0, r)
		k.e("add.s32", g1, g0, 1)
		k.store2x2(t, r, c, k.loadPair16(base2, g0, c, lim, t.D), k.loadPair16(base2, g1, c, lim, t.D))
	})
}

// loadTileF32 genel bellekteki float32 [T][D] matrisin r0'dan başlayan R satırını 16 bite
// (en yakına-çifte) yuvarlayarak döşemeye yükler; lim'in dışındaki satırlar 0.
func (k *emitter) loadTileF32(t tile16, base2, r0, lim, tid reg, f16 bool) {
	k.tileWork(tid, t.R, t.D, func(r, c reg) {
		var v [2][2]reg
		for i := range 2 {
			g := k.r()
			k.e("add.s32", g, r0, r)
			k.e("add.s32", g, g, i)
			p := k.p()
			k.e("setp.lt.u32", p, g, lim)
			idx := k.r()
			k.e("mad.lo.s32", idx, g, t.D, c)
			ad := k.address(base2, idx, 4)
			for j := range 2 {
				v[i][j] = k.constant(0)
				k.ifCase(p, "ld.global.f32", v[i][j], mem(ad, 4*j))
			}
		}
		k.store2x2(t, r, c, k.pack2(v[0][0], v[0][1], f16), k.pack2(v[1][0], v[1][1], f16))
	})
}

// flashMmaForwardKernel tensör çekirdekli dikkatin ileri yönü; ızgara (⌈Tq/mmaBr⌉, H, Batch).
func flashMmaForwardKernel(a flashMma) func() *emitter {
	return func() *emitter {
		k := newEmitter(a.key("dikkat_ileri_tc"))
		k.target = 80
		f := &flashShared{k: k, a: flashConfig{Br: mmaBr, Bc: mmaBc, Dh: a.Dh, Dv: a.Dv}}
		pq, pk, pv := k.pointer("q"), k.pointer("k"), k.pointer("v")
		pout, plse := k.written("cikti"), k.written("lse")
		f.sharedParams()
		f.headIndexes(false)
		ks := k.shared("ks", 2*mmaBc*(a.Dh+8))
		vts := k.shared("vts", 2*a.Dv*(mmaBc+8))
		l := k.lanes(f.tid)
		q0 := k.r()
		k.e("mul.lo.s32", q0, k.special("%ctaid.x"), mmaBr)
		qb := k.base16(pq, f.bh, f.Tq, a.Dh)
		kb := k.base16(pk, f.bhkv, f.Tk, a.Dh)
		vb := k.base16(pv, f.bhkv, f.Tk, a.Dv)
		// Warp'ın satırları: r[0] = q0 + 16w + g, r[1] = r[0] + 8.
		var rows [2]reg
		rows[0], rows[1] = k.r(), k.r()
		k.e("mad.lo.s32", rows[0], l.w, 16, q0)
		k.e("add.s32", rows[0], rows[0], l.g)
		k.e("add.s32", rows[1], rows[0], 8)
		t2 := k.r()
		k.e("shl.b32", t2, l.t, 1)
		// Q parçaları (A): her k adımı için (r0, 2t), (r1, 2t), (r0, 2t+8), (r1, 2t+8).
		nks := a.Dh / 16
		qa := make([][4]reg, nks)
		for s := range nks {
			for i, o := range [4][2]int{{0, 0}, {1, 0}, {0, 8}, {1, 8}} {
				c := k.r()
				k.e("add.s32", c, t2, 16*s+o[1])
				qa[s][i] = k.loadPair16(qb, rows[o[0]], c, f.Tq, a.Dh)
			}
		}
		var lo, hi [2]reg
		for i := range rows {
			lo[i], hi[i] = k.attnLimits(rows[i], f.Tq, f.Tk, f.reason2, f.pending2)
			outside := k.p()
			k.e("setp.ge.u32", outside, rows[i], f.Tq)
			k.e("selp.b32", hi[i], lo[i], hi[i], outside)
		}
		jStart, jEnd := f.keyRange(q0)
		sc2 := k.f()
		k.e("mul.rn.f32", sc2, f.scaleF, float32(math.Log2E))
		var m, sum [2]reg
		for i := range m {
			m[i], sum[i] = k.constant(float32(math.Inf(-1))), k.constant(0)
		}
		nS, nO := mmaBc/8, a.Dv/8
		S, O := make([][4]reg, nS), make([][4]reg, nO)
		for i := range S {
			for c := range 4 {
				S[i][c] = k.f()
			}
		}
		for i := range O {
			for c := range 4 {
				O[i][c] = k.constant(0)
			}
		}
		// Paylaşımlı bellekte şeridin ilk okuma adresleri: K[g][2t], Vᵀ[g][2t].
		kAddr, vAddr := k.r(), k.r()
		k.e("mad.lo.s32", kAddr, l.g, a.Dh+8, t2)
		k.e("mad.lo.s32", kAddr, kAddr, 2, ks)
		k.e("mad.lo.s32", vAddr, l.g, mmaBc+8, t2)
		k.e("mad.lo.s32", vAddr, vAddr, 2, vts)
		j0 := k.r()
		k.e("mov.u32", j0, jStart)
		loop, done := k.label(), k.label()
		k.put(loop)
		p := k.p()
		k.e("setp.ge.u32", p, j0, jEnd)
		k.jumpIf(p, done)
		k.barrier() // önceki döşemeyi okuyan warp'lar bitti
		k.loadTile16(tile16{row: ks, R: mmaBc, D: a.Dh}, kb, j0, f.Tk, f.tid)
		k.loadTile16(tile16{trans: vts, R: mmaBc, D: a.Dv}, vb, j0, f.Tk, f.tid)
		k.barrier()
		// S = Q·Kᵀ
		for i := range S {
			for c := range 4 {
				k.e("mov.f32", S[i][c], float32(0))
			}
		}
		for s := range nks {
			for n := range nS {
				var b [2]reg
				b[0], b[1] = k.r(), k.r()
				ofs := 2 * (n*8*(a.Dh+8) + 16*s)
				k.e("ld.shared.b32", b[0], mem(kAddr, ofs))
				k.e("ld.shared.b32", b[1], mem(kAddr, ofs+16))
				k.mma(S[n], S[n], qa[s], b, a.f16)
			}
		}
		f.mmaSoftmax(S, O, rows, lo, hi, m, sum, j0, t2, sc2)
		// O += P·V: P parçaları S'den (iki n döşemesi bir k adımı).
		for s := range mmaBc / 16 {
			var pa [4]reg
			pa[0] = k.pack2(S[2*s][0], S[2*s][1], a.f16)
			pa[1] = k.pack2(S[2*s][2], S[2*s][3], a.f16)
			pa[2] = k.pack2(S[2*s+1][0], S[2*s+1][1], a.f16)
			pa[3] = k.pack2(S[2*s+1][2], S[2*s+1][3], a.f16)
			for n := range nO {
				var b [2]reg
				b[0], b[1] = k.r(), k.r()
				ofs := 2 * (n*8*(mmaBc+8) + 16*s)
				k.e("ld.shared.b32", b[0], mem(vAddr, ofs))
				k.e("ld.shared.b32", b[1], mem(vAddr, ofs+16))
				k.mma(O[n], O[n], pa, b, a.f16)
			}
		}
		k.e("add.s32", j0, j0, mmaBc)
		k.jump(loop)
		k.put(done)
		f.mmaWriteForward(O, rows, m, sum, t2, l.t, pout, plse)
		return k
	}
}

// mmaSoftmax çevrimiçi softmax adımı (2 tabanında): S görünmeyenlerde −∞ olur, satırın yeni en
// büyüğüyle α = 2^{m_eski−m}, P = 2^{S−m} (S yerinde P olur), payda ve O ölçeklenir. Şeridin
// satırları r0 (parça 0, 1) ve r1 (parça 2, 3); satır dört şeride (t) dağılmıştır, indirgeme
// şeritler arası değiş tokuşla yapılır.
func (f *flashShared) mmaSoftmax(S, O [][4]reg, rows, lo, hi, m, sum [2]reg, j0, t2, sc2 reg) {
	k := f.k
	col := k.r()
	k.e("add.s32", col, j0, t2)
	for n := range S {
		for c := range 4 {
			j := k.r()
			k.e("add.s32", j, col, 8*n+c%2)
			r := c / 2
			v := S[n][c]
			k.e("mul.rn.f32", v, v, sc2)
			p1, p2 := k.p(), k.p()
			k.e("setp.ge.s32", p1, j, lo[r])
			k.e("setp.lt.s32", p2, j, hi[r])
			k.e("and.pred", p1, p1, p2)
			k.e("selp.f32", v, v, float32(math.Inf(-1)), p1)
		}
	}
	for r := range 2 {
		mx := S[0][2*r]
		for n := range S {
			for c := 2 * r; c < 2*r+2; c++ {
				if n == 0 && c == 2*r {
					continue
				}
				mx = k.largest(mx, S[n][c])
			}
		}
		mx = k.largest(mx, k.shflXor(mx, 1))
		mx = k.largest(mx, k.shflXor(mx, 2))
		mn := k.f()
		k.e("max.f32", mn, m[r], mx)
		empty := k.p()
		k.e("setp.eq.f32", empty, mn, float32(math.Inf(-1)))
		base := k.f()
		k.e("selp.f32", base, float32(0), mn, empty)
		d := k.f()
		k.e("sub.rn.f32", d, m[r], base)
		alpha := k.f()
		k.e("ex2.approx.f32", alpha, d)
		total := k.constant(0)
		for n := range S {
			for c := 2 * r; c < 2*r+2; c++ {
				x := k.f()
				k.e("sub.rn.f32", x, S[n][c], base)
				k.e("ex2.approx.f32", S[n][c], x)
				k.e("add.rn.f32", total, total, S[n][c])
			}
		}
		total = k.sum(total, k.shflXor(total, 1))
		total = k.sum(total, k.shflXor(total, 2))
		k.e("fma.rn.f32", sum[r], sum[r], alpha, total)
		for n := range O {
			for c := 2 * r; c < 2*r+2; c++ {
				k.e("mul.rn.f32", O[n][c], O[n][c], alpha)
			}
		}
		k.e("mov.f32", m[r], mn)
	}
}

// mmaWriteForward O/payda ve (t = 0 şeritleri) lse = m·ln2 + ln(payda) yazar; Tq dışındaki
// satırlar yazılmaz, boş satırda O = 0, lse = +∞ (birleşik yolla aynı).
func (f *flashShared) mmaWriteForward(O [][4]reg, rows, m, sum [2]reg, t2, t, pout, plse reg) {
	k := f.k
	ob := k.matrixBase(pout, f.bh, f.Tq, f.a.Dv)
	lb := k.r()
	k.e("mul.lo.s32", lb, f.bh, f.Tq)
	for r := range 2 {
		last := k.label()
		p := k.p()
		k.e("setp.ge.u32", p, rows[r], f.Tq)
		k.jumpIf(p, last)
		empty := k.p()
		k.e("setp.eq.f32", empty, sum[r], float32(0))
		inverse := k.f()
		k.e("rcp.rn.f32", inverse, sum[r])
		k.e("selp.f32", inverse, float32(0), inverse, empty)
		for n := range O {
			for c := 2 * r; c < 2*r+2; c++ {
				v := O[n][c]
				k.e("mul.rn.f32", v, v, inverse)
				idx := k.r()
				k.e("add.s32", idx, t2, 8*n+c%2)
				k.e("mad.lo.s32", idx, rows[r], f.a.Dv, idx)
				k.store(tensor.Float32, k.address(ob, idx, 4), v)
			}
		}
		nz := k.p()
		k.e("setp.ne.u32", nz, t, 0)
		k.jumpIf(nz, last)
		lv := k.logarithm(sum[r])
		k.e("fma.rn.f32", lv, m[r], float32(math.Ln2), lv)
		k.e("selp.f32", lv, float32(math.Inf(1)), lv, empty)
		li := k.r()
		k.e("add.s32", li, lb, rows[r])
		k.store(tensor.Float32, k.address(plse, li, 4), lv)
		k.put(last)
	}
}
