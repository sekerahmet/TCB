package gpu

import (
	"fmt"

	"tr_dili/internal/hesap/tensor"
)

// Arka uç: softmax, çapraz entropi, normlar ve gömme.

// eksenDuzeni şekli eksen etrafında (dış, eksen, iç) üçlüsüne ayırır.
func eksenDuzeni(shape []int, axis int) (dis, L, ic int) {
	dis, ic = 1, 1
	for i := 0; i < axis; i++ {
		dis *= shape[i]
	}
	for i := axis + 1; i < len(shape); i++ {
		ic *= shape[i]
	}
	return dis, shape[axis], ic
}

func (g *arkaUc) Softmax(out tensor.Storage, x tensor.View, axis int, logKip bool) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	o, son := gt.yaz(out, false)
	xv := gt.oku(x)
	dis, L, ic := eksenDuzeni(x.Shape, axis)
	satir := dis * ic
	if satir == 0 || L == 0 {
		return
	}
	boyDenetle(satir * L)
	B := ikiKuvvet(L, 32, 256)
	c := g.cekirdekAl(fmt.Sprintf("softmax:%d:%v", B, logKip), softmaxCekirdegi(B, logKip))
	g.baslat(c, [3]uint32{uint32(satir), 1, 1}, [3]uint32{uint32(B), 1, 1}, g.param().okuV(xv).yaz(o, 0).u32(L).u32(ic))
	son()
}

func (g *arkaUc) SoftmaxGrad(gx, gs, y tensor.Storage, shape []int, axis int, logKip bool) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	d, son := gt.yaz(gx, true)
	gd, yd := gt.okuD(gs), gt.okuD(y)
	dis, L, ic := eksenDuzeni(shape, axis)
	satir := dis * ic
	if satir == 0 || L == 0 {
		return
	}
	boyDenetle(satir * L)
	B := ikiKuvvet(L, 32, 256)
	c := g.cekirdekAl(fmt.Sprintf("softmax_turev:%d:%v", B, logKip), softmaxTurevCekirdegi(B, logKip))
	g.baslat(c, [3]uint32{uint32(satir), 1, 1}, [3]uint32{uint32(B), 1, 1}, g.param().yaz(d, 0).oku(gd, 0).oku(yd, 0).u32(L).u32(ic))
	son()
}

// tamlar []int'i 32 bitlik tam sayılara çevirir (indeksler, hedefler).
func tamlar(v []int) []int32 {
	r := make([]int32, len(v))
	for i, x := range v {
		if x < -1<<31 || x >= 1<<31 {
			panic(fmt.Sprintf("tensor: GPU: indeks %d 32 bite sığmıyor", x))
		}
		r[i] = int32(x)
	}
	return r
}

// yukleGecici tam sayıları geçici bir cihaz tamponuna yükler (çağrı sonunda iade).
func (t *gecici) yukleTam(v []int) *depo {
	d := t.g.yukleTam(tamlar(v))
	t.t = append(t.t, d)
	return d
}

func gecerliSayisi(targets []int) int {
	n := 0
	for _, t := range targets {
		if t >= 0 {
			n++
		}
	}
	return n
}

func (g *arkaUc) CrossEntropy(x tensor.View, targets []int, C int) (tensor.Storage, tensor.Storage) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	R := len(targets)
	loss := g.ayir(1, tensor.Float32, true)
	lse := g.ayir(R, tensor.Float32, false)
	if R == 0 || C == 0 {
		return loss, lse
	}
	boyDenetle(R * C)
	xv := gt.oku(x)
	hedef := gt.yukleTam(targets)
	kayip := gt.tampon(R, false)
	B := ikiKuvvet(C, 32, 256)
	c := g.cekirdekAl(fmt.Sprintf("ce_ileri:%d", B), ceIleriCekirdegi(B))
	g.baslat(c, [3]uint32{uint32(R), 1, 1}, [3]uint32{uint32(B), 1, 1}, g.param().okuV(xv).oku(hedef, 0).yaz(lse, 0).yaz(kayip, 0).u32(C))
	olcek := float32(0)
	if n := gecerliSayisi(targets); n > 0 {
		olcek = float32(1 / float64(n))
	}
	g.indirge(indirgeIs{a: kayip, cikti: loss, sekil: []int{R}, stA: []int{1}, st: []int{0}, terim: tA, olcek: olcek})
	return loss, lse
}

func (g *arkaUc) CrossEntropyGrad(gx, gs tensor.Storage, x tensor.View, lse tensor.Storage, targets []int, C int) {
	g.gir()
	defer g.cik()
	n := gecerliSayisi(targets)
	R := len(targets)
	if n == 0 || C == 0 {
		return
	}
	gt := g.gecici()
	defer gt.bitir()
	d, son := gt.yaz(gx, true)
	gd := gt.okuD(gs)
	xv := gt.oku(x)
	hedef := gt.yukleTam(targets)
	c := g.cekirdekAl("ce_geri", ceGeriCekirdegi)
	sutun := bloklar(C, esBlok)
	if sutun > 65535 {
		yok(fmt.Sprintf("%d sınıflı çapraz entropi (en çok %d)", C, 65535*esBlok))
	}
	g.baslat(c, [3]uint32{uint32(R), sutun, 1}, [3]uint32{esBlok, 1, 1},
		g.param().yaz(d, 0).oku(gd, 0).okuV(xv).oku(gt.okuD(lse), 0).oku(hedef, 0).u32(C).f32(float32(1/float64(n))))
	son()
}

func (g *arkaUc) Embedding(out tensor.Storage, w tensor.View, idx []int, D int) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	o, son := gt.yaz(out, false)
	if len(idx) == 0 || D == 0 {
		return
	}
	boyDenetle(len(idx) * D)
	wv := gt.oku(w)
	boyDenetle(wv.Numel())
	c := g.cekirdekAl("gomme", gommeCekirdegi)
	g.baslat(c, [3]uint32{uint32(len(idx)), 1, 1}, [3]uint32{gommeBlok, 1, 1},
		g.param().okuV(wv).oku(gt.yukleTam(idx), 0).yaz(o, 0).u32(D))
	son()
}

func (g *arkaUc) EmbeddingGrad(gw, gs tensor.Storage, idx []int, D int) {
	g.gir()
	defer g.cik()
	if len(idx) == 0 || D == 0 {
		return
	}
	gt := g.gecici()
	defer gt.bitir()
	d, son := gt.yaz(gw, true)
	boyDenetle(d.n)
	gd := gt.okuD(gs)
	tekil, bas, konum := gommeGruplari(idx)
	td, bd, kd := g.yukleTam(tekil), g.yukleTam(bas), g.yukleTam(konum)
	gt.t = append(gt.t, td, bd, kd)
	c := g.cekirdekAl("gomme_geri", gommeGeriCekirdegi)
	g.baslat(c, [3]uint32{uint32(len(tekil)), 1, 1}, [3]uint32{gommeBlok, 1, 1},
		g.param().yaz(d, 0).oku(gd, 0).oku(td, 0).oku(bd, 0).oku(kd, 0).u32(D))
	son()
}

// normIleri katman ya da RMS normunun ileri yönü.
func (g *arkaUc) normIleri(out tensor.Storage, x tensor.View, gamma, beta *tensor.View, D int, eps float32, katman bool) (*depo, *depo) {
	gt := g.gecici()
	defer gt.bitir()
	R := 0
	if D > 0 {
		R = x.Numel() / D
	}
	var ort *depo
	if katman {
		ort = g.ayir(R, tensor.Float32, false)
	}
	rstd := g.ayir(R, tensor.Float32, false)
	o, son := gt.yaz(out, false)
	if R == 0 {
		return ort, rstd
	}
	boyDenetle(R * D)
	xv := gt.oku(x)
	gv, bv := gt.okuP(gamma), gt.okuP(beta)
	a := normAyar{B: ikiKuvvet(D, 32, 256), katman: katman, gamma: gv != nil, beta: bv != nil}
	c := g.cekirdekAl(a.anahtar("norm_ileri"), normIleriCekirdegi(a))
	p := g.param().okuV(xv)
	for _, v := range []*tensor.View{gv, bv} {
		if v != nil {
			p.okuV(*v)
		} else {
			p.bos()
		}
	}
	p.yaz(o, 0).yazYa(ort).yaz(rstd, 0).u32(D).f32(eps)
	g.baslat(c, [3]uint32{uint32(R), 1, 1}, [3]uint32{uint32(a.B), 1, 1}, p)
	son()
	return ort, rstd
}

// satirlarToplami hedef (D) += Σ_satır kaynak (R×D).
func (g *arkaUc) satirlarToplami(hedef tensor.Storage, kaynak *depo, R, D int) {
	gt := g.gecici()
	defer gt.bitir()
	d, son := gt.yaz(hedef, true)
	g.indirge(indirgeIs{a: kaynak, cikti: d, sekil: []int{R, D}, stA: []int{D, 1}, st: []int{0, 1}, terim: tA, ekle: true, olcek: 1})
	son()
}

// normGeri katman ya da RMS normunun geri yönü.
func (g *arkaUc) normGeri(gx, ggamma, gbeta, gs tensor.Storage, x tensor.View, gamma *tensor.View, ort, rstd tensor.Storage, D int, katman bool) {
	gt := g.gecici()
	defer gt.bitir()
	R := 0
	if D > 0 {
		R = x.Numel() / D
	}
	if R == 0 || (gx == nil && ggamma == nil && gbeta == nil) {
		return
	}
	boyDenetle(R * D)
	gd := gt.okuD(gs)
	if gx != nil || ggamma != nil {
		xv := gt.oku(x)
		gv := gt.okuP(gamma)
		dx, son := gt.yaz(gx, true)
		var terim *depo
		if ggamma != nil {
			terim = gt.tampon(R*D, false)
		}
		a := normAyar{B: ikiKuvvet(D, 32, 256), katman: katman, gamma: gv != nil, xGrad: dx != nil, gammaGrad: terim != nil}
		c := g.cekirdekAl(a.anahtar("norm_geri"), normGeriCekirdegi(a))
		p := g.param().yazYa(dx).yazYa(terim).oku(gd, 0).okuV(xv)
		if gv != nil {
			p.okuV(*gv)
		} else {
			p.bos()
		}
		var od *depo
		if ort != nil {
			od = g.depoOf(ort)
		}
		p.okuYa(od).oku(g.depoOf(rstd), 0).u32(D)
		g.baslat(c, [3]uint32{uint32(R), 1, 1}, [3]uint32{uint32(a.B), 1, 1}, p)
		son()
		if terim != nil {
			g.satirlarToplami(ggamma, terim, R, D)
		}
	}
	if gbeta != nil {
		g.satirlarToplami(gbeta, gd, R, D)
	}
}

func (g *arkaUc) LayerNorm(out tensor.Storage, x tensor.View, gamma, beta *tensor.View, D int, eps float32) (tensor.Storage, tensor.Storage) {
	g.gir()
	defer g.cik()
	ort, rstd := g.normIleri(out, x, gamma, beta, D, eps, true)
	return ort, rstd
}

func (g *arkaUc) LayerNormGrad(gx, ggamma, gbeta, gs tensor.Storage, x tensor.View, gamma *tensor.View, mean, rstd tensor.Storage, D int) {
	g.gir()
	defer g.cik()
	g.normGeri(gx, ggamma, gbeta, gs, x, gamma, mean, rstd, D, true)
}

func (g *arkaUc) RMSNorm(out tensor.Storage, x tensor.View, gamma *tensor.View, D int, eps float32) tensor.Storage {
	g.gir()
	defer g.cik()
	_, rinv := g.normIleri(out, x, gamma, nil, D, eps, false)
	return rinv
}

func (g *arkaUc) RMSNormGrad(gx, ggamma, gs tensor.Storage, x tensor.View, gamma *tensor.View, rinv tensor.Storage, D int) {
	g.gir()
	defer g.cik()
	g.normGeri(gx, ggamma, nil, gs, x, gamma, nil, rinv, D, false)
}
