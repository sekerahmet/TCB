package gpu

import (
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Genel indirgeme çekirdeği ve blok içi indirgeme
// ============================================================================
//
// Tek bir üreteç bütün toplama indirgemelerini yürütür: eksen toplamı, hepsinin
// toplamı, yayınlamalı ikili işlemlerin gradyanları (g, g·o, g/o, g·y/o terimleri) ve
// adımlı dağıtma (görünümlerin geri yayılımı). Çıktının her elemanı (j, korunan eksenler)
// tek bir iş parçacığında ya da tek bir blokta, sabit sırayla toplanır: sonuç
// belirlenimcidir (atomik toplama yok).
//
// Meta düzeni: korunan eksenler (nk) için şekil, A adımı, O adımı, çıktı adımı; indirgenen
// eksenler (nr) için şekil, A adımı, O adımı. Y (gerekirse) A ile aynı adımlıdır.

// termType indirgenen terim.
type termType int

const (
	tA          termType = iota // a
	tAO                         // a·o
	tAbolO                      // a/o
	tAYbO                       // a·y/o
	tAsecO                      // o ≠ 0 (NaN dahil) ise a, değilse 0 (koşullu seçimin gradyanı)
	tAsecOdegil                 // o = 0 ise a, değilse 0
)

// reduceConfig indirgeme çekirdeğinin üretim ayarları.
type reduceConfig struct {
	nk, nr int
	term   termType
	add    bool // çıktı += ölçek·toplam (değilse çıktı = ölçek·toplam)
	block  int  // 0: çıktı başına iş parçacığı; > 0: çıktı başına bu boyda blok
	// chunked: indirgenen doğrusal aralık ızgaranın y'sindeki parçalara bölünür; her parça
	// kısmi toplamını (ölçeksiz) kismi[parça·K + j]'ye yazar (iki aşamalı indirgemenin
	// ilk aşaması).
	chunked bool
	// plusZero: toplam sonda +0 ile toplanır; yalnızca −0'lardan oluşan toplam +0 olur, başka
	// hiçbir değer değişmez (x + 0 = x). Birikim −0'dan başlar ki kopyalayan kullanımlar −0'ı
	// korusun (newCompensatedSum).
	plusZero bool
	// dt: A, O, Y ve çıktının türleri (kendi türlerinde okunup yazılır; toplam float32'dedir).
	// Kullanılmayan işlenenin türü Float32 kalır. Parçalı ilk aşamanın kısmi toplamları
	// float32'dir.
	dt [4]tensor.DType
}

func (a reduceConfig) key() string {
	return fmt.Sprintf("indirge:%d:%d:%d:%v:%d:%v:%v:%v", a.nk, a.nr, a.term, a.add, a.block, a.chunked, a.plusZero, a.dt)
}

// computeTerm A (ve gerekirse O, Y) ofsetlerinden terimi yükler; dt A, O, Y'nin türleri.
func (k *emitter) computeTerm(t termType, dt [4]tensor.DType, a, o, y reg, oa, oo reg) reg {
	at := func(p, ofs reg, d tensor.DType) reg { return k.load(d, k.addressS(p, ofs, dtypeSize(d))) }
	av := at(a, oa, dt[0])
	r := k.f()
	switch t {
	case tA:
		return av
	case tAO:
		ov := at(o, oo, dt[1])
		k.e("mul.rn.f32", r, av, ov)
	case tAbolO:
		ov := at(o, oo, dt[1])
		k.e("div.rn.f32", r, av, ov)
	case tAYbO:
		ov := at(o, oo, dt[1])
		yv := at(y, oa, dt[2])
		k.e("mul.rn.f32", r, av, yv)
		k.e("div.rn.f32", r, r, ov)
	case tAsecO, tAsecOdegil:
		ov := at(o, oo, dt[1])
		p := k.p()
		mode2 := map[termType]string{tAsecO: "neu", tAsecOdegil: "eq"}[t]
		k.e("setp."+mode2+".f32", p, ov, float32(0))
		k.e("selp.f32", r, av, float32(0), p)
	}
	return r
}

// parseBase ayristir'in meta'nın taban konumundan başlayan biçimi.
func (k *emitter) parseBase(i reg, nd, nOperand, base2 int, meta string) []reg {
	ofs := make([]reg, nOperand)
	for j := range ofs {
		ofs[j] = k.whole(0)
	}
	if nd == 0 {
		return ofs
	}
	rest := k.r()
	k.e("mov.u32", rest, i)
	for d := nd - 1; d >= 0; d-- {
		size := k.readArr(meta, base2+d)
		r := k.r()
		if d > 0 {
			q := k.r()
			k.e("div.u32", q, rest, size)
			k.e("mul.lo.s32", r, q, size)
			k.e("sub.s32", r, rest, r)
			k.e("mov.u32", rest, q)
		} else {
			k.e("mov.u32", r, rest)
		}
		for j := range ofs {
			a := k.readArr(meta, base2+(j+1)*nd+d)
			k.e("mad.lo.s32", ofs[j], r, a, ofs[j])
		}
	}
	return ofs
}

// blockReduce bloktaki iş parçacıklarının v değerlerini birlestir ile ağaç biçiminde
// (sabit sıra) indirger; sonuç bütün iş parçacıklarına döner. sm en az 4·B baytlık
// paylaşımlı bellek adresi, tid iş parçacığı numarasıdır. Sonda engel vardır: paylaşımlı
// bellek yeniden kullanılabilir.
func (k *emitter) blockReduce(v, sm, tid reg, B int, join func(a, b reg) reg) reg {
	addr := k.r()
	k.e("mad.lo.s32", addr, tid, 4, sm)
	k.e("st.shared.f32", mem(addr, 0), v)
	k.barrier()
	for s := B / 2; s > 0; s /= 2 {
		skip := k.label()
		p := k.p()
		k.e("setp.ge.u32", p, tid, s)
		k.jumpIf(p, skip)
		a, b := k.f(), k.f()
		k.e("ld.shared.f32", a, mem(addr, 0))
		k.e("ld.shared.f32", b, mem(addr, 4*s))
		k.e("st.shared.f32", mem(addr, 0), join(a, b))
		k.put(skip)
		k.barrier()
	}
	r := k.f()
	k.e("ld.shared.f32", r, mem(sm, 0))
	k.barrier()
	return r
}

// partRange parçalı indirgemede ızgaranın y'sindeki parça s'nin aralığı: r'ye başını
// ekler, sonunu (min(R, (s+1)·parça)) verir.
func (k *emitter) partRange(r, part, R reg) (s, last reg) {
	s = k.special("%ctaid.y")
	p0 := k.r()
	k.e("mul.lo.s32", p0, s, part)
	k.e("add.s32", r, r, p0)
	last = k.r()
	k.e("add.s32", last, p0, part)
	k.e("min.u32", last, last, R)
	return s, last
}

// sum birlestir işlevi: a + b.
func (k *emitter) sum(a, b reg) reg {
	r := k.f()
	k.e("add.rn.f32", r, a, b)
	return r
}

// compensatedSum bir iş parçacığının sıralı toplamıdır: Neumaier'in telafili toplaması
// (Kahan–Babuška). Her toplamanın yuvarlama hatası c'de ayrıca toplanır ve sonda eklenir;
// sıralı float32 toplamın hatası terim sayısıyla büyür, telafilininki büyümez (~2u·Σ|x|).
// Kahan'ınkinden farkı: terim toplamdan büyükken de doğru kalır; s'nin bağımlılık zinciri tek
// toplamadır. Toplamaların hepsi açık yuvarlamalıdır (.rn): PTX iyileştiricisi onları yeniden
// düzenlemez, telafi kaybolmaz.
type compensatedSum struct{ s, c reg }

// newCompensatedSum -0'dan başlar: -0 toplamanın etkisiz elemanıdır (-0 + x = x, -0 dahil);
// +0'dan başlasa tek terimli çıktılar (dağıtma, birleştirme) -0'ı +0'a çevirirdi. Gerçek
// toplamlar sonda +0 ekler (reduceConfig.plusZero).
func (k *emitter) newCompensatedSum() *compensatedSum {
	return &compensatedSum{s: k.constant(float32(math.Copysign(0, -1))), c: k.constant(0)}
}

// addCompensated t'yi toplama ekler: n = s + t; kaybolan kısım büyük ve küçük terimden
// (|s| ≥ |t| ise (s − n) + t, değilse (t − n) + s) tam olarak bulunur ve c'ye eklenir.
func (k *emitter) addCompensated(a *compensatedSum, t reg) {
	n := k.f()
	k.e("add.rn.f32", n, a.s, t)
	as, at := k.f(), k.f()
	k.e("abs.f32", as, a.s)
	k.e("abs.f32", at, t)
	p := k.p()
	k.e("setp.ge.f32", p, as, at)
	big, small := k.f(), k.f()
	k.e("selp.f32", big, a.s, t, p)
	k.e("selp.f32", small, t, a.s, p)
	d := k.f()
	k.e("sub.rn.f32", d, big, n)
	k.e("add.rn.f32", d, d, small)
	k.e("add.rn.f32", a.c, a.c, d)
	k.e("mov.f32", a.s, n)
}

// compensatedTotal toplamı verir. Telafi yalnızca toplam sonluysa ve telafi sıfır değilse
// eklenir: sonsuz bir terimde telafi NaN olur (∞ − ∞), sıfır telafiyi eklemek -0'ı +0'a
// çevirirdi.
func (k *emitter) compensatedTotal(a *compensatedSum) reg {
	as := k.f()
	k.e("abs.f32", as, a.s)
	use, nz := k.p(), k.p()
	k.e("setp.lt.f32", use, as, float32(math.Inf(1)))
	k.e("setp.ne.f32", nz, a.c, float32(0))
	k.e("and.pred", use, use, nz)
	sc := k.f()
	k.e("add.rn.f32", sc, a.s, a.c)
	r := k.f()
	k.e("selp.f32", r, sc, a.s, use)
	return r
}

func reduceKernel(a reduceConfig) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("indirge_%d_%d_%d_%v_%d_%v_%v_%s_%s_%s_%s", a.nk, a.nr, a.term, a.add, a.block, a.chunked,
			a.plusZero, a.dt[0], a.dt[1], a.dt[2], a.dt[3]))
		pa := k.pointer("a")
		po := k.pointer("o")
		py := k.pointer("y")
		out := k.written("cikti")
		K := k.u32p("k")
		R := k.u32p("r")
		scaleF := k.f32p("olcek")
		k.arr("meta", 4*a.nk+3*a.nr)
		var part, rEnd, s reg
		if a.chunked {
			part = k.u32p("parca")
		}
		last := k.label()
		var j, tid, step, begin reg
		var sm reg
		if a.block == 0 {
			j = k.globalIndex()
			p := k.p()
			k.e("setp.ge.u32", p, j, K)
			k.jumpIf(p, last)
			begin, step = k.whole(0), k.whole(1)
		} else {
			sm = k.shared("sm", 4*a.block)
			j = k.special("%ctaid.x")
			tid = k.special("%tid.x")
			begin, step = tid, k.whole(a.block)
		}
		kofs := k.parseBase(j, a.nk, 3, 0, "meta")
		sum := k.newCompensatedSum()
		r := k.r()
		k.e("mov.u32", r, begin)
		rEnd = R
		if a.chunked {
			s, rEnd = k.partRange(r, part, R)
		}
		k.whileLess("u32", r, rEnd, func() {
			rofs := k.parseBase(r, a.nr, 2, 4*a.nk, "meta")
			oa, oo := k.r(), k.r()
			k.e("add.s32", oa, kofs[0], rofs[0])
			k.e("add.s32", oo, kofs[1], rofs[1])
			t := k.computeTerm(a.term, a.dt, pa, po, py, oa, oo)
			k.addCompensated(sum, t)
			k.e("add.s32", r, r, step)
		})
		acc := k.compensatedTotal(sum)
		if a.block > 0 {
			acc = k.blockReduce(acc, sm, tid, a.block, k.sum)
			p0 := k.p()
			k.e("setp.ne.u32", p0, tid, 0)
			k.jumpIf(p0, last)
		}
		if a.chunked {
			o := k.r()
			k.e("mad.lo.s32", o, s, K, j)
			k.store(tensor.Float32, k.address(out, o, 4), acc)
			k.put(last)
			return k
		}
		k.storeReduced(a, k.addressS(out, kofs[2], dtypeSize(a.dt[3])), acc, scaleF)
		k.put(last)
		return k
	}
}

// storeReduced toplamı çıktıya yazar: gerçek toplamda +0 eklenir (plusZero), ölçekle çarpılır,
// ekleme kipinde çıktının eski değerine eklenir; çıktının türünde yuvarlanır.
func (k *emitter) storeReduced(a reduceConfig, ha, acc, scaleF reg) {
	ot := a.dt[3]
	if a.plusZero {
		k.e("add.rn.f32", acc, acc, float32(0))
	}
	k.e("mul.rn.f32", acc, acc, scaleF)
	if a.add {
		old := k.load(ot, ha)
		k.e("add.rn.f32", acc, old, acc)
	}
	k.store(ot, ha, acc)
}

// argmaxKernel her korunan çıktı için tek eksen boyunca ilk en büyük elemanı bulur
// (NaN her sayıdan büyük sayılır, ilk NaN kazanır: CPU argmaxCPU ile aynı kural). kip:
// 0 değer yazar, 1 indeksi (32 bitlik tam sayı) yazar, 2 gradyanı (gx[konum] += g[j])
// ekler. Çıktı başına bir iş parçacığı ekseni sırayla gezer.
// Meta: korunan eksenler için şekil, x adımı, gx adımı; ardından eksen boyu, x adımı,
// gx adımı.
func argmaxKernel(nk, mode2 int) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("argmax_%d_%d", nk, mode2))
		px := k.pointer("x")
		out := k.written("cikti")
		pg := k.pointer("g")
		K := k.u32p("k")
		k.arr("meta", 3*nk+3)
		j := k.globalIndex()
		last := k.bounded(j, K)
		kofs := k.parseBase(j, nk, 2, 0, "meta")
		L := k.readArr("meta", 3*nk)
		sx := k.readArr("meta", 3*nk+1)
		best := k.constant(float32(math.Inf(-1)))
		bi := k.whole(0)
		d := k.whole(0)
		ofs := k.r()
		k.e("mov.u32", ofs, kofs[0])
		k.whileLess("u32", d, L, func() {
			v := k.load(tensor.Float32, k.addressS(px, ofs, 4))
			large, vNan, bCount, pick := k.p(), k.p(), k.p(), k.p()
			k.e("setp.gt.f32", large, v, best)
			k.e("setp.nan.f32", vNan, v, v)
			k.e("setp.num.f32", bCount, best, best)
			k.e("and.pred", vNan, vNan, bCount)
			k.e("or.pred", pick, large, vNan)
			k.e("selp.f32", best, v, best, pick)
			k.e("selp.b32", bi, d, bi, pick)
			k.e("add.s32", ofs, ofs, sx)
			k.e("add.s32", d, d, 1)
		})
		switch mode2 {
		case 0:
			k.store(tensor.Float32, k.address(out, j, 4), best)
		case 1:
			k.e("st.global.u32", mem(k.address(out, j, 4), 0), bi)
		default:
			sg := k.readArr("meta", 3*nk+2)
			o := k.r()
			k.e("mad.lo.s32", o, bi, sg, kofs[1])
			ha := k.addressS(out, o, 4)
			gv := k.load(tensor.Float32, k.address(pg, j, 4))
			old := k.load(tensor.Float32, ha)
			k.e("add.rn.f32", old, old, gv)
			k.store(tensor.Float32, ha, old)
		}
		k.put(last)
		return k
	}
}
