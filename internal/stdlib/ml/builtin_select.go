package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// koşullu(koşul, a, b): eleman bazlı seçim (bkz. tensor.Where).

// whereBuiltin koşullu(koşul, a, b). Hiçbiri tensör değilse koşul Mantık ya da sayı
// olmalıdır ve a ya da b olduğu gibi döner. Değilse sayılar ve Mantık tensöre çevrilir
// (a ile b'deki sayılar ilk tensör işlenenin türünü alır), üç şekil yayınlanır.
func whereBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	ad := codes.Name(codes.T234)
	if err := bind.CheckNamed(ad+"()", named); err != nil {
		return nil, err
	}
	if len(args) != 3 {
		return nil, messages.New(messages.H6201, ad, len(args))
	}
	for _, a := range args {
		if err := listError(ad, a, "tensörde, sayıda ve "+codes.Name(codes.A203)+" değerinde", ad+"("+codes.Name(codes.T100)+"(...), a, b)"); err != nil {
			return nil, err
		}
	}
	if !isTensor(args) {
		d, err := condValue(ad, args[0])
		if err != nil {
			return nil, err
		}
		for _, a := range args[1:] {
			if _, ok := interp.ToFloat(a); !ok {
				return nil, messages.New(messages.H6204, ad, typeName(a))
			}
		}
		if d {
			return args[1], nil
		}
		return args[2], nil
	}
	ts, err := condTensors(ad, args)
	if err != nil {
		return nil, err
	}
	if err := checkBroadcast3(ad, ts); err != nil {
		return nil, err
	}
	r, err := safe(func() *tensor.Tensor { return tensor.Where(ts[0], ts[1], ts[2]) })
	if err != nil {
		return nil, err
	}
	s := New(r)
	s.hasInts = isIntValued(args[1]) && isIntValued(args[2])
	return s, nil
}

// condValue tensör olmayan koşulun doğruluğu: Mantık ya da sayı (0 dışı doğru).
func condValue(ad string, v interp.Value) (bool, error) {
	if b, ok := v.(*interp.BoolVal); ok {
		return b.Val, nil
	}
	if f, ok := interp.ToFloat(v); ok {
		return f != 0, nil
	}
	return false, messages.New(messages.H6207, ad, codes.Name(codes.A203), typeName(v))
}

// isIntValued değer tam sayılı bir tensör ya da Tam mı (sonucun tam sayılı kalması için).
func isIntValued(v interp.Value) bool {
	switch x := v.(type) {
	case *TensorValue:
		return x.hasInts
	case *interp.IntVal:
		return true
	}
	return false
}

// condTensors koşul, a ve b'yi aynı cihazda tensöre çevirir. Mantık koşul 1 ya da 0
// olur; a ve b'deki sayılar ilk tensör işlenenin (yoksa float32) türünü alır.
func condTensors(ad string, args []interp.Value) ([]*tensor.Tensor, error) {
	cond := args[0]
	if b, ok := cond.(*interp.BoolVal); ok {
		cond = interp.NewInt(0)
		if b.Val {
			cond = interp.NewInt(1)
		}
	} else if _, ok := cond.(*TensorValue); !ok {
		if _, number := interp.ToFloat(cond); !number {
			return nil, messages.New(messages.H6207, ad, codes.Name(codes.A203), typeName(cond))
		}
	}
	for _, a := range args[1:] {
		if _, ok := a.(*TensorValue); ok {
			continue
		}
		if _, ok := interp.ToFloat(a); !ok {
			return nil, messages.New(messages.H6204, ad, typeName(a))
		}
	}
	values := []interp.Value{args[1], args[2], cond}
	if !isTensor(values[:2]) {
		// a ve b sayı: koşul tensördür; a ve b onun cihazında float32 tensörler olur.
		c := cond.(*TensorValue).t
		ts := []*tensor.Tensor{c, nil, nil}
		for i, a := range args[1:] {
			t, err := toTensorOn(ad, a, c)
			if err != nil {
				return nil, err
			}
			ts[i+1] = t
		}
		return ts, nil
	}
	ts, err := toTensors(ad, values)
	if err != nil {
		return nil, err
	}
	return []*tensor.Tensor{ts[2], ts[0], ts[1]}, nil
}

// checkBroadcast3 koşul, a ve b şekillerinin birlikte yayınlanabilirliğini denetler.
func checkBroadcast3(ad string, ts []*tensor.Tensor) error {
	nd := 0
	for _, t := range ts {
		nd = max(nd, t.Dim())
	}
	for i := 1; i <= nd; i++ {
		size := 1
		for _, t := range ts {
			d := axisFromEnd(t, i)
			if d != 1 && size != 1 && d != size {
				return messages.New(messages.H6214, ad, shapeText(ts[0].Shape()), shapeText(ts[1].Shape()), shapeText(ts[2].Shape()), i)
			}
			if d != 1 {
				size = d
			}
		}
	}
	return nil
}
