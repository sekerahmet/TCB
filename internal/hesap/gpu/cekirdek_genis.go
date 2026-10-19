package gpu

import (
	"fmt"
	"math"

	"tr_dili/internal/hesap/tensor"
)

// ============================================================================
// Genişletilmiş eleman bazlı çekirdekler: tekli işlemlerin ifadeleri ve türevleri,
// ikili (atan2, karşılaştırmalar), sınırlama, koşullu seçim, atan2 gradyanı
// ============================================================================

// tekliIfadeGenis genişletilmiş tekli işlemlerin ifadesi y = f(x).
func (k *yazici) tekliIfadeGenis(op tensor.UnOp, x yazmac) yazmac {
	switch op {
	case tensor.OpSilu:
		return k.silu(x)
	case tensor.OpTan:
		return k.tanjant(x)
	case tensor.OpAsin:
		return k.arksinus(x)
	case tensor.OpAcos:
		return k.arkkosinus(x)
	case tensor.OpAtan:
		return k.arktanjant(x)
	case tensor.OpSinh:
		return k.hipSinus(x)
	case tensor.OpCosh:
		return k.hipKosinus(x)
	case tensor.OpLog1p:
		return k.log1Arti(x)
	case tensor.OpLog2:
		return k.log2Hassas(x)
	case tensor.OpLog10:
		return k.log10Hassas(x)
	case tensor.OpExpm1:
		return k.ustelEksiBir(x)
	case tensor.OpFloor, tensor.OpCeil:
		y := k.f()
		kip := map[tensor.UnOp]string{tensor.OpFloor: "rmi", tensor.OpCeil: "rpi"}[op]
		k.e("cvt."+kip+".f32.f32", y, x)
		return y
	case tensor.OpSign:
		return k.isaretF(x)
	case tensor.OpAbs:
		// Yalnızca işaret biti silinir (CPU'daki gibi NaN yükü korunur); abs.f32 bazı
		// mimarilerde (sm_120) NaN'ı 0x7fffffff'e çevirir.
		b := k.r()
		k.e("mov.b32", b, x)
		k.e("and.b32", b, b, 0x7FFFFFFF)
		y := k.f()
		k.e("mov.b32", y, b)
		return y
	}
	yok(fmt.Sprintf("tekli işlem %d", op))
	return ""
}

// tekliTurevGenis genişletilmiş tekli işlemlerin türev çarpanı f'(x, y); çağıran g ile
// çarpar.
func (k *yazici) tekliTurevGenis(op tensor.UnOp, x, y yazmac) yazmac {
	d := k.f()
	switch op {
	case tensor.OpSilu:
		return k.siluTurev(x)
	case tensor.OpTan:
		k.e("fma.rn.f32", d, y, y, float32(1))
	case tensor.OpAsin, tensor.OpAcos:
		a, b := k.f(), k.f()
		k.e("sub.rn.f32", a, float32(1), x)
		k.e("add.rn.f32", b, float32(1), x)
		k.e("mul.rn.f32", a, a, b)
		k.e("sqrt.rn.f32", a, a)
		pay := float32(1)
		if op == tensor.OpAcos {
			pay = -1
		}
		k.e("div.rn.f32", d, pay, a)
	case tensor.OpAtan:
		k.e("fma.rn.f32", d, x, x, float32(1))
		k.e("div.rn.f32", d, float32(1), d)
	case tensor.OpSinh:
		return k.hipKosinus(x)
	case tensor.OpCosh:
		return k.hipSinus(x)
	case tensor.OpLog1p:
		k.e("add.rn.f32", d, x, float32(1))
		k.e("div.rn.f32", d, float32(1), d)
	case tensor.OpLog2, tensor.OpLog10:
		c := float32(math.Ln2)
		if op == tensor.OpLog10 {
			c = float32(math.Ln10)
		}
		k.e("mul.rn.f32", d, x, c)
		k.e("div.rn.f32", d, float32(1), d)
	case tensor.OpExpm1:
		k.e("add.rn.f32", d, y, float32(1))
	case tensor.OpAbs:
		return k.isaretF(x) // 0'da 0, NaN'da NaN (CPU ile aynı)
	default:
		yok(fmt.Sprintf("tekli işlemin türevi %d", op))
	}
	return d
}

// ikiliIfade ikili işlemin a op b ifadesi; karşılaştırmalar 1/0 verir (NaN: 0).
func (k *yazici) ikiliIfade(op tensor.BinOp, a, b yazmac) yazmac {
	if c, ok := ikiliKomut[op]; ok {
		r := k.f()
		k.e(c, r, a, b)
		return r
	}
	kars := map[tensor.BinOp]string{tensor.OpLt: "lt", tensor.OpGt: "gt", tensor.OpLe: "le", tensor.OpGe: "ge"}
	if c, ok := kars[op]; ok {
		p := k.p()
		k.e("setp."+c+".f32", p, a, b)
		return k.sec(p, float32(1), float32(0))
	}
	if op == tensor.OpAtan2 {
		return k.arktanjant2(a, b)
	}
	yok(fmt.Sprintf("ikili işlem %d", op))
	return ""
}

// sinirlaCekirdegi y = min(max(x, alt), üst) (NaN korunur); turev doğruysa
// gx += [alt ≤ x ≤ üst]·g.
func sinirlaCekirdegi(turev bool) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("sinirla_%v", turev))
		var gx, g, y yazmac
		if turev {
			gx = k.yazilan("gx")
			g = k.gosterici("g")
		} else {
			y = k.yazilan("y")
		}
		x := k.gosterici("x")
		n := k.u32p("n")
		alt := k.f32p("alt")
		ust := k.f32p("ust")
		i := k.genelIndeks()
		son := k.sinirli(i, n)
		v := k.yukle(tensor.Float32, k.adres(x, i, 4))
		if !turev {
			kucuk, buyuk := k.p(), k.p()
			k.e("setp.lt.f32", kucuk, v, alt)
			r := k.sec(kucuk, alt, v)
			k.e("setp.gt.f32", buyuk, r, ust)
			k.sakla(tensor.Float32, k.adres(y, i, 4), k.sec(buyuk, ust, r))
			k.koy(son)
			return k
		}
		ic, ic2 := k.p(), k.p()
		k.e("setp.ge.f32", ic, v, alt)
		k.e("setp.le.f32", ic2, v, ust)
		k.e("and.pred", ic, ic, ic2)
		gv := k.yukle(tensor.Float32, k.adres(g, i, 4))
		ha := k.adres(gx, i, 4)
		eski := k.yukle(tensor.Float32, ha)
		yeni := k.f()
		k.e("add.rn.f32", yeni, eski, gv)
		k.sakla(tensor.Float32, ha, k.sec(ic, yeni, eski))
		k.koy(son)
		return k
	}
}

// ucluOfsetler nd = 0 ise üç işlenen de i'de (düz bitişik), değilse meta'dan ayrıştırılır.
func (k *yazici) ucluOfsetler(i yazmac, nd, nIslenen int) []yazmac {
	if nd == 0 {
		ofs := make([]yazmac, nIslenen)
		for j := range ofs {
			ofs[j] = i
		}
		return ofs
	}
	return k.ayristir(i, nd, nIslenen, "meta")
}

// secimCekirdegi cikti[i] = c ≠ 0 (NaN dahil) ise a, değilse b; c, a, b yayınlamalı.
// Meta: şekil, c adımı, a adımı, b adımı.
func secimCekirdegi(nd int) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("secim_%d", nd))
		c := k.gosterici("c")
		a := k.gosterici("a")
		b := k.gosterici("b")
		out := k.yazilan("cikti")
		n := k.u32p("n")
		k.dizi("meta", 4*nd)
		i := k.genelIndeks()
		son := k.sinirli(i, n)
		ofs := k.ucluOfsetler(i, nd, 3)
		cv := k.yukle(tensor.Float32, k.adresS(c, ofs[0], 4))
		av := k.yukle(tensor.Float32, k.adresS(a, ofs[1], 4))
		bv := k.yukle(tensor.Float32, k.adresS(b, ofs[2], 4))
		p := k.p()
		k.e("setp.neu.f32", p, cv, float32(0))
		k.sakla(tensor.Float32, k.adres(out, i, 4), k.sec(p, av, bv))
		k.koy(son)
		return k
	}
}

// atan2TurevCekirdegi çıktı şeklinde iki terim yazar: ta = g·x/(x²+y²), tb = −g·y/(x²+y²)
// (y = a, x = b; yayınlamalı). Taşma ve alt taşmaya karşı m = max(|x|, |y|) ile
// ölçeklenir; m = 0 ya da sonsuzsa terimler 0'dır. Meta: şekil, a adımı, b adımı.
func atan2TurevCekirdegi(nd int) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("atan2_turev_%d", nd))
		pa := k.gosterici("a")
		pb := k.gosterici("b")
		pg := k.gosterici("g")
		ta := k.yazilan("ta")
		tb := k.yazilan("tb")
		n := k.u32p("n")
		k.dizi("meta", 3*nd)
		i := k.genelIndeks()
		son := k.sinirli(i, n)
		ofs := k.ucluOfsetler(i, nd, 2)
		y := k.yukle(tensor.Float32, k.adresS(pa, ofs[0], 4))
		x := k.yukle(tensor.Float32, k.adresS(pb, ofs[1], 4))
		g := k.yukle(tensor.Float32, k.adres(pg, i, 4))
		ax, ay := k.f(), k.f()
		k.e("abs.f32", ax, x)
		k.e("abs.f32", ay, y)
		m := k.f()
		k.e("max.f32", m, ax, ay)
		ys, xs := k.f(), k.f()
		k.e("div.rn.f32", ys, y, m)
		k.e("div.rn.f32", xs, x, m)
		r := k.f()
		k.e("mul.rn.f32", r, xs, xs)
		k.e("fma.rn.f32", r, ys, ys, r)
		k.e("mul.rn.f32", r, r, m)
		ga, gb := k.f(), k.f()
		k.e("mul.rn.f32", ga, g, xs)
		k.e("div.rn.f32", ga, ga, r)
		k.e("mul.rn.f32", gb, g, ys)
		k.e("div.rn.f32", gb, gb, r)
		k.e("neg.f32", gb, gb)
		bos, sonsuz := k.p(), k.p()
		k.e("setp.eq.f32", bos, m, float32(0))
		k.e("setp.eq.f32", sonsuz, m, float32(math.Inf(1)))
		k.e("or.pred", bos, bos, sonsuz)
		k.sakla(tensor.Float32, k.adres(ta, i, 4), k.sec(bos, float32(0), ga))
		k.sakla(tensor.Float32, k.adres(tb, i, 4), k.sec(bos, float32(0), gb))
		k.koy(son)
		return k
	}
}
