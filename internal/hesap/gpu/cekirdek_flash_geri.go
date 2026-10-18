package gpu

import (
	"fmt"
	"math"

	"tr_dili/internal/hesap/tensor"
)

// ============================================================================
// Birleşik dikkat: geri yön (atomiksiz, belirlenimci)
// ============================================================================
//
// D_i = Σ_d dO[i,d]·O[i,d] önce satır başına hesaplanır (dikkat_d). Ardından iki çekirdek
// puanları lse'den yeniden kurar (P = e^{S·ölçek − lse}, dP = dO·Vᵀ, dS = P·(dP − D)·ölçek):
//   - dikkat_geri_kv: her blok bir anahtar döşemesinin dK += dSᵀ·Q ve dV += Pᵀ·dO'sunu,
//     o anahtar başını paylaşan bütün sorgu başları ve görünür sorgu döşemeleri üzerinden,
//   - dikkat_geri_q: her blok bir sorgu döşemesinin dQ += dS·K'sını görünür anahtar
//     döşemeleri üzerinden
// biriktirir. Her çıktı tek blokta, sabit sırayla toplanır (atomik yok). Çıktı kipleri:
// 0 yok, 1 eskisine ekle, 2 üzerine yaz.

const (
	gradYok = iota
	gradEkle
	gradUzerine
)

// flashDCekirdegi D[r] = Σ_d dO[r,d]·O[r,d]; satır başına iş parçacığı.
func flashDCekirdegi() *yazici {
	k := yeniYazici("dikkat_d")
	do, o := k.gosterici("do"), k.gosterici("o")
	d := k.yazilan("d")
	R, Dv := k.u32p("r"), k.u32p("dv")
	r := k.genelIndeks()
	son := k.sinirli(r, R)
	taban := k.r()
	k.e("mul.lo.s32", taban, r, Dv)
	acc := k.sabit(0)
	j := k.tam(0)
	dongu, bitti := k.etiket(), k.etiket()
	k.koy(dongu)
	p := k.p()
	k.e("setp.ge.u32", p, j, Dv)
	k.gitEger(p, bitti)
	i := k.r()
	k.e("add.s32", i, taban, j)
	a := k.yukle(tensor.Float32, k.adres(do, i, 4))
	b := k.yukle(tensor.Float32, k.adres(o, i, 4))
	k.e("fma.rn.f32", acc, a, b, acc)
	k.e("add.s32", j, j, 1)
	k.git(dongu)
	k.koy(bitti)
	k.sakla(tensor.Float32, k.adres(d, r, 4), acc)
	k.koy(son)
	return k
}

// flashGeriDurum geri çekirdeklerin döşemeleri ve satır bilgileri.
type flashGeriDurum struct {
	*flashOrtak
	qs, dos, ks, vs, ps smDoseme
	dhp, dvp            int
}

func (f *flashGeriDurum) dosemeler() {
	k, a := f.k, f.a
	f.dhp, f.dvp = yuvarla16(a.Dh), yuvarla16(a.Dv)
	f.qs = smDoseme{k.ortak("qs", 4*a.Br*(f.dhp+1)), a.Br, f.dhp + 1}
	f.dos = smDoseme{k.ortak("dos", 4*a.Br*(f.dvp+1)), a.Br, f.dvp + 1}
	f.ks = smDoseme{k.ortak("ks", 4*a.Bc*(f.dhp+1)), a.Bc, f.dhp + 1}
	f.vs = smDoseme{k.ortak("vs", 4*a.Bc*(f.dvp+1)), a.Bc, f.dvp + 1}
	f.ps = smDoseme{k.ortak("ps", 4*a.Br*(a.Bc+1)), a.Br, a.Bc + 1}
}

// satirBilgisi sorgu döşemesinin (q0) iş parçacığına düşen satırları: indeksleri, sınırları,
// lse'leri (Tq dışında +∞) ve D'leri.
func (f *flashGeriDurum) satirBilgisi(q0, satir0, bh, plse, pd yazmac) ([]yazmac, [][3]yazmac, []yazmac, []yazmac) {
	k := f.k
	rpt := f.a.Br / 16
	satirlar, sinir := make([]yazmac, rpt), make([][3]yazmac, rpt)
	lse, dd := make([]yazmac, rpt), make([]yazmac, rpt)
	lb := k.r()
	k.e("mul.lo.s32", lb, bh, f.Tq)
	for i := range satirlar {
		satirlar[i] = k.r()
		k.e("add.s32", satirlar[i], q0, satir0)
		k.e("add.s32", satirlar[i], satirlar[i], i)
		lo, hi, mt := f.gorunurluk(satirlar[i], bh)
		sinir[i] = [3]yazmac{lo, hi, mt}
		gec := k.p()
		k.e("setp.lt.u32", gec, satirlar[i], f.Tq)
		li := k.r()
		k.e("add.s32", li, lb, satirlar[i])
		lse[i], dd[i] = k.sabit(float32(math.Inf(1))), k.sabit(0)
		k.eger(gec, "ld.global.f32", lse[i], bel(k.adres(plse, li, 4), 0))
		k.eger(gec, "ld.global.f32", dd[i], bel(k.adres(pd, li, 4), 0))
	}
	return satirlar, sinir, lse, dd
}

// olasiliklar S ve dP'yi kurar ve P ile dS'ye çevirir (yerinde: S → P, dP → dS).
func (f *flashGeriDurum) olasiliklar(satir0 yazmac, satirlar []yazmac, j0 yazmac, sinir [][3]yazmac, lse, dd []yazmac) ([][]yazmac, [][]yazmac) {
	k, a := f.k, f.a
	rpt, cpt := a.Br/16, a.Bc/16
	S := k.sifirMatris(rpt, cpt)
	sut0 := k.carp(f.tx, cpt)
	k.mikroCarp(S, k.thAdres(f.qs.taban, satir0, f.qs.adim), f.qs.adim, 1, k.thAdres(f.ks.taban, sut0, f.ks.adim), f.ks.adim, 1, a.Dh)
	dP := k.sifirMatris(rpt, cpt)
	k.mikroCarp(dP, k.thAdres(f.dos.taban, satir0, f.dos.adim), f.dos.adim, 1, k.thAdres(f.vs.taban, sut0, f.vs.adim), f.vs.adim, 1, a.Dv)
	f.puanlar(S, satirlar, j0, sinir)
	for i := range S {
		for j := range S[i] {
			x := k.f()
			k.e("sub.rn.f32", x, S[i][j], lse[i])
			k.e("mov.f32", S[i][j], k.ustel(x))
			t := k.f()
			k.e("sub.rn.f32", t, dP[i][j], dd[i])
			k.e("mul.rn.f32", t, t, S[i][j])
			k.e("mul.rn.f32", dP[i][j], t, f.olcek)
		}
	}
	return S, dP
}

// psYaz P ya da dS'yi ps döşemesine yazar ([sorgu][anahtar]).
func (f *flashGeriDurum) psYaz(M [][]yazmac, satir0 yazmac) { f.dosemeyeYaz(M, satir0, f.ps) }

// gradYazKip biriktirilen döşemeyi (satırlar r0 + ty·rpt + i < lim, sütunlar < D) yazar.
func (f *flashGeriDurum) gradYazKip(M [][]yazmac, kip int, ptr, r0, lim yazmac, D int) {
	if kip == gradYok {
		return
	}
	k := f.k
	for i := range M {
		satir := k.r()
		k.e("mad.lo.s32", satir, f.ty, len(M), r0)
		k.e("add.s32", satir, satir, i)
		atlaS := k.etiket()
		p := k.p()
		k.e("setp.ge.u32", p, satir, lim)
		k.gitEger(p, atlaS)
		for j, v := range M[i] {
			sut := k.r()
			k.e("mad.lo.s32", sut, f.tx, len(M[i]), j)
			atla := k.etiket()
			q := k.p()
			k.e("setp.ge.u32", q, sut, D)
			k.gitEger(q, atla)
			idx := k.r()
			k.e("mad.lo.s32", idx, satir, D, sut)
			ha := k.adres(ptr, idx, 4)
			if kip == gradEkle {
				eski := k.yukle(tensor.Float32, ha)
				k.e("add.rn.f32", v, eski, v)
			}
			k.sakla(tensor.Float32, ha, v)
			k.koy(atla)
		}
		k.koy(atlaS)
	}
}

// sorguAraligi anahtar döşemesini [j0, j0+Bc) görebilecek sorgu satırlarını kapsayan aralık
// [iBas, iSon) (iBas Br'nin katı; sınırlar satırla azalmadığından iki uç yeter).
func (f *flashGeriDurum) sorguAraligi(j0 yazmac) (iBas, iSon yazmac) {
	k := f.k
	fark := k.r() // Tq − Tk
	k.e("sub.s32", fark, f.Tq, f.Tk)
	iBas, iSon = k.tam(0), k.r()
	k.e("mov.u32", iSon, f.Tq)
	ned, pen := k.p(), k.p()
	k.e("setp.ne.u32", ned, f.ned, 0)
	k.e("setp.gt.s32", pen, f.pen, 0)
	// Nedensel: c(i) ≥ j0 ⟺ i ≥ j0 + Tq − Tk.
	b1 := k.r()
	k.e("add.s32", b1, j0, fark)
	k.e("selp.b32", iBas, b1, iBas, ned)
	// Nedensel olmayan pencere: c + W > j0 ⟺ i ≥ j0 − W + Tq − Tk + 1.
	b2 := k.r()
	k.e("sub.s32", b2, b1, f.pen)
	k.e("add.s32", b2, b2, 1)
	iki := k.p()
	k.e("not.pred", iki, ned)
	k.e("and.pred", iki, iki, pen)
	k.e("selp.b32", iBas, b2, iBas, iki)
	// Pencere: c − W + 1 < j0 + Bc ⟺ i < j0 + Bc + W − 1 + Tq − Tk.
	s := k.r()
	k.e("add.s32", s, b1, f.a.Bc-1)
	k.e("add.s32", s, s, f.pen)
	k.e("min.s32", s, s, iSon)
	k.e("selp.b32", iSon, s, iSon, pen)
	k.e("max.s32", iBas, iBas, 0)
	k.e("max.s32", iSon, iSon, 0)
	k.e("div.u32", iBas, iBas, f.a.Br)
	k.e("mul.lo.s32", iBas, iBas, f.a.Br)
	return iBas, iSon
}

// flashGeriKVCekirdegi dK ve dV; ızgara (anahtar döşemeleri, Hkv, Batch).
func flashGeriKVCekirdegi(a flashAyar, kipK, kipV int) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("%s_%d_%d", a.anahtar("dikkat_geri_kv"), kipK, kipV))
		f := &flashGeriDurum{flashOrtak: &flashOrtak{k: k, a: a}}
		pq, pk, pv, pdo := k.gosterici("q"), k.gosterici("k"), k.gosterici("v"), k.gosterici("do")
		plse, pd := k.gosterici("lse"), k.gosterici("d")
		pdk, pdv := k.yazilan("dk"), k.yazilan("dv")
		f.ortakParametreler()
		f.basIndeksleri(true)
		f.dosemeler()
		j0 := k.r()
		k.e("mul.lo.s32", j0, k.ozel("%ctaid.x"), a.Bc)
		kb := k.matrisTabani(pk, f.bhkv, f.Tk, a.Dh)
		vb := k.matrisTabani(pv, f.bhkv, f.Tk, a.Dv)
		k.dosemeYukle(f.ks, a.Dh, f.dhp, kb, j0, f.Tk, f.tid)
		k.dosemeYukle(f.vs, a.Dv, f.dvp, vb, j0, f.Tk, f.tid)
		rptK := a.Bc / 16
		dK, dV := k.sifirMatris(rptK, f.dhp/16), k.sifirMatris(rptK, f.dvp/16)
		satir0 := k.carp(f.ty, a.Br/16)
		anahtar0 := k.carp(f.ty, rptK)
		iBas, iSon := f.sorguAraligi(j0)
		g := k.tam(0)
		basD, basB := k.etiket(), k.etiket()
		k.koy(basD)
		pg := k.p()
		k.e("setp.ge.u32", pg, g, f.G)
		k.gitEger(pg, basB)
		bh := k.r()
		k.e("add.s32", bh, f.h, g)
		k.e("mad.lo.s32", bh, f.b, f.H, bh)
		qb := k.matrisTabani(pq, bh, f.Tq, a.Dh)
		dob := k.matrisTabani(pdo, bh, f.Tq, a.Dv)
		i0 := k.r()
		k.e("mov.u32", i0, iBas)
		dongu, bitti := k.etiket(), k.etiket()
		k.koy(dongu)
		p := k.p()
		k.e("setp.ge.s32", p, i0, iSon)
		k.gitEger(p, bitti)
		k.dosemeYukle(f.qs, a.Dh, f.dhp, qb, i0, f.Tq, f.tid)
		k.dosemeYukle(f.dos, a.Dv, f.dvp, dob, i0, f.Tq, f.tid)
		satirlar, sinir, lse, dd := f.satirBilgisi(i0, satir0, bh, plse, pd)
		k.engel()
		P, dS := f.olasiliklar(satir0, satirlar, j0, sinir, lse, dd)
		f.psYaz(P, satir0)
		k.engel()
		// dV += Pᵀ·dO ve dK += dSᵀ·Q: satırlar anahtar, iç eksen sorgu.
		k.mikroCarp(dV, k.thAdres(f.ps.taban, anahtar0, 1), 1, f.ps.adim, k.thAdres(f.dos.taban, k.carp(f.tx, f.dvp/16), 1), 1, f.dos.adim, a.Br)
		k.engel()
		f.psYaz(dS, satir0)
		k.engel()
		k.mikroCarp(dK, k.thAdres(f.ps.taban, anahtar0, 1), 1, f.ps.adim, k.thAdres(f.qs.taban, k.carp(f.tx, f.dhp/16), 1), 1, f.qs.adim, a.Br)
		k.engel()
		k.e("add.s32", i0, i0, a.Br)
		k.git(dongu)
		k.koy(bitti)
		k.e("add.s32", g, g, 1)
		k.git(basD)
		k.koy(basB)
		f.gradYazKip(dK, kipK, k.matrisTabani(pdk, f.bhkv, f.Tk, a.Dh), j0, f.Tk, a.Dh)
		f.gradYazKip(dV, kipV, k.matrisTabani(pdv, f.bhkv, f.Tk, a.Dv), j0, f.Tk, a.Dv)
		return k
	}
}

// flashGeriQCekirdegi dQ; ızgara (sorgu döşemeleri, H, Batch).
func flashGeriQCekirdegi(a flashAyar, kipQ int) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("%s_%d", a.anahtar("dikkat_geri_q"), kipQ))
		f := &flashGeriDurum{flashOrtak: &flashOrtak{k: k, a: a}}
		pq, pk, pv, pdo := k.gosterici("q"), k.gosterici("k"), k.gosterici("v"), k.gosterici("do")
		plse, pd := k.gosterici("lse"), k.gosterici("d")
		pdq := k.yazilan("dq")
		f.ortakParametreler()
		f.basIndeksleri(false)
		f.dosemeler()
		q0 := k.r()
		k.e("mul.lo.s32", q0, k.ozel("%ctaid.x"), a.Br)
		qb := k.matrisTabani(pq, f.bh, f.Tq, a.Dh)
		dob := k.matrisTabani(pdo, f.bh, f.Tq, a.Dv)
		kb := k.matrisTabani(pk, f.bhkv, f.Tk, a.Dh)
		vb := k.matrisTabani(pv, f.bhkv, f.Tk, a.Dv)
		k.dosemeYukle(f.qs, a.Dh, f.dhp, qb, q0, f.Tq, f.tid)
		k.dosemeYukle(f.dos, a.Dv, f.dvp, dob, q0, f.Tq, f.tid)
		satir0 := k.carp(f.ty, a.Br/16)
		satirlar, sinir, lse, dd := f.satirBilgisi(q0, satir0, f.bh, plse, pd)
		jBas, jSon := f.anahtarAraligi(q0)
		dQ := k.sifirMatris(a.Br/16, f.dhp/16)
		j0 := k.r()
		k.e("mov.u32", j0, jBas)
		dongu, bitti := k.etiket(), k.etiket()
		k.koy(dongu)
		p := k.p()
		k.e("setp.ge.u32", p, j0, jSon)
		k.gitEger(p, bitti)
		k.dosemeYukle(f.ks, a.Dh, f.dhp, kb, j0, f.Tk, f.tid)
		k.dosemeYukle(f.vs, a.Dv, f.dvp, vb, j0, f.Tk, f.tid)
		k.engel()
		_, dS := f.olasiliklar(satir0, satirlar, j0, sinir, lse, dd)
		f.psYaz(dS, satir0)
		k.engel()
		k.mikroCarp(dQ, k.thAdres(f.ps.taban, satir0, f.ps.adim), f.ps.adim, 1, k.thAdres(f.ks.taban, k.carp(f.tx, f.dhp/16), 1), 1, f.ks.adim, a.Bc)
		k.engel()
		k.e("add.s32", j0, j0, a.Bc)
		k.git(dongu)
		k.koy(bitti)
		f.gradYazKip(dQ, kipQ, k.matrisTabani(pdq, f.bh, f.Tq, a.Dh), q0, f.Tq, a.Dh)
		return k
	}
}
