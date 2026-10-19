package gpu

import (
	"fmt"
	"math/bits"

	"tr_dili/internal/hesap/tensor"
)

// Arka uç: eleman bazlı işlemler, gradyan biriktirme ve genel indirgeme planı.

// indirgeIs genel indirgemenin bir çağrısı: ortak gezinme şekli üzerinde A, O (ve Y = A
// adımlı) okunur; çıktı adımı 0 olan eksenler indirgenir.
type indirgeIs struct {
	a, o, y      *depo
	aOf, oOf     int
	cikti        *depo
	ciktiOf      int
	sekil        []int
	stA, stO, st []int // st: çıktı adımları
	terim        terimTuru
	ekle         bool
	olcek        float32
}

// ikiKuvvet n'den küçük olmayan en küçük ikinin kuvveti (sınırlı).
func ikiKuvvet(n, enAz, enCok int) int {
	if n <= enAz {
		return enAz
	}
	return min(1<<bits.Len(uint(n-1)), enCok)
}

func (g *arkaUc) indirge(is indirgeIs) {
	var kSh, kA, kO, kC, rSh, rA, rO []int
	for d, n := range is.sekil {
		if n == 1 {
			continue
		}
		oa := 0
		if is.stO != nil {
			oa = is.stO[d]
		}
		if is.st[d] != 0 || n == 0 {
			kSh, kA, kO, kC = append(kSh, n), append(kA, is.stA[d]), append(kO, oa), append(kC, is.st[d])
		} else {
			rSh, rA, rO = append(rSh, n), append(rA, is.stA[d]), append(rO, oa)
		}
	}
	kSh, ks := eksenBirlestir(kSh, kA, kO, kC)
	rSh, rs := eksenBirlestir(rSh, rA, rO)
	K, R := numel(kSh), numel(rSh)
	if K == 0 {
		return
	}
	boyDenetle(K)
	boyDenetle(R)
	if len(kSh) > enCokEksen || len(rSh) > enCokEksen {
		yok(fmt.Sprintf("%d korunan, %d indirgenen eksenli indirgeme (en çok %d)", len(kSh), len(rSh), enCokEksen))
	}
	ayar := indirgeAyar{nk: len(kSh), nr: len(rSh), terim: is.terim, ekle: is.ekle}
	// Çıktı başına blok: indirgenen eksen yeterince uzunsa; ancak çıktılar bitişik
	// sütunlarsa (A'nın korunan iç adımı 1) ve çok sayıdaysa iş parçacığı başına çıktı
	// okumaları birleştirir.
	sutun := ks[0][len(kSh)-1] == 1 && K >= 256
	if R >= 64 && !sutun {
		ayar.blok = ikiKuvvet(R, 32, 256)
	}
	meta := append(append(append(append(append(append(append([]int{}, kSh...), ks[0]...), ks[1]...), ks[2]...), rSh...), rs[0]...), rs[1]...)
	if S := indirgeParcaSayisi(K, R, ayar.blok); S > 1 {
		g.indirgeIkiAsama(is, ayar, meta, kSh, ks[2], K, R, S)
		return
	}
	c := g.cekirdekAl(ayar.anahtar(), indirgeCekirdegi(ayar))
	p := g.param().oku(is.a, is.aOf)
	if is.o != nil {
		p.oku(is.o, is.oOf)
	} else {
		p.bos()
	}
	if is.y != nil {
		p.oku(is.y, 0)
	} else {
		p.bos()
	}
	p.yaz(is.cikti, is.ciktiOf).u32(K).u32(R).f32(is.olcek).dizi(len(meta), meta...)
	if ayar.blok > 0 {
		g.baslat(c, [3]uint32{uint32(K), 1, 1}, [3]uint32{uint32(ayar.blok), 1, 1}, p)
		return
	}
	iz, bl := duz(K, esBlok)
	g.baslat(c, iz, bl, p)
}

// indirgeParcaSayisi indirgenen aralığın kaç parçaya bölüneceği: çıktı az, indirgenen eksen
// uzunken (ör. sapma gradyanı K = 512, R = 8192; hepsinin toplamı K = 1) tek aşamada çok az
// iş parçacığı çalışırdı. Parça sayısı yalnızca K ve R'ye bağlıdır (belirlenimci).
func indirgeParcaSayisi(K, R, blok int) int {
	is := K // aynı anda çalışan iş parçacığı
	if blok > 0 {
		is = K * blok
	}
	const hedef = 1 << 17
	if is >= hedef/2 || R < 2048 {
		return 1
	}
	return min((hedef+is-1)/is, R/512, 512)
}

// indirgeIkiAsama belirlenimci iki aşamalı indirgeme: her parça kısmi toplamını yazar,
// ikinci aşama kısmi toplamları parça sırasıyla toplayıp çıktıya (ölçek, ekleme ve çıktı
// adımlarıyla) yazar.
func (g *arkaUc) indirgeIkiAsama(is indirgeIs, ayar indirgeAyar, meta, kSh, kSt []int, K, R, S int) {
	P := (R + S - 1) / S
	kismi := g.ayir(S*K, tensor.Float32, false)
	defer g.birak(kismi)
	ayar.parcali, ayar.ekle = true, false
	c := g.cekirdekAl(ayar.anahtar(), indirgeCekirdegi(ayar))
	p := g.param().oku(is.a, is.aOf)
	if is.o != nil {
		p.oku(is.o, is.oOf)
	} else {
		p.bos()
	}
	p.okuYa(is.y).yaz(kismi, 0).u32(K).u32(R).f32(1).dizi(len(meta), meta...).u32(P)
	if ayar.blok > 0 {
		g.baslat(c, [3]uint32{uint32(K), uint32(S), 1}, [3]uint32{uint32(ayar.blok), 1, 1}, p)
	} else {
		g.baslat(c, [3]uint32{bloklar(K, esBlok), uint32(S), 1}, [3]uint32{esBlok, 1, 1}, p)
	}
	sekil := append([]int{S}, kSh...)
	g.indirge(indirgeIs{a: kismi, cikti: is.cikti, ciktiOf: is.ciktiOf, sekil: sekil,
		stA: append([]int{K}, bitisik(kSh)...), st: append([]int{0}, kSt...), terim: tA, ekle: is.ekle, olcek: is.olcek})
}

// duzV depolamanın tamamına bitişik görünüm.
func duzV(d *depo) tensor.View { return tensor.View{S: d, Shape: []int{d.n}, Strides: []int{1}} }

func (g *arkaUc) AddTo(dst, src tensor.Storage, scale float32) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	d, son := gt.yaz(dst, true)
	s := gt.okuD(src)
	v := duzV(d)
	g.kopyaAdimli(v, tensor.View{S: s, Shape: v.Shape, Strides: v.Strides}, true, scale)
	son()
}

func (g *arkaUc) ScatterAdd(dst tensor.Storage, dstOff int, dstStrides []int, src tensor.Storage, shape []int) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	d, son := gt.yaz(dst, true)
	s := gt.okuD(src)
	g.indirge(indirgeIs{a: s, cikti: d, ciktiOf: dstOff, sekil: shape, stA: bitisik(shape), st: dstStrides,
		terim: tA, ekle: true, olcek: 1})
	son()
}

func (g *arkaUc) ExpandAdd(dst tensor.Storage, dstShape []int, src tensor.Storage, srcShape []int, scale float32) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	d, son := gt.yaz(dst, true)
	s := gt.okuD(src)
	kv := tensor.View{S: s, Shape: dstShape, Strides: yayinAdim(bitisik(srcShape), srcShape, dstShape)}
	g.kopyaAdimli(tensor.View{S: d, Shape: dstShape, Strides: bitisik(dstShape)}, kv, true, scale)
	son()
}

func esitSekil(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (g *arkaUc) Binary(op tensor.BinOp, out tensor.Storage, outShape []int, a, b tensor.View) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	o, son := gt.yaz(out, false)
	a, b = gt.okuAdimli(a), gt.okuAdimli(b)
	n := numel(outShape)
	if n == 0 {
		return
	}
	boyDenetle(n)
	sa := yayinAdim(a.Strides, a.Shape, outShape)
	sb := yayinAdim(b.Strides, b.Shape, outShape)
	sh, st := eksenBirlestir(outShape, sa, sb, bitisik(outShape))
	nd := len(sh)
	if nd == 1 && st[0][0] == 1 && st[1][0] == 1 {
		nd = 0 // üçü de düz bitişik
	}
	if nd > enCokEksen {
		yok(fmt.Sprintf("%d eksenli yayınlamalı ikili işlem (en çok %d)", nd, enCokEksen))
	}
	var meta []int
	if nd > 0 {
		meta = append(append(append(meta, sh...), st[0]...), st[1]...)
	}
	c := g.cekirdekAl(fmt.Sprintf("ikili:%d:%d", op, nd), ikiliCekirdegi(op, nd))
	iz, bl := duz(n, esBlok)
	g.baslat(c, iz, bl, g.param().okuV(a).okuV(b).yaz(o, 0).u32(n).dizi(3*nd, meta...))
	son()
}

// okuAdimli float32 görünümü olduğu gibi (adımlarıyla) bırakır; 16 bitlik görünümü
// float32 bitişiğe açar.
func (t *gecici) okuAdimli(v tensor.View) tensor.View {
	if t.g.depoOf(v.S).tur == tensor.Float32 {
		return v
	}
	return t.oku(v)
}

var gradTerimi = map[tensor.GradMode]struct {
	t     terimTuru
	olcek float32
}{
	tensor.GradPlus: {tA, 1}, tensor.GradMinus: {tA, -1}, tensor.GradMul: {tAO, 1},
	tensor.GradDiv: {tAbolO, 1}, tensor.GradDivB: {tAYbO, -1},
}

func (g *arkaUc) BinaryGrad(mode tensor.GradMode, dst tensor.Storage, dstShape, outShape []int, gs tensor.Storage, o tensor.View, y tensor.Storage) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	d, son := gt.yaz(dst, true)
	gd := gt.okuD(gs)
	yd := gt.okuD(y)
	tm := gradTerimi[mode]
	is := indirgeIs{a: gd, y: yd, cikti: d, sekil: outShape, stA: bitisik(outShape),
		st: yayinAdim(bitisik(dstShape), dstShape, outShape), terim: tm.t, ekle: true, olcek: tm.olcek}
	if o.S != nil && tm.t != tA {
		o = gt.okuAdimli(o)
		is.o, is.oOf, is.stO = g.depoOf(o.S), o.Offset, yayinAdim(o.Strides, o.Shape, outShape)
	}
	g.indirge(is)
	son()
}

func (g *arkaUc) Unary(op tensor.UnOp, p float32, out tensor.Storage, x tensor.View) {
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
	kip := ""
	if op == tensor.OpPow {
		kip = usKipi(p)
	}
	c := g.cekirdekAl(fmt.Sprintf("tekli:%d:%s", op, kip), tekliCekirdegi(op, kip))
	iz, bl := duz(n, esBlok)
	g.baslat(c, iz, bl, g.param().okuV(xv).yaz(o, 0).u32(n).f32(p))
	son()
}

func (g *arkaUc) UnaryGrad(op tensor.UnOp, p float32, gx, gs tensor.Storage, x tensor.View, y tensor.Storage) {
	g.gir()
	defer g.cik()
	if op == tensor.OpPow && p == 0 || op == tensor.OpFloor || op == tensor.OpCeil || op == tensor.OpSign {
		return // x^0 sabittir, taban/tavan/işaret parçalı sabittir: gradyan 0
	}
	gt := g.gecici()
	defer gt.bitir()
	d, son := gt.yaz(gx, true)
	gd, yd := gt.okuD(gs), gt.okuD(y)
	xv := gt.oku(x)
	n := xv.Numel()
	if n == 0 {
		return
	}
	boyDenetle(n)
	kip := ""
	if op == tensor.OpPow {
		kip = usKipi(p)
	}
	c := g.cekirdekAl(fmt.Sprintf("tekli_turev:%d:%s", op, kip), tekliTurevCekirdegi(op, kip))
	iz, bl := duz(n, esBlok)
	pr := g.param().yaz(d, 0).oku(gd, 0).okuV(xv)
	if yd == nil {
		yd = gd // y'yi kullanmayan işlemler için okunabilir yer tutucu (aynı boyda)
	}
	pr.oku(yd, 0)
	g.baslat(c, iz, bl, pr.u32(n).f32(p))
	son()
}
