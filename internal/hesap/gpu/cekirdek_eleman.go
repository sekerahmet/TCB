package gpu

import (
	"fmt"

	"tr_dili/internal/hesap/tensor"
)

// ============================================================================
// Eleman bazlı çekirdek aileleri (tekli, ikili, türevleri)
// ============================================================================
//
// Her aile tek bir üreteçtir; işlem, işleme özgü ifadeyi yazan küçük bir Go işleviyle
// seçilir (yüzlerce el yazısı çekirdek yerine). Bütün aileler float32 bitişik çalışır;
// 16 bitlik ve adımlı girdileri arka uç önce açar (gecici).

// usKipi OpPow'un çekirdek biçimi: p = 0, 1, 2 özel, diğerleri genel.
func usKipi(p float32) string {
	switch p {
	case 0, 1, 2:
		return fmt.Sprint(p)
	}
	return "genel"
}

// tekliIfade y = f(x, p).
func (k *yazici) tekliIfade(op tensor.UnOp, kip string, x, p yazmac) yazmac {
	y := k.f()
	switch op {
	case tensor.OpExp:
		return k.ustel(x)
	case tensor.OpLog:
		return k.logaritma(x)
	case tensor.OpSqrt:
		k.e("sqrt.rn.f32", y, x)
	case tensor.OpPow:
		switch kip {
		case "0":
			k.e("mov.f32", y, float32(1))
		case "1":
			k.e("mov.f32", y, x)
		case "2":
			k.e("mul.rn.f32", y, x, x)
		default:
			return k.us(x, p)
		}
	case tensor.OpTanh:
		return k.tanh(x)
	case tensor.OpRelu:
		b := k.p()
		k.e("setp.gtu.f32", b, x, float32(0)) // x > 0 ya da NaN: x
		k.e("selp.f32", y, x, float32(0), b)
	case tensor.OpGelu:
		return k.gelu(x)
	case tensor.OpSigmoid:
		return k.sigmoid(x)
	case tensor.OpAddScalar:
		k.e("add.rn.f32", y, x, p)
	case tensor.OpMulScalar:
		k.e("mul.rn.f32", y, x, p)
	case tensor.OpRSubScalar:
		k.e("sub.rn.f32", y, p, x)
	case tensor.OpRDivScalar:
		k.e("div.rn.f32", y, p, x)
	case tensor.OpSin:
		return k.sinus(x)
	case tensor.OpCos:
		return k.kosinus(x)
	default:
		return k.tekliIfadeGenis(op, x) // cekirdek_genis.go
	}
	return y
}

// tekliTurev gx'e eklenecek terim: g·f'(x, y, p).
func (k *yazici) tekliTurev(op tensor.UnOp, kip string, g, x, y, p yazmac) yazmac {
	t := k.f()
	switch op {
	case tensor.OpExp:
		k.e("mul.rn.f32", t, g, y)
	case tensor.OpLog:
		k.e("rcp.rn.f32", t, x)
		k.e("mul.rn.f32", t, g, t)
	case tensor.OpSqrt:
		k.e("div.rn.f32", t, float32(0.5), y)
		k.e("mul.rn.f32", t, g, t)
	case tensor.OpPow:
		switch kip {
		case "1":
			k.e("mov.f32", t, g)
		case "2":
			k.e("add.rn.f32", t, x, x)
			k.e("mul.rn.f32", t, g, t)
		default:
			p1 := k.f()
			k.e("sub.rn.f32", p1, p, float32(1))
			u := k.us(x, p1)
			k.e("mul.rn.f32", t, p, u)
			k.e("mul.rn.f32", t, g, t)
		}
	case tensor.OpTanh:
		k.e("mul.rn.f32", t, y, y)
		k.e("sub.rn.f32", t, float32(1), t)
		k.e("mul.rn.f32", t, g, t)
	case tensor.OpRelu:
		b := k.p()
		k.e("setp.gt.f32", b, x, float32(0))
		s := k.f()
		k.e("mul.rn.f32", s, g, float32(0)) // NaN/sonsuz gradyan yayılır
		k.e("selp.f32", t, g, s, b)
	case tensor.OpGelu:
		k.e("mul.rn.f32", t, g, k.geluTurev(x))
	case tensor.OpSigmoid:
		k.e("sub.rn.f32", t, float32(1), y)
		k.e("mul.rn.f32", t, y, t)
		k.e("mul.rn.f32", t, g, t)
	case tensor.OpAddScalar:
		k.e("mov.f32", t, g)
	case tensor.OpMulScalar:
		k.e("mul.rn.f32", t, g, p)
	case tensor.OpRSubScalar:
		k.e("neg.f32", t, g)
	case tensor.OpRDivScalar:
		k.e("neg.f32", t, y)
		k.e("div.rn.f32", t, t, x)
		k.e("mul.rn.f32", t, g, t)
	case tensor.OpSin:
		k.e("mul.rn.f32", t, g, k.kosinus(x))
	case tensor.OpCos:
		k.e("mul.rn.f32", t, g, k.sinus(x))
		k.e("neg.f32", t, t)
	default:
		k.e("mul.rn.f32", t, g, k.tekliTurevGenis(op, x, y)) // cekirdek_genis.go
	}
	return t
}

// sinirli i ≥ n ise sona atlar; atlanacak etiketi verir.
func (k *yazici) sinirli(i, n yazmac) string {
	son := k.etiket()
	p := k.p()
	k.e("setp.ge.u32", p, i, n)
	k.gitEger(p, son)
	return son
}

func tekliCekirdegi(op tensor.UnOp, kip string) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("tekli_%d_%s", op, kip))
		x := k.gosterici("x")
		y := k.yazilan("y")
		n := k.u32p("n")
		p := k.f32p("p")
		i := k.genelIndeks()
		son := k.sinirli(i, n)
		v := k.yukle(tensor.Float32, k.adres(x, i, 4))
		k.sakla(tensor.Float32, k.adres(y, i, 4), k.tekliIfade(op, kip, v, p))
		k.koy(son)
		return k
	}
}

func tekliTurevCekirdegi(op tensor.UnOp, kip string) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("tekli_turev_%d_%s", op, kip))
		gx := k.yazilan("gx")
		g := k.gosterici("g")
		x := k.gosterici("x")
		y := k.gosterici("y")
		n := k.u32p("n")
		p := k.f32p("p")
		i := k.genelIndeks()
		son := k.sinirli(i, n)
		gv := k.yukle(tensor.Float32, k.adres(g, i, 4))
		xv := k.yukle(tensor.Float32, k.adres(x, i, 4))
		yv := k.yukle(tensor.Float32, k.adres(y, i, 4))
		t := k.tekliTurev(op, kip, gv, xv, yv, p)
		ha := k.adres(gx, i, 4)
		eski := k.yukle(tensor.Float32, ha)
		k.e("add.rn.f32", eski, eski, t)
		k.sakla(tensor.Float32, ha, eski)
		k.koy(son)
		return k
	}
}

var ikiliKomut = map[tensor.BinOp]string{tensor.OpAdd: "add.rn.f32", tensor.OpSub: "sub.rn.f32",
	tensor.OpMul: "mul.rn.f32", tensor.OpDiv: "div.rn.f32"}

// ikiliCekirdegi out[i] = a[ofsA] op b[ofsB]; nd = 0 ise üç taraf da düz bitişiktir,
// değilse a ve b'nin ofsetleri meta'dan (şekil, adımA, adımB) ayrıştırılır.
func ikiliCekirdegi(op tensor.BinOp, nd int) func() *yazici {
	return func() *yazici {
		k := yeniYazici(fmt.Sprintf("ikili_%d_%d", op, nd))
		a := k.gosterici("a")
		b := k.gosterici("b")
		out := k.yazilan("cikti")
		n := k.u32p("n")
		k.dizi("meta", 3*nd)
		i := k.genelIndeks()
		son := k.sinirli(i, n)
		oa, ob := i, i
		if nd > 0 {
			ofs := k.ayristir(i, nd, 2, "meta")
			oa, ob = ofs[0], ofs[1]
		}
		av := k.yukle(tensor.Float32, k.adresS(a, oa, 4))
		bv := k.yukle(tensor.Float32, k.adresS(b, ob, 4))
		k.sakla(tensor.Float32, k.adres(out, i, 4), k.ikiliIfade(op, av, bv))
		k.koy(son)
		return k
	}
}
