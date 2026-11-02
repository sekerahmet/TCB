package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ----------------------------------------------------------------------------
// Cinsli işleçler (15.6, 15.6a, 15.6c): TR_Int, TR_Bool ve float64 tensörleri
// ----------------------------------------------------------------------------
//
// İki tensör işlenen aynı cinsten olmalıdır; sayı işlenen tensörün cinsinden 0 boyutlu bir
// tensör gibi işlenir (TR_Int'te hassasiyete sığmalıdır). Tam sayı aritmetiği taşmada, sıfıra
// bölmede ve eksi üste ilk konumla hata verir; `/` ondalık (float32) verir. Cinsli işlemler
// türev izlemez.

// typedOperand tensör olmayan işleneni (sayı, mantık değeri) d'nin cinsinden 0 boyutlu bir
// tensöre çevirir; tensör işleneni olduğu gibi verir.
func (d *TensorValue) typedOperand(op string, other interp.Value, onRight bool) (*tensor.Tensor, error) {
	return d.typedOperandFor(op, op, other, onRight)
}

// typedOperandFor typedOperand'dır; fn iletide işlemi adlandırır (işleç ya da yerleşik).
func (d *TensorValue) typedOperandFor(fn any, op string, other interp.Value, onRight bool) (*tensor.Tensor, error) {
	if o, ok := other.(*TensorValue); ok {
		return o.t, nil
	}
	dt, dev := d.t.DType(), d.t.Device()
	mismatch := func(kind string) error {
		return messages.New(messages.H6968, fn, dt.Name(), interp.Shown(other), kind)
	}
	switch x := other.(type) {
	case *interp.IntVal:
		switch d.t.Elem() {
		case tensor.ElemInt:
			if (op == "<<" || op == ">>") && !onRight {
				// Kaydırma miktarı herhangi bir TR_Int'tir (15.6c uygulanmaz); aralığı işlem denetler.
				return safe(func() *tensor.Tensor { return tensor.Int64sOn(dev, tensor.Int64, []int64{x.Val}) })
			}
			if !dt.Fits(x.Val) {
				return nil, messages.New(messages.H6964, fn, "", x.Val, dt.Name(), "other")
			}
			return safe(func() *tensor.Tensor { return tensor.Int64sOn(dev, dt, []int64{x.Val}) })
		case tensor.ElemFloat:
			return safe(func() *tensor.Tensor { return tensor.Float64sOn(dev, []float64{float64(x.Val)}) })
		}
		return nil, mismatch("number")
	case *interp.FloatVal:
		switch d.t.Elem() {
		case tensor.ElemFloat:
			return safe(func() *tensor.Tensor { return tensor.Float64sOn(dev, []float64{x.Val}) })
		case tensor.ElemInt:
			return nil, mismatch("float")
		}
		return nil, mismatch("number")
	case *interp.BoolVal:
		if d.t.Elem() == tensor.ElemBool {
			return safe(func() *tensor.Tensor { return tensor.Int64sOn(dev, tensor.Bool, []int64{b2i(x.Val)}) })
		}
		return nil, mismatch("bool")
	case *interp.ListVal:
		return nil, mismatch("list")
	}
	return nil, messages.New(messages.H6204, fn, typeName(other))
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// isTypedOperand işlenen cinsli (TR_Int, TR_Bool, float64) bir tensör mü.
func isTypedOperand(v interp.Value) bool {
	o, ok := v.(*TensorValue)
	return ok && o.t.DType().IsTyped()
}

var cmpOps = map[string]tensor.CmpOp{"==": tensor.CmpEq, "!=": tensor.CmpNe, "<": tensor.CmpLt, ">": tensor.CmpGt, "<=": tensor.CmpLe, ">=": tensor.CmpGe}
var arithOps = map[string]tensor.ArithOp{"+": tensor.ArithAdd, "-": tensor.ArithSub, "*": tensor.ArithMul, "/": tensor.ArithDiv, "//": tensor.ArithFloorDiv, "%": tensor.ArithMod, "**": tensor.ArithPow}
var logicOps = map[string]tensor.LogicOp{"&": tensor.LogicAnd, "|": tensor.LogicOr, "^": tensor.LogicXor, "<<": tensor.LogicShl, ">>": tensor.LogicShr}

// typedOperator iki işlenenden en az biri cinsli tensörken ikili işleci uygular (onRight: d
// işlecin sağında). Kural tablosu 15.6'dadır.
func (d *TensorValue) typedOperator(m *interp.VM, op string, other interp.Value, onRight bool) (interp.Value, error) {
	o, err := d.typedOperand(op, other, onRight)
	if err != nil {
		return nil, err
	}
	a, b := d.t, o
	if onRight {
		a, b = o, d.t
	}
	if a.Elem() != b.Elem() {
		return nil, messages.New(messages.H6966, op, a.DType().Name(), b.DType().Name(), "other")
	}
	if err := checkBroadcast(m, op, a, b); err != nil {
		return nil, err
	}
	if err := checkDevice(op, a, b); err != nil {
		return nil, err
	}
	if c, ok := cmpOps[op]; ok {
		if a.Elem() == tensor.ElemBool && c != tensor.CmpEq && c != tensor.CmpNe {
			return nil, messages.New(messages.H6967, op, a.DType().Name(), "bool_order")
		}
		r, err := safe(func() *tensor.Tensor { return tensor.CompareTyped(a, b, c) })
		if err != nil {
			return nil, err
		}
		return New(r), nil
	}
	if _, ok := logicOps[op]; ok {
		return typedLogic(op, a, b)
	}
	if _, ok := arithOps[op]; ok {
		return typedArith(op, a, b)
	}
	if op == "@" {
		return nil, messages.New(messages.H6967, op, d.t.DType().Name(), "matmul")
	}
	return nil, messages.New(messages.H6703, interp.ValTensor, op, supportedOps)
}

// typedLogic & | ^ << >> (TR_Int'te bit, TR_Bool'da mantık; kaydırma yalnız TR_Int'te).
func typedLogic(op string, a, b *tensor.Tensor) (interp.Value, error) {
	lop := logicOps[op]
	var dt tensor.DType
	switch {
	case a.Elem() == tensor.ElemFloat:
		return nil, messages.New(messages.H6967, op, a.DType().Name(), "float_bit")
	case a.Elem() == tensor.ElemBool && (lop == tensor.LogicShl || lop == tensor.LogicShr):
		return nil, messages.New(messages.H6967, op, a.DType().Name(), "none")
	case a.Elem() == tensor.ElemBool:
		dt = tensor.Bool
	case lop == tensor.LogicShl || lop == tensor.LogicShr:
		dt = a.DType() // sonucun hassasiyeti sol işlenenden (15.6)
	default:
		dt = tensor.MergeInt(a.DType(), b.DType())
	}
	var f *tensor.ElemFault
	r, err := safe(func() (r *tensor.Tensor) { r, f = tensor.Logic(a, b, lop, dt); return })
	if err != nil {
		return nil, err
	}
	return typedResult(op, op, r, f)
}

// typedArith + - * / // % ** (TR_Int ve float64; TR_Bool'da tanımsız).
func typedArith(op string, a, b *tensor.Tensor) (interp.Value, error) {
	if a.Elem() == tensor.ElemBool {
		return nil, messages.New(messages.H6967, op, a.DType().Name(), "other")
	}
	aop := arithOps[op]
	if a.Elem() == tensor.ElemInt && aop == tensor.ArithDiv {
		// `/` tam sayıdan ondalığa geçiştir (15.3): iki yan "float32"ye çevrilip bölünür.
		return safe(func() interp.Value {
			x, _ := tensor.Convert(a, tensor.Float32)
			y, _ := tensor.Convert(b, tensor.Float32)
			return New(tensor.Div(x, y))
		})
	}
	dt := tensor.Float64
	if a.Elem() == tensor.ElemInt {
		dt = tensor.MergeInt(a.DType(), b.DType())
	}
	var f *tensor.ElemFault
	r, err := safe(func() (r *tensor.Tensor) { r, f = tensor.Arith(a, b, aop, dt); return })
	if err != nil {
		return nil, err
	}
	return typedResult(op, op, r, f)
}

var _ interp.Invertible = (*TensorValue)(nil)

// Invert tekli `~`dir: TR_Bool'da değil, TR_Int'te tümleme (4.12); ondalıkta tanımsız.
func (d *TensorValue) Invert() (interp.Value, error) {
	if d.t.Elem() == tensor.ElemFloat {
		return nil, messages.New(messages.H6967, "~", d.t.DType().Name(), "float_bit")
	}
	r, err := safe(func() *tensor.Tensor { return tensor.Not(d.t) })
	if err != nil {
		return nil, err
	}
	return New(r), nil
}
