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
var ortakSiralamaSiniri = 4096

// birikimCekirdegi satır başına B iş parçacıklı blokla birikimli toplam. ters: sondan;
// ekle: y'ye eklenir.
func birikimCekirdegi(B int, ters, ekle bool) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("birikim_%d_%v_%v", B, ters, ekle))
		x := k.gosterici("x")
		y := k.yazilan("y")
		L := k.u32p("l")
		ic := k.u32p("ic")
		C := k.u32p("parca")
		sm := k.ortak("sm", 4*B)
		satir, tid := k.ozel("%ctaid.x"), k.ozel("%tid.x")
		_, _, taban := k.satirTabani(satir, L, ic)
		bas := k.r()
		k.e("mul.lo.s32", bas, tid, C)
		k.e("min.u32", bas, bas, L)
		bit := k.r()
		k.e("add.s32", bit, bas, C)
		k.e("min.u32", bit, bit, L)
		// adresi p konumunun (ters ise L−1−p) elemanının adresi.
		adresi := func(taban0 yazmac, p yazmac) yazmac {
			pos := p
			if ters {
				pos = k.r()
				k.e("sub.s32", pos, L, p)
				k.e("sub.s32", pos, pos, 1)
			}
			j := k.r()
			k.e("mad.lo.s32", j, pos, ic, taban)
			return k.adres(taban0, j, 4)
		}
		gez := func(govde func(p yazmac)) {
			p := k.r()
			k.e("mov.u32", p, bas)
			dongu, bitti := k.etiket(), k.etiket()
			k.koy(dongu)
			q := k.p()
			k.e("setp.ge.u32", q, p, bit)
			k.gitEger(q, bitti)
			govde(p)
			k.e("add.s32", p, p, 1)
			k.git(dongu)
			k.koy(bitti)
		}
		s := k.sabit(0)
		gez(func(p yazmac) { k.e("add.rn.f32", s, s, k.yukle(tensor.Float32, adresi(x, p))) })
		run := k.oncekiToplam(s, tid, sm, B)
		gez(func(p yazmac) {
			k.e("add.rn.f32", run, run, k.yukle(tensor.Float32, adresi(x, p)))
			ha := adresi(y, p)
			v := run
			if ekle {
				v = k.yukle(tensor.Float32, ha)
				k.e("add.rn.f32", v, v, run)
			}
			k.sakla(tensor.Float32, ha, v)
		})
		return k
	}
}

// oncekiToplam bloktaki iş parçacıklarının s değerlerinin dışlayıcı önek toplamını (bu iş
// parçacığından öncekilerin toplamı) sabit sıralı kapsayıcı taramayla (Hillis-Steele;
// paylaşımlı bellek sm, 4·B bayt) verir.
func (k *yazici) oncekiToplam(s, tid, sm yazmac, B int) yazmac {
	adr := k.r()
	k.e("mad.lo.s32", adr, tid, 4, sm)
	k.e("st.shared.f32", bel(adr, 0), s)
	k.engel()
	onceki := k.r()
	for off := 1; off < B; off *= 2 {
		v := k.sabit(0)
		atla := k.etiket()
		q := k.p()
		k.e("setp.lt.u32", q, tid, off)
		k.gitEger(q, atla)
		k.e("sub.s32", onceki, adr, 4*off)
		k.e("ld.shared.f32", v, bel(onceki, 0))
		k.koy(atla)
		k.engel()
		k.e("add.rn.f32", s, s, v)
		k.e("st.shared.f32", bel(adr, 0), s)
		k.engel()
	}
	run := k.sabit(0)
	ilk, devam := k.p(), k.etiket()
	k.e("setp.eq.u32", ilk, tid, 0)
	k.gitEger(ilk, devam)
	k.e("sub.s32", onceki, adr, 4)
	k.e("ld.shared.f32", run, bel(onceki, 0))
	k.koy(devam)
	return run
}

// siraAnahtariPTX v'nin 64 bitlik sıralama anahtarı (değer anahtarı << 32 | d).
func (k *yazici) siraAnahtariPTX(v, d yazmac, azalan bool) yazmac {
	b := k.r()
	k.e("mov.b32", b, v)
	neg := k.p()
	k.e("setp.lt.s32", neg, b, 0)
	a1, a2 := k.r(), k.r()
	k.e("not.b32", a1, b)
	k.e("or.b32", a2, b, uint32(0x80000000))
	a := k.r()
	k.e("selp.b32", a, a1, a2, neg)
	sifir, nan := k.p(), k.p()
	k.e("setp.eq.f32", sifir, v, float32(0))
	k.e("setp.nan.f32", nan, v, v)
	k.e("selp.b32", a, uint32(0x80000000), a, sifir)
	k.e("selp.b32", a, uint32(0xFFFFFFFF), a, nan)
	if azalan {
		k.e("not.b32", a, a)
	}
	return k.paketle(a, d)
}

// paketle a << 32 | d (64 bit).
func (k *yazici) paketle(a, d yazmac) yazmac {
	u, w := k.rd(), k.rd()
	k.e("cvt.u64.u32", u, a)
	k.e("shl.b64", u, u, 32)
	k.e("cvt.u64.u32", w, d)
	k.e("or.b64", u, u, w)
	return u
}

// satirTabani satır numarasından (o, i) ve satırın ilk elemanının konumunu (o·L·iç + i) verir.
func (k *yazici) satirTabani(satir, L, ic yazmac) (o, i, taban yazmac) {
	o = k.r()
	k.e("div.u32", o, satir, ic)
	i = k.r()
	k.e("mul.lo.s32", i, o, ic)
	k.e("sub.s32", i, satir, i)
	taban = k.r()
	k.e("mul.lo.s32", taban, o, L)
	k.e("mad.lo.s32", taban, taban, ic, i)
	return o, i, taban
}

// anahtarYukle satırın t. elemanının anahtarı; t ≥ L ise dolgu (0xFFFFFFFF << 32 | t).
func (k *yazici) anahtarYukle(x, t, L, ic, taban yazmac, azalan bool) yazmac {
	r := k.rd()
	dolgu := k.p()
	k.e("setp.ge.u32", dolgu, t, L)
	gercek, son := k.etiket(), k.etiket()
	k.gitDegilse(dolgu, gercek)
	tumu := k.r()
	k.e("mov.u32", tumu, uint32(0xFFFFFFFF))
	k.e("mov.b64", r, k.paketle(tumu, t))
	k.git(son)
	k.koy(gercek)
	j := k.r()
	k.e("mad.lo.s32", j, t, ic, taban)
	v := k.yukle(tensor.Float32, k.adres(x, j, 4))
	k.e("mov.b64", r, k.siraAnahtariPTX(v, t, azalan))
	k.koy(son)
	return r
}

// bitonikCift t. karşılaştırma çiftinin küçük indeksi i1 = ((t >> lj) << (lj+1)) |
// (t & (j−1)); büyüğü i1 + j (j = 2^lj). lj ve j sabit (int) ya da yazmaç olabilir.
func (k *yazici) bitonikCift(t yazmac, lj, j any) (i1, i2 yazmac) {
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
	alt := k.r()
	k.e("and.b32", alt, t, m)
	k.e("or.b32", i1, i1, alt)
	i2 = k.r()
	k.e("add.s32", i2, i1, j)
	return i1, i2
}

// karsilastirDegistir a = [ai], b = [bi] çifti için (i1 & kk) = 0 ise artan, değilse azalan
// düzene göre gerekirse yer değiştirir. adr: 64 bitlik elemanların adresi (paylaşımlı ya
// da genel).
func (k *yazici) karsilastirDegistir(i1, kk yazmac, adrA, adrB func(st bool) string, yukleOp, saklaOp string) {
	a, b := k.rd(), k.rd()
	k.e(yukleOp, a, adrA(false))
	k.e(yukleOp, b, adrB(false))
	yon := k.r()
	k.e("and.b32", yon, i1, kk)
	artan := k.p()
	k.e("setp.eq.u32", artan, yon, 0)
	buyuk, kucuk := k.p(), k.p()
	k.e("setp.gt.u64", buyuk, a, b)
	k.e("setp.lt.u64", kucuk, a, b)
	degis := k.p()
	k.e("selp.b32", yon, 1, 0, buyuk)
	ky := k.r()
	k.e("selp.b32", ky, 1, 0, kucuk)
	k.e("selp.b32", yon, yon, ky, artan)
	k.e("setp.ne.u32", degis, yon, 0)
	atla := k.etiket()
	k.gitDegilse(degis, atla)
	k.e(saklaOp, adrA(true), b)
	k.e(saklaOp, adrB(true), a)
	k.koy(atla)
}

// ciktiYaz sıralanmış satırın t. anahtarından indeksi (ve değerli ise değeri) yazar.
func (k *yazici) ciktiYaz(anahtar, t, o, i, kSay, ic, taban yazmac, x, idx, vals yazmac, degerli bool) {
	d := k.r()
	k.e("cvt.u32.u64", d, anahtar)
	hedef := k.r()
	k.e("mad.lo.s32", hedef, o, kSay, t)
	k.e("mad.lo.s32", hedef, hedef, ic, i)
	df := k.f()
	k.e("cvt.rn.f32.u32", df, d)
	k.sakla(tensor.Float32, k.adres(idx, hedef, 4), df)
	if degerli {
		j := k.r()
		k.e("mad.lo.s32", j, d, ic, taban)
		k.sakla(tensor.Float32, k.adres(vals, hedef, 4), k.yukle(tensor.Float32, k.adres(x, j, 4)))
	}
}

// siralaOrtakCekirdegi satır başına bir blokla paylaşımlı bellekte bitonik sıralama (P
// ikinin kuvveti, T iş parçacığı); ilk k anahtarın indeksini (ve değerini) yazar.
func siralaOrtakCekirdegi(P int, azalan, degerli bool) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("sirala_ortak_%d_%v_%v", P, azalan, degerli))
		x := k.gosterici("x")
		idx := k.yazilan("idx")
		var vals yazmac
		if degerli {
			vals = k.yazilan("vals")
		}
		L := k.u32p("l")
		ic := k.u32p("ic")
		kSay := k.u32p("k")
		T := min(P/2, 512)
		if P == 1 {
			T = 1
		}
		sm := k.ortak("sm", 8*P)
		satir, tid := k.ozel("%ctaid.x"), k.ozel("%tid.x")
		o, i, taban := k.satirTabani(satir, L, ic)
		adres64 := func(j yazmac) yazmac {
			a := k.r()
			k.e("mad.lo.s32", a, j, 8, sm)
			return a
		}
		// t = tid, tid+T, ... < sinir döngüsü.
		dongu := func(sinir any, govde func(t yazmac)) {
			t := k.r()
			k.e("mov.u32", t, tid)
			bas, bitti := k.etiket(), k.etiket()
			k.koy(bas)
			q := k.p()
			k.e("setp.ge.u32", q, t, sinir)
			k.gitEger(q, bitti)
			govde(t)
			k.e("add.s32", t, t, T)
			k.git(bas)
			k.koy(bitti)
		}
		dongu(P, func(t yazmac) {
			k.e("st.shared.u64", bel(adres64(t), 0), k.anahtarYukle(x, t, L, ic, taban, azalan))
		})
		k.engel()
		for kk := 2; kk <= P; kk *= 2 {
			for j := kk / 2; j > 0; j /= 2 {
				lj := 0
				for 1<<lj < j {
					lj++
				}
				dongu(P/2, func(t yazmac) {
					i1, i2 := k.bitonikCift(t, lj, j)
					a1, a2 := adres64(i1), adres64(i2)
					k.karsilastirDegistir(i1, k.tam(kk), func(bool) string { return bel(a1, 0) }, func(bool) string { return bel(a2, 0) },
						"ld.shared.u64", "st.shared.u64")
				})
				k.engel()
			}
		}
		dongu(kSay, func(t yazmac) {
			a := k.rd()
			k.e("ld.shared.u64", a, bel(adres64(t), 0))
			k.ciktiYaz(a, t, o, i, kSay, ic, taban, x, idx, vals, degerli)
		})
		return k
	}
}

// siralaBaslatCekirdegi genel bellekteki anahtar tamponunu (satır × P) doldurur: iş
// parçacığı başına bir eleman.
func siralaBaslatCekirdegi(azalan bool) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("sirala_baslat_%v", azalan))
		x := k.gosterici("x")
		buf := k.yazilan("anahtar")
		n := k.u32p("n")
		lp := k.u32p("lp") // log2(P)
		L := k.u32p("l")
		ic := k.u32p("ic")
		g := k.genelIndeks()
		son := k.sinirli(g, n)
		satir := k.r()
		k.e("shr.u32", satir, g, lp)
		P1 := k.r()
		k.e("shl.b32", P1, k.tam(1), lp)
		k.e("sub.s32", P1, P1, 1)
		t := k.r()
		k.e("and.b32", t, g, P1)
		_, _, taban := k.satirTabani(satir, L, ic)
		k.e("st.global.u64", bel(k.adres(buf, g, 8), 0), k.anahtarYukle(x, t, L, ic, taban, azalan))
		k.koy(son)
		return k
	}
}

// siralaAdimCekirdegi genel bellekte bir bitonik adım (kk, j): iş parçacığı başına bir çift.
func siralaAdimCekirdegi() *yazici {
	k := yeniYazici("sirala_adim")
	buf := k.yazilan("anahtar")
	n := k.u32p("n") // satır × P/2
	lp := k.u32p("lp")
	lj := k.u32p("lj")
	kk := k.u32p("kk")
	g := k.genelIndeks()
	son := k.sinirli(g, n)
	lp1 := k.r()
	k.e("sub.s32", lp1, lp, 1)
	satir := k.r()
	k.e("shr.u32", satir, g, lp1)
	m := k.r()
	k.e("shl.b32", m, k.tam(1), lp1)
	k.e("sub.s32", m, m, 1)
	t := k.r()
	k.e("and.b32", t, g, m)
	j := k.r()
	k.e("shl.b32", j, k.tam(1), lj)
	i1, i2 := k.bitonikCift(t, lj, j)
	sb := k.r()
	k.e("shl.b32", sb, satir, lp)
	g1, g2 := k.r(), k.r()
	k.e("add.s32", g1, sb, i1)
	k.e("add.s32", g2, sb, i2)
	a1, a2 := k.adres(buf, g1, 8), k.adres(buf, g2, 8)
	k.karsilastirDegistir(i1, kk, func(bool) string { return bel(a1, 0) }, func(bool) string { return bel(a2, 0) },
		"ld.global.u64", "st.global.u64")
	k.koy(son)
	return k
}

// siralaCiktiCekirdegi genel bellekte sıralanmış satırların ilk k anahtarını yazar.
func siralaCiktiCekirdegi(degerli bool) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("sirala_cikti_%v", degerli))
		buf := k.gosterici("anahtar")
		x := k.gosterici("x")
		idx := k.yazilan("idx")
		var vals yazmac
		if degerli {
			vals = k.yazilan("vals")
		}
		n := k.u32p("n") // satır × k
		lp := k.u32p("lp")
		L := k.u32p("l")
		ic := k.u32p("ic")
		kSay := k.u32p("k")
		g := k.genelIndeks()
		son := k.sinirli(g, n)
		satir := k.r()
		k.e("div.u32", satir, g, kSay)
		t := k.r()
		k.e("mul.lo.s32", t, satir, kSay)
		k.e("sub.s32", t, g, t)
		o, i, taban := k.satirTabani(satir, L, ic)
		ga := k.r()
		k.e("shl.b32", ga, satir, lp)
		k.e("add.s32", ga, ga, t)
		a := k.rd()
		k.e("ld.global.u64", a, bel(k.adres(buf, ga, 8), 0))
		k.ciktiYaz(a, t, o, i, kSay, ic, taban, x, idx, vals, degerli)
		k.koy(son)
		return k
	}
}

// toplaGeriCekirdegi gx[(o·L + idx)·iç + i] += g[(o·k + j)·iç + i] (iş parçacığı başına bir
// g elemanı; bir satırdaki indeksler tekil olduğundan yazmalar çakışmaz).
func toplaGeriCekirdegi() *yazici {
	k := yeniYazici("topla_geri")
	gx := k.yazilan("gx")
	pg := k.gosterici("g")
	pidx := k.gosterici("idx")
	n := k.u32p("n")
	L := k.u32p("l")
	ic := k.u32p("ic")
	kSay := k.u32p("k")
	g := k.genelIndeks()
	son := k.sinirli(g, n)
	ki := k.r()
	k.e("mul.lo.s32", ki, kSay, ic)
	o := k.r()
	k.e("div.u32", o, g, ki)
	i := k.r()
	k.e("rem.u32", i, g, ic)
	d := k.r()
	k.e("cvt.rzi.s32.f32", d, k.yukle(tensor.Float32, k.adres(pidx, g, 4)))
	h := k.r()
	k.e("mad.lo.s32", h, o, L, d)
	k.e("mad.lo.s32", h, h, ic, i)
	ha := k.adres(gx, h, 4)
	eski := k.yukle(tensor.Float32, ha)
	k.e("add.rn.f32", eski, eski, k.yukle(tensor.Float32, k.adres(pg, g, 4)))
	k.sakla(tensor.Float32, ha, eski)
	k.koy(son)
	return k
}
