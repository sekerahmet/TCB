package gpu

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Çok tensörlü eleman işleri: AdamW, SGD, ölçekleme, kareler toplamı (tek başlatma)
// ============================================================================
//
// Bütün parametreler tek başlatmada işlenir. Konakta iki tablo kurulur ve cihaza bir kez
// yüklenir: tensör tablosu (her tensör için 64 bayt: dört gösterici, eleman sayısı ve iki
// tensöre özgü sayı) ve blok tablosu (her blok için tensör sırası ve parçanın başı). Her blok
// bir tensörün cokParca elemanlık bir parçasını işler. Formüller tek tensörlü çekirdeklerle
// aynıdır (aynı yardımcılar: adamWAdim, sgdAdim); kareler toplamında her bloğun kısmi
// toplamı ayrı yazılır ve konakta tensör sırasıyla toplanır (belirlenimci: blok bölüşümü
// yalnızca n'lere bağlı).

const (
	multiPart   = 16384 // blok başına eleman
	multiTensor = 64    // tensör tablosunda kayıt boyu (bayt)
	multiBlock  = 256
	squareBlock = multiBlock // kareler toplamının blok içi indirgeme boyu
)

// Çok tensörlü işin kipleri.
const (
	multiAdamW = iota
	multiSGD
	multiSGDMomentum
	multiScale
	multiSquare
	// multiScaleMem ölçeği bellekteki bir float32'den okur (konak beklemeden kırpma); ölçek
	// 1 ise blok hiçbir şey yazmaz.
	multiScaleMem
)

var multiNames = [...]string{"cok_adamw", "cok_sgd", "cok_sgd_momentum", "cok_olcek", "cok_kare", "cok_olcek_bellek"}

// multiSharedNames kiplerin ortak (bütün tensörler için aynı) sayı parametreleri.
var multiSharedNames = [...][]string{
	multiAdamW:       {"b1", "b1c", "b2", "b2c", "eps", "cur"},
	multiSGD:         {"lr", "momentum", "wd"},
	multiSGDMomentum: {"lr", "momentum", "wd"},
	multiScale:       {"k"},
	multiSquare:      nil,
	multiScaleMem:    nil,
}

// multiKernel kipe göre çok tensörlü çekirdeği üretir. Parametreler: tensör tablosu, blok
// tablosu, (kare kipinde) kısmi toplamlar ve kipin ortak sayıları.
func multiKernel(mode2 int) func() *emitter {
	return func() *emitter {
		k := newEmitter(multiNames[mode2])
		table, blocksT := k.pointer("tablo"), k.pointer("bloklar")
		var partial reg
		switch mode2 {
		case multiSquare:
			partial = k.written("kismi")
		case multiScaleMem:
			partial = k.pointer("olcek")
		}
		var s []reg
		for _, ad := range multiSharedNames[mode2] {
			s = append(s, k.f32p(ad))
		}
		done := k.label()
		if mode2 == multiScaleMem {
			s = append(s, k.load(tensor.Float32, partial))
			one := k.p()
			k.e("setp.eq.f32", one, s[0], float32(1))
			k.jumpIf(one, done)
		}
		b := k.special("%ctaid.x")
		tid := k.special("%tid.x")
		ba := k.address(blocksT, b, 8)
		ti, begin := k.r(), k.r()
		k.e("ld.global.u32", ti, mem(ba, 0))
		k.e("ld.global.u32", begin, mem(ba, 4))
		ta := k.address(table, ti, multiTensor)
		ptr := make([]reg, 4)
		for i := range ptr {
			ptr[i] = k.rd()
			k.e("ld.global.u64", ptr[i], mem(ta, 8*i))
			k.e("cvta.to.global.u64", ptr[i], ptr[i])
		}
		n := k.r()
		k.e("ld.global.u32", n, mem(ta, 32))
		f1, f2 := k.f(), k.f()
		k.e("ld.global.f32", f1, mem(ta, 36))
		k.e("ld.global.f32", f2, mem(ta, 40))
		last := k.r()
		k.e("add.s32", last, begin, multiPart)
		k.e("min.u32", last, last, n)
		acc := k.fd()
		k.e("mov.f64", acc, float64(0))
		i := k.r()
		k.e("add.s32", i, begin, tid)
		loop := k.label()
		k.put(loop)
		p := k.p()
		k.e("setp.ge.u32", p, i, last)
		k.jumpIf(p, done)
		a := func(j int) reg { return k.address(ptr[j], i, 4) }
		switch mode2 {
		case multiAdamW:
			k.adamWStep(a(0), a(1), a(2), a(3), f1, f2, s)
		case multiSGD, multiSGDMomentum:
			k.sgdStep(a(0), a(1), a(2), s[0], s[1], s[2], mode2 == multiSGDMomentum)
		case multiScale, multiScaleMem:
			ad := a(0)
			v := k.load(tensor.Float32, ad)
			k.e("mul.rn.f32", v, v, s[0])
			k.store(tensor.Float32, ad, v)
		case multiSquare:
			v := k.load(tensor.Float32, a(0))
			d := k.fd()
			k.e("cvt.f64.f32", d, v)
			k.e("fma.rn.f64", acc, d, d, acc)
		}
		k.e("add.s32", i, i, multiBlock)
		k.jump(loop)
		k.put(done)
		if mode2 == multiSquare {
			k.writeSquareBlock(acc, partial, b, tid)
		}
		return k
	}
}

// adamWStep tek elemanın AdamW adımı (CPU f32AdamWStep ile aynı sıra). s: b1, 1−b1, b2,
// 1−b2, eps, 1−lr·wd.
func (k *emitter) adamWStep(pa, ga, ma, va, stepLR, bc2s reg, s []reg) {
	b1, b1c, b2, b2c, eps, cur := s[0], s[1], s[2], s[3], s[4], s[5]
	gi := k.load(tensor.Float32, ga)
	mv, vv, pv := k.load(tensor.Float32, ma), k.load(tensor.Float32, va), k.load(tensor.Float32, pa)
	t := k.f()
	k.e("mul.rn.f32", mv, b1, mv)
	k.e("mul.rn.f32", t, b1c, gi)
	k.e("add.rn.f32", mv, mv, t)
	k.e("mul.rn.f32", vv, b2, vv)
	k.e("mul.rn.f32", t, b2c, gi)
	k.e("mul.rn.f32", t, t, gi)
	k.e("add.rn.f32", vv, vv, t)
	share, denom := k.f(), k.f()
	k.e("mul.rn.f32", share, stepLR, mv)
	k.e("sqrt.rn.f32", denom, vv)
	k.e("div.rn.f32", denom, denom, bc2s)
	k.e("add.rn.f32", denom, denom, eps)
	k.e("div.rn.f32", share, share, denom)
	k.e("mul.rn.f32", pv, pv, cur)
	k.e("sub.rn.f32", pv, pv, share)
	k.store(tensor.Float32, ma, mv)
	k.store(tensor.Float32, va, vv)
	k.store(tensor.Float32, pa, pv)
}

// sgdStep tek elemanın SGD adımı (CPU f32SGDStep ile aynı sıra).
func (k *emitter) sgdStep(pa, ga, va, lr, mom, wd reg, momentum bool) {
	pv := k.load(tensor.Float32, pa)
	gi := k.load(tensor.Float32, ga)
	t := k.f()
	k.e("mul.rn.f32", t, wd, pv)
	k.e("add.rn.f32", gi, gi, t)
	if momentum {
		vv := k.load(tensor.Float32, va)
		k.e("mul.rn.f32", vv, mom, vv)
		k.e("add.rn.f32", vv, vv, gi)
		k.store(tensor.Float32, va, vv)
		k.e("mov.f32", gi, vv)
	}
	k.e("mul.rn.f32", t, lr, gi)
	k.e("sub.rn.f32", pv, pv, t)
	k.store(tensor.Float32, pa, pv)
}

// clipNormKernel kareler toplamının blok kısmi toplamlarından (float64, kismi[0..n))
// normu ve kırpma ölçeğini hesaplar (tek blok): norm = √Σ, ölçek = norm > en_fazla ise
// en_fazla / norm, değilse 1. İş parçacıkları kısmi toplamları sabit adımla, sonra ağaçla
// toplar (belirlenimci). Konak beklemez: ölçeği cok_olcek_bellek okur.
func clipNormKernel() *emitter {
	k := newEmitter("kirp_norm")
	partial := k.pointer("kismi")
	n := k.u32p("n")
	most := k.f32p("en_fazla")
	norm, scaleF := k.written("norm"), k.written("olcek")
	tid := k.special("%tid.x")
	acc := k.fd()
	k.e("mov.f64", acc, float64(0))
	i := k.r()
	k.e("mov.u32", i, tid)
	loop, last := k.label(), k.label()
	k.put(loop)
	p := k.p()
	k.e("setp.ge.u32", p, i, n)
	k.jumpIf(p, last)
	v := k.fd()
	k.e("ld.global.f64", v, mem(k.address(partial, i, 8), 0))
	k.e("add.f64", acc, acc, v)
	k.e("add.s32", i, i, squareBlock)
	k.jump(loop)
	k.put(last)
	sm := k.shared("sm", 8*squareBlock)
	addr := k.r()
	k.e("mad.lo.s32", addr, tid, 8, sm)
	k.e("st.shared.f64", mem(addr, 0), acc)
	k.barrier()
	for s := squareBlock / 2; s > 0; s /= 2 {
		skip := k.label()
		q := k.p()
		k.e("setp.ge.u32", q, tid, s)
		k.jumpIf(q, skip)
		a, c := k.fd(), k.fd()
		k.e("ld.shared.f64", a, mem(addr, 0))
		k.e("ld.shared.f64", c, mem(addr, 8*s))
		k.e("add.f64", a, a, c)
		k.e("st.shared.f64", mem(addr, 0), a)
		k.put(skip)
		k.barrier()
	}
	end := k.label()
	k.jumpIf(k.tidNonZero(tid), end)
	t, nd, ef := k.fd(), k.fd(), k.fd()
	k.e("ld.shared.f64", t, mem(sm, 0))
	k.e("sqrt.rn.f64", nd, t)
	k.e("cvt.f64.f32", ef, most)
	nf := k.f()
	k.e("cvt.rn.f32.f64", nf, nd)
	k.store(tensor.Float32, norm, nf)
	// ölçek = (norm > en_fazla && norm > 0) ? en_fazla / norm : 1 (NaN norm: 1, kırpılmaz;
	// CPU yolundaki karşılaştırmayla aynı).
	o := k.f()
	k.e("mov.f32", o, float32(1))
	b1, b2 := k.p(), k.p()
	k.e("setp.gt.f64", b1, nd, ef)
	k.e("setp.gt.f64", b2, nd, float64(0))
	k.e("and.pred", b1, b1, b2)
	od := k.fd()
	k.e("div.rn.f64", od, ef, nd)
	of := k.f()
	k.e("cvt.rn.f32.f64", of, od)
	k.e("selp.f32", o, of, o, b1)
	k.store(tensor.Float32, scaleF, o)
	k.put(end)
	return k
}

// writeSquareBlock bloğun float64 kısmi toplamlarını ağaç biçiminde indirger ve kismi[b]'ye yazar.
func (k *emitter) writeSquareBlock(acc, partial, b, tid reg) {
	sm := k.shared("sm", 8*squareBlock)
	addr := k.r()
	k.e("mad.lo.s32", addr, tid, 8, sm)
	k.e("st.shared.f64", mem(addr, 0), acc)
	k.barrier()
	for s := squareBlock / 2; s > 0; s /= 2 {
		skip := k.label()
		q := k.p()
		k.e("setp.ge.u32", q, tid, s)
		k.jumpIf(q, skip)
		a, c := k.fd(), k.fd()
		k.e("ld.shared.f64", a, mem(addr, 0))
		k.e("ld.shared.f64", c, mem(addr, 8*s))
		k.e("add.f64", a, a, c)
		k.e("st.shared.f64", mem(addr, 0), a)
		k.put(skip)
		k.barrier()
	}
	last := k.label()
	k.jumpIf(k.tidNonZero(tid), last)
	r := k.fd()
	k.e("ld.shared.f64", r, mem(sm, 0))
	k.e("st.global.f64", mem(k.address(partial, b, 8), 0), r)
	k.put(last)
}
