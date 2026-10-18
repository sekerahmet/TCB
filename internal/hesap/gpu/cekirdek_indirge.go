package gpu

import (
	"fmt"
	"math"

	"tr_dili/internal/hesap/tensor"
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

// terimTuru indirgenen terim.
type terimTuru int

const (
	tA     terimTuru = iota // a
	tAO                     // a·o
	tAbolO                  // a/o
	tAYbO                   // a·y/o
)

// indirgeAyar indirgeme çekirdeğinin üretim ayarları.
type indirgeAyar struct {
	nk, nr int
	terim  terimTuru
	ekle   bool // çıktı += ölçek·toplam (değilse çıktı = ölçek·toplam)
	blok   int  // 0: çıktı başına iş parçacığı; > 0: çıktı başına bu boyda blok
	// parcali: indirgenen doğrusal aralık ızgaranın y'sindeki parçalara bölünür; her parça
	// kısmi toplamını (ölçeksiz) kismi[parça·K + j]'ye yazar (iki aşamalı indirgemenin
	// ilk aşaması).
	parcali bool
}

func (a indirgeAyar) anahtar() string {
	return fmt.Sprintf("indirge:%d:%d:%d:%v:%d:%v", a.nk, a.nr, a.terim, a.ekle, a.blok, a.parcali)
}

// terimHesapla A (ve gerekirse O, Y) ofsetlerinden terimi yükler.
func (k *yazici) terimHesapla(t terimTuru, a, o, y yazmac, oa, oo yazmac) yazmac {
	av := k.yukle(tensor.Float32, k.adresS(a, oa, 4))
	r := k.f()
	switch t {
	case tA:
		return av
	case tAO:
		ov := k.yukle(tensor.Float32, k.adresS(o, oo, 4))
		k.e("mul.rn.f32", r, av, ov)
	case tAbolO:
		ov := k.yukle(tensor.Float32, k.adresS(o, oo, 4))
		k.e("div.rn.f32", r, av, ov)
	case tAYbO:
		ov := k.yukle(tensor.Float32, k.adresS(o, oo, 4))
		yv := k.yukle(tensor.Float32, k.adresS(y, oa, 4))
		k.e("mul.rn.f32", r, av, yv)
		k.e("div.rn.f32", r, r, ov)
	}
	return r
}

// ayristirTaban ayristir'in meta'nın taban konumundan başlayan biçimi.
func (k *yazici) ayristirTaban(i yazmac, nd, nIslenen, taban int, meta string) []yazmac {
	ofs := make([]yazmac, nIslenen)
	for j := range ofs {
		ofs[j] = k.tam(0)
	}
	if nd == 0 {
		return ofs
	}
	kalan := k.r()
	k.e("mov.u32", kalan, i)
	for d := nd - 1; d >= 0; d-- {
		boy := k.diziOku(meta, taban+d)
		r := k.r()
		if d > 0 {
			q := k.r()
			k.e("div.u32", q, kalan, boy)
			k.e("mul.lo.s32", r, q, boy)
			k.e("sub.s32", r, kalan, r)
			k.e("mov.u32", kalan, q)
		} else {
			k.e("mov.u32", r, kalan)
		}
		for j := range ofs {
			a := k.diziOku(meta, taban+(j+1)*nd+d)
			k.e("mad.lo.s32", ofs[j], r, a, ofs[j])
		}
	}
	return ofs
}

// blokIndirge bloktaki iş parçacıklarının v değerlerini birlestir ile ağaç biçiminde
// (sabit sıra) indirger; sonuç bütün iş parçacıklarına döner. sm en az 4·B baytlık
// paylaşımlı bellek adresi, tid iş parçacığı numarasıdır. Sonda engel vardır: paylaşımlı
// bellek yeniden kullanılabilir.
func (k *yazici) blokIndirge(v, sm, tid yazmac, B int, birlestir func(a, b yazmac) yazmac) yazmac {
	adr := k.r()
	k.e("mad.lo.s32", adr, tid, 4, sm)
	k.e("st.shared.f32", bel(adr, 0), v)
	k.engel()
	for s := B / 2; s > 0; s /= 2 {
		atla := k.etiket()
		p := k.p()
		k.e("setp.ge.u32", p, tid, s)
		k.gitEger(p, atla)
		a, b := k.f(), k.f()
		k.e("ld.shared.f32", a, bel(adr, 0))
		k.e("ld.shared.f32", b, bel(adr, 4*s))
		k.e("st.shared.f32", bel(adr, 0), birlestir(a, b))
		k.koy(atla)
		k.engel()
	}
	r := k.f()
	k.e("ld.shared.f32", r, bel(sm, 0))
	k.engel()
	return r
}

// topla birlestir işlevi: a + b.
func (k *yazici) topla(a, b yazmac) yazmac {
	r := k.f()
	k.e("add.rn.f32", r, a, b)
	return r
}

func indirgeCekirdegi(a indirgeAyar) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("indirge_%d_%d_%d_%v_%d_%v", a.nk, a.nr, a.terim, a.ekle, a.blok, a.parcali))
		pa := k.gosterici("a")
		po := k.gosterici("o")
		py := k.gosterici("y")
		out := k.yazilan("cikti")
		K := k.u32p("k")
		R := k.u32p("r")
		olcek := k.f32p("olcek")
		k.dizi("meta", 4*a.nk+3*a.nr)
		var parca, rSon, s yazmac
		if a.parcali {
			parca = k.u32p("parca")
		}
		son := k.etiket()
		var j, tid, adim, bas yazmac
		var sm yazmac
		if a.blok == 0 {
			j = k.genelIndeks()
			p := k.p()
			k.e("setp.ge.u32", p, j, K)
			k.gitEger(p, son)
			bas, adim = k.tam(0), k.tam(1)
		} else {
			sm = k.ortak("sm", 4*a.blok)
			j = k.ozel("%ctaid.x")
			tid = k.ozel("%tid.x")
			bas, adim = tid, k.tam(a.blok)
		}
		kofs := k.ayristirTaban(j, a.nk, 3, 0, "meta")
		acc := k.sabit(0)
		r := k.r()
		k.e("mov.u32", r, bas)
		rSon = R
		if a.parcali {
			s = k.ozel("%ctaid.y")
			p0 := k.r()
			k.e("mul.lo.s32", p0, s, parca)
			k.e("add.s32", r, r, p0)
			rSon = k.r()
			k.e("add.s32", rSon, p0, parca)
			k.e("min.u32", rSon, rSon, R)
		}
		dongu, bitti := k.etiket(), k.etiket()
		k.koy(dongu)
		p := k.p()
		k.e("setp.ge.u32", p, r, rSon)
		k.gitEger(p, bitti)
		rofs := k.ayristirTaban(r, a.nr, 2, 4*a.nk, "meta")
		oa, oo := k.r(), k.r()
		k.e("add.s32", oa, kofs[0], rofs[0])
		k.e("add.s32", oo, kofs[1], rofs[1])
		t := k.terimHesapla(a.terim, pa, po, py, oa, oo)
		k.e("add.rn.f32", acc, acc, t)
		k.e("add.s32", r, r, adim)
		k.git(dongu)
		k.koy(bitti)
		if a.blok > 0 {
			acc = k.blokIndirge(acc, sm, tid, a.blok, k.topla)
			p0 := k.p()
			k.e("setp.ne.u32", p0, tid, 0)
			k.gitEger(p0, son)
		}
		if a.parcali {
			o := k.r()
			k.e("mad.lo.s32", o, s, K, j)
			k.sakla(tensor.Float32, k.adres(out, o, 4), acc)
			k.koy(son)
			return k
		}
		ha := k.adresS(out, kofs[2], 4)
		k.e("mul.rn.f32", acc, acc, olcek)
		if a.ekle {
			eski := k.yukle(tensor.Float32, ha)
			k.e("add.rn.f32", acc, eski, acc)
		}
		k.sakla(tensor.Float32, ha, acc)
		k.koy(son)
		return k
	}
}

// argmaxCekirdegi her korunan çıktı için tek eksen boyunca ilk en büyük elemanı bulur
// (NaN her sayıdan büyük sayılır, ilk NaN kazanır: CPU argmaxCPU ile aynı kural). kip:
// 0 değer yazar, 1 indeksi (32 bitlik tam sayı) yazar, 2 gradyanı (gx[konum] += g[j])
// ekler. Çıktı başına bir iş parçacığı ekseni sırayla gezer.
// Meta: korunan eksenler için şekil, x adımı, gx adımı; ardından eksen boyu, x adımı,
// gx adımı.
func argmaxCekirdegi(nk, kip int) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("argmax_%d_%d", nk, kip))
		px := k.gosterici("x")
		out := k.yazilan("cikti")
		pg := k.gosterici("g")
		K := k.u32p("k")
		k.dizi("meta", 3*nk+3)
		j := k.genelIndeks()
		son := k.sinirli(j, K)
		kofs := k.ayristirTaban(j, nk, 2, 0, "meta")
		L := k.diziOku("meta", 3*nk)
		sx := k.diziOku("meta", 3*nk+1)
		best := k.sabit(float32(math.Inf(-1)))
		bi := k.tam(0)
		d := k.tam(0)
		ofs := k.r()
		k.e("mov.u32", ofs, kofs[0])
		dongu, bitti := k.etiket(), k.etiket()
		k.koy(dongu)
		p := k.p()
		k.e("setp.ge.u32", p, d, L)
		k.gitEger(p, bitti)
		v := k.yukle(tensor.Float32, k.adresS(px, ofs, 4))
		buyuk, vNan, bSay, sec := k.p(), k.p(), k.p(), k.p()
		k.e("setp.gt.f32", buyuk, v, best)
		k.e("setp.nan.f32", vNan, v, v)
		k.e("setp.num.f32", bSay, best, best)
		k.e("and.pred", vNan, vNan, bSay)
		k.e("or.pred", sec, buyuk, vNan)
		k.e("selp.f32", best, v, best, sec)
		k.e("selp.b32", bi, d, bi, sec)
		k.e("add.s32", ofs, ofs, sx)
		k.e("add.s32", d, d, 1)
		k.git(dongu)
		k.koy(bitti)
		switch kip {
		case 0:
			k.sakla(tensor.Float32, k.adres(out, j, 4), best)
		case 1:
			k.e("st.global.u32", bel(k.adres(out, j, 4), 0), bi)
		default:
			sg := k.diziOku("meta", 3*nk+2)
			o := k.r()
			k.e("mad.lo.s32", o, bi, sg, kofs[1])
			ha := k.adresS(out, o, 4)
			gv := k.yukle(tensor.Float32, k.adres(pg, j, 4))
			eski := k.yukle(tensor.Float32, ha)
			k.e("add.rn.f32", eski, eski, gv)
			k.sakla(tensor.Float32, ha, eski)
		}
		k.koy(son)
		return k
	}
}
