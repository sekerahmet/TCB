package gpu

import (
	"sort"

	"tr_dili/internal/hesap/tensor"
)

// ============================================================================
// Gömme (ileri: satır seçme; geri: belirlenimci gruplu toplama)
// ============================================================================
//
// Geri yönde atomik toplama kullanılmaz: indeksler konakta olduğundan konakta tekil
// satırlara göre gruplanır (her grup indeks sırasıyla); her tekil satırı bir blok, grubun
// gradyan satırlarını sırayla ekler. Sonuç CPU arka ucuyla aynı sırada biriktirilir (bit
// düzeyinde aynı) ve her çalıştırmada aynıdır.

const gommeBlok = 128

// gommeCekirdegi out[r, :] = w[idx[r], :]; satır başına blok. Tablo kendi türünde (16 bitlik
// tablo float32'ye açılmadan) okunur.
func gommeCekirdegi(dt tensor.DType) func() *yazici {
	return func() *yazici { return gommeUret(dt) }
}

func gommeUret(dt tensor.DType) *yazici {
	k := yeniYazici("gomme_" + dt.String())
	w := k.gosterici("w")
	idx := k.gosterici("idx")
	out := k.yazilan("cikti")
	D := k.u32p("d")
	r := k.ozel("%ctaid.x")
	tid := k.ozel("%tid.x")
	satir := k.r()
	k.e("ld.global.u32", satir, bel(k.adres(idx, r, 4), 0))
	kaynak, hedef := k.r(), k.r()
	k.e("mul.lo.s32", kaynak, satir, D)
	k.e("mul.lo.s32", hedef, r, D)
	k.satirDongu(tid, D, k.tam(1), k.tam(0), gommeBlok, func(ofs, j yazmac) {
		ks, hs := k.r(), k.r()
		k.e("add.s32", ks, kaynak, j)
		k.e("add.s32", hs, hedef, j)
		k.sakla(tensor.Float32, k.adres(out, hs, 4), k.yukle(dt, k.adres(w, ks, turBoyu(dt))))
	})
	return k
}

// gommeGeriCekirdegi gw[tekil[u], :] += Σ_{p ∈ grup u} g[konum[p], :] (sırayla).
func gommeGeriCekirdegi() *yazici {
	k := yeniYazici("gomme_geri")
	gw := k.yazilan("gw")
	g := k.gosterici("g")
	tekil := k.gosterici("tekil")
	bas := k.gosterici("bas")
	konum := k.gosterici("konum")
	D := k.u32p("d")
	u := k.ozel("%ctaid.x")
	tid := k.ozel("%tid.x")
	satir, p0, p1 := k.r(), k.r(), k.r()
	k.e("ld.global.u32", satir, bel(k.adres(tekil, u, 4), 0))
	ba := k.adres(bas, u, 4)
	k.e("ld.global.u32", p0, bel(ba, 0))
	k.e("ld.global.u32", p1, bel(ba, 4))
	hedef := k.r()
	k.e("mul.lo.s32", hedef, satir, D)
	k.satirDongu(tid, D, k.tam(1), k.tam(0), gommeBlok, func(ofs, j yazmac) {
		hs := k.r()
		k.e("add.s32", hs, hedef, j)
		ha := k.adres(gw, hs, 4)
		acc := k.yukle(tensor.Float32, ha)
		p := k.r()
		k.e("mov.u32", p, p0)
		dongu, bitti := k.etiket(), k.etiket()
		k.koy(dongu)
		b := k.p()
		k.e("setp.ge.u32", b, p, p1)
		k.gitEger(b, bitti)
		kn := k.r()
		k.e("ld.global.u32", kn, bel(k.adres(konum, p, 4), 0))
		gs := k.r()
		k.e("mad.lo.s32", gs, kn, D, j)
		k.e("add.rn.f32", acc, acc, k.yukle(tensor.Float32, k.adres(g, gs, 4)))
		k.e("add.s32", p, p, 1)
		k.git(dongu)
		k.koy(bitti)
		k.sakla(tensor.Float32, ha, acc)
	})
	return k
}

// gommeGruplari indeksleri tekil satırlara göre gruplar: tekil satırlar (artan),
// grupların başlangıçları (son eleman toplam) ve her gruptaki konumlar (artan).
func gommeGruplari(idx []int) (tekil, bas, konum []int32) {
	sayi := map[int]int{}
	for _, v := range idx {
		sayi[v]++
	}
	sirali := make([]int, 0, len(sayi))
	for v := range sayi {
		sirali = append(sirali, v)
	}
	sort.Ints(sirali)
	yer := make(map[int]int, len(sirali))
	bas = make([]int32, len(sirali)+1)
	for i, v := range sirali {
		yer[v] = int(bas[i])
		bas[i+1] = bas[i] + int32(sayi[v])
		tekil = append(tekil, int32(v))
	}
	konum = make([]int32, len(idx))
	for p, v := range idx {
		konum[yer[v]] = int32(p)
		yer[v]++
	}
	return tekil, bas, konum
}
