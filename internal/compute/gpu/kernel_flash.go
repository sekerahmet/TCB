package gpu

import (
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Birleşik (döşemeli, çevrimiçi softmax'lı) dikkat: ortak parçalar ve ileri yön
// ============================================================================
//
// Puan matrisi (Tq×Tk) belleğe yazılmaz. İleri yönde her blok bir (toplu, bakış) çiftinin Br
// sorgu satırını tutar ve anahtarları Bc'lik döşemelerle gezer: S = Q·Kᵀ·ölçek (maskeli),
// satır başına koşan en büyük m ve payda l güncellenir, O = O·e^{m_eski−m} + P·V. Sonda
// O/l ve lse = m + log l yazılır. Nedensellik ve pencere dışında kalan anahtar döşemeleri
// hiç gezilmez. Geri yön cekirdek_flash_geri.go'da.
//
// Düzen: 256 iş parçacığı 16×16 ızgaradır (ty satır, tx sütun); her iş parçacığı çıktı
// döşemesinin (satır/16)×(sütun/16)'lık bitişik parçasını kayıtlarda tutar. Paylaşımlı
// bellekteki döşemeler satır öncelikli [satır][Dp+1] düzendedir (Dp: 16'nın katına
// yuvarlanmış boy; +1 dolgu sütun erişiminde bank çakışmasını önler); boyun dışı sütunlar ve
// satırlar 0'la doldurulur. Küçük çarpımlar (mikroCarp) paylaşımlı bellekten skaler okur:
// A işleneni satır (ty) yayınıyla, B işleneni tx'e göre çakışmasız okunur.
//
// Bu düzen 2b'deki tensör çekirdekli (mma) yol için de iskelettir: döşeme yükleme ve
// çevrimiçi softmax aynı kalır, yalnızca mikroCarp değişir.

const flashJob = 256

// flashConfig birleşik dikkat çekirdeklerinin üretim ayarları.
type flashConfig struct {
	Br, Bc, Dh, Dv int
	mask2          bool
}

// round16 d'yi 16'nın katına yuvarlar.
func round16(d int) int { return (d + 15) / 16 * 16 }

func (a flashConfig) key(ad string) string {
	return fmt.Sprintf("%s_%d_%d_%d_%d_%v", ad, a.Br, a.Bc, a.Dh, a.Dv, a.mask2)
}

// smBytes ileri (geri yanlış) ya da geri çekirdeğin paylaşımlı belleği.
func (a flashConfig) smBytes(backward bool) int {
	dh, dv := round16(a.Dh)+1, round16(a.Dv)+1
	if !backward {
		// Q, K/V (ortak), P, iki satır indirgeme tamponu.
		return 4 * (a.Br*dh + a.Bc*max(dh, dv) + a.Br*(a.Bc+1) + 2*a.Br*17)
	}
	// K, V, Q, dO, P/dS.
	return 4 * (a.Bc*(dh+dv) + a.Br*(dh+dv) + a.Br*(a.Bc+1))
}

// pickFlash boylara uyan en büyük döşemeyi seçer; uyan yoksa tamam yanlış (dikkat matris
// çarpımı yoluyla yapılır). İleri yön statik paylaşımlı belleğe (48 KB) sığar; geri yön
// kartın blok başına açtığı paylaşımlı belleği (sinir, ör. 99 KB) kullanabilir: 64×64
// döşemede iş parçacığı başına 4×4 çıktı (32×32'de 2×2) paylaşımlı bellek okumalarını
// yarıya indirir.
func pickFlash(Dh, Dv int, mask2, backward bool, limit int) (flashConfig, bool) {
	if !backward {
		limit = staticSharedMemLimit
	}
	for _, d := range [][2]int{{64, 64}, {64, 32}, {32, 32}, {32, 16}, {16, 16}} {
		a := flashConfig{Br: d[0], Bc: d[1], Dh: Dh, Dv: Dv, mask2: mask2}
		if backward && a.Br != a.Bc {
			continue
		}
		if a.smBytes(backward) <= min(limit, staticSharedMemLimit) || (backward && a.smRounded(backward) <= limit) {
			return a, true
		}
	}
	return flashConfig{}, false
}

// smRounded paylaşımlı dizilerin 16'ya hizalı gerçek toplamı (yazici.ortak'taki gibi; geri
// çekirdeğin beş döşemesi); devingen paylaşımlı bellek bu kadar açılır.
func (a flashConfig) smRounded(backward bool) int {
	dh, dv := round16(a.Dh)+1, round16(a.Dv)+1
	t := 0
	for _, b := range []int{a.Br * dh, a.Br * dv, a.Bc * dh, a.Bc * dv, a.Br * (a.Bc + 1)} {
		t = (t+15)&^15 + 4*b
	}
	return t
}

// isDynamic çekirdeğin paylaşımlı belleği statik sınırı aşıyor mu (devingen diziyle
// üretilir ve başlatmada açılır).
func (a flashConfig) isDynamic(backward bool) bool {
	return backward && a.smRounded(backward) > staticSharedMemLimit
}

// smTile paylaşımlı bellekte satır öncelikli bir döşeme.
type smTile struct {
	base2     reg
	row, step int // satır sayısı ve satır adımı (eleman)
}

// loadTile genel bellekteki [T][D] matrisin (m) r0'dan başlayan satırlarını döşemeye yükler;
// lim'den sonraki satırlar ve D'den sonraki sütunlar (Dp'ye kadar) 0 olur.
func (k *emitter) loadTile(d smTile, D, Dp int, m hmat, r0, lim, tid reg) {
	total := d.row * Dp
	for q := 0; q*flashJob < total; q++ {
		e := k.r()
		k.e("add.s32", e, tid, q*flashJob)
		skip := k.label()
		if (q+1)*flashJob > total {
			p := k.p()
			k.e("setp.ge.u32", p, e, total)
			k.jumpIf(p, skip)
		}
		row2, column := k.r(), k.r()
		k.e("div.u32", row2, e, Dp)
		k.e("rem.u32", column, e, Dp)
		gr := k.r()
		k.e("add.s32", gr, r0, row2)
		p1, p2 := k.p(), k.p()
		k.e("setp.lt.u32", p1, gr, lim)
		k.e("setp.lt.u32", p2, column, D)
		k.e("and.pred", p1, p1, p2)
		idx := k.r()
		k.e("mad.lo.s32", idx, gr, m.stride, column)
		v := k.constant(0)
		k.ifCase(p1, "ld.global.f32", v, mem(k.address(m.base, idx, 4), 0))
		so := k.r()
		k.e("mad.lo.s32", so, row2, d.step, column)
		k.e("mad.lo.s32", so, so, 4, d.base2)
		k.e("st.shared.f32", mem(so, 0), v)
		k.put(skip)
	}
}

// microMul acc[i][j] += Σ_{kk<K} A[i·aIdx + kk·aK]·B[j·bIdx + kk·bK]; aT ve bT iş
// parçacığının ilk satır/sütununun paylaşımlı bellek adresi (bayt), adımlar eleman.
func (k *emitter) microMul(acc [][]reg, aT reg, aIdx, aK int, bT reg, bIdx, bK, K int) {
	av, bv := make([]reg, len(acc)), make([]reg, len(acc[0]))
	for i := range av {
		av[i] = k.f()
	}
	for j := range bv {
		bv[j] = k.f()
	}
	for kk := 0; kk < K; kk++ {
		for i := range av {
			k.e("ld.shared.f32", av[i], mem(aT, 4*(i*aIdx+kk*aK)))
		}
		for j := range bv {
			k.e("ld.shared.f32", bv[j], mem(bT, 4*(j*bIdx+kk*bK)))
		}
		for i := range av {
			for j := range bv {
				k.e("fma.rn.f32", acc[i][j], av[i], bv[j], acc[i][j])
			}
		}
	}
}

// zeroMatrix r×c sıfır kayıt matrisi.
func (k *emitter) zeroMatrix(r, c int) [][]reg {
	m := make([][]reg, r)
	for i := range m {
		m[i] = make([]reg, c)
		for j := range m[i] {
			m[i][j] = k.constant(0)
		}
	}
	return m
}

// thAddr sm + (ilk·adim)·4: iş parçacığının ilk satırının/sütununun adresi.
func (k *emitter) thAddr(sm, first reg, step int) reg {
	r := k.r()
	k.e("mad.lo.s32", r, first, 4*step, sm)
	return r
}

// rowReduce iş parçacığının satırlarındaki değerleri (her satırda cpt değer) birleştirir,
// 16 tx'in kısmi sonuçlarını red [Br][17] tamponuyla toplar; her satırın sonucu (aynı
// sırayla, bütün tx'lerde aynı) döner. İçinde engel vardır.
func (k *emitter) rowReduce(v [][]reg, red, row0, tx reg, join func(a, b reg) reg) []reg {
	for i, s := range v {
		p := s[0]
		for _, x := range s[1:] {
			p = join(p, x)
		}
		ad := k.r()
		k.e("add.s32", ad, row0, i)
		k.e("mad.lo.s32", ad, ad, 17, tx)
		k.e("mad.lo.s32", ad, ad, 4, red)
		k.e("st.shared.f32", mem(ad, 0), p)
	}
	k.barrier()
	r := make([]reg, len(v))
	for i := range v {
		ad := k.r()
		k.e("add.s32", ad, row0, i)
		k.e("mad.lo.s32", ad, ad, 4*17, red)
		t := k.f()
		k.e("ld.shared.f32", t, mem(ad, 0))
		for j := 1; j < 16; j++ {
			x := k.f()
			k.e("ld.shared.f32", x, mem(ad, 4*j))
			t = join(t, x)
		}
		r[i] = t
	}
	return r
}

// flashShared ileri ve geri çekirdeklerin ortak parametreleri ve indeksleri.
type flashShared struct {
	k                           *emitter
	a                           flashConfig
	md, mo                      reg
	Tq, Tk, H, G                reg
	reason2, pending2, msQ, msK reg
	scaleF                      reg
	tid, tx, ty                 reg
	b, h, bh, kvh, bhkv, Hkv    reg
}

// sharedParams maske, boylar ve dikkat seçeneklerinin parametrelerini bildirir.
func (f *flashShared) sharedParams() {
	k := f.k
	f.md = k.pointer("maske")
	f.mo = k.pointer("maske_ofset")
	f.Tq, f.Tk, f.H, f.G = k.u32p("tq"), k.u32p("tk"), k.u32p("h"), k.u32p("g")
	f.reason2, f.pending2 = k.u32p("nedensel"), k.u32p("pencere")
	f.msQ, f.msK = k.u32p("msq"), k.u32p("msk")
	f.scaleF = k.f32p("olcek")
	f.tid = k.special("%tid.x")
	f.tx, f.ty = k.r(), k.r()
	k.e("and.b32", f.tx, f.tid, 15)
	k.e("shr.u32", f.ty, f.tid, 4)
}

// headIndexes b, h (ızgaranın z ve y'si; geri dK/dV çekirdeğinde y anahtar bakışıdır).
func (f *flashShared) headIndexes(yKV bool) {
	k := f.k
	f.b = k.special("%ctaid.z")
	y := k.special("%ctaid.y")
	f.Hkv = k.r()
	k.e("div.u32", f.Hkv, f.H, f.G)
	f.bh, f.bhkv = k.r(), k.r()
	if yKV {
		f.kvh = y
		f.h = k.r()
		k.e("mul.lo.s32", f.h, y, f.G)
	} else {
		f.h = y
		f.kvh = k.r()
		k.e("div.u32", f.kvh, y, f.G)
	}
	k.e("mad.lo.s32", f.bh, f.b, f.H, f.h)
	k.e("mad.lo.s32", f.bhkv, f.b, f.Hkv, f.kvh)
}

// hmat bir bakışın genel bellekteki [T][D] matrisi: ilk elemanın adresi ve satır adımı
// (eleman). Dikkat girdileri [toplu, T, bakış, D] düzenindedir (dil tanımı 15.17): bakışın
// satırları Hx·D adımlıdır.
type hmat struct{ base, stride reg }

// seqBase sıra = b·Hx + h bakışının matrisi: taban ptr + ((b·T)·Hx + h)·D·elem, adım Hx·D.
func (k *emitter) seqBase(ptr, order, T, Hx reg, D, elem int) hmat {
	b, h, e := k.r(), k.r(), k.r()
	k.e("div.u32", b, order, Hx)
	k.e("rem.u32", h, order, Hx)
	k.e("mul.lo.s32", e, b, T)
	k.e("mad.lo.s32", e, e, Hx, h)
	r := k.rd()
	k.e("mul.wide.u32", r, e, elem*D)
	k.e("add.s64", r, r, ptr)
	st := k.r()
	k.e("mul.lo.s32", st, Hx, D)
	return hmat{r, st}
}

// visibility sorgu satırı i için sınırlar ve maske tabanı (bh: maske ofsetinin başı).
func (f *flashShared) visibility(i, bh reg) (lo, hi, mt reg) {
	k := f.k
	lo, hi = k.attnLimits(i, f.Tq, f.Tk, f.reason2, f.pending2)
	// Tq dışındaki (döşemenin artık) satırlar hiçbir anahtara bakmaz (maske de okunmaz).
	outside := k.p()
	k.e("setp.ge.u32", outside, i, f.Tq)
	k.e("selp.b32", hi, lo, hi, outside)
	mt = k.whole(0)
	if f.a.mask2 {
		k.e("ld.global.u32", mt, mem(k.address(f.mo, bh, 4), 0))
		k.e("mad.lo.s32", mt, i, f.msQ, mt)
	}
	return lo, hi, mt
}

// scores S·ölçek'i maskeler: görünmeyen (ya da satırı Tq dışı) puan −sonsuz olur.
func (f *flashShared) scores(S [][]reg, rows []reg, j0 reg, limit [][3]reg) {
	k := f.k
	for i := range S {
		for j := range S[i] {
			jj := k.r()
			k.e("mad.lo.s32", jj, f.tx, len(S[i]), j0)
			k.e("add.s32", jj, jj, j)
			v := S[i][j]
			k.e("mul.rn.f32", v, v, f.scaleF)
			g := k.attnVisible(jj, limit[i][0], limit[i][1], limit[i][2], f.md, f.msK, f.a.mask2)
			valid := k.p()
			k.e("setp.lt.u32", valid, rows[i], f.Tq)
			k.e("and.pred", g, g, valid)
			k.e("selp.f32", v, v, float32(math.Inf(-1)), g)
		}
	}
}

// flashForwardKernel birleşik dikkatin ileri yönü; ızgara (sorgu döşemeleri, H, Batch).
func flashForwardKernel(a flashConfig) func() *emitter {
	return func() *emitter {
		k := newEmitter(a.key("dikkat_ileri"))
		f := &flashShared{k: k, a: a}
		pq, pk, pv := k.pointer("q"), k.pointer("k"), k.pointer("v")
		pout, plse := k.written("cikti"), k.written("lse")
		f.sharedParams()
		f.headIndexes(false)
		dhp, dvp := round16(a.Dh), round16(a.Dv)
		rpt, cpt, ocpt := a.Br/16, a.Bc/16, dvp/16
		qs := smTile{k.shared("qs", 4*a.Br*(dhp+1)), a.Br, dhp + 1}
		kvs := k.shared("kvs", 4*a.Bc*(max(dhp, dvp)+1))
		ps := smTile{k.shared("ps", 4*a.Br*(a.Bc+1)), a.Br, a.Bc + 1}
		red1, red2 := k.shared("red1", 4*a.Br*17), k.shared("red2", 4*a.Br*17)
		q0 := k.r()
		k.e("mul.lo.s32", q0, k.special("%ctaid.x"), a.Br)
		qb := k.seqBase(pq, f.bh, f.Tq, f.H, a.Dh, 4)
		kb := k.seqBase(pk, f.bhkv, f.Tk, f.Hkv, a.Dh, 4)
		vb := k.seqBase(pv, f.bhkv, f.Tk, f.Hkv, a.Dv, 4)
		k.loadTile(qs, a.Dh, dhp, qb, q0, f.Tq, f.tid)
		// Satırların sınırları ve bloğun gezeceği anahtar döşemeleri.
		row0 := k.r()
		k.e("mad.lo.s32", row0, f.ty, rpt, 0)
		rows := make([]reg, rpt)
		limit := make([][3]reg, rpt)
		for i := range rows {
			rows[i] = k.r()
			k.e("add.s32", rows[i], q0, row0)
			k.e("add.s32", rows[i], rows[i], i)
			lo, hi, mt := f.visibility(rows[i], f.bh)
			limit[i] = [3]reg{lo, hi, mt}
		}
		jStart, jEnd := f.keyRange(q0)
		m, l := make([]reg, rpt), make([]reg, rpt)
		for i := range m {
			m[i], l[i] = k.constant(float32(math.Inf(-1))), k.constant(0)
		}
		O := k.zeroMatrix(rpt, ocpt)
		j0 := k.r()
		k.e("mov.u32", j0, jStart)
		loop, done := k.label(), k.label()
		k.put(loop)
		p := k.p()
		k.e("setp.ge.u32", p, j0, jEnd)
		k.jumpIf(p, done)
		ks := smTile{kvs, a.Bc, dhp + 1}
		k.loadTile(ks, a.Dh, dhp, kb, j0, f.Tk, f.tid)
		k.barrier()
		S := k.zeroMatrix(rpt, cpt)
		k.microMul(S, k.thAddr(qs.base2, row0, qs.step), qs.step, 1, k.thAddr(ks.base2, k.mul(f.tx, cpt), ks.step), ks.step, 1, a.Dh)
		f.scores(S, rows, j0, limit)
		newM := k.rowReduce(S, red1, row0, f.tx, k.largest)
		// V, K'nin yerine (K'yi okuyan bütün iş parçacıkları engeli geçti).
		vs := smTile{kvs, a.Bc, dvp + 1}
		k.loadTile(vs, a.Dv, dvp, vb, j0, f.Tk, f.tid)
		f.onlineSoftmax(S, newM, m, l, O, red2, row0, ps)
		k.barrier()
		k.microMul(O, k.thAddr(ps.base2, row0, ps.step), ps.step, 1, k.thAddr(vs.base2, k.mul(f.tx, ocpt), 1), 1, vs.step, a.Bc)
		k.barrier()
		k.e("add.s32", j0, j0, a.Bc)
		k.jump(loop)
		k.put(done)
		ob := k.seqBase(pout, f.bh, f.Tq, f.H, a.Dv, 4)
		lb := k.r()
		k.e("mul.lo.s32", lb, f.bh, f.Tq)
		for i := range O {
			f.writeForward(O[i], m[i], l[i], rows[i], ob, plse, lb)
		}
		return k
	}
}

// onlineSoftmax çevrimiçi softmax adımı: yeni en büyükle (m) ölçek α = e^{m_eski−m},
// P = e^{S−m} (S yerinde P olur), l = l·α + Σ P, O = O·α; P paylaşımlı belleğe ([sorgu]
// [anahtar]) yazılır. İçinde satır indirgemesinin engeli vardır.
func (f *flashShared) onlineSoftmax(S [][]reg, newM, m, l []reg, O [][]reg, red, row0 reg, ps smTile) {
	k := f.k
	alpha := make([]reg, len(S))
	for i := range S {
		mn := k.largest(m[i], newM[i])
		empty := k.p()
		k.e("setp.eq.f32", empty, mn, float32(math.Inf(-1)))
		d := k.f()
		k.e("sub.rn.f32", d, m[i], mn)
		alpha[i] = k.exponential(d)
		k.e("selp.f32", alpha[i], float32(1), alpha[i], empty)
		for j := range S[i] {
			x := k.f()
			k.e("sub.rn.f32", x, S[i][j], mn)
			e := k.exponential(x)
			k.e("selp.f32", S[i][j], float32(0), e, empty)
		}
		k.e("mov.f32", m[i], mn)
	}
	total2 := k.rowReduce(S, red, row0, f.tx, k.sum)
	for i := range S {
		k.e("fma.rn.f32", l[i], l[i], alpha[i], total2[i])
		for j := range O[i] {
			k.e("mul.rn.f32", O[i][j], O[i][j], alpha[i])
		}
	}
	f.writeTile(S, row0, ps)
}

// writeTile iş parçacığının kayıt döşemesini (satırlar satir0+i, sütunlar tx·cpt+j)
// paylaşımlı bellek döşemesine yazar.
func (f *flashShared) writeTile(M [][]reg, row0 reg, d smTile) {
	k := f.k
	cpt := len(M[0])
	for i := range M {
		ad := k.r()
		k.e("add.s32", ad, row0, i)
		k.e("mad.lo.s32", ad, ad, d.step, k.mul(f.tx, cpt))
		k.e("mad.lo.s32", ad, ad, 4, d.base2)
		for j, v := range M[i] {
			k.e("st.shared.f32", mem(ad, 4*j), v)
		}
	}
}

// mul r·c (c sabit).
func (k *emitter) mul(r reg, c int) reg {
	t := k.r()
	k.e("mul.lo.s32", t, r, c)
	return t
}

// writeForward bir satırın O/l'sini ve (tx = 0) lse'sini yazar; boş satırda O = 0, lse = +∞.
func (f *flashShared) writeForward(O []reg, m, l, row reg, ob hmat, plse, lb reg) {
	k := f.k
	last := k.label()
	p := k.p()
	k.e("setp.ge.u32", p, row, f.Tq)
	k.jumpIf(p, last)
	empty := k.p()
	k.e("setp.eq.f32", empty, l, float32(0))
	inverse := k.f()
	k.e("rcp.rn.f32", inverse, l)
	k.e("selp.f32", inverse, float32(0), inverse, empty)
	for j, v := range O {
		column := k.r()
		k.e("mad.lo.s32", column, f.tx, len(O), j)
		skip := k.label()
		q := k.p()
		k.e("setp.ge.u32", q, column, f.a.Dv)
		k.jumpIf(q, skip)
		k.e("mul.rn.f32", v, v, inverse)
		idx := k.r()
		k.e("mad.lo.s32", idx, row, ob.stride, column)
		k.store(tensor.Float32, k.address(ob.base, idx, 4), v)
		k.put(skip)
	}
	k.jumpIf(k.tidNonZeroTx(f.tx), last)
	lv := k.logarithm(l)
	k.e("add.rn.f32", lv, lv, m)
	k.e("selp.f32", lv, float32(math.Inf(1)), lv, empty)
	li := k.r()
	k.e("add.s32", li, lb, row)
	k.store(tensor.Float32, k.address(plse, li, 4), lv)
	k.put(last)
}

// tidNonZeroTx tx ≠ 0 yüklemi.
func (k *emitter) tidNonZeroTx(tx reg) reg {
	p := k.p()
	k.e("setp.ne.u32", p, tx, 0)
	return p
}

// keyRange sorgu döşemesinin [q0, q0+Br) bakabileceği anahtarları kapsayan döşeme
// aralığı [jBas, jSon) (Bc'nin katlarına yuvarlı): ilk satırın alt ve son satırın üst sınırı
// (sınırlar satırla azalmaz).
func (f *flashShared) keyRange(q0 reg) (jStart, jEnd reg) {
	k := f.k
	lastRow := k.r()
	k.e("add.s32", lastRow, q0, f.a.Br-1)
	tq1 := k.r()
	k.e("sub.s32", tq1, f.Tq, 1)
	k.e("min.u32", lastRow, lastRow, tq1)
	lo, _ := k.attnLimits(q0, f.Tq, f.Tk, f.reason2, f.pending2)
	_, hi := k.attnLimits(lastRow, f.Tq, f.Tk, f.reason2, f.pending2)
	jStart, jEnd = k.r(), k.r()
	k.e("div.u32", jStart, lo, f.a.Bc)
	k.e("mul.lo.s32", jStart, jStart, f.a.Bc)
	k.e("mov.u32", jEnd, hi)
	return jStart, jEnd
}
