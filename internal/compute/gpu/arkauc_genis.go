package gpu

import (
	"fmt"
	"math/bits"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Arka uç: genişletilmiş eleman bazlı işlemler (sınırlama, koşullu seçim, atan2
// gradyanı), birikimli toplam ve sıralama.

func (g *arkaUc) Clamp(out tensor.Storage, x tensor.View, lo, hi float32) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	o, son := gt.yaz(out, false)
	xv := gt.oku(x)
	n := xv.Numel()
	if n == 0 {
		return
	}
	boyDenetle(n)
	c := g.cekirdekAl("sinirla:false", sinirlaCekirdegi(false))
	iz, bl := duz(n, esBlok)
	g.baslat(c, iz, bl, g.param().yaz(o, 0).okuV(xv).u32(n).f32(lo).f32(hi))
	son()
}

func (g *arkaUc) ClampGrad(gx, gs tensor.Storage, x tensor.View, lo, hi float32) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	d, son := gt.yaz(gx, true)
	gd := gt.okuD(gs)
	xv := gt.oku(x)
	n := xv.Numel()
	if n == 0 {
		return
	}
	boyDenetle(n)
	c := g.cekirdekAl("sinirla:true", sinirlaCekirdegi(true))
	iz, bl := duz(n, esBlok)
	g.baslat(c, iz, bl, g.param().yaz(d, 0).oku(gd, 0).okuV(xv).u32(n).f32(lo).f32(hi))
	son()
}

// yayinMeta görünümlerin çıktı şekline yayınlanmış adımlarını birleştirir: nd = 0 ise
// hepsi düz bitişiktir (meta boş).
func yayinMeta(islem string, outShape []int, vs ...tensor.View) (int, []int) {
	adimlar := make([][]int, 0, len(vs)+1)
	for _, v := range vs {
		adimlar = append(adimlar, yayinAdim(v.Strides, v.Shape, outShape))
	}
	adimlar = append(adimlar, bitisik(outShape))
	sh, st := eksenBirlestir(outShape, adimlar...)
	nd := len(sh)
	duzMu := nd == 1
	for _, s := range st {
		duzMu = duzMu && s[0] == 1
	}
	if duzMu {
		return 0, nil
	}
	if nd > enCokEksen {
		yok(fmt.Sprintf("%d eksenli yayınlamalı %s (en çok %d)", nd, islem, enCokEksen))
	}
	meta := append([]int{}, sh...)
	for _, s := range st[:len(vs)] {
		meta = append(meta, s...)
	}
	return nd, meta
}

func (g *arkaUc) Where(out tensor.Storage, outShape []int, c, a, b tensor.View) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	o, son := gt.yaz(out, false)
	c, a, b = gt.okuAdimli(c), gt.okuAdimli(a), gt.okuAdimli(b)
	n := numel(outShape)
	if n == 0 {
		return
	}
	boyDenetle(n)
	nd, meta := yayinMeta("koşullu seçim", outShape, c, a, b)
	k := g.cekirdekAl(fmt.Sprintf("secim:%d", nd), secimCekirdegi(nd))
	iz, bl := duz(n, esBlok)
	g.baslat(k, iz, bl, g.param().okuV(c).okuV(a).okuV(b).yaz(o, 0).u32(n).dizi(4*nd, meta...))
	son()
}

func (g *arkaUc) WhereGrad(dst tensor.Storage, dstShape, outShape []int, gs tensor.Storage, c tensor.View, sec bool) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	d, son := gt.yaz(dst, true)
	gd := gt.okuD(gs)
	c = gt.okuAdimli(c)
	terim := tAsecO
	if !sec {
		terim = tAsecOdegil
	}
	g.indirge(indirgeIs{a: gd, o: g.depoOf(c.S), oOf: c.Offset, stO: yayinAdim(c.Strides, c.Shape, outShape),
		cikti: d, sekil: outShape, stA: bitisik(outShape), st: yayinAdim(bitisik(dstShape), dstShape, outShape),
		terim: terim, ekle: true, olcek: 1})
	son()
}

func (g *arkaUc) Atan2Grad(ga, gb tensor.Storage, aShape, bShape, outShape []int, gs tensor.Storage, a, b tensor.View) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	n := numel(outShape)
	if n == 0 {
		return
	}
	boyDenetle(n)
	da, sonA := gt.yaz(ga, true)
	db, sonB := gt.yaz(gb, true)
	gd := gt.okuD(gs)
	a, b = gt.okuAdimli(a), gt.okuAdimli(b)
	ta, tb := gt.tampon(n, false), gt.tampon(n, false)
	nd, meta := yayinMeta("atan2 gradyanı", outShape, a, b)
	k := g.cekirdekAl(fmt.Sprintf("atan2_turev:%d", nd), atan2TurevCekirdegi(nd))
	iz, bl := duz(n, esBlok)
	g.baslat(k, iz, bl, g.param().okuV(a).okuV(b).oku(gd, 0).yaz(ta, 0).yaz(tb, 0).u32(n).dizi(3*nd, meta...))
	for _, h := range []struct {
		d  *depo
		t  *depo
		sh []int
	}{{da, ta, aShape}, {db, tb, bShape}} {
		if h.d == nil {
			continue
		}
		g.indirge(indirgeIs{a: h.t, cikti: h.d, sekil: outShape, stA: bitisik(outShape),
			st: yayinAdim(bitisik(h.sh), h.sh, outShape), terim: tA, ekle: true, olcek: 1})
	}
	sonA()
	sonB()
}

// ----------------------------------------------------------------------------
// Birikimli toplam
// ----------------------------------------------------------------------------

// eksenUcu (dış, eksen, iç) üçlüsü.
func eksenUcu(shape []int, axis int) (dis, L, ic int) {
	dis, ic = 1, 1
	for _, d := range shape[:axis] {
		dis *= d
	}
	for _, d := range shape[axis+1:] {
		ic *= d
	}
	return dis, shape[axis], ic
}

// birikim satır başına bir blokla birikimli toplam başlatır.
func (g *arkaUc) birikim(y *depo, x tensor.View, shape []int, axis int, ters, ekle bool) {
	dis, L, ic := eksenUcu(shape, axis)
	satir := dis * ic
	if satir == 0 || L == 0 {
		return
	}
	boyDenetle(dis * L * ic)
	B := ikiKuvvet((L+7)/8, 32, 256)
	C := (L + B - 1) / B
	k := g.cekirdekAl(fmt.Sprintf("birikim:%d:%v:%v", B, ters, ekle), birikimCekirdegi(B, ters, ekle))
	g.baslat(k, [3]uint32{uint32(satir), 1, 1}, [3]uint32{uint32(B), 1, 1},
		g.param().okuV(x).yaz(y, 0).u32(L).u32(ic).u32(C))
}

func (g *arkaUc) CumSum(out tensor.Storage, x tensor.View, axis int) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	o, son := gt.yaz(out, false)
	g.birikim(o, gt.oku(x), x.Shape, axis, false, false)
	son()
}

func (g *arkaUc) CumSumGrad(gx, gs tensor.Storage, shape []int, axis int) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	d, son := gt.yaz(gx, true)
	gd := gt.okuD(gs)
	g.birikim(d, tensor.View{S: gd, Shape: shape, Strides: bitisik(shape)}, shape, axis, true, true)
	son()
}

// ----------------------------------------------------------------------------
// Sıralama
// ----------------------------------------------------------------------------

func (g *arkaUc) Sort(vals, idx tensor.Storage, x tensor.View, axis, kSay int, desc bool) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	v, sonV := gt.yaz(vals, false)
	id := g.depoOf(idx)
	xv := gt.oku(x)
	dis, L, ic := eksenUcu(x.Shape, axis)
	satir := dis * ic
	if satir == 0 || kSay == 0 {
		return
	}
	boyDenetle(dis * L * ic)
	P := 1 << bits.Len(uint(L-1))
	if L <= 1 {
		P = 1
	}
	degerli := v != nil
	if P <= ortakSiralamaSiniri {
		k := g.cekirdekAl(fmt.Sprintf("sirala_ortak:%d:%v:%v", P, desc, degerli), siralaOrtakCekirdegi(P, desc, degerli))
		p := g.param().okuV(xv).yaz(id, 0)
		if degerli {
			p.yaz(v, 0)
		}
		g.baslat(k, [3]uint32{uint32(satir), 1, 1}, [3]uint32{uint32(max(1, min(P/2, 512))), 1, 1},
			p.u32(L).u32(ic).u32(kSay))
		sonV()
		return
	}
	g.siralaGenel(v, id, xv, satir, L, ic, kSay, P, desc)
	sonV()
}

// siralaGenel P > ortakSiralamaSiniri satırları genel bellekteki anahtar tamponunda her
// bitonik adım için bir başlatmayla sıralar.
func (g *arkaUc) siralaGenel(v, id *depo, xv tensor.View, satir, L, ic, kSay, P int, desc bool) {
	toplam := satir * P
	boyDenetle(toplam)
	lp := bits.Len(uint(P)) - 1
	buf := g.ayir(2*toplam, tensor.Float32, false) // 64 bitlik anahtarlar
	defer g.birak(buf)
	kb := g.cekirdekAl(fmt.Sprintf("sirala_baslat:%v", desc), siralaBaslatCekirdegi(desc))
	iz, bl := duz(toplam, esBlok)
	g.baslat(kb, iz, bl, g.param().okuV(xv).yaz(buf, 0).u32(toplam).u32(lp).u32(L).u32(ic))
	ka := g.cekirdekAl("sirala_adim", siralaAdimCekirdegi)
	iz, bl = duz(toplam/2, esBlok)
	for kk := 2; kk <= P; kk *= 2 {
		for j := kk / 2; j > 0; j /= 2 {
			g.baslat(ka, iz, bl, g.param().yaz(buf, 0).u32(toplam/2).u32(lp).u32(bits.Len(uint(j))-1).u32(kk))
		}
	}
	degerli := v != nil
	kc := g.cekirdekAl(fmt.Sprintf("sirala_cikti:%v", degerli), siralaCiktiCekirdegi(degerli))
	iz, bl = duz(satir*kSay, esBlok)
	p := g.param().oku(buf, 0).okuV(xv).yaz(id, 0)
	if degerli {
		p.yaz(v, 0)
	}
	g.baslat(kc, iz, bl, p.u32(satir*kSay).u32(lp).u32(L).u32(ic).u32(kSay))
}

func (g *arkaUc) GatherGrad(gx, gs, idx tensor.Storage, shape []int, axis, kSay int) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	d, son := gt.yaz(gx, true)
	gd, id := gt.okuD(gs), gt.okuD(idx)
	dis, L, ic := eksenUcu(shape, axis)
	n := dis * kSay * ic
	if n == 0 {
		return
	}
	boyDenetle(dis * L * ic)
	k := g.cekirdekAl("topla_geri", toplaGeriCekirdegi)
	iz, bl := duz(n, esBlok)
	g.baslat(k, iz, bl, g.param().yaz(d, 0).oku(gd, 0).oku(id, 0).u32(n).u32(L).u32(ic).u32(kSay))
	son()
}
