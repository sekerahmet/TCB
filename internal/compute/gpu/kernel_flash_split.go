package gpu

import (
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Bölünmüş dikkat: az sorgulu ileri yön
// ============================================================================
//
// Birleşik yolun bloğu (kernel_flash.go) Br sorgu satırı tutar ve anahtarları sırayla gezer: tek
// sorguda satırların çoğu boştur ve anahtar ekseni bölünmez. Bu çekirdekte aynı anahtar bakışını
// paylaşan G sorgu bakışının satırları birlikte işlenir (satır rr: sorgu rr / G, bakış rr % G);
// blok bir (toplu, anahtar bakışı, R satır, anahtar parçası) dörtlüsüdür. Bloğun W warp'ı parçanın
// KC'lik bitişik anahtar dilimlerini gezer, warp'ın 32 şeridi D'yi paylaşır (şerit başına D/32 öğe)
// ve bir adımda S = 32/R anahtarın R satırla puanını çıkarır. Çevrimiçi softmax warp'ın içindedir;
// warp'ların (m, l, O)'su blokta warp sırasıyla, parçalarınki ikinci çekirdekte
// (flashSplitMergeKernel) parça sırasıyla birleşir. Bitler yalnız şekle bağlıdır (15.22a):
// R, W, KC ve parça sayısı şekilden seçilir (backend_attention_split.go), SM sayısına bakılmaz.
//
// Görünmeyen (satır, anahtar) çifti O'ya hiç eklenmez: anahtarın K ya da V'sindeki NaN bakmayan
// satıra sızmaz (15.24). Bir adımın bütün anahtarlarını bloğun bütün satırları görüyorsa toplama
// koşulsuzdur; görmüyorsa görünmeyen çift P = −1 işaretiyle atlanır. İki yolun aritmetiği aynıdır.
//
// Şerit düzeni: göz t'deki puan, şeritler arası indirgemeden sonra t ^ şerit'e düşer (seçim komutu
// yoktur; yarılayarak indirgeme 31 karıştırmadır), bir sonucun yayılması da shfl ^ iledir.

// flashSplit bölünmüş çekirdeğin ayarları (bkz. pickSplit).
type flashSplit struct {
	R, W, KC, D int
	single      bool // anahtarlar tek parça: blok sonucu doğrudan yazar
	loose       bool // girdilerin biri 16 bayta hizalı değil: yüklemeler skaler, bitler aynı (15.22a)
}

func (a flashSplit) key() string {
	name := fmt.Sprintf("dikkat_bolme_%d_%d_%d_%d_%v", a.R, a.W, a.KC, a.D, a.single)
	if a.loose {
		name += "_hizasiz"
	}
	return name
}

// part bir parçanın anahtar sayısı.
func (a flashSplit) part() int { return a.W * a.KC }

// splitRun çekirdeğin ortak yazmaçları.
type splitRun struct {
	k              *emitter
	a              flashSplit
	f              *flashShared
	el, S          int // şeridin öğe sayısı, adımın anahtar sayısı
	lane, warp     reg
	lr, ls         reg // şeridin satırı (lane % R) ve anahtarı (lane / R)
	GT, RG         reg // G·Tq satır; satır grubu sayısı ⌈GT/R⌉
	rg, part, parN reg // bloğun satır grubu, parçası; parça sayısı
	b, kvh, kv     reg // toplu, anahtar bakışı, toplu·Hkv + bakış
	rowBase        reg // rg·R
	lo, hi         reg // kendi satırının (lr) görünürlük sınırları
	fullLo, fullHi reg // bloğun bütün satırlarının gördüğü anahtar aralığı
}

// flashSplitKernel (q, k, v) → (çıktı, lse): tek parçada sonuçlar, birden çokta kısmi (O, m, l).
func flashSplitKernel(a flashSplit) func() *emitter {
	return func() *emitter {
		k := newEmitter(a.key())
		s := &splitRun{k: k, a: a, el: a.D / 32, S: 32 / a.R}
		s.f = &flashShared{k: k, a: flashConfig{Dh: a.D, Dv: a.D}}
		pq, pk, pv := k.pointer("q"), k.pointer("k"), k.pointer("v")
		pout, pst := k.written("cikti"), k.written("lse")
		s.f.sharedParams()
		so := k.shared("so", 4*a.W*a.R*a.D)
		sst := k.shared("sst", 8*a.W*a.R)
		s.setup()
		Q := s.loadQ(pq)
		m, l := k.constant(float32(math.Inf(-1))), k.constant(0)
		O := k.zeroMatrix(a.R, s.el)
		j0, jEnd := k.r(), k.r()
		k.e("mul.lo.s32", j0, s.part, a.part())
		k.e("mad.lo.s32", j0, s.warp, a.KC, j0)
		k.e("add.s32", jEnd, j0, a.KC)
		s.visibleRange(j0, jEnd)
		k.whileLess("s32", j0, jEnd, func() { s.step(pk, pv, j0, Q, O, m, l) })
		s.mergeWarps(so, sst, m, l, O, pout, pst)
		return k
	}
}

// setup şerit, blok ve satır grubu dizinleri; kendi satırının görünürlük sınırları. Olmayan satır
// (GT'den sonra) hiç anahtar görmez.
func (s *splitRun) setup() {
	k, f, a := s.k, s.f, s.a
	tid := k.special("%tid.x")
	s.lane, s.warp, s.lr, s.ls = k.r(), k.r(), k.r(), k.r()
	k.e("and.b32", s.lane, tid, 31)
	k.e("shr.u32", s.warp, tid, 5)
	k.e("and.b32", s.lr, s.lane, a.R-1)
	k.e("shr.u32", s.ls, s.lane, log2(a.R))
	s.b, s.kvh = k.special("%ctaid.z"), k.special("%ctaid.y")
	f.Hkv = k.r()
	k.e("div.u32", f.Hkv, f.H, f.G)
	s.kv = k.r()
	k.e("mad.lo.s32", s.kv, s.b, f.Hkv, s.kvh)
	s.GT, s.RG, s.rg, s.part, s.parN = k.r(), k.r(), k.r(), k.r(), k.r()
	k.e("mul.lo.s32", s.GT, f.G, f.Tq)
	k.e("add.s32", s.RG, s.GT, a.R-1)
	k.e("shr.u32", s.RG, s.RG, log2(a.R))
	x := k.special("%ctaid.x")
	k.e("div.u32", s.part, x, s.RG)
	k.e("rem.u32", s.rg, x, s.RG)
	k.e("add.s32", s.parN, f.Tk, a.part()-1)
	k.e("div.u32", s.parN, s.parN, a.part())
	s.rowBase = k.r()
	k.e("mul.lo.s32", s.rowBase, s.rg, a.R)
	own, i := k.r(), k.r()
	k.e("add.s32", own, s.rowBase, s.lr)
	k.e("div.u32", i, own, f.G)
	s.lo, s.hi = k.attnLimits(i, f.Tq, f.Tk, f.reason2, f.pending2)
	out := k.p()
	k.e("setp.ge.u32", out, own, s.GT)
	k.e("selp.b32", s.hi, s.lo, s.hi, out)
}

// visibleRange warp'ın anahtar aralığını bloğun satırlarının gördüğü birleşime (S'ye yuvarlanmış)
// daraltır; bütün satırların gördüğü aralığı (fullLo, fullHi) da kurar. Satır sınırları satırla
// azalmaz: birleşim ilk satırın alt sınırı ve son satırın üst sınırıdır, ortak aralık tersi.
func (s *splitRun) visibleRange(j0, jEnd reg) {
	k, f, a := s.k, s.f, s.a
	first, last := k.r(), k.r()
	k.e("div.u32", first, s.rowBase, f.G)
	k.e("add.s32", last, s.rowBase, a.R)
	k.e("min.u32", last, last, s.GT)
	k.e("sub.s32", last, last, 1)
	k.e("div.u32", last, last, f.G)
	lo, firstHi := k.attnLimits(first, f.Tq, f.Tk, f.reason2, f.pending2)
	lastLo, hi := k.attnLimits(last, f.Tq, f.Tk, f.reason2, f.pending2)
	s.fullLo, s.fullHi = lastLo, firstHi
	k.e("and.b32", lo, lo, -s.S)
	k.e("max.s32", j0, j0, lo)
	k.e("min.s32", jEnd, jEnd, f.Tk)
	k.e("min.s32", jEnd, jEnd, hi)
}

// xor şeridin yerel dizini a ^ n (n = 0 kendisi).
func (s *splitRun) xor(a reg, n int) reg {
	r := s.k.r()
	s.k.e("xor.b32", r, a, n)
	return r
}

// vec şeridin D/32 öğesini dizin idx'ten yükler; p yanlışsa 0. Vektör yükleme öğe sayısı 2 ya da
// 4 iken ve adres hizalıyken kullanılır.
func (s *splitRun) vec(ptr, idx, p reg) []reg {
	k := s.k
	ad := k.address(ptr, idx, 4)
	if (s.el == 2 || s.el == 4) && !s.a.loose {
		return k.loadV("global", p, ad, 0, s.el)
	}
	r := make([]reg, s.el)
	for i := range r {
		r[i] = k.constant(0)
		k.ifCase(p, "ld.global.f32", r[i], mem(ad, 4*i))
	}
	return r
}

// queryIndex satır rr'nin ilk öğesinin dizini: ((b·Tq + rr/G)·H + kvh·G + rr%G)·D.
func (s *splitRun) queryIndex(rr reg) reg {
	k, f := s.k, s.f
	i, h, e := k.r(), k.r(), k.r()
	k.e("div.u32", i, rr, f.G)
	k.e("rem.u32", h, rr, f.G)
	k.e("mad.lo.s32", h, s.kvh, f.G, h)
	k.e("mad.lo.s32", e, s.b, f.Tq, i)
	k.e("mad.lo.s32", e, e, f.H, h)
	k.e("mul.lo.s32", e, e, s.a.D)
	return e
}

// loadQ şeridin Q öğeleri: göz tr, satır tr ^ lr.
func (s *splitRun) loadQ(pq reg) [][]reg {
	k := s.k
	Q := make([][]reg, s.a.R)
	for tr := range Q {
		rr := k.r()
		k.e("add.s32", rr, s.rowBase, s.xor(s.lr, tr))
		p := k.p()
		k.e("setp.lt.u32", p, rr, s.GT)
		e := s.queryIndex(rr)
		k.e("mad.lo.s32", e, s.lane, s.el, e)
		Q[tr] = s.vec(pq, e, p)
	}
	return Q
}

// keyVec anahtar (ya da değer) j0 + (ts ^ ls)'nin şeritteki öğeleri.
func (s *splitRun) keyVec(ptr, j0 reg, ts int) []reg {
	k, f := s.k, s.f
	j, e := k.r(), k.r()
	k.e("add.s32", j, j0, s.xor(s.ls, ts))
	p := k.p()
	k.e("setp.lt.u32", p, j, f.Tk)
	k.e("mad.lo.s32", e, s.b, f.Tk, j)
	k.e("mad.lo.s32", e, e, f.Hkv, s.kvh)
	k.e("mul.lo.s32", e, e, s.a.D)
	k.e("mad.lo.s32", e, s.lane, s.el, e)
	return s.vec(ptr, e, p)
}

// fromLane şerit ^ n'nin değeri (n = 0 kendisi).
func (s *splitRun) fromLane(v reg, n int) reg {
	if n == 0 {
		return v
	}
	return s.k.shflXor(v, n)
}

// scores S anahtarın R satırla puanı: şerit n'nin sonucu n'inci puandır (satır n % R, anahtar n / R).
func (s *splitRun) scores(Q, K [][]reg) reg {
	k, R := s.k, s.a.R
	var v [32]reg
	for t := range v {
		q, kv := Q[t%R], K[t/R]
		v[t] = k.f()
		k.e("mul.rn.f32", v[t], q[0], kv[0])
		for e := 1; e < s.el; e++ {
			k.e("fma.rn.f32", v[t], q[e], kv[e], v[t])
		}
	}
	for b := 16; b > 0; b /= 2 {
		for t := 0; t < b; t++ {
			v[t] = k.sum(v[t], k.shflXor(v[t|b], b))
		}
	}
	return v[0]
}

// step S anahtarın katkısı: puanlar, çevrimiçi softmax (satır başına m, l) ve O.
func (s *splitRun) step(pk, pv, j0 reg, Q, O [][]reg, m, l reg) {
	k, f, R := s.k, s.f, s.a.R
	K := make([][]reg, s.S)
	for ts := range K {
		K[ts] = s.keyVec(pk, j0, ts)
	}
	x, jj := s.scores(Q, K), k.r()
	k.e("mul.rn.f32", x, x, f.scaleF)
	k.e("add.s32", jj, j0, s.ls)
	vis := k.attnVisible(jj, s.lo, s.hi, "", "", "", false)
	k.e("selp.f32", x, x, float32(math.Inf(-1)), vis)
	mx := x
	for b := R; b < 32; b *= 2 {
		mx = k.largest(mx, k.shflXor(mx, b))
	}
	mn := k.largest(m, mx)
	empty := k.p()
	k.e("setp.eq.f32", empty, mn, float32(math.Inf(-1)))
	d, z := k.f(), k.f()
	k.e("sub.rn.f32", d, m, mn)
	alpha := k.exponential(d)
	k.e("selp.f32", alpha, float32(1), alpha, empty)
	k.e("sub.rn.f32", z, x, mn)
	P := k.exponential(z)
	k.e("selp.f32", P, float32(0), P, empty)
	seen := k.f() // görünmeyen çiftin işareti −1
	k.e("selp.f32", seen, P, float32(-1), vis)
	k.e("mov.f32", m, mn)
	total := P
	for b := R; b < 32; b *= 2 {
		total = k.sum(total, k.shflXor(total, b))
	}
	k.e("fma.rn.f32", l, l, alpha, total)
	for tr := range O {
		al := s.fromLane(alpha, tr)
		for e := range O[tr] {
			k.e("mul.rn.f32", O[tr][e], O[tr][e], al)
		}
	}
	for ts := range K {
		K[ts] = s.keyVec(pv, j0, ts)
	}
	s.accumulate(j0, P, seen, K, O)
	k.e("add.s32", j0, j0, s.S)
}

// accumulate O += P·V. Adımın bütün anahtarlarını bloğun bütün satırları görüyorsa koşulsuz, yoksa
// görünmeyen çift atlanır; NaN puanlı çift görünür sayılır (geu).
func (s *splitRun) accumulate(j0, P, seen reg, K, O [][]reg) {
	k, R := s.k, s.a.R
	full, p2, end, partial, j1 := k.p(), k.p(), k.label(), k.label(), k.r()
	k.e("add.s32", j1, j0, s.S)
	k.e("setp.ge.s32", full, j0, s.fullLo)
	k.e("setp.le.s32", p2, j1, s.fullHi)
	k.e("and.pred", full, full, p2)
	k.jumpUnless(full, partial)
	for t := 0; t < 32; t++ {
		p := s.fromLane(P, t)
		for e, o := range O[t%R] {
			k.e("fma.rn.f32", o, p, K[t/R][e], o)
		}
	}
	k.jump(end)
	k.put(partial)
	for t := 0; t < 32; t++ {
		p, see := s.fromLane(seen, t), k.p()
		k.e("setp.geu.f32", see, p, float32(0))
		for e, o := range O[t%R] {
			k.ifCase(see, "fma.rn.f32", o, p, K[t/R][e], o)
		}
	}
	k.put(end)
}

// mergeWarps warp'ların (m, l, O)'sunu paylaşımlı bellekte warp sırasıyla birleştirir; warp w satır
// w, w + W, ...'yi yazar.
func (s *splitRun) mergeWarps(so, sst, m, l reg, O [][]reg, pout, pst reg) {
	k, a := s.k, s.a
	for tr := range O {
		ad := k.r()
		k.e("mad.lo.s32", ad, s.warp, a.R, s.xor(s.lr, tr))
		k.e("mul.lo.s32", ad, ad, a.D)
		k.e("mad.lo.s32", ad, s.lane, s.el, ad)
		k.e("mad.lo.s32", ad, ad, 4, so)
		for e, v := range O[tr] {
			k.e("st.shared.f32", mem(ad, 4*e), v)
		}
	}
	skip := k.label()
	p := k.p()
	k.e("setp.ne.u32", p, s.ls, 0)
	k.jumpIf(p, skip)
	ad := k.r()
	k.e("mad.lo.s32", ad, s.warp, a.R, s.lr)
	k.e("mad.lo.s32", ad, ad, 8, sst)
	k.e("st.shared.f32", mem(ad, 0), m)
	k.e("st.shared.f32", mem(ad, 4), l)
	k.put(skip)
	k.barrier()
	for q := 0; q*a.W < a.R; q++ {
		s.mergeRow(so, sst, q, pout, pst)
	}
}

// mergeRow warp'ın q'uncu satırı (warp + q·W): warp'ların katkısı, sonra sonuç.
func (s *splitRun) mergeRow(so, sst reg, q int, pout, pst reg) {
	k, a := s.k, s.a
	end := k.label()
	row, rr := k.r(), k.r()
	k.e("add.s32", row, s.warp, q*a.W)
	k.e("add.s32", rr, s.rowBase, row)
	p, p2 := k.p(), k.p()
	k.e("setp.ge.u32", p, row, a.R)
	k.e("setp.ge.u32", p2, rr, s.GT)
	k.e("or.pred", p, p, p2)
	k.jumpIf(p, end)
	var ms, stats, outs []reg
	for w := 0; w < a.W; w++ {
		ost, oo, x := k.r(), k.r(), k.f()
		k.e("add.s32", ost, row, w*a.R)
		k.e("mul.lo.s32", oo, ost, a.D)
		k.e("mad.lo.s32", oo, s.lane, s.el, oo)
		k.e("mad.lo.s32", oo, oo, 4, so)
		k.e("mad.lo.s32", ost, ost, 8, sst)
		k.e("ld.shared.f32", x, mem(ost, 0))
		ms, stats, outs = append(ms, x), append(stats, ost), append(outs, oo)
	}
	best := ms[0]
	for _, x := range ms[1:] {
		best = k.largest(best, x)
	}
	l, o := k.constant(0), make([]reg, s.el)
	for e := range o {
		o[e] = k.constant(0)
	}
	for w := range ms {
		wt, lw := k.mergeWeight(ms[w], best), k.f()
		k.e("ld.shared.f32", lw, mem(stats[w], 4))
		k.e("fma.rn.f32", l, wt, lw, l)
		for e := range o {
			v := k.f()
			k.e("ld.shared.f32", v, mem(outs[w], 4*e))
			k.e("fma.rn.f32", o[e], wt, v, o[e])
		}
	}
	s.writeRow(rr, o, best, l, end, pout, pst)
	k.put(end)
}

// writeRow satırın sonucu: tek parçada (O/l, lse), yoksa parçanın (O, m, l)'si [kv·GT + rr][parça].
func (s *splitRun) writeRow(rr reg, o []reg, m, l reg, end string, pout, pst reg) {
	k, f := s.k, s.f
	if s.a.single {
		i, h, li := k.r(), k.r(), k.r()
		k.e("div.u32", i, rr, f.G)
		k.e("rem.u32", h, rr, f.G)
		k.e("mad.lo.s32", h, s.kvh, f.G, h)
		k.e("mad.lo.s32", li, s.b, f.H, h)
		k.e("mad.lo.s32", li, li, f.Tq, i)
		k.finalRow(o, m, l, s.queryIndex(rr), li, s.lane, s.el, pout, pst)
		return
	}
	at, oi := k.r(), k.r()
	k.e("mad.lo.s32", at, s.kv, s.GT, rr)
	k.e("mad.lo.s32", at, at, s.parN, s.part)
	k.e("mul.lo.s32", oi, at, s.a.D)
	k.e("mad.lo.s32", oi, s.lane, s.el, oi)
	oad := k.address(pout, oi, 4)
	for e := range o {
		k.e("st.global.f32", mem(oad, 4*e), o[e])
	}
	k.jumpIf(k.tidNonZero(s.lane), end)
	sad := k.address(pst, at, 8)
	k.e("st.global.f32", mem(sad, 0), m)
	k.e("st.global.f32", mem(sad, 4), l)
}

// mergeWeight e^{m − en büyük}; m = −sonsuz (hiç anahtar görmemiş) ise 0.
func (k *emitter) mergeWeight(m, best reg) reg {
	d, p := k.f(), k.p()
	k.e("setp.eq.f32", p, m, float32(math.Inf(-1)))
	k.e("sub.rn.f32", d, m, best)
	w := k.exponential(d)
	k.e("selp.f32", w, float32(0), w, p)
	return w
}

// finalRow satırın O/l'si (öğe dizini e + şerit·el) ve 0. şeritte lse'si (dizin li); boş satırda
// O = 0, lse = +∞.
func (k *emitter) finalRow(o []reg, m, l, e, li, lane reg, el int, pout, plse reg) {
	empty := k.p()
	k.e("setp.eq.f32", empty, l, float32(0))
	inverse := k.f()
	k.e("rcp.rn.f32", inverse, l)
	k.e("selp.f32", inverse, float32(0), inverse, empty)
	idx := k.r()
	k.e("mad.lo.s32", idx, lane, el, e)
	ad := k.address(pout, idx, 4)
	for i, v := range o {
		k.e("mul.rn.f32", v, v, inverse)
		k.e("st.global.f32", mem(ad, 4*i), v)
	}
	end := k.label()
	k.jumpIf(k.tidNonZero(lane), end)
	lv := k.logarithm(l)
	k.e("add.rn.f32", lv, lv, m)
	k.e("selp.f32", lv, float32(math.Inf(1)), lv, empty)
	k.store(tensor.Float32, k.address(plse, li, 4), lv)
	k.put(end)
}

// flashSplitMergeKernel bölünmüş dikkatin parçalarının (m, l, O)'sunu parça
// sırasıyla birleştirir: ızgara satır (kv·GT + rr), blok 32 iş parçacığı (şerit D/32 öğe). Kısmi
// düzen: O [satır][parça][D], durum [satır][parça][m, l].
func flashSplitMergeKernel(D int) func() *emitter {
	return func() *emitter {
		k := newEmitter(flashSplitMergeKey(D))
		po, ps := k.pointer("kismi"), k.pointer("durum")
		pout, plse := k.written("cikti"), k.written("lse")
		Tq, H, G, parN := k.u32p("tq"), k.u32p("h"), k.u32p("g"), k.u32p("parca")
		el, lane, row := D/32, k.special("%tid.x"), k.special("%ctaid.x")
		e, li := k.mergeRowIndex(row, Tq, H, G, D)
		first := k.r()
		k.e("mul.lo.s32", first, row, parN)
		base := k.address(ps, first, 8)
		best, n := k.constant(float32(math.Inf(-1))), k.whole(0)
		k.whileLess("u32", n, parN, func() {
			x := k.f()
			k.e("ld.global.f32", x, mem(k.address(base, n, 8), 0))
			k.e("mov.f32", best, k.largest(best, x))
			k.e("add.s32", n, n, 1)
		})
		l, o := k.constant(0), make([]reg, el)
		for i := range o {
			o[i] = k.constant(0)
		}
		n = k.whole(0)
		k.whileLess("u32", n, parN, func() {
			sa := k.address(base, n, 8)
			x, lw := k.f(), k.f()
			k.e("ld.global.f32", x, mem(sa, 0))
			k.e("ld.global.f32", lw, mem(sa, 4))
			w := k.mergeWeight(x, best)
			k.e("fma.rn.f32", l, w, lw, l)
			oi := k.r()
			k.e("add.s32", oi, first, n)
			k.e("mul.lo.s32", oi, oi, D)
			k.e("mad.lo.s32", oi, lane, el, oi)
			oa := k.address(po, oi, 4)
			for i := range o {
				v := k.f()
				k.e("ld.global.f32", v, mem(oa, 4*i))
				k.e("fma.rn.f32", o[i], w, v, o[i])
			}
			k.e("add.s32", n, n, 1)
		})
		k.finalRow(o, best, l, e, li, lane, el, pout, plse)
		return k
	}
}

func flashSplitMergeKey(D int) string { return fmt.Sprintf("dikkat_bolme_son_%d", D) }

// mergeRowIndex satır kv·GT + rr'nin çıktısının ilk öğesi ve lse'sinin dizini.
func (k *emitter) mergeRowIndex(row, Tq, H, G reg, D int) (e, li reg) {
	GT, Hkv, kv, rr := k.r(), k.r(), k.r(), k.r()
	k.e("mul.lo.s32", GT, G, Tq)
	k.e("div.u32", Hkv, H, G)
	k.e("div.u32", kv, row, GT)
	k.e("rem.u32", rr, row, GT)
	b, kvh, i, h := k.r(), k.r(), k.r(), k.r()
	k.e("div.u32", b, kv, Hkv)
	k.e("rem.u32", kvh, kv, Hkv)
	k.e("div.u32", i, rr, G)
	k.e("rem.u32", h, rr, G)
	k.e("mad.lo.s32", h, kvh, G, h)
	e, li = k.r(), k.r()
	k.e("mad.lo.s32", e, b, Tq, i)
	k.e("mad.lo.s32", e, e, H, h)
	k.e("mul.lo.s32", e, e, D)
	k.e("mad.lo.s32", li, b, H, h)
	k.e("mad.lo.s32", li, li, Tq, i)
	return e, li
}
