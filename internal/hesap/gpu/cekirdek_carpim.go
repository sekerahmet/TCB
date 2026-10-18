package gpu

import (
	"fmt"

	"tr_dili/internal/hesap/tensor"
)

// ============================================================================
// Toplu matris çarpımı (paylaşımlı bellek döşemeli, float32)
// ============================================================================
//
// C[z] (+)= op(A[z])·op(B[z]); her blok C'nin BM×BN'lik bir döşemesini hesaplar. K
// ekseni BK'lık dilimlerle gezilir: A ve B döşemeleri paylaşımlı belleğe yüklenir (sınır
// dışı elemanlar 0), engelden sonra her iş parçacığı TM×TN'lik kayıt bloğunu fma ile
// biriktirir. İş parçacığının satırları w = min(TM, 4)'lü bitişik gruplardır:
// g·(BM·w/TM) + ty·w + i (g < TM/w, i < w), sütunları aynı biçimde tx'ten; böylece
// paylaşımlı bellekten her grup tek okumayla (w = 4: ld.shared.v4) gelir (A satırları çeyrek warp'ta yayınla, B
// sütunları çakışmasız). Paylaşımlı dizilerin satırları 4 elemanla dolgulanır (yükleme
// sırasında bank çakışmalarını azaltır, 16 bayt hizayı korur). Döşeme boyları arka ucun
// ayarıdır (carpimAyari), kullanıcıya görünmez. Makinede üreticinin kitaplığı varsa çarpım
// onunla yapılır (carpim_kitaplik.go); bu çekirdek yedektir.

// carpimAyari matris çarpımı döşeme boyları (hepsi ikinin kuvveti).
type carpimAyari struct {
	BM, BN, BK, TM, TN int
}

var (
	// varsayilanCarpim küçük ve orta matrisler için.
	varsayilanCarpim = carpimAyari{64, 64, 16, 4, 4}
	// buyukCarpim büyük matrisler için (iş parçacığı başına 8×8).
	buyukCarpim = carpimAyari{128, 128, 8, 8, 8}
)

func (a carpimAyari) is() int { return (a.BM / a.TM) * (a.BN / a.TN) }

// gecerli ayarın yükleme bölüşümünün tam olduğunu denetler.
func (a carpimAyari) gecerli() bool {
	t := a.is()
	ikiK := func(n int) bool { return n > 0 && n&(n-1) == 0 }
	return ikiK(a.BM) && ikiK(a.BN) && ikiK(a.BK) && ikiK(a.TM) && ikiK(a.TN) && t <= 1024 &&
		(a.BM*a.BK)%t == 0 && (a.BN*a.BK)%t == 0 && 4*(a.BK*(a.BM+4)+a.BK*(a.BN+4)) <= 48<<10
}

func log2(n int) int {
	r := 0
	for n > 1 {
		n >>= 1
		r++
	}
	return r
}

// donemeYukle bir döşemenin q. yükleme payının satır/sütun indekslerini verir: e = tid +
// T·q; iç eksen (genel bellekte bitişik olan) e'nin alt bitleridir.
func (k *yazici) donemeIndeks(tid yazmac, q, T, ic int) (disI, icI yazmac) {
	e := k.r()
	k.e("add.s32", e, tid, q*T)
	disI, icI = k.r(), k.r()
	k.e("and.b32", icI, e, ic-1)
	k.e("shr.u32", disI, e, log2(ic))
	return disI, icI
}

// carpimTasarimi döşeme yüklemesinin bir payı: genel bellekteki satır/sütun (döşeme
// içi) ve paylaşımlı bellek ofseti.
type carpimPayi struct {
	dis, ic yazmac
	smOfs   yazmac
}

func carpimCekirdegi(a carpimAyari, transA, transB, uzerine bool) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("carpim_%d_%d_%d_%d_%d_%v_%v_%v", a.BM, a.BN, a.BK, a.TM, a.TN, transA, transB, uzerine))
		pa, pb := k.gosterici("a"), k.gosterici("b")
		pc := k.yazilan("c")
		M, N, K := k.u32p("m"), k.u32p("n"), k.u32p("k")
		T := a.is()
		sa := k.ortak("sa", 4*a.BK*(a.BM+4))
		sb := k.ortak("sb", 4*a.BK*(a.BN+4))
		tid := k.ozel("%tid.x")
		tx, ty := k.r(), k.r()
		k.e("and.b32", tx, tid, a.BN/a.TN-1)
		k.e("shr.u32", ty, tid, log2(a.BN/a.TN))
		m0, n0, z := k.r(), k.r(), k.ozel("%ctaid.z")
		k.e("mul.lo.s32", m0, k.ozel("%ctaid.y"), a.BM)
		k.e("mul.lo.s32", n0, k.ozel("%ctaid.x"), a.BN)
		// Toplu iş kaydırmaları (64 bit).
		for _, s := range []struct {
			p    yazmac
			x, y yazmac
		}{{pa, M, K}, {pb, K, N}, {pc, M, N}} {
			mk, off := k.r(), k.rd()
			k.e("mul.lo.s32", mk, s.x, s.y)
			k.e("mul.wide.u32", off, z, mk)
			k.e("shl.b64", off, off, 2)
			k.e("add.s64", s.p, s.p, off)
		}
		// Yükleme payları (döngü dışında hesaplanır).
		aPay := k.carpimPaylari(tid, a.BM, a.BK, a.BM+4, T, transA)
		bPay := k.carpimPaylari(tid, a.BN, a.BK, a.BN+4, T, !transB)
		acc := make([]yazmac, a.TM*a.TN)
		for i := range acc {
			acc[i] = k.sabit(0)
		}
		k0 := k.tam(0)
		dongu, bitti := k.etiket(), k.etiket()
		k.koy(dongu)
		p := k.p()
		k.e("setp.ge.u32", p, k0, K)
		k.gitEger(p, bitti)
		k.carpimYukle(aPay, pa, sa, m0, k0, M, K, transA)
		k.carpimYukle(bPay, pb, sb, n0, k0, N, K, !transB)
		k.engel()
		k.carpimHesapla(a, acc, sa, sb, tx, ty)
		k.engel()
		k.e("add.s32", k0, k0, a.BK)
		k.git(dongu)
		k.koy(bitti)
		k.carpimYaz(a, acc, pc, m0, n0, tx, ty, M, N, uzerine)
		return k
	}
}

// carpimPaylari bir döşemenin (dış boy D, K boyu BK) yükleme paylarını kurar. kBitisik
// doğruysa genel bellekte K ekseni bitişiktir (A: transA değil; B: transB), değilse dış
// eksen bitişiktir. Paylaşımlı bellekte düzen [k][dış] (satır adımı sat).
func (k *yazici) carpimPaylari(tid yazmac, D, BK, sat, T int, disBitisik bool) []carpimPayi {
	var paylar []carpimPayi
	for q := 0; q < D*BK/T; q++ {
		var dis, kk yazmac
		if disBitisik {
			kk, dis = k.donemeIndeks(tid, q, T, D)
		} else {
			dis, kk = k.donemeIndeks(tid, q, T, BK)
		}
		so := k.r()
		k.e("mad.lo.s32", so, kk, sat, dis)
		k.e("shl.b32", so, so, 2)
		paylar = append(paylar, carpimPayi{dis: dis, ic: kk, smOfs: so})
	}
	return paylar
}

// carpimYukle payları genel bellekten (sınır dışı 0) paylaşımlı belleğe taşır. Genel
// bellekteki eleman: disBitisik ise (k0+kk)·Dboy + (d0+dis), değilse (d0+dis)·K + k0+kk.
func (k *yazici) carpimYukle(paylar []carpimPayi, ptr, sm, d0, k0, Dboy, K yazmac, disBitisik bool) {
	for _, py := range paylar {
		gd, gk := k.r(), k.r()
		k.e("add.s32", gd, d0, py.dis)
		k.e("add.s32", gk, k0, py.ic)
		p1, p2 := k.p(), k.p()
		k.e("setp.lt.u32", p1, gd, Dboy)
		k.e("setp.lt.u32", p2, gk, K)
		k.e("and.pred", p1, p1, p2)
		idx := k.r()
		if disBitisik {
			k.e("mad.lo.s32", idx, gk, Dboy, gd)
		} else {
			k.e("mad.lo.s32", idx, gd, K, gk)
		}
		v := k.sabit(0)
		k.eger(p1, "ld.global.f32", v, bel(k.adres(ptr, idx, 4), 0))
		ad := k.r()
		k.e("add.s32", ad, sm, py.smOfs)
		k.e("st.shared.f32", bel(ad, 0), v)
	}
}

// grupGenisligi iş parçacığının bitişik satır (sütun) grubunun boyu.
func grupGenisligi(T int) int { return min(T, 4) }

// grupOfseti iş parçacığının i. satırının (ya da sütununun) döşeme içindeki ofsetinin
// iş parçacığından bağımsız kısmı: g·(D·w/T) + i%w (g = i/w).
func grupOfseti(i, D, T int) int {
	w := grupGenisligi(T)
	return i/w*(D*w/T) + i%w
}

// grupYukle paylaşımlı bellekten bir grubu (w = 4, 2, 1 eleman) yükler.
func (k *yazici) grupYukle(adr yazmac, ofset int, r []yazmac) {
	if len(r) == 1 {
		k.e("ld.shared.f32", r[0], bel(adr, ofset))
		return
	}
	k.yukleVIcine("shared", "", adr, ofset, r)
}

// carpimHesapla döşemenin BK adımını açılmış olarak biriktirir.
func (k *yazici) carpimHesapla(a carpimAyari, acc []yazmac, sa, sb, tx, ty yazmac) {
	wm, wn := grupGenisligi(a.TM), grupGenisligi(a.TN)
	at, bt := k.r(), k.r()
	k.e("mad.lo.s32", at, ty, 4*wm, sa)
	k.e("mad.lo.s32", bt, tx, 4*wn, sb)
	av, bv := make([]yazmac, a.TM), make([]yazmac, a.TN)
	for i := range av {
		av[i] = k.f()
	}
	for j := range bv {
		bv[j] = k.f()
	}
	for kk := 0; kk < a.BK; kk++ {
		for i := 0; i < a.TM; i += wm {
			k.grupYukle(at, 4*(kk*(a.BM+4)+grupOfseti(i, a.BM, a.TM)), av[i:i+wm])
		}
		for j := 0; j < a.TN; j += wn {
			k.grupYukle(bt, 4*(kk*(a.BN+4)+grupOfseti(j, a.BN, a.TN)), bv[j:j+wn])
		}
		for i := range av {
			for j := range bv {
				c := acc[i*a.TN+j]
				k.e("fma.rn.f32", c, av[i], bv[j], c)
			}
		}
	}
}

// carpimYaz biriktirilen bloğu C'ye yazar (uzerine değilse ekler).
func (k *yazici) carpimYaz(a carpimAyari, acc []yazmac, pc, m0, n0, tx, ty, M, N yazmac, uzerine bool) {
	for i := 0; i < a.TM; i++ {
		row := k.r()
		k.e("mad.lo.s32", row, ty, grupGenisligi(a.TM), m0)
		k.e("add.s32", row, row, grupOfseti(i, a.BM, a.TM))
		for j := 0; j < a.TN; j++ {
			col := k.r()
			k.e("mad.lo.s32", col, tx, grupGenisligi(a.TN), n0)
			k.e("add.s32", col, col, grupOfseti(j, a.BN, a.TN))
			atla := k.etiket()
			p1, p2 := k.p(), k.p()
			k.e("setp.ge.u32", p1, row, M)
			k.e("setp.ge.u32", p2, col, N)
			k.e("or.pred", p1, p1, p2)
			k.gitEger(p1, atla)
			idx := k.r()
			k.e("mad.lo.s32", idx, row, N, col)
			ha := k.adres(pc, idx, 4)
			v := acc[i*a.TN+j]
			if !uzerine {
				eski := k.yukle(tensor.Float32, ha)
				k.e("add.rn.f32", v, eski, v)
			}
			k.sakla(tensor.Float32, ha, v)
			k.koy(atla)
		}
	}
}
