package gpu

import (
	"fmt"
	"unsafe"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Arka uç: eksen indirgemeleri (Sum, SumAll, Max, MaxGrad, ArgMax).

// eksensiz şeklin axis ekseni çıkarılmış hali ve adımları.
func eksensiz(sh, st []int, axis int) ([]int, []int) {
	s := append(append([]int{}, sh[:axis]...), sh[axis+1:]...)
	a := append(append([]int{}, st[:axis]...), st[axis+1:]...)
	return s, a
}

func (g *arkaUc) Sum(out tensor.Storage, x tensor.View, axis int, scale float32) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	o, son := gt.yaz(out, false)
	x = gt.okuAdimli(x)
	// Çıktı adımları: eksensiz bitişik düzen, indirgenen eksende 0.
	ks, _ := eksensiz(x.Shape, x.Strides, axis)
	kst := bitisik(ks)
	st := append(append(append([]int{}, kst[:axis]...), 0), kst[axis:]...)
	g.indirge(indirgeIs{a: g.depoOf(x.S), aOf: x.Offset, cikti: o, sekil: x.Shape, stA: x.Strides, st: st, terim: tA, olcek: scale})
	son()
}

func (g *arkaUc) SumAll(out tensor.Storage, x tensor.View, scale float32) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	o, son := gt.yaz(out, false)
	x = gt.okuAdimli(x)
	g.indirge(indirgeIs{a: g.depoOf(x.S), aOf: x.Offset, cikti: o, sekil: x.Shape, stA: x.Strides,
		st: make([]int, len(x.Shape)), terim: tA, olcek: scale})
	son()
}

// argmax çekirdeğini kurar ve başlatır: kip 0 değer, 1 indeks, 2 gradyan.
func (g *arkaUc) argmax(kip int, out, gd *depo, x tensor.View, axis int, gxSt []int) {
	ks, kx := eksensiz(x.Shape, x.Strides, axis)
	var kg []int
	if gxSt != nil {
		ks, kg = eksensiz(x.Shape, gxSt, axis)
	} else {
		kg = make([]int, len(ks))
	}
	sh, st := eksenBirlestir(ks, kx, kg)
	K, L := numel(sh), x.Shape[axis]
	if K == 0 || (kip == 2 && L == 0) {
		return
	}
	boyDenetle(K)
	if len(sh) > enCokEksen {
		yok(fmt.Sprintf("%d eksenli en büyük değer araması (en çok %d)", len(sh), enCokEksen))
	}
	sg := 0
	if gxSt != nil {
		sg = gxSt[axis]
	}
	meta := append(append(append(append([]int{}, sh...), st[0]...), st[1]...), L, x.Strides[axis], sg)
	c := g.cekirdekAl(fmt.Sprintf("argmax:%d:%d", len(sh), kip), argmaxCekirdegi(len(sh), kip))
	p := g.param().okuV(x).yaz(out, 0)
	if gd != nil {
		p.oku(gd, 0)
	} else {
		p.bos()
	}
	iz, bl := duz(K, esBlok)
	g.baslat(c, iz, bl, p.u32(K).dizi(len(meta), meta...))
}

func (g *arkaUc) Max(out tensor.Storage, x tensor.View, axis int) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	o, son := gt.yaz(out, false)
	g.argmax(0, o, nil, gt.okuAdimli(x), axis, nil)
	son()
}

func (g *arkaUc) MaxGrad(gx, gs tensor.Storage, x tensor.View, axis int) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	d, son := gt.yaz(gx, true)
	g.argmax(2, d, gt.okuD(gs), gt.okuAdimli(x), axis, bitisik(x.Shape))
	son()
}

func (g *arkaUc) ArgMax(x tensor.View, axis int) []int {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	ks, _ := eksensiz(x.Shape, x.Strides, axis)
	n := numel(ks)
	tmp := gt.tampon(n, false)
	g.argmax(1, tmp, nil, gt.okuAdimli(x), axis, nil)
	f := make([]uint32, n)
	if n > 0 {
		g.indir(unsafe.Pointer(&f[0]), tmp.adr, 4*n)
	}
	r := make([]int, n)
	for i, v := range f {
		r[i] = int(v)
	}
	return r
}
