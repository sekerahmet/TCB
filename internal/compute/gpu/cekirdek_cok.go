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
	cokParca  = 16384 // blok başına eleman
	cokTensor = 64    // tensör tablosunda kayıt boyu (bayt)
	cokBlok   = 256
	kareBlok  = cokBlok // kareler toplamının blok içi indirgeme boyu
)

// Çok tensörlü işin kipleri.
const (
	cokAdamW = iota
	cokSGD
	cokSGDMomentum
	cokOlcek
	cokKare
	// cokOlcekBellek ölçeği bellekteki bir float32'den okur (konak beklemeden kırpma); ölçek
	// 1 ise blok hiçbir şey yazmaz.
	cokOlcekBellek
)

var cokAdlari = [...]string{"cok_adamw", "cok_sgd", "cok_sgd_momentum", "cok_olcek", "cok_kare", "cok_olcek_bellek"}

// cokOrtakAdlari kiplerin ortak (bütün tensörler için aynı) sayı parametreleri.
var cokOrtakAdlari = [...][]string{
	cokAdamW:       {"b1", "b1c", "b2", "b2c", "eps", "cur"},
	cokSGD:         {"lr", "momentum", "wd"},
	cokSGDMomentum: {"lr", "momentum", "wd"},
	cokOlcek:       {"k"},
	cokKare:        nil,
	cokOlcekBellek: nil,
}

// cokCekirdegi kipe göre çok tensörlü çekirdeği üretir. Parametreler: tensör tablosu, blok
// tablosu, (kare kipinde) kısmi toplamlar ve kipin ortak sayıları.
func cokCekirdegi(kip int) func() *yazici {
	return func() *yazici {
		k := yeniYazici(cokAdlari[kip])
		tablo, bloklarT := k.gosterici("tablo"), k.gosterici("bloklar")
		var kismi yazmac
		switch kip {
		case cokKare:
			kismi = k.yazilan("kismi")
		case cokOlcekBellek:
			kismi = k.gosterici("olcek")
		}
		var s []yazmac
		for _, ad := range cokOrtakAdlari[kip] {
			s = append(s, k.f32p(ad))
		}
		bitti := k.etiket()
		if kip == cokOlcekBellek {
			s = append(s, k.yukle(tensor.Float32, kismi))
			bir := k.p()
			k.e("setp.eq.f32", bir, s[0], float32(1))
			k.gitEger(bir, bitti)
		}
		b := k.ozel("%ctaid.x")
		tid := k.ozel("%tid.x")
		ba := k.adres(bloklarT, b, 8)
		ti, bas := k.r(), k.r()
		k.e("ld.global.u32", ti, bel(ba, 0))
		k.e("ld.global.u32", bas, bel(ba, 4))
		ta := k.adres(tablo, ti, cokTensor)
		ptr := make([]yazmac, 4)
		for i := range ptr {
			ptr[i] = k.rd()
			k.e("ld.global.u64", ptr[i], bel(ta, 8*i))
			k.e("cvta.to.global.u64", ptr[i], ptr[i])
		}
		n := k.r()
		k.e("ld.global.u32", n, bel(ta, 32))
		f1, f2 := k.f(), k.f()
		k.e("ld.global.f32", f1, bel(ta, 36))
		k.e("ld.global.f32", f2, bel(ta, 40))
		son := k.r()
		k.e("add.s32", son, bas, cokParca)
		k.e("min.u32", son, son, n)
		acc := k.fd()
		k.e("mov.f64", acc, float64(0))
		i := k.r()
		k.e("add.s32", i, bas, tid)
		dongu := k.etiket()
		k.koy(dongu)
		p := k.p()
		k.e("setp.ge.u32", p, i, son)
		k.gitEger(p, bitti)
		a := func(j int) yazmac { return k.adres(ptr[j], i, 4) }
		switch kip {
		case cokAdamW:
			k.adamWAdim(a(0), a(1), a(2), a(3), f1, f2, s)
		case cokSGD, cokSGDMomentum:
			k.sgdAdim(a(0), a(1), a(2), s[0], s[1], s[2], kip == cokSGDMomentum)
		case cokOlcek, cokOlcekBellek:
			ad := a(0)
			v := k.yukle(tensor.Float32, ad)
			k.e("mul.rn.f32", v, v, s[0])
			k.sakla(tensor.Float32, ad, v)
		case cokKare:
			v := k.yukle(tensor.Float32, a(0))
			d := k.fd()
			k.e("cvt.f64.f32", d, v)
			k.e("fma.rn.f64", acc, d, d, acc)
		}
		k.e("add.s32", i, i, cokBlok)
		k.git(dongu)
		k.koy(bitti)
		if kip == cokKare {
			k.kareBlokYaz(acc, kismi, b, tid)
		}
		return k
	}
}

// adamWAdim tek elemanın AdamW adımı (CPU f32AdamWStep ile aynı sıra). s: b1, 1−b1, b2,
// 1−b2, eps, 1−lr·wd.
func (k *yazici) adamWAdim(pa, ga, ma, va, adimLR, bc2s yazmac, s []yazmac) {
	b1, b1c, b2, b2c, eps, cur := s[0], s[1], s[2], s[3], s[4], s[5]
	gi := k.yukle(tensor.Float32, ga)
	mv, vv, pv := k.yukle(tensor.Float32, ma), k.yukle(tensor.Float32, va), k.yukle(tensor.Float32, pa)
	t := k.f()
	k.e("mul.rn.f32", mv, b1, mv)
	k.e("mul.rn.f32", t, b1c, gi)
	k.e("add.rn.f32", mv, mv, t)
	k.e("mul.rn.f32", vv, b2, vv)
	k.e("mul.rn.f32", t, b2c, gi)
	k.e("mul.rn.f32", t, t, gi)
	k.e("add.rn.f32", vv, vv, t)
	pay, payda := k.f(), k.f()
	k.e("mul.rn.f32", pay, adimLR, mv)
	k.e("sqrt.rn.f32", payda, vv)
	k.e("div.rn.f32", payda, payda, bc2s)
	k.e("add.rn.f32", payda, payda, eps)
	k.e("div.rn.f32", pay, pay, payda)
	k.e("mul.rn.f32", pv, pv, cur)
	k.e("sub.rn.f32", pv, pv, pay)
	k.sakla(tensor.Float32, ma, mv)
	k.sakla(tensor.Float32, va, vv)
	k.sakla(tensor.Float32, pa, pv)
}

// sgdAdim tek elemanın SGD adımı (CPU f32SGDStep ile aynı sıra).
func (k *yazici) sgdAdim(pa, ga, va, lr, mom, wd yazmac, momentum bool) {
	pv := k.yukle(tensor.Float32, pa)
	gi := k.yukle(tensor.Float32, ga)
	t := k.f()
	k.e("mul.rn.f32", t, wd, pv)
	k.e("add.rn.f32", gi, gi, t)
	if momentum {
		vv := k.yukle(tensor.Float32, va)
		k.e("mul.rn.f32", vv, mom, vv)
		k.e("add.rn.f32", vv, vv, gi)
		k.sakla(tensor.Float32, va, vv)
		k.e("mov.f32", gi, vv)
	}
	k.e("mul.rn.f32", t, lr, gi)
	k.e("sub.rn.f32", pv, pv, t)
	k.sakla(tensor.Float32, pa, pv)
}

// kirpNormCekirdegi kareler toplamının blok kısmi toplamlarından (float64, kismi[0..n))
// normu ve kırpma ölçeğini hesaplar (tek blok): norm = √Σ, ölçek = norm > en_fazla ise
// en_fazla / norm, değilse 1. İş parçacıkları kısmi toplamları sabit adımla, sonra ağaçla
// toplar (belirlenimci). Konak beklemez: ölçeği cok_olcek_bellek okur.
func kirpNormCekirdegi() *yazici {
	k := yeniYazici("kirp_norm")
	kismi := k.gosterici("kismi")
	n := k.u32p("n")
	enFazla := k.f32p("en_fazla")
	norm, olcek := k.yazilan("norm"), k.yazilan("olcek")
	tid := k.ozel("%tid.x")
	acc := k.fd()
	k.e("mov.f64", acc, float64(0))
	i := k.r()
	k.e("mov.u32", i, tid)
	dongu, son := k.etiket(), k.etiket()
	k.koy(dongu)
	p := k.p()
	k.e("setp.ge.u32", p, i, n)
	k.gitEger(p, son)
	v := k.fd()
	k.e("ld.global.f64", v, bel(k.adres(kismi, i, 8), 0))
	k.e("add.f64", acc, acc, v)
	k.e("add.s32", i, i, kareBlok)
	k.git(dongu)
	k.koy(son)
	sm := k.ortak("sm", 8*kareBlok)
	adr := k.r()
	k.e("mad.lo.s32", adr, tid, 8, sm)
	k.e("st.shared.f64", bel(adr, 0), acc)
	k.engel()
	for s := kareBlok / 2; s > 0; s /= 2 {
		atla := k.etiket()
		q := k.p()
		k.e("setp.ge.u32", q, tid, s)
		k.gitEger(q, atla)
		a, c := k.fd(), k.fd()
		k.e("ld.shared.f64", a, bel(adr, 0))
		k.e("ld.shared.f64", c, bel(adr, 8*s))
		k.e("add.f64", a, a, c)
		k.e("st.shared.f64", bel(adr, 0), a)
		k.koy(atla)
		k.engel()
	}
	bitis := k.etiket()
	k.gitEger(k.tidSifirDegil(tid), bitis)
	t, nd, ef := k.fd(), k.fd(), k.fd()
	k.e("ld.shared.f64", t, bel(sm, 0))
	k.e("sqrt.rn.f64", nd, t)
	k.e("cvt.f64.f32", ef, enFazla)
	nf := k.f()
	k.e("cvt.rn.f32.f64", nf, nd)
	k.sakla(tensor.Float32, norm, nf)
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
	k.sakla(tensor.Float32, olcek, o)
	k.koy(bitis)
	return k
}

// kareBlokYaz bloğun float64 kısmi toplamlarını ağaç biçiminde indirger ve kismi[b]'ye yazar.
func (k *yazici) kareBlokYaz(acc, kismi, b, tid yazmac) {
	sm := k.ortak("sm", 8*kareBlok)
	adr := k.r()
	k.e("mad.lo.s32", adr, tid, 8, sm)
	k.e("st.shared.f64", bel(adr, 0), acc)
	k.engel()
	for s := kareBlok / 2; s > 0; s /= 2 {
		atla := k.etiket()
		q := k.p()
		k.e("setp.ge.u32", q, tid, s)
		k.gitEger(q, atla)
		a, c := k.fd(), k.fd()
		k.e("ld.shared.f64", a, bel(adr, 0))
		k.e("ld.shared.f64", c, bel(adr, 8*s))
		k.e("add.f64", a, a, c)
		k.e("st.shared.f64", bel(adr, 0), a)
		k.koy(atla)
		k.engel()
	}
	son := k.etiket()
	k.gitEger(k.tidSifirDegil(tid), son)
	r := k.fd()
	k.e("ld.shared.f64", r, bel(sm, 0))
	k.e("st.global.f64", bel(k.adres(kismi, b, 8), 0), r)
	k.koy(son)
}
