package gpu

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Eleman bazlı çekirdek aileleri (tekli, ikili, türevleri)
// ============================================================================
//
// Her aile tek bir üreteçtir; işlem, işleme özgü ifadeyi yazan küçük bir Go işleviyle
// seçilir (yüzlerce el yazısı çekirdek yerine). Bütün aileler float32 bitişik çalışır;
// 16 bitlik ve adımlı girdileri arka uç önce açar (gecici).

// usKipi OpPow'un çekirdek biçimi: p = 0, 1, 2 özel, diğerleri genel.
func powMode(p float32) string {
	switch p {
	case 0, 1, 2:
		return fmt.Sprint(p)
	}
	return "genel"
}

// tekliIfade y = f(x, p).
func (k *emitter) unaryExpr(op tensor.UnOp, mode2 string, x, p reg) reg {
	y := k.f()
	switch op {
	case tensor.OpExp:
		return k.exponential(x)
	case tensor.OpLog:
		return k.logarithm(x)
	case tensor.OpSqrt:
		k.e("sqrt.rn.f32", y, x)
	case tensor.OpPow:
		switch mode2 {
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
		return k.sine(x)
	case tensor.OpCos:
		return k.cosine(x)
	default:
		return k.unaryExprWide(op, x) // cekirdek_genis.go
	}
	return y
}

// tekliTurev gx'e eklenecek terim: g·f'(x, y, p).
func (k *emitter) unaryGrad(op tensor.UnOp, mode2 string, g, x, y, p reg) reg {
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
		switch mode2 {
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
		k.e("mul.rn.f32", t, g, k.geluGrad(x))
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
		k.e("mul.rn.f32", t, g, k.cosine(x))
	case tensor.OpCos:
		k.e("mul.rn.f32", t, g, k.sine(x))
		k.e("neg.f32", t, t)
	default:
		k.e("mul.rn.f32", t, g, k.unaryGradWide(op, x, y)) // cekirdek_genis.go
	}
	return t
}

// sinirli i ≥ n ise sona atlar; atlanacak etiketi verir.
func (k *emitter) bounded(i, n reg) string {
	last := k.label()
	p := k.p()
	k.e("setp.ge.u32", p, i, n)
	k.jumpIf(p, last)
	return last
}

func unaryKernel(op tensor.UnOp, mode2 string) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("tekli_%d_%s", op, mode2))
		x := k.pointer("x")
		y := k.written("y")
		n := k.u32p("n")
		p := k.f32p("p")
		i := k.globalIndex()
		last := k.bounded(i, n)
		v := k.load(tensor.Float32, k.address(x, i, 4))
		k.store(tensor.Float32, k.address(y, i, 4), k.unaryExpr(op, mode2, v, p))
		k.put(last)
		return k
	}
}

func unaryGradKernel(op tensor.UnOp, mode2 string) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("tekli_turev_%d_%s", op, mode2))
		gx := k.written("gx")
		g := k.pointer("g")
		x := k.pointer("x")
		y := k.pointer("y")
		n := k.u32p("n")
		p := k.f32p("p")
		i := k.globalIndex()
		last := k.bounded(i, n)
		gv := k.load(tensor.Float32, k.address(g, i, 4))
		xv := k.load(tensor.Float32, k.address(x, i, 4))
		yv := k.load(tensor.Float32, k.address(y, i, 4))
		t := k.unaryGrad(op, mode2, gv, xv, yv, p)
		ha := k.address(gx, i, 4)
		old := k.load(tensor.Float32, ha)
		k.e("add.rn.f32", old, old, t)
		k.store(tensor.Float32, ha, old)
		k.put(last)
		return k
	}
}

var binaryInst = map[tensor.BinOp]string{tensor.OpAdd: "add.rn.f32", tensor.OpSub: "sub.rn.f32",
	tensor.OpMul: "mul.rn.f32", tensor.OpDiv: "div.rn.f32"}

// ikiliCekirdegi out[i] = a[ofsA] op b[ofsB]; nd = 0 ise üç taraf da düz bitişiktir,
// değilse a ve b'nin ofsetleri meta'dan (şekil, adımA, adımB) ayrıştırılır.
func binaryKernel(op tensor.BinOp, nd int) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("ikili_%d_%d", op, nd))
		a := k.pointer("a")
		b := k.pointer("b")
		out := k.written("cikti")
		n := k.u32p("n")
		k.arr("meta", 3*nd)
		i := k.globalIndex()
		last := k.bounded(i, n)
		oa, ob := i, i
		if nd > 0 {
			ofs := k.parse(i, nd, 2, "meta")
			oa, ob = ofs[0], ofs[1]
		}
		av := k.load(tensor.Float32, k.addressS(a, oa, 4))
		bv := k.load(tensor.Float32, k.addressS(b, ob, 4))
		k.store(tensor.Float32, k.address(out, i, 4), k.binaryExpr(op, av, bv))
		k.put(last)
		return k
	}
}
