package gpu

import (
	"fmt"
	"math"

	"tr_dili/internal/hesap/tensor"
)

// ============================================================================
// Birleşik çapraz entropi: tek geçişli logsumexp ve doğrudan gradyan
// ============================================================================
//
// İleri: satır başına bir blok; her iş parçacığı satırı adımla gezerken çevrimiçi
// logsumexp tutar (m en büyük, s = Σ e^{x−m}; yeni en büyükte s ölçeklenir), çiftler blok
// içinde sabit sırayla birleştirilir. Logitler bir kez okunur; softmax ara tensörü yoktur.
// Geri: gx = k·e^{x−lse} − k·[c = hedef] (k = g/geçerli) doğrudan yazılır (üzerine ya da
// eskisine eklenerek). vek doğruysa satır 4'lü vektörlerle okunur/yazılır (C % 4 = 0 ve
// hizalı görünüm).

const ceBlok = 256

// ceGez satırın elemanlarını (vek ise 4'lülerle) iş parçacığı adımıyla gezer; govde her
// elemanın değerini ve sütununu alır. adr satır tabanının bayt adresi (x için), ofs
// işlevi diğer göstericiler için aynı bayt ofsetini verir.
func (k *yazici) ceGez(tid, C yazmac, vek bool, govde func(ofs yazmac, c yazmac, n int)) {
	adim, n := ceBlok, 1
	if vek {
		adim, n = 4*ceBlok, 4
	}
	c := k.r()
	k.e("mul.lo.s32", c, tid, n)
	bas, son := k.etiket(), k.etiket()
	k.koy(bas)
	p := k.p()
	k.e("setp.ge.u32", p, c, C)
	k.gitEger(p, son)
	ofs := k.rd()
	k.e("mul.wide.u32", ofs, c, 4)
	govde(ofs, c, n)
	k.e("add.s32", c, c, adim)
	k.git(bas)
	k.koy(son)
}

// ceDegerler adresteki 1 ya da 4 float32.
func (k *yazici) ceDegerler(adr yazmac, n int) []yazmac {
	if n == 1 {
		return []yazmac{k.yukle(tensor.Float32, adr)}
	}
	return k.yukleV("global", "", adr, 0, 4)
}

// cevrimiciEkle (m, s) çiftine v'yi ekler: v > m ise s = s·e^{m−v} + 1, m = v; değilse
// s += e^{v−m}. v = −∞ katkı vermez (m = −∞ iken −∞ − (−∞) = NaN olurdu; CPU'da e^{−∞} = 0).
// NaN, s'yi NaN yapar (CPU ile aynı).
func (k *yazici) cevrimiciEkle(m, s, v yazmac) {
	buyuk := k.p()
	k.e("setp.gt.f32", buyuk, v, m)
	d1, d2 := k.f(), k.f()
	k.e("sub.rn.f32", d1, m, v)
	k.e("sub.rn.f32", d2, v, m)
	k.e("selp.f32", d1, d1, d2, buyuk)
	eksiSonsuz := k.p()
	k.e("setp.eq.f32", eksiSonsuz, v, float32(math.Inf(-1)))
	k.e("selp.f32", d1, float32(math.Inf(-1)), d1, eksiSonsuz)
	e := k.ustel(d1)
	s1 := k.f()
	k.e("fma.rn.f32", s1, s, e, float32(1))
	k.e("add.rn.f32", s, s, e)
	k.e("selp.f32", s, s1, s, buyuk)
	k.e("selp.f32", m, v, m, buyuk)
}

// ciftBirlestir iki (m, s) çiftinin birleşimi; s = 0 olan (boş) çift katkı vermez.
func (k *yazici) ciftBirlestir(m1, s1, m2, s2 yazmac) (yazmac, yazmac) {
	m := k.f()
	k.e("max.f32", m, m1, m2)
	terim := func(mi, si yazmac) yazmac {
		d := k.f()
		k.e("sub.rn.f32", d, mi, m)
		t := k.f()
		k.e("mul.rn.f32", t, si, k.ustel(d))
		bos := k.p()
		k.e("setp.eq.f32", bos, si, float32(0))
		k.e("selp.f32", t, float32(0), t, bos)
		return t
	}
	s := k.f()
	k.e("add.rn.f32", s, terim(m1, s1), terim(m2, s2))
	return m, s
}

// ciftIndirge blokta (m, s) çiftlerini ağaç biçiminde birleştirir; sonuç bütün iş
// parçacıklarına döner. smM, smS en az 4·B baytlık paylaşımlı bellek.
func (k *yazici) ciftIndirge(m, s, smM, smS, tid yazmac, B int) (yazmac, yazmac) {
	am, as := k.r(), k.r()
	k.e("mad.lo.s32", am, tid, 4, smM)
	k.e("mad.lo.s32", as, tid, 4, smS)
	k.e("st.shared.f32", bel(am, 0), m)
	k.e("st.shared.f32", bel(as, 0), s)
	k.engel()
	for adim := B / 2; adim > 0; adim /= 2 {
		atla := k.etiket()
		p := k.p()
		k.e("setp.ge.u32", p, tid, adim)
		k.gitEger(p, atla)
		m1, s1, m2, s2 := k.f(), k.f(), k.f(), k.f()
		k.e("ld.shared.f32", m1, bel(am, 0))
		k.e("ld.shared.f32", s1, bel(as, 0))
		k.e("ld.shared.f32", m2, bel(am, 4*adim))
		k.e("ld.shared.f32", s2, bel(as, 4*adim))
		mm, ss := k.ciftBirlestir(m1, s1, m2, s2)
		k.e("st.shared.f32", bel(am, 0), mm)
		k.e("st.shared.f32", bel(as, 0), ss)
		k.koy(atla)
		k.engel()
	}
	rm, rs := k.f(), k.f()
	k.e("ld.shared.f32", rm, bel(smM, 0))
	k.e("ld.shared.f32", rs, bel(smS, 0))
	k.engel()
	return rm, rs
}

// ceIleriCekirdegi satır başına lse = m + log s ve kayıp = lse − x[hedef]; hedefi negatif
// satırlarda ikisi de 0.
func ceIleriCekirdegi(vek bool) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("ce_ileri_%v", vek))
		x := k.gosterici("x")
		hedef := k.gosterici("hedef")
		lse := k.yazilan("lse")
		kayip := k.yazilan("kayip")
		C := k.u32p("c")
		smM, smS := k.ortak("sm_m", 4*ceBlok), k.ortak("sm_s", 4*ceBlok)
		tid := k.ozel("%tid.x")
		r := k.ozel("%ctaid.x")
		taban := k.rd()
		k.e("mul.wide.u32", taban, r, C)
		k.e("shl.b64", taban, taban, 2)
		k.e("add.s64", taban, taban, x)
		t := k.r()
		k.e("ld.global.s32", t, bel(k.adres(hedef, r, 4), 0))
		son, gecerli := k.etiket(), k.etiket()
		p := k.p()
		k.e("setp.ge.s32", p, t, 0)
		k.gitEger(p, gecerli)
		k.gitEger(k.tidSifirDegil(tid), son)
		sifir := k.sabit(0)
		k.sakla(tensor.Float32, k.adres(lse, r, 4), sifir)
		k.sakla(tensor.Float32, k.adres(kayip, r, 4), sifir)
		k.git(son)
		k.koy(gecerli)
		m, s := k.sabit(float32(math.Inf(-1))), k.sabit(0)
		k.ceGez(tid, C, vek, func(ofs, _ yazmac, n int) {
			for _, v := range k.ceDegerler(k.satirAdresi(taban, ofs), n) {
				k.cevrimiciEkle(m, s, v)
			}
		})
		m, s = k.ciftIndirge(m, s, smM, smS, tid, ceBlok)
		k.gitEger(k.tidSifirDegil(tid), son)
		l := k.logaritma(s)
		k.e("add.rn.f32", l, l, m)
		k.sakla(tensor.Float32, k.adres(lse, r, 4), l)
		xv := k.yukle(tensor.Float32, k.adresS(taban, t, 4))
		kv := k.f()
		k.e("sub.rn.f32", kv, l, xv)
		k.sakla(tensor.Float32, k.adres(kayip, r, 4), kv)
		k.koy(son)
		return k
	}
}

// ceGeriCekirdegi gx[r, c] (+)= k·e^{x−lse} − (c = hedef ? k : 0), k = g·ters; satır başına
// blok. uzerine doğruysa eski içerik okunmaz ve hedefi negatif satırlar 0 yazılır.
func ceGeriCekirdegi(vek, uzerine bool) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("ce_geri_%v_%v", vek, uzerine))
		gx := k.yazilan("gx")
		g := k.gosterici("g")
		x := k.gosterici("x")
		lse := k.gosterici("lse")
		hedef := k.gosterici("hedef")
		C := k.u32p("c")
		ters := k.f32p("ters")
		r := k.ozel("%ctaid.x")
		tid := k.ozel("%tid.x")
		satir := k.rd()
		k.e("mul.wide.u32", satir, r, C)
		k.e("shl.b64", satir, satir, 2)
		xt, gt := k.rd(), k.rd()
		k.e("add.s64", xt, satir, x)
		k.e("add.s64", gt, satir, gx)
		t := k.r()
		k.e("ld.global.s32", t, bel(k.adres(hedef, r, 4), 0))
		son := k.etiket()
		gec := k.p()
		k.e("setp.ge.s32", gec, t, 0)
		if !uzerine {
			k.gitDegilse(gec, son)
		}
		kat := k.yukle(tensor.Float32, g)
		k.e("mul.rn.f32", kat, kat, ters)
		lv := k.yukle(tensor.Float32, k.adres(lse, r, 4))
		k.ceGez(tid, C, vek, func(ofs, c yazmac, n int) {
			xs := k.ceDegerler(k.satirAdresi(xt, ofs), n)
			ha := k.satirAdresi(gt, ofs)
			var eski []yazmac
			if !uzerine {
				eski = k.ceDegerler(ha, n)
			}
			out := make([]yazmac, n)
			for i, xv := range xs {
				d := k.f()
				k.e("sub.rn.f32", d, xv, lv)
				v := k.f()
				k.e("mul.rn.f32", v, kat, k.ustel(d))
				ci := k.r()
				k.e("add.s32", ci, c, i)
				esit := k.p()
				k.e("setp.eq.s32", esit, ci, t)
				vk := k.f()
				k.e("sub.rn.f32", vk, v, kat)
				k.e("selp.f32", v, vk, v, esit)
				if uzerine {
					k.e("selp.f32", v, v, float32(0), gec) // yok sayılan satır: 0
				} else {
					k.e("add.rn.f32", v, eski[i], v)
				}
				out[i] = v
			}
			if n == 1 {
				k.sakla(tensor.Float32, ha, out[0])
			} else {
				k.saklaV("global", "", ha, 0, out)
			}
		})
		k.koy(son)
		return k
	}
}
