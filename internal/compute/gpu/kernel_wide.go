package gpu

import (
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// Genişletilmiş eleman bazlı çekirdekler: tekli işlemlerin ifadeleri ve türevleri,
// ikili (atan2, karşılaştırmalar), sınırlama, koşullu seçim, atan2 gradyanı
// ============================================================================

// unaryExprWide genişletilmiş tekli işlemlerin ifadesi y = f(x).
func (k *emitter) unaryExprWide(op tensor.UnOp, x reg) reg {
	switch op {
	case tensor.OpSilu:
		return k.silu(x)
	case tensor.OpTan:
		return k.tangent(x)
	case tensor.OpAsin:
		return k.arcsin(x)
	case tensor.OpAcos:
		return k.arccos(x)
	case tensor.OpAtan:
		return k.arctan(x)
	case tensor.OpSinh:
		return k.hypSin(x)
	case tensor.OpCosh:
		return k.hypCos(x)
	case tensor.OpLog1p:
		return k.log1Plus(x)
	case tensor.OpLog2:
		return k.log2Precise(x)
	case tensor.OpLog10:
		return k.log10Precise(x)
	case tensor.OpExpm1:
		return k.expMinusOne(x)
	case tensor.OpFloor, tensor.OpCeil:
		y := k.f()
		mode2 := map[tensor.UnOp]string{tensor.OpFloor: "rmi", tensor.OpCeil: "rpi"}[op]
		k.e("cvt."+mode2+".f32.f32", y, x)
		return y
	case tensor.OpSign:
		return k.signF(x)
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
	none(messages.H4058, op)
	return ""
}

// unaryGradWide genişletilmiş tekli işlemlerin türev çarpanı f'(x, y); çağıran g ile
// çarpar.
func (k *emitter) unaryGradWide(op tensor.UnOp, x, y reg) reg {
	d := k.f()
	switch op {
	case tensor.OpSilu:
		return k.siluGrad(x)
	case tensor.OpTan:
		k.e("fma.rn.f32", d, y, y, float32(1))
	case tensor.OpAsin, tensor.OpAcos:
		a, b := k.f(), k.f()
		k.e("sub.rn.f32", a, float32(1), x)
		k.e("add.rn.f32", b, float32(1), x)
		k.e("mul.rn.f32", a, a, b)
		k.e("sqrt.rn.f32", a, a)
		share := float32(1)
		if op == tensor.OpAcos {
			share = -1
		}
		k.e("div.rn.f32", d, share, a)
	case tensor.OpAtan:
		k.e("fma.rn.f32", d, x, x, float32(1))
		k.e("div.rn.f32", d, float32(1), d)
	case tensor.OpSinh:
		return k.hypCos(x)
	case tensor.OpCosh:
		return k.hypSin(x)
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
		return k.exponential(x) // exp(x): y+1, x çok negatifken 0'a iptal olurdu (CPU ile aynı)
	case tensor.OpAbs:
		return k.signF(x) // 0'da 0, NaN'da NaN (CPU ile aynı)
	default:
		none(messages.H4059, op)
	}
	return d
}

// binaryExpr ikili işlemin a op b ifadesi; karşılaştırmalar 1/0 verir (NaN: 0).
func (k *emitter) binaryExpr(op tensor.BinOp, a, b reg) reg {
	if c, ok := binaryInst[op]; ok {
		r := k.f()
		k.e(c, r, a, b)
		return r
	}
	compare2 := map[tensor.BinOp]string{tensor.OpLt: "lt", tensor.OpGt: "gt", tensor.OpLe: "le", tensor.OpGe: "ge"}
	if c, ok := compare2[op]; ok {
		p := k.p()
		k.e("setp."+c+".f32", p, a, b)
		return k.pick(p, float32(1), float32(0))
	}
	if op == tensor.OpAtan2 {
		return k.arctan2(a, b)
	}
	none(messages.H4060, op)
	return ""
}

// clampKernel y = min(max(x, alt), üst) (NaN korunur); turev doğruysa
// gx += [alt ≤ x ≤ üst]·g.
func clampKernel(derivative bool) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("sinirla_%v", derivative))
		var gx, g, y reg
		if derivative {
			gx = k.written("gx")
			g = k.pointer("g")
		} else {
			y = k.written("y")
		}
		x := k.pointer("x")
		n := k.u32p("n")
		sub := k.f32p("alt")
		upper := k.f32p("ust")
		i := k.globalIndex()
		last := k.bounded(i, n)
		v := k.load(tensor.Float32, k.address(x, i, 4))
		if !derivative {
			small, large := k.p(), k.p()
			k.e("setp.lt.f32", small, v, sub)
			r := k.pick(small, sub, v)
			k.e("setp.gt.f32", large, r, upper)
			k.store(tensor.Float32, k.address(y, i, 4), k.pick(large, upper, r))
			k.put(last)
			return k
		}
		ic, ic2 := k.p(), k.p()
		k.e("setp.ge.f32", ic, v, sub)
		k.e("setp.le.f32", ic2, v, upper)
		k.e("and.pred", ic, ic, ic2)
		gv := k.load(tensor.Float32, k.address(g, i, 4))
		ha := k.address(gx, i, 4)
		old := k.load(tensor.Float32, ha)
		fresh2 := k.f()
		k.e("add.rn.f32", fresh2, old, gv)
		k.store(tensor.Float32, ha, k.pick(ic, fresh2, old))
		k.put(last)
		return k
	}
}

// tripleOffsets nd = 0 ise üç işlenen de i'de (düz bitişik), değilse meta'dan ayrıştırılır.
func (k *emitter) tripleOffsets(i reg, nd, nOperand int) []reg {
	if nd == 0 {
		ofs := make([]reg, nOperand)
		for j := range ofs {
			ofs[j] = i
		}
		return ofs
	}
	return k.parse(i, nd, nOperand, "meta")
}

// selectKernel cikti[i] = c ≠ 0 (NaN dahil) ise a, değilse b; c, a, b yayınlamalı.
// Meta: şekil, c adımı, a adımı, b adımı.
func selectKernel(nd int) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("secim_%d", nd))
		c := k.pointer("c")
		a := k.pointer("a")
		b := k.pointer("b")
		out := k.written("cikti")
		n := k.u32p("n")
		k.arr("meta", 4*nd)
		i := k.globalIndex()
		last := k.bounded(i, n)
		ofs := k.tripleOffsets(i, nd, 3)
		cv := k.load(tensor.Float32, k.addressS(c, ofs[0], 4))
		av := k.load(tensor.Float32, k.addressS(a, ofs[1], 4))
		bv := k.load(tensor.Float32, k.addressS(b, ofs[2], 4))
		p := k.p()
		k.e("setp.neu.f32", p, cv, float32(0))
		k.store(tensor.Float32, k.address(out, i, 4), k.pick(p, av, bv))
		k.put(last)
		return k
	}
}

// atan2GradKernel çıktı şeklinde iki terim yazar: ta = g·x/(x²+y²), tb = −g·y/(x²+y²)
// (y = a, x = b; yayınlamalı). Taşma ve alt taşmaya karşı m = max(|x|, |y|) ile
// ölçeklenir; m = 0 ya da sonsuzsa terimler 0'dır. Meta: şekil, a adımı, b adımı.
func atan2GradKernel(nd int) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("atan2_turev_%d", nd))
		pa := k.pointer("a")
		pb := k.pointer("b")
		pg := k.pointer("g")
		ta := k.written("ta")
		tb := k.written("tb")
		n := k.u32p("n")
		k.arr("meta", 3*nd)
		i := k.globalIndex()
		last := k.bounded(i, n)
		ofs := k.tripleOffsets(i, nd, 2)
		y := k.load(tensor.Float32, k.addressS(pa, ofs[0], 4))
		x := k.load(tensor.Float32, k.addressS(pb, ofs[1], 4))
		g := k.load(tensor.Float32, k.address(pg, i, 4))
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
		empty, inf2 := k.p(), k.p()
		k.e("setp.eq.f32", empty, m, float32(0))
		k.e("setp.eq.f32", inf2, m, float32(math.Inf(1)))
		k.e("or.pred", empty, empty, inf2)
		k.store(tensor.Float32, k.address(ta, i, 4), k.pick(empty, float32(0), ga))
		k.store(tensor.Float32, k.address(tb, i, 4), k.pick(empty, float32(0), gb))
		k.put(last)
		return k
	}
}
