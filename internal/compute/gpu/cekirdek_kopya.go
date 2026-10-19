package gpu

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Adımlı kopya, tür dönüşümü, yayınlamalı ekleme ve doldurma çekirdekleri
// ============================================================================

// enCokEksen adımlı çekirdeklerin desteklediği en çok eksen (birleştirmeden sonra).
const enCokEksen = 8

// esBlok eleman bazlı çekirdeklerin blok boyu.
const esBlok = 256

// eksenBirlestir, bütün işlenenler için bitişik olan komşu eksenleri birleştirir ve
// uzunluğu 1 olan eksenleri atar (CPU coalesce ile aynı kural). Boş şekil tek eksene
// ([1], adım 0) döner.
func eksenBirlestir(shape []int, adimlar ...[]int) ([]int, [][]int) {
	ns := make([]int, 0, len(shape))
	nst := make([][]int, len(adimlar))
	for d := range shape {
		if shape[d] == 1 {
			continue
		}
		if son := len(ns) - 1; son >= 0 {
			birlesir := true
			for k := range adimlar {
				if nst[k][son] != adimlar[k][d]*shape[d] {
					birlesir = false
					break
				}
			}
			if birlesir {
				ns[son] *= shape[d]
				for k := range adimlar {
					nst[k][son] = adimlar[k][d]
				}
				continue
			}
		}
		ns = append(ns, shape[d])
		for k := range adimlar {
			nst[k] = append(nst[k], adimlar[k][d])
		}
	}
	if len(ns) == 0 {
		ns = append(ns, 1)
		for k := range nst {
			nst[k] = append(nst[k], 0)
		}
	}
	return ns, nst
}

// ayristir i doğrusal indeksini (satır öncelikli) şekle göre ayrıştırıp her işlenenin
// ofsetini verir. sekil ve adımlar "meta" dizisinden okunur: sekil[d] = meta[d],
// adim[k][d] = meta[(k+1)·nd + d]. Ofsetler işaretli 32 bittir.
func (k *yazici) ayristir(i yazmac, nd, nIslenen int, meta string) []yazmac {
	return k.ayristirTaban(i, nd, nIslenen, 0, meta)
}

// kopyaCekirdegi hedef (td, adımlı) ← kaynak (ts, adımlı); ekle doğruysa hedef += ölçek·kaynak
// (yalnızca float32 hedef).
func kopyaCekirdegi(nd int, ts, td tensor.DType, ekle bool) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("kopya_%d_%s_%s_%v", nd, ts, td, ekle))
		src := k.gosterici("kaynak")
		dst := k.yazilan("hedef")
		n := k.u32p("n")
		olcek := k.f32p("olcek")
		k.dizi("meta", 3*nd)
		i := k.genelIndeks()
		son := k.etiket()
		p := k.p()
		k.e("setp.ge.u32", p, i, n)
		k.gitEger(p, son)
		ofs := k.ayristir(i, nd, 2, "meta")
		v := k.yukle(ts, k.adresS(src, ofs[0], turBoyu(ts)))
		ha := k.adresS(dst, ofs[1], turBoyu(td))
		if ekle {
			eski := k.yukle(td, ha)
			k.e("fma.rn.f32", v, v, olcek, eski)
		}
		k.sakla(td, ha, v)
		k.koy(son)
		return k
	}
}

// doldurCekirdegi hedefin n elemanını v yapar (dt türüne yuvarlanmış).
func doldurCekirdegi(dt tensor.DType) func() *yazici {
	return func() *yazici {
		k := yeniYazici("doldur_" + dt.String())
		dst := k.yazilan("hedef")
		n := k.u32p("n")
		v := k.f32p("deger")
		i := k.genelIndeks()
		son := k.etiket()
		p := k.p()
		k.e("setp.ge.u32", p, i, n)
		k.gitEger(p, son)
		k.sakla(dt, k.adres(dst, i, turBoyu(dt)), v)
		k.koy(son)
		return k
	}
}

// kopyaAdimli hedef görünümüne kaynak görünümünü yazar (ekle: hedef += ölçek·kaynak).
// Şekiller aynıdır; kaynak yayınlanmış olabilir (adım 0).
func (g *arkaUc) kopyaAdimli(hedef, kaynak tensor.View, ekle bool, olcek float32) {
	n := numel(hedef.Shape)
	if n == 0 {
		return
	}
	boyDenetle(n)
	hd, kd := g.depoOf(hedef.S), g.depoOf(kaynak.S)
	if ekle && hd.tur != tensor.Float32 {
		panic("tensor: GPU: ekleyen kopya float32 hedef ister (iç hata)")
	}
	sh, st := eksenBirlestir(hedef.Shape, kaynak.Strides, hedef.Strides)
	// Aynı tür, iki taraf bitişik: sürücünün cihaz içi kopyası.
	if !ekle && kd.tur == hd.tur && len(sh) == 1 && st[0][0] == 1 && st[1][0] == 1 {
		g.dKopyala(hd, hedef.Offset, kd, kaynak.Offset, n)
		return
	}
	if len(sh) > enCokEksen {
		yok(fmt.Sprintf("%d eksenli adımlı kopya (en çok %d)", len(sh), enCokEksen))
	}
	meta := append(append(append([]int{}, sh...), st[0]...), st[1]...)
	c := g.cekirdekAl(fmt.Sprintf("kopya:%d:%d:%d:%v", len(sh), kd.tur, hd.tur, ekle), kopyaCekirdegi(len(sh), kd.tur, hd.tur, ekle))
	iz, bl := duz(n, esBlok)
	g.baslat(c, iz, bl, g.param().oku(kd, kaynak.Offset).yaz(hd, hedef.Offset).u32(n).f32(olcek).dizi(3*len(sh), meta...))
}

// doldur depolamanın bütün elemanlarını v yapar.
func (g *arkaUc) doldur(d *depo, v float32) {
	if d.n == 0 {
		return
	}
	if d.tur == tensor.Float32 {
		g.doldur32(d, f32bit(v))
		return
	}
	boyDenetle(d.n)
	c := g.cekirdekAl("doldur:"+d.tur.String(), doldurCekirdegi(d.tur))
	iz, bl := duz(d.n, esBlok)
	g.baslat(c, iz, bl, g.param().yaz(d, 0).u32(d.n).f32(v))
}

// bitisik görünümün satır öncelikli adımları.
func bitisik(shape []int) []int {
	st := make([]int, len(shape))
	s := 1
	for i := len(shape) - 1; i >= 0; i-- {
		st[i] = s
		s *= shape[i]
	}
	return st
}

func numel(shape []int) int {
	n := 1
	for _, d := range shape {
		n *= d
	}
	return n
}

// yayinAdim, şekli shape ve adımları st olan görünümün hedef şekle yayınlanmış adımları
// (eksik ve 1 uzunluklu eksenlerde 0).
func yayinAdim(st, shape, hedef []int) []int {
	out := make([]int, len(hedef))
	fark := len(hedef) - len(shape)
	for i := range shape {
		if shape[i] != 1 {
			out[fark+i] = st[i]
		}
	}
	return out
}
