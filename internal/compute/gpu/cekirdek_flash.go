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
// Puan matrisi (Tq×Tk) belleğe yazılmaz. İleri yönde her blok bir (toplu, baş) çiftinin Br
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

const flashIs = 256

// flashAyar birleşik dikkat çekirdeklerinin üretim ayarları.
type flashAyar struct {
	Br, Bc, Dh, Dv int
	maske          bool
}

// yuvarla16 d'yi 16'nın katına yuvarlar.
func yuvarla16(d int) int { return (d + 15) / 16 * 16 }

func (a flashAyar) anahtar(ad string) string {
	return fmt.Sprintf("%s_%d_%d_%d_%d_%v", ad, a.Br, a.Bc, a.Dh, a.Dv, a.maske)
}

// smBayt ileri (geri yanlış) ya da geri çekirdeğin paylaşımlı belleği.
func (a flashAyar) smBayt(geri bool) int {
	dh, dv := yuvarla16(a.Dh)+1, yuvarla16(a.Dv)+1
	if !geri {
		// Q, K/V (ortak), P, iki satır indirgeme tamponu.
		return 4 * (a.Br*dh + a.Bc*max(dh, dv) + a.Br*(a.Bc+1) + 2*a.Br*17)
	}
	// K, V, Q, dO, P/dS.
	return 4 * (a.Bc*(dh+dv) + a.Br*(dh+dv) + a.Br*(a.Bc+1))
}

// flashSec boylara uyan en büyük döşemeyi seçer; uyan yoksa tamam yanlış (dikkat matris
// çarpımı yoluyla yapılır). İleri yön statik paylaşımlı belleğe (48 KB) sığar; geri yön
// kartın blok başına açtığı paylaşımlı belleği (sinir, ör. 99 KB) kullanabilir: 64×64
// döşemede iş parçacığı başına 4×4 çıktı (32×32'de 2×2) paylaşımlı bellek okumalarını
// yarıya indirir.
func flashSec(Dh, Dv int, maske, geri bool, sinir int) (flashAyar, bool) {
	if !geri {
		sinir = statikPaylasimSiniri
	}
	for _, d := range [][2]int{{64, 64}, {64, 32}, {32, 32}, {32, 16}, {16, 16}} {
		a := flashAyar{Br: d[0], Bc: d[1], Dh: Dh, Dv: Dv, maske: maske}
		if geri && a.Br != a.Bc {
			continue
		}
		if a.smBayt(geri) <= min(sinir, statikPaylasimSiniri) || (geri && a.smYuvarli(geri) <= sinir) {
			return a, true
		}
	}
	return flashAyar{}, false
}

// smYuvarli paylaşımlı dizilerin 16'ya hizalı gerçek toplamı (yazici.ortak'taki gibi; geri
// çekirdeğin beş döşemesi); devingen paylaşımlı bellek bu kadar açılır.
func (a flashAyar) smYuvarli(geri bool) int {
	dh, dv := yuvarla16(a.Dh)+1, yuvarla16(a.Dv)+1
	t := 0
	for _, b := range []int{a.Br * dh, a.Br * dv, a.Bc * dh, a.Bc * dv, a.Br * (a.Bc + 1)} {
		t = (t+15)&^15 + 4*b
	}
	return t
}

// dinamikMi çekirdeğin paylaşımlı belleği statik sınırı aşıyor mu (devingen diziyle
// üretilir ve başlatmada açılır).
func (a flashAyar) dinamikMi(geri bool) bool {
	return geri && a.smYuvarli(geri) > statikPaylasimSiniri
}

// smDoseme paylaşımlı bellekte satır öncelikli bir döşeme.
type smDoseme struct {
	taban       yazmac
	satir, adim int // satır sayısı ve satır adımı (eleman)
}

// dosemeYukle genel bellekteki satır öncelikli [T][D] matrisin r0'dan başlayan satırlarını
// döşemeye yükler; lim'den sonraki satırlar ve D'den sonraki sütunlar (Dp'ye kadar) 0 olur.
func (k *yazici) dosemeYukle(d smDoseme, D, Dp int, ptr, r0, lim, tid yazmac) {
	toplam := d.satir * Dp
	for q := 0; q*flashIs < toplam; q++ {
		e := k.r()
		k.e("add.s32", e, tid, q*flashIs)
		atla := k.etiket()
		if (q+1)*flashIs > toplam {
			p := k.p()
			k.e("setp.ge.u32", p, e, toplam)
			k.gitEger(p, atla)
		}
		sat, sut := k.r(), k.r()
		k.e("div.u32", sat, e, Dp)
		k.e("rem.u32", sut, e, Dp)
		gr := k.r()
		k.e("add.s32", gr, r0, sat)
		p1, p2 := k.p(), k.p()
		k.e("setp.lt.u32", p1, gr, lim)
		k.e("setp.lt.u32", p2, sut, D)
		k.e("and.pred", p1, p1, p2)
		idx := k.r()
		k.e("mad.lo.s32", idx, gr, D, sut)
		v := k.sabit(0)
		k.eger(p1, "ld.global.f32", v, bel(k.adres(ptr, idx, 4), 0))
		so := k.r()
		k.e("mad.lo.s32", so, sat, d.adim, sut)
		k.e("mad.lo.s32", so, so, 4, d.taban)
		k.e("st.shared.f32", bel(so, 0), v)
		k.koy(atla)
	}
}

// mikroCarp acc[i][j] += Σ_{kk<K} A[i·aIdx + kk·aK]·B[j·bIdx + kk·bK]; aT ve bT iş
// parçacığının ilk satır/sütununun paylaşımlı bellek adresi (bayt), adımlar eleman.
func (k *yazici) mikroCarp(acc [][]yazmac, aT yazmac, aIdx, aK int, bT yazmac, bIdx, bK, K int) {
	av, bv := make([]yazmac, len(acc)), make([]yazmac, len(acc[0]))
	for i := range av {
		av[i] = k.f()
	}
	for j := range bv {
		bv[j] = k.f()
	}
	for kk := 0; kk < K; kk++ {
		for i := range av {
			k.e("ld.shared.f32", av[i], bel(aT, 4*(i*aIdx+kk*aK)))
		}
		for j := range bv {
			k.e("ld.shared.f32", bv[j], bel(bT, 4*(j*bIdx+kk*bK)))
		}
		for i := range av {
			for j := range bv {
				k.e("fma.rn.f32", acc[i][j], av[i], bv[j], acc[i][j])
			}
		}
	}
}

// sifirMatris r×c sıfır kayıt matrisi.
func (k *yazici) sifirMatris(r, c int) [][]yazmac {
	m := make([][]yazmac, r)
	for i := range m {
		m[i] = make([]yazmac, c)
		for j := range m[i] {
			m[i][j] = k.sabit(0)
		}
	}
	return m
}

// thAdres sm + (ilk·adim)·4: iş parçacığının ilk satırının/sütununun adresi.
func (k *yazici) thAdres(sm, ilk yazmac, adim int) yazmac {
	r := k.r()
	k.e("mad.lo.s32", r, ilk, 4*adim, sm)
	return r
}

// satirIndirge iş parçacığının satırlarındaki değerleri (her satırda cpt değer) birleştirir,
// 16 tx'in kısmi sonuçlarını red [Br][17] tamponuyla toplar; her satırın sonucu (aynı
// sırayla, bütün tx'lerde aynı) döner. İçinde engel vardır.
func (k *yazici) satirIndirge(v [][]yazmac, red, satir0, tx yazmac, birlestir func(a, b yazmac) yazmac) []yazmac {
	for i, s := range v {
		p := s[0]
		for _, x := range s[1:] {
			p = birlestir(p, x)
		}
		ad := k.r()
		k.e("add.s32", ad, satir0, i)
		k.e("mad.lo.s32", ad, ad, 17, tx)
		k.e("mad.lo.s32", ad, ad, 4, red)
		k.e("st.shared.f32", bel(ad, 0), p)
	}
	k.engel()
	r := make([]yazmac, len(v))
	for i := range v {
		ad := k.r()
		k.e("add.s32", ad, satir0, i)
		k.e("mad.lo.s32", ad, ad, 4*17, red)
		t := k.f()
		k.e("ld.shared.f32", t, bel(ad, 0))
		for j := 1; j < 16; j++ {
			x := k.f()
			k.e("ld.shared.f32", x, bel(ad, 4*j))
			t = birlestir(t, x)
		}
		r[i] = t
	}
	return r
}

// flashOrtak ileri ve geri çekirdeklerin ortak parametreleri ve indeksleri.
type flashOrtak struct {
	k                   *yazici
	a                   flashAyar
	md, mo              yazmac
	Tq, Tk, H, G        yazmac
	ned, pen, msQ, msK  yazmac
	olcek               yazmac
	tid, tx, ty         yazmac
	b, h, bh, kvh, bhkv yazmac
}

// ortakParametreler maske, boylar ve dikkat seçeneklerinin parametrelerini bildirir.
func (f *flashOrtak) ortakParametreler() {
	k := f.k
	f.md = k.gosterici("maske")
	f.mo = k.gosterici("maske_ofset")
	f.Tq, f.Tk, f.H, f.G = k.u32p("tq"), k.u32p("tk"), k.u32p("h"), k.u32p("g")
	f.ned, f.pen = k.u32p("nedensel"), k.u32p("pencere")
	f.msQ, f.msK = k.u32p("msq"), k.u32p("msk")
	f.olcek = k.f32p("olcek")
	f.tid = k.ozel("%tid.x")
	f.tx, f.ty = k.r(), k.r()
	k.e("and.b32", f.tx, f.tid, 15)
	k.e("shr.u32", f.ty, f.tid, 4)
}

// basIndeksleri b, h (ızgaranın z ve y'si; geri dK/dV çekirdeğinde y anahtar başıdır).
func (f *flashOrtak) basIndeksleri(yKv bool) {
	k := f.k
	f.b = k.ozel("%ctaid.z")
	y := k.ozel("%ctaid.y")
	Hkv := k.r()
	k.e("div.u32", Hkv, f.H, f.G)
	f.bh, f.bhkv = k.r(), k.r()
	if yKv {
		f.kvh = y
		f.h = k.r()
		k.e("mul.lo.s32", f.h, y, f.G)
	} else {
		f.h = y
		f.kvh = k.r()
		k.e("div.u32", f.kvh, y, f.G)
	}
	k.e("mad.lo.s32", f.bh, f.b, f.H, f.h)
	k.e("mad.lo.s32", f.bhkv, f.b, Hkv, f.kvh)
}

// matrisTabani ptr + (sira·T·D)·4 (bir başın [T][D] matrisinin başı).
func (k *yazici) matrisTabani(ptr, sira, T yazmac, D int) yazmac {
	e := k.r()
	k.e("mul.lo.s32", e, sira, T)
	r := k.rd()
	k.e("mul.wide.u32", r, e, 4*D)
	k.e("add.s64", r, r, ptr)
	return r
}

// gorunurluk sorgu satırı i için sınırlar ve maske tabanı (bh: maske ofsetinin başı).
func (f *flashOrtak) gorunurluk(i, bh yazmac) (lo, hi, mt yazmac) {
	k := f.k
	lo, hi = k.dikkatSinirlari(i, f.Tq, f.Tk, f.ned, f.pen)
	// Tq dışındaki (döşemenin artık) satırlar hiçbir anahtara bakmaz (maske de okunmaz).
	disari := k.p()
	k.e("setp.ge.u32", disari, i, f.Tq)
	k.e("selp.b32", hi, lo, hi, disari)
	mt = k.tam(0)
	if f.a.maske {
		k.e("ld.global.u32", mt, bel(k.adres(f.mo, bh, 4), 0))
		k.e("mad.lo.s32", mt, i, f.msQ, mt)
	}
	return lo, hi, mt
}

// puanlar S·ölçek'i maskeler: görünmeyen (ya da satırı Tq dışı) puan −sonsuz olur.
func (f *flashOrtak) puanlar(S [][]yazmac, satirlar []yazmac, j0 yazmac, sinir [][3]yazmac) {
	k := f.k
	for i := range S {
		for j := range S[i] {
			jj := k.r()
			k.e("mad.lo.s32", jj, f.tx, len(S[i]), j0)
			k.e("add.s32", jj, jj, j)
			v := S[i][j]
			k.e("mul.rn.f32", v, v, f.olcek)
			g := k.dikkatGorunur(jj, sinir[i][0], sinir[i][1], sinir[i][2], f.md, f.msK, f.a.maske)
			gecerli := k.p()
			k.e("setp.lt.u32", gecerli, satirlar[i], f.Tq)
			k.e("and.pred", g, g, gecerli)
			k.e("selp.f32", v, v, float32(math.Inf(-1)), g)
		}
	}
}

// flashIleriCekirdegi birleşik dikkatin ileri yönü; ızgara (sorgu döşemeleri, H, Batch).
func flashIleriCekirdegi(a flashAyar) func() *yazici {
	return func() *yazici {
		k := yeniYazici(a.anahtar("dikkat_ileri"))
		f := &flashOrtak{k: k, a: a}
		pq, pk, pv := k.gosterici("q"), k.gosterici("k"), k.gosterici("v")
		pout, plse := k.yazilan("cikti"), k.yazilan("lse")
		f.ortakParametreler()
		f.basIndeksleri(false)
		dhp, dvp := yuvarla16(a.Dh), yuvarla16(a.Dv)
		rpt, cpt, ocpt := a.Br/16, a.Bc/16, dvp/16
		qs := smDoseme{k.ortak("qs", 4*a.Br*(dhp+1)), a.Br, dhp + 1}
		kvs := k.ortak("kvs", 4*a.Bc*(max(dhp, dvp)+1))
		ps := smDoseme{k.ortak("ps", 4*a.Br*(a.Bc+1)), a.Br, a.Bc + 1}
		red1, red2 := k.ortak("red1", 4*a.Br*17), k.ortak("red2", 4*a.Br*17)
		q0 := k.r()
		k.e("mul.lo.s32", q0, k.ozel("%ctaid.x"), a.Br)
		qb := k.matrisTabani(pq, f.bh, f.Tq, a.Dh)
		kb := k.matrisTabani(pk, f.bhkv, f.Tk, a.Dh)
		vb := k.matrisTabani(pv, f.bhkv, f.Tk, a.Dv)
		k.dosemeYukle(qs, a.Dh, dhp, qb, q0, f.Tq, f.tid)
		// Satırların sınırları ve bloğun gezeceği anahtar döşemeleri.
		satir0 := k.r()
		k.e("mad.lo.s32", satir0, f.ty, rpt, 0)
		satirlar := make([]yazmac, rpt)
		sinir := make([][3]yazmac, rpt)
		for i := range satirlar {
			satirlar[i] = k.r()
			k.e("add.s32", satirlar[i], q0, satir0)
			k.e("add.s32", satirlar[i], satirlar[i], i)
			lo, hi, mt := f.gorunurluk(satirlar[i], f.bh)
			sinir[i] = [3]yazmac{lo, hi, mt}
		}
		jBas, jSon := f.anahtarAraligi(q0)
		m, l := make([]yazmac, rpt), make([]yazmac, rpt)
		for i := range m {
			m[i], l[i] = k.sabit(float32(math.Inf(-1))), k.sabit(0)
		}
		O := k.sifirMatris(rpt, ocpt)
		j0 := k.r()
		k.e("mov.u32", j0, jBas)
		dongu, bitti := k.etiket(), k.etiket()
		k.koy(dongu)
		p := k.p()
		k.e("setp.ge.u32", p, j0, jSon)
		k.gitEger(p, bitti)
		ks := smDoseme{kvs, a.Bc, dhp + 1}
		k.dosemeYukle(ks, a.Dh, dhp, kb, j0, f.Tk, f.tid)
		k.engel()
		S := k.sifirMatris(rpt, cpt)
		k.mikroCarp(S, k.thAdres(qs.taban, satir0, qs.adim), qs.adim, 1, k.thAdres(ks.taban, k.carp(f.tx, cpt), ks.adim), ks.adim, 1, a.Dh)
		f.puanlar(S, satirlar, j0, sinir)
		yeniM := k.satirIndirge(S, red1, satir0, f.tx, k.enBuyuk)
		// V, K'nin yerine (K'yi okuyan bütün iş parçacıkları engeli geçti).
		vs := smDoseme{kvs, a.Bc, dvp + 1}
		k.dosemeYukle(vs, a.Dv, dvp, vb, j0, f.Tk, f.tid)
		f.cevrimiciSoftmax(S, yeniM, m, l, O, red2, satir0, ps)
		k.engel()
		k.mikroCarp(O, k.thAdres(ps.taban, satir0, ps.adim), ps.adim, 1, k.thAdres(vs.taban, k.carp(f.tx, ocpt), 1), 1, vs.adim, a.Bc)
		k.engel()
		k.e("add.s32", j0, j0, a.Bc)
		k.git(dongu)
		k.koy(bitti)
		ob := k.matrisTabani(pout, f.bh, f.Tq, a.Dv)
		lb := k.r()
		k.e("mul.lo.s32", lb, f.bh, f.Tq)
		for i := range O {
			f.ileriYaz(O[i], m[i], l[i], satirlar[i], ob, plse, lb)
		}
		return k
	}
}

// cevrimiciSoftmax çevrimiçi softmax adımı: yeni en büyükle (m) ölçek α = e^{m_eski−m},
// P = e^{S−m} (S yerinde P olur), l = l·α + Σ P, O = O·α; P paylaşımlı belleğe ([sorgu]
// [anahtar]) yazılır. İçinde satır indirgemesinin engeli vardır.
func (f *flashOrtak) cevrimiciSoftmax(S [][]yazmac, yeniM, m, l []yazmac, O [][]yazmac, red, satir0 yazmac, ps smDoseme) {
	k := f.k
	alfa := make([]yazmac, len(S))
	for i := range S {
		mn := k.f()
		k.e("max.f32", mn, m[i], yeniM[i])
		bos := k.p()
		k.e("setp.eq.f32", bos, mn, float32(math.Inf(-1)))
		d := k.f()
		k.e("sub.rn.f32", d, m[i], mn)
		alfa[i] = k.ustel(d)
		k.e("selp.f32", alfa[i], float32(1), alfa[i], bos)
		for j := range S[i] {
			x := k.f()
			k.e("sub.rn.f32", x, S[i][j], mn)
			e := k.ustel(x)
			k.e("selp.f32", S[i][j], float32(0), e, bos)
		}
		k.e("mov.f32", m[i], mn)
	}
	top := k.satirIndirge(S, red, satir0, f.tx, k.topla)
	for i := range S {
		k.e("fma.rn.f32", l[i], l[i], alfa[i], top[i])
		for j := range O[i] {
			k.e("mul.rn.f32", O[i][j], O[i][j], alfa[i])
		}
	}
	f.dosemeyeYaz(S, satir0, ps)
}

// dosemeyeYaz iş parçacığının kayıt döşemesini (satırlar satir0+i, sütunlar tx·cpt+j)
// paylaşımlı bellek döşemesine yazar.
func (f *flashOrtak) dosemeyeYaz(M [][]yazmac, satir0 yazmac, d smDoseme) {
	k := f.k
	cpt := len(M[0])
	for i := range M {
		ad := k.r()
		k.e("add.s32", ad, satir0, i)
		k.e("mad.lo.s32", ad, ad, d.adim, k.carp(f.tx, cpt))
		k.e("mad.lo.s32", ad, ad, 4, d.taban)
		for j, v := range M[i] {
			k.e("st.shared.f32", bel(ad, 4*j), v)
		}
	}
}

// carp r·c (c sabit).
func (k *yazici) carp(r yazmac, c int) yazmac {
	t := k.r()
	k.e("mul.lo.s32", t, r, c)
	return t
}

// ileriYaz bir satırın O/l'sini ve (tx = 0) lse'sini yazar; boş satırda O = 0, lse = +∞.
func (f *flashOrtak) ileriYaz(O []yazmac, m, l, satir, ob, plse, lb yazmac) {
	k := f.k
	son := k.etiket()
	p := k.p()
	k.e("setp.ge.u32", p, satir, f.Tq)
	k.gitEger(p, son)
	bos := k.p()
	k.e("setp.eq.f32", bos, l, float32(0))
	ters := k.f()
	k.e("rcp.rn.f32", ters, l)
	k.e("selp.f32", ters, float32(0), ters, bos)
	for j, v := range O {
		sut := k.r()
		k.e("mad.lo.s32", sut, f.tx, len(O), j)
		atla := k.etiket()
		q := k.p()
		k.e("setp.ge.u32", q, sut, f.a.Dv)
		k.gitEger(q, atla)
		k.e("mul.rn.f32", v, v, ters)
		idx := k.r()
		k.e("mad.lo.s32", idx, satir, f.a.Dv, sut)
		k.sakla(tensor.Float32, k.adres(ob, idx, 4), v)
		k.koy(atla)
	}
	k.gitEger(k.tidSifirDegilTx(f.tx), son)
	lv := k.logaritma(l)
	k.e("add.rn.f32", lv, lv, m)
	k.e("selp.f32", lv, float32(math.Inf(1)), lv, bos)
	li := k.r()
	k.e("add.s32", li, lb, satir)
	k.sakla(tensor.Float32, k.adres(plse, li, 4), lv)
	k.koy(son)
}

// tidSifirDegilTx tx ≠ 0 yüklemi.
func (k *yazici) tidSifirDegilTx(tx yazmac) yazmac {
	p := k.p()
	k.e("setp.ne.u32", p, tx, 0)
	return p
}

// anahtarAraligi sorgu döşemesinin [q0, q0+Br) bakabileceği anahtarları kapsayan döşeme
// aralığı [jBas, jSon) (Bc'nin katlarına yuvarlı): ilk satırın alt ve son satırın üst sınırı
// (sınırlar satırla azalmaz).
func (f *flashOrtak) anahtarAraligi(q0 yazmac) (jBas, jSon yazmac) {
	k := f.k
	sonSatir := k.r()
	k.e("add.s32", sonSatir, q0, f.a.Br-1)
	tq1 := k.r()
	k.e("sub.s32", tq1, f.Tq, 1)
	k.e("min.u32", sonSatir, sonSatir, tq1)
	lo, _ := k.dikkatSinirlari(q0, f.Tq, f.Tk, f.ned, f.pen)
	_, hi := k.dikkatSinirlari(sonSatir, f.Tq, f.Tk, f.ned, f.pen)
	jBas, jSon = k.r(), k.r()
	k.e("div.u32", jBas, lo, f.a.Bc)
	k.e("mul.lo.s32", jBas, jBas, f.a.Bc)
	k.e("mov.u32", jSon, hi)
	return jBas, jSon
}
