package gpu

import (
	"fmt"
	"math"
	"unsafe"

	"tr_dili/internal/hesap/tensor"
)

// ============================================================================
// İyileştirici adımları, kareler toplamı ve ölçekleme
// ============================================================================
//
// AdamW ve SGD formülleri CPU arka ucuyla aynıdır (f32AdamWStep, f32SGDStep). Kareler
// toplamı float64 biriktirir: sabit sayıda blok (yalnızca n'ye bağlı) her biri kendi
// aralığını float64'te toplar ve blok içinde ağaç biçiminde indirger; blok toplamları
// konağa indirilip sırayla toplanır (belirlenimci). Birden çok depolamanın toplamı
// (gradyan kırpma) tek indirmeyle alınır.

func adamWCekirdegi() *yazici {
	k := yeniYazici("adamw")
	p, g := k.yazilan("p"), k.gosterici("g")
	m, v := k.yazilan("m"), k.yazilan("v")
	n := k.u32p("n")
	adimLR, b1, b1c, b2, b2c := k.f32p("adim_lr"), k.f32p("b1"), k.f32p("b1c"), k.f32p("b2"), k.f32p("b2c")
	eps, bc2s, cur := k.f32p("eps"), k.f32p("bc2s"), k.f32p("cur")
	i := k.genelIndeks()
	son := k.sinirli(i, n)
	pa, ga, ma, va := k.adres(p, i, 4), k.adres(g, i, 4), k.adres(m, i, 4), k.adres(v, i, 4)
	gi := k.yukle(tensor.Float32, ga)
	mv, vv, pv := k.yukle(tensor.Float32, ma), k.yukle(tensor.Float32, va), k.yukle(tensor.Float32, pa)
	t := k.f()
	k.e("mul.rn.f32", mv, b1, mv)
	k.e("mul.rn.f32", t, b1c, gi)
	k.e("add.rn.f32", mv, mv, t)
	k.e("mul.rn.f32", vv, b2, vv)
	k.e("mul.rn.f32", t, b2c, gi)
	k.e("mul.rn.f32", t, t, gi)
	k.e("add.rn.f32", vv, vv, t)
	pay, payda := k.f(), k.f()
	k.e("mul.rn.f32", pay, adimLR, mv)
	k.e("sqrt.rn.f32", payda, vv)
	k.e("div.rn.f32", payda, payda, bc2s)
	k.e("add.rn.f32", payda, payda, eps)
	k.e("div.rn.f32", pay, pay, payda)
	k.e("mul.rn.f32", pv, pv, cur)
	k.e("sub.rn.f32", pv, pv, pay)
	k.sakla(tensor.Float32, ma, mv)
	k.sakla(tensor.Float32, va, vv)
	k.sakla(tensor.Float32, pa, pv)
	k.koy(son)
	return k
}

func sgdCekirdegi(momentum bool) func() *yazici {
	return func() *yazici {
		k := yeniYazici("sgd_" + map[bool]string{true: "momentumlu", false: "duz"}[momentum])
		p, g, vel := k.yazilan("p"), k.gosterici("g"), k.yazilan("hiz")
		n := k.u32p("n")
		lr, mom, wd := k.f32p("lr"), k.f32p("momentum"), k.f32p("wd")
		i := k.genelIndeks()
		son := k.sinirli(i, n)
		pa := k.adres(p, i, 4)
		pv := k.yukle(tensor.Float32, pa)
		gi := k.yukle(tensor.Float32, k.adres(g, i, 4))
		t := k.f()
		k.e("mul.rn.f32", t, wd, pv)
		k.e("add.rn.f32", gi, gi, t)
		if momentum {
			va := k.adres(vel, i, 4)
			vv := k.yukle(tensor.Float32, va)
			k.e("mul.rn.f32", vv, mom, vv)
			k.e("add.rn.f32", vv, vv, gi)
			k.sakla(tensor.Float32, va, vv)
			k.e("mov.f32", gi, vv)
		}
		k.e("mul.rn.f32", t, lr, gi)
		k.e("sub.rn.f32", pv, pv, t)
		k.sakla(tensor.Float32, pa, pv)
		k.koy(son)
		return k
	}
}

// kareBlok kareler toplamının blok boyu ve en çok blok sayısı.
const (
	kareBlok   = 256
	kareEnCokB = 1024
)

// kareToplamCekirdegi blok b, [b·parça, (b+1)·parça) aralığının kareler toplamını
// (float64) kismi[b]'ye yazar.
func kareToplamCekirdegi() *yazici {
	k := yeniYazici("kare_toplam")
	x := k.gosterici("x")
	kismi := k.yazilan("kismi")
	n, parca := k.u32p("n"), k.u32p("parca")
	sm := k.ortak("sm", 8*kareBlok)
	tid := k.ozel("%tid.x")
	b := k.ozel("%ctaid.x")
	bas, bit := k.r(), k.r()
	k.e("mul.lo.s32", bas, b, parca)
	k.e("add.s32", bit, bas, parca)
	k.e("min.u32", bit, bit, n)
	acc := k.fd()
	k.e("mov.f64", acc, float64(0))
	i := k.r()
	k.e("add.s32", i, bas, tid)
	dongu, bitti := k.etiket(), k.etiket()
	k.koy(dongu)
	p := k.p()
	k.e("setp.ge.u32", p, i, bit)
	k.gitEger(p, bitti)
	v := k.yukle(tensor.Float32, k.adres(x, i, 4))
	d := k.fd()
	k.e("cvt.f64.f32", d, v)
	k.e("fma.rn.f64", acc, d, d, acc)
	k.e("add.s32", i, i, kareBlok)
	k.git(dongu)
	k.koy(bitti)
	// Blok içi ağaç indirgemesi (float64).
	adr := k.r()
	k.e("mad.lo.s32", adr, tid, 8, sm)
	k.e("st.shared.f64", bel(adr, 0), acc)
	k.engel()
	for s := kareBlok / 2; s > 0; s /= 2 {
		atla := k.etiket()
		q := k.p()
		k.e("setp.ge.u32", q, tid, s)
		k.gitEger(q, atla)
		a, c := k.fd(), k.fd()
		k.e("ld.shared.f64", a, bel(adr, 0))
		k.e("ld.shared.f64", c, bel(adr, 8*s))
		k.e("add.f64", a, a, c)
		k.e("st.shared.f64", bel(adr, 0), a)
		k.koy(atla)
		k.engel()
	}
	son := k.etiket()
	k.gitEger(k.tidSifirDegil(tid), son)
	r := k.fd()
	k.e("ld.shared.f64", r, bel(sm, 0))
	k.e("st.global.f64", bel(k.adres(kismi, b, 8), 0), r)
	k.koy(son)
	return k
}

// kareParcalari n eleman için blok sayısı ve blok başına parça boyu.
func kareParcalari(n int) (nb, parca int) {
	nb = min(int(bloklar(n, kareBlok*4)), kareEnCokB)
	parca = (n + int(nb) - 1) / int(nb)
	return int(nb), parca
}

func (g *arkaUc) SGDStep(p, gs, vel tensor.Storage, lr, momentum, wd float32) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	pd, son1 := gt.yaz(p, true)
	vd, son2 := gt.yaz(vel, true)
	gd := gt.okuD(gs)
	if pd.n == 0 {
		return
	}
	boyDenetle(pd.n)
	c := g.cekirdekAl("sgd:"+map[bool]string{true: "m", false: "d"}[vd != nil], sgdCekirdegi(vd != nil))
	iz, bl := duz(pd.n, esBlok)
	g.baslat(c, iz, bl, g.param().yaz(pd, 0).oku(gd, 0).yazYa(vd).u32(pd.n).f32(lr).f32(momentum).f32(wd))
	son1()
	son2()
}

func (g *arkaUc) AdamWStep(p, gs, m, v tensor.Storage, lr, b1, b2, eps, wd, bc1, bc2 float32) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	pd, son1 := gt.yaz(p, true)
	md, son2 := gt.yaz(m, true)
	vd, son3 := gt.yaz(v, true)
	gd := gt.okuD(gs)
	if pd.n == 0 {
		return
	}
	boyDenetle(pd.n)
	adimLR := lr / bc1
	bc2s := float32(math.Sqrt(float64(bc2)))
	cur := 1 - lr*wd
	c := g.cekirdekAl("adamw", adamWCekirdegi)
	iz, bl := duz(pd.n, esBlok)
	g.baslat(c, iz, bl, g.param().yaz(pd, 0).oku(gd, 0).yaz(md, 0).yaz(vd, 0).u32(pd.n).
		f32(adimLR).f32(b1).f32(1-b1).f32(b2).f32(1-b2).f32(eps).f32(bc2s).f32(cur))
	son1()
	son2()
	son3()
}

// kareSirala depolamaların kareler toplamı kısmi toplamlarını tek tampona yazar ve
// her depolamanın blok sayısını verir.
func (g *arkaUc) kareSirala(gt *gecici, ss []tensor.Storage) (*depo, []int) {
	toplamB := 0
	nbs := make([]int, len(ss))
	ds := make([]*depo, len(ss))
	for i, s := range ss {
		ds[i] = gt.okuD(s)
		if ds[i].n > 0 {
			boyDenetle(ds[i].n)
			nbs[i], _ = kareParcalari(ds[i].n)
		}
		toplamB += nbs[i]
	}
	kismi := gt.tampon(2*max(toplamB, 1), false) // float64: iki float32 yeri
	c := g.cekirdekAl("kare_toplam", kareToplamCekirdegi)
	o := 0
	for i, d := range ds {
		if nbs[i] == 0 {
			continue
		}
		_, parca := kareParcalari(d.n)
		g.baslat(c, [3]uint32{uint32(nbs[i]), 1, 1}, [3]uint32{kareBlok, 1, 1},
			g.param().oku(d, 0).yaz(kismi, 2*o).u32(d.n).u32(parca))
		o += nbs[i]
	}
	return kismi, nbs
}

// SumSquaresAll (tensor.SquareSummer) depolamaların kareler toplamı (tek eşzamanlama).
func (g *arkaUc) SumSquaresAll(ss []tensor.Storage) float64 {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	kismi, nbs := g.kareSirala(gt, ss)
	toplamB := 0
	for _, n := range nbs {
		toplamB += n
	}
	if toplamB == 0 {
		return 0
	}
	f := make([]float64, toplamB)
	g.indir(unsafe.Pointer(&f[0]), kismi.adr, 8*toplamB)
	var s float64
	o := 0
	for _, n := range nbs {
		var t float64
		for _, v := range f[o : o+n] {
			t += v
		}
		s += t
		o += n
	}
	return s
}

func (g *arkaUc) SumSquares(s tensor.Storage) float64 {
	return g.SumSquaresAll([]tensor.Storage{s})
}

func (g *arkaUc) Scale(s tensor.Storage, k float32) {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	d, son := gt.yaz(s, true)
	if d.n == 0 {
		return
	}
	boyDenetle(d.n)
	c := g.cekirdekAl(fmt.Sprintf("tekli:%d:", tensor.OpMulScalar), tekliCekirdegi(tensor.OpMulScalar, ""))
	iz, bl := duz(d.n, esBlok)
	g.baslat(c, iz, bl, g.param().oku(d, 0).yaz(d, 0).u32(d.n).f32(k))
	son()
}
