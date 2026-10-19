package gpu

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Eksen boyunca birikimli toplam ve sıralama çekirdekleri
// ============================================================================
//
// Girdiler float32 bitişik (dış, eksen, iç) düzenindedir: (o, d, i) elemanı
// (o·L + d)·iç + i'de. Bir "satır" sabit (o, i) için eksen boyunca L elemandır.
//
// Birikimli toplam: satır başına bir blok. Her iş parçacığı satırın ardışık bir parçasını
// toplar, parça toplamları paylaşımlı bellekte sabit sıralı bir taramayla (Hillis-Steele)
// önek toplamına çevrilir, sonra her parça önekiyle yeniden taranıp yazılır. Toplama sırası
// yalnızca L'ye bağlıdır (belirlenimci).
//
// Sıralama: her eleman 64 bitlik bir anahtara çevrilir: üst 32 bit değerin sıra anahtarı
// (CPU siraAnahtari ile aynı: −0 = +0, NaN en büyük; azalan sıralamada tersi), alt 32 bit
// eksen indeksi. Anahtarlar tekil olduğundan bitonik sıralama (kararsız bir ağ) kararlı
// sıralama verir: eşitlerde küçük indeks önce. Satır ikinin kuvvetine (P) dolgu
// anahtarlarıyla (her gerçek anahtardan büyük) tamamlanır. P ≤ ortakSiralamaSiniri ise satır
// başına bir blok paylaşımlı bellekte sıralar (tek başlatma); daha uzun satırlar genel
// bellekte, her bitonik adım için bir başlatmayla sıralanır (log2(P)·(log2(P)+1)/2
// başlatma; yavaş ama doğru).

// ortakSiralamaSiniri paylaşımlı bellekte sıralanan en uzun (dolgulu) satır: 8 bayt × 4096 =
// 32 KB paylaşımlı bellek. Sınamalar genel bellek yolunu denemek için küçültebilir.
var sharedSortLimit = 4096

// birikimCekirdegi satır başına B iş parçacıklı blokla birikimli toplam. ters: sondan;
// ekle: y'ye eklenir.
func accumKernel(B int, inverse, add bool) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("birikim_%d_%v_%v", B, inverse, add))
		x := k.pointer("x")
		y := k.written("y")
		L := k.u32p("l")
		ic := k.u32p("ic")
		C := k.u32p("parca")
		sm := k.shared("sm", 4*B)
		row, tid := k.special("%ctaid.x"), k.special("%tid.x")
		_, _, base2 := k.rowBase(row, L, ic)
		begin := k.r()
		k.e("mul.lo.s32", begin, tid, C)
		k.e("min.u32", begin, begin, L)
		bit := k.r()
		k.e("add.s32", bit, begin, C)
		k.e("min.u32", bit, bit, L)
		// adresi p konumunun (ters ise L−1−p) elemanının adresi.
		addressOf := func(base0 reg, p reg) reg {
			pos := p
			if inverse {
				pos = k.r()
				k.e("sub.s32", pos, L, p)
				k.e("sub.s32", pos, pos, 1)
			}
			j := k.r()
			k.e("mad.lo.s32", j, pos, ic, base2)
			return k.address(base0, j, 4)
		}
		walk := func(body func(p reg)) {
			p := k.r()
			k.e("mov.u32", p, begin)
			loop, done := k.label(), k.label()
			k.put(loop)
			q := k.p()
			k.e("setp.ge.u32", q, p, bit)
			k.jumpIf(q, done)
			body(p)
			k.e("add.s32", p, p, 1)
			k.jump(loop)
			k.put(done)
		}
		s := k.constant(0)
		walk(func(p reg) { k.e("add.rn.f32", s, s, k.load(tensor.Float32, addressOf(x, p))) })
		run := k.prevTotal(s, tid, sm, B)
		walk(func(p reg) {
			k.e("add.rn.f32", run, run, k.load(tensor.Float32, addressOf(x, p)))
			ha := addressOf(y, p)
			v := run
			if add {
				v = k.load(tensor.Float32, ha)
				k.e("add.rn.f32", v, v, run)
			}
			k.store(tensor.Float32, ha, v)
		})
		return k
	}
}

// oncekiToplam bloktaki iş parçacıklarının s değerlerinin dışlayıcı önek toplamını (bu iş
// parçacığından öncekilerin toplamı) sabit sıralı kapsayıcı taramayla (Hillis-Steele;
// paylaşımlı bellek sm, 4·B bayt) verir.
func (k *emitter) prevTotal(s, tid, sm reg, B int) reg {
	addr := k.r()
	k.e("mad.lo.s32", addr, tid, 4, sm)
	k.e("st.shared.f32", mem(addr, 0), s)
	k.barrier()
	prev := k.r()
	for off := 1; off < B; off *= 2 {
		v := k.constant(0)
		skip := k.label()
		q := k.p()
		k.e("setp.lt.u32", q, tid, off)
		k.jumpIf(q, skip)
		k.e("sub.s32", prev, addr, 4*off)
		k.e("ld.shared.f32", v, mem(prev, 0))
		k.put(skip)
		k.barrier()
		k.e("add.rn.f32", s, s, v)
		k.e("st.shared.f32", mem(addr, 0), s)
		k.barrier()
	}
	run := k.constant(0)
	first, cont := k.p(), k.label()
	k.e("setp.eq.u32", first, tid, 0)
	k.jumpIf(first, cont)
	k.e("sub.s32", prev, addr, 4)
	k.e("ld.shared.f32", run, mem(prev, 0))
	k.put(cont)
	return run
}

// siraAnahtariPTX v'nin 64 bitlik sıralama anahtarı (değer anahtarı << 32 | d).
func (k *emitter) orderKeyPTX(v, d reg, descending bool) reg {
	b := k.r()
	k.e("mov.b32", b, v)
	neg := k.p()
	k.e("setp.lt.s32", neg, b, 0)
	a1, a2 := k.r(), k.r()
	k.e("not.b32", a1, b)
	k.e("or.b32", a2, b, uint32(0x80000000))
	a := k.r()
	k.e("selp.b32", a, a1, a2, neg)
	zero, nan := k.p(), k.p()
	k.e("setp.eq.f32", zero, v, float32(0))
	k.e("setp.nan.f32", nan, v, v)
	k.e("selp.b32", a, uint32(0x80000000), a, zero)
	k.e("selp.b32", a, uint32(0xFFFFFFFF), a, nan)
	if descending {
		k.e("not.b32", a, a)
	}
	return k.pack(a, d)
}

// paketle a << 32 | d (64 bit).
func (k *emitter) pack(a, d reg) reg {
	u, w := k.rd(), k.rd()
	k.e("cvt.u64.u32", u, a)
	k.e("shl.b64", u, u, 32)
	k.e("cvt.u64.u32", w, d)
	k.e("or.b64", u, u, w)
	return u
}

// satirTabani satır numarasından (o, i) ve satırın ilk elemanının konumunu (o·L·iç + i) verir.
func (k *emitter) rowBase(row, L, ic reg) (o, i, base2 reg) {
	o = k.r()
	k.e("div.u32", o, row, ic)
	i = k.r()
	k.e("mul.lo.s32", i, o, ic)
	k.e("sub.s32", i, row, i)
	base2 = k.r()
	k.e("mul.lo.s32", base2, o, L)
	k.e("mad.lo.s32", base2, base2, ic, i)
	return o, i, base2
}

// anahtarYukle satırın t. elemanının anahtarı; t ≥ L ise dolgu (0xFFFFFFFF << 32 | t).
func (k *emitter) loadKey(x, t, L, ic, base2 reg, descending bool) reg {
	r := k.rd()
	padding := k.p()
	k.e("setp.ge.u32", padding, t, L)
	actual, last := k.label(), k.label()
	k.jumpUnless(padding, actual)
	everything := k.r()
	k.e("mov.u32", everything, uint32(0xFFFFFFFF))
	k.e("mov.b64", r, k.pack(everything, t))
	k.jump(last)
	k.put(actual)
	j := k.r()
	k.e("mad.lo.s32", j, t, ic, base2)
	v := k.load(tensor.Float32, k.address(x, j, 4))
	k.e("mov.b64", r, k.orderKeyPTX(v, t, descending))
	k.put(last)
	return r
}

// bitonikCift t. karşılaştırma çiftinin küçük indeksi i1 = ((t >> lj) << (lj+1)) |
// (t & (j−1)); büyüğü i1 + j (j = 2^lj). lj ve j sabit (int) ya da yazmaç olabilir.
func (k *emitter) bitonicPair(t reg, lj, j any) (i1, i2 reg) {
	var lj1, m any
	if l, ok := lj.(int); ok {
		lj1, m = l+1, (1<<l)-1
	} else {
		r1, r2 := k.r(), k.r()
		k.e("add.s32", r1, lj, 1)
		k.e("sub.s32", r2, j, 1)
		lj1, m = r1, r2
	}
	i1 = k.r()
	k.e("shr.u32", i1, t, lj)
	k.e("shl.b32", i1, i1, lj1)
	sub := k.r()
	k.e("and.b32", sub, t, m)
	k.e("or.b32", i1, i1, sub)
	i2 = k.r()
	k.e("add.s32", i2, i1, j)
	return i1, i2
}

// karsilastirDegistir a = [ai], b = [bi] çifti için (i1 & kk) = 0 ise artan, değilse azalan
// düzene göre gerekirse yer değiştirir. adr: 64 bitlik elemanların adresi (paylaşımlı ya
// da genel).
func (k *emitter) compareSwap(i1, kk reg, addrA, addrB func(st bool) string, loadOp, storeOp string) {
	a, b := k.rd(), k.rd()
	k.e(loadOp, a, addrA(false))
	k.e(loadOp, b, addrB(false))
	direction := k.r()
	k.e("and.b32", direction, i1, kk)
	ascending := k.p()
	k.e("setp.eq.u32", ascending, direction, 0)
	large, small := k.p(), k.p()
	k.e("setp.gt.u64", large, a, b)
	k.e("setp.lt.u64", small, a, b)
	change := k.p()
	k.e("selp.b32", direction, 1, 0, large)
	ky := k.r()
	k.e("selp.b32", ky, 1, 0, small)
	k.e("selp.b32", direction, direction, ky, ascending)
	k.e("setp.ne.u32", change, direction, 0)
	skip := k.label()
	k.jumpUnless(change, skip)
	k.e(storeOp, addrA(true), b)
	k.e(storeOp, addrB(true), a)
	k.put(skip)
}

// ciktiYaz sıralanmış satırın t. anahtarından indeksi (ve değerli ise değeri) yazar.
func (k *emitter) writeOutput(key, t, o, i, kCount, ic, base2 reg, x, idx, vals reg, valued bool) {
	d := k.r()
	k.e("cvt.u32.u64", d, key)
	target := k.r()
	k.e("mad.lo.s32", target, o, kCount, t)
	k.e("mad.lo.s32", target, target, ic, i)
	df := k.f()
	k.e("cvt.rn.f32.u32", df, d)
	k.store(tensor.Float32, k.address(idx, target, 4), df)
	if valued {
		j := k.r()
		k.e("mad.lo.s32", j, d, ic, base2)
		k.store(tensor.Float32, k.address(vals, target, 4), k.load(tensor.Float32, k.address(x, j, 4)))
	}
}

// siralaOrtakCekirdegi satır başına bir blokla paylaşımlı bellekte bitonik sıralama (P
// ikinin kuvveti, T iş parçacığı); ilk k anahtarın indeksini (ve değerini) yazar.
func sortSharedKernel(P int, descending, valued bool) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("sirala_ortak_%d_%v_%v", P, descending, valued))
		x := k.pointer("x")
		idx := k.written("idx")
		var vals reg
		if valued {
			vals = k.written("vals")
		}
		L := k.u32p("l")
		ic := k.u32p("ic")
		kCount := k.u32p("k")
		T := min(P/2, 512)
		if P == 1 {
			T = 1
		}
		sm := k.shared("sm", 8*P)
		row, tid := k.special("%ctaid.x"), k.special("%tid.x")
		o, i, base2 := k.rowBase(row, L, ic)
		address64 := func(j reg) reg {
			a := k.r()
			k.e("mad.lo.s32", a, j, 8, sm)
			return a
		}
		// t = tid, tid+T, ... < sinir döngüsü.
		loop := func(limit any, body func(t reg)) {
			t := k.r()
			k.e("mov.u32", t, tid)
			begin, done := k.label(), k.label()
			k.put(begin)
			q := k.p()
			k.e("setp.ge.u32", q, t, limit)
			k.jumpIf(q, done)
			body(t)
			k.e("add.s32", t, t, T)
			k.jump(begin)
			k.put(done)
		}
		loop(P, func(t reg) {
			k.e("st.shared.u64", mem(address64(t), 0), k.loadKey(x, t, L, ic, base2, descending))
		})
		k.barrier()
		for kk := 2; kk <= P; kk *= 2 {
			for j := kk / 2; j > 0; j /= 2 {
				lj := 0
				for 1<<lj < j {
					lj++
				}
				loop(P/2, func(t reg) {
					i1, i2 := k.bitonicPair(t, lj, j)
					a1, a2 := address64(i1), address64(i2)
					k.compareSwap(i1, k.whole(kk), func(bool) string { return mem(a1, 0) }, func(bool) string { return mem(a2, 0) },
						"ld.shared.u64", "st.shared.u64")
				})
				k.barrier()
			}
		}
		loop(kCount, func(t reg) {
			a := k.rd()
			k.e("ld.shared.u64", a, mem(address64(t), 0))
			k.writeOutput(a, t, o, i, kCount, ic, base2, x, idx, vals, valued)
		})
		return k
	}
}

// siralaBaslatCekirdegi genel bellekteki anahtar tamponunu (satır × P) doldurur: iş
// parçacığı başına bir eleman.
func sortInitKernel(descending bool) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("sirala_baslat_%v", descending))
		x := k.pointer("x")
		buf := k.written("anahtar")
		n := k.u32p("n")
		lp := k.u32p("lp") // log2(P)
		L := k.u32p("l")
		ic := k.u32p("ic")
		g := k.globalIndex()
		last := k.bounded(g, n)
		row := k.r()
		k.e("shr.u32", row, g, lp)
		P1 := k.r()
		k.e("shl.b32", P1, k.whole(1), lp)
		k.e("sub.s32", P1, P1, 1)
		t := k.r()
		k.e("and.b32", t, g, P1)
		_, _, base2 := k.rowBase(row, L, ic)
		k.e("st.global.u64", mem(k.address(buf, g, 8), 0), k.loadKey(x, t, L, ic, base2, descending))
		k.put(last)
		return k
	}
}

// siralaAdimCekirdegi genel bellekte bir bitonik adım (kk, j): iş parçacığı başına bir çift.
func sortStepKernel() *emitter {
	k := newEmitter("sirala_adim")
	buf := k.written("anahtar")
	n := k.u32p("n") // satır × P/2
	lp := k.u32p("lp")
	lj := k.u32p("lj")
	kk := k.u32p("kk")
	g := k.globalIndex()
	last := k.bounded(g, n)
	lp1 := k.r()
	k.e("sub.s32", lp1, lp, 1)
	row := k.r()
	k.e("shr.u32", row, g, lp1)
	m := k.r()
	k.e("shl.b32", m, k.whole(1), lp1)
	k.e("sub.s32", m, m, 1)
	t := k.r()
	k.e("and.b32", t, g, m)
	j := k.r()
	k.e("shl.b32", j, k.whole(1), lj)
	i1, i2 := k.bitonicPair(t, lj, j)
	sb := k.r()
	k.e("shl.b32", sb, row, lp)
	g1, g2 := k.r(), k.r()
	k.e("add.s32", g1, sb, i1)
	k.e("add.s32", g2, sb, i2)
	a1, a2 := k.address(buf, g1, 8), k.address(buf, g2, 8)
	k.compareSwap(i1, kk, func(bool) string { return mem(a1, 0) }, func(bool) string { return mem(a2, 0) },
		"ld.global.u64", "st.global.u64")
	k.put(last)
	return k
}

// siralaCiktiCekirdegi genel bellekte sıralanmış satırların ilk k anahtarını yazar.
func sortOutputKernel(valued bool) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("sirala_cikti_%v", valued))
		buf := k.pointer("anahtar")
		x := k.pointer("x")
		idx := k.written("idx")
		var vals reg
		if valued {
			vals = k.written("vals")
		}
		n := k.u32p("n") // satır × k
		lp := k.u32p("lp")
		L := k.u32p("l")
		ic := k.u32p("ic")
		kCount := k.u32p("k")
		g := k.globalIndex()
		last := k.bounded(g, n)
		row := k.r()
		k.e("div.u32", row, g, kCount)
		t := k.r()
		k.e("mul.lo.s32", t, row, kCount)
		k.e("sub.s32", t, g, t)
		o, i, base2 := k.rowBase(row, L, ic)
		ga := k.r()
		k.e("shl.b32", ga, row, lp)
		k.e("add.s32", ga, ga, t)
		a := k.rd()
		k.e("ld.global.u64", a, mem(k.address(buf, ga, 8), 0))
		k.writeOutput(a, t, o, i, kCount, ic, base2, x, idx, vals, valued)
		k.put(last)
		return k
	}
}

// toplaGeriCekirdegi gx[(o·L + idx)·iç + i] += g[(o·k + j)·iç + i] (iş parçacığı başına bir
// g elemanı; bir satırdaki indeksler tekil olduğundan yazmalar çakışmaz).
func sumBackwardKernel() *emitter {
	k := newEmitter("topla_geri")
	gx := k.written("gx")
	pg := k.pointer("g")
	pidx := k.pointer("idx")
	n := k.u32p("n")
	L := k.u32p("l")
	ic := k.u32p("ic")
	kCount := k.u32p("k")
	g := k.globalIndex()
	last := k.bounded(g, n)
	ki := k.r()
	k.e("mul.lo.s32", ki, kCount, ic)
	o := k.r()
	k.e("div.u32", o, g, ki)
	i := k.r()
	k.e("rem.u32", i, g, ic)
	d := k.r()
	k.e("cvt.rzi.s32.f32", d, k.load(tensor.Float32, k.address(pidx, g, 4)))
	h := k.r()
	k.e("mad.lo.s32", h, o, L, d)
	k.e("mad.lo.s32", h, h, ic, i)
	ha := k.address(gx, h, 4)
	old := k.load(tensor.Float32, ha)
	k.e("add.rn.f32", old, old, k.load(tensor.Float32, k.address(pg, g, 4)))
	k.store(tensor.Float32, ha, old)
	k.put(last)
	return k
}
