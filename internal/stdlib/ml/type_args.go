package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Öğe cinsli kurucular (15.2): TR_Tensor[E](veri), TR_zeros[E](şekil), TR_ones[E](şekil),
// TR_eye[E](n). E TR_Float, TR_Int ya da TR_Bool'dur; TR_randn yalnız TR_Float kurar. x[E]
// çağrılabilir bir yerleşik verir (interp.RegisterTypeArgs); yerleşik cinssiz kurucuyu çağırıp
// sonucu E'nin varsayılan hassasiyetine çevirir. Çeviri kesindir: sığmayan ya da NaN öğe
// hatadır (15.3), türev yalnız ondalık tensörde izlenir.

// kindDTypes öğe cinsinin varsayılan hassasiyeti (15.1).
var kindDTypes = map[codes.Code]tensor.DType{
	codes.TypeFloat: tensor.Float32,
	codes.TypeInt:   tensor.Int64,
	codes.TypeBool:  tensor.Bool,
}

// registerTypeArgs cinsli kurucuları kaydeder; tensorFn TR_Tensor'ün cinssiz çağrısıdır.
func registerTypeArgs(v *interp.VM, tensorFn interp.BuiltinFunc) {
	for _, k := range []codes.Code{codes.FnZeros, codes.FnOnes, codes.FnEye, codes.FnRandn} {
		v.RegisterTypeArgs(k, kindConstructor(k, v.Builtin(k).Fn))
	}
	v.RegisterTypeArgs(codes.TypeTensor, kindConstructor(codes.TypeTensor, tensorFn))
}

// kindConstructor k[E]'nin yerleşiğini kurar; base k'nin cinssiz çağrısıdır.
func kindConstructor(k codes.Code, base interp.BuiltinFunc) interp.TypeArgsFunc {
	fn := codes.NameRef(k)
	return func(_ *interp.VM, args []*interp.TypeVal) (*interp.BuiltinVal, error) {
		dt, ok := kindDTypes[args[0].Code]
		if !ok || args[0].Struct != nil || args[0].Elems != nil {
			return nil, messages.New(messages.H6971, fn, interp.Shown(args[0]))
		}
		if k == codes.FnRandn && dt != tensor.Float32 {
			return nil, messages.New(messages.H6972, fn, interp.Shown(args[0]))
		}
		call := func(m *interp.VM, a []interp.Value, named map[string]interp.Value) (interp.Value, error) {
			if dt == tensor.Float32 {
				return base(m, a, named)
			}
			if grad, given := bind.GetNamed(named, nameRequiresGrad); given && grad.IsTruthy() {
				return nil, messages.New(messages.H6973, fn, codes.NameRef(codes.ArgRequiresGrad), codes.NameRef(codes.KwTrue), dt.Name())
			}
			if k == codes.TypeTensor && len(a) == 1 && !hasNamed(named, nameDType) {
				if _, isTensor := a[0].(*TensorValue); !isTensor {
					return kindData(fn, args[0].Code, dt, a[0])
				}
			}
			r, err := base(m, a, named)
			if err != nil {
				return nil, err
			}
			t, f := tensor.Convert(r.(*TensorValue).t, dt)
			return typedResult(fn, "", t, f)
		}
		return &interp.BuiltinVal{Name: codes.Name(k), Fn: guarded(call), Pure: true}, nil
	}
}

// kindData TR_Tensor[E](veri)'yi kurar (E TR_Int ya da TR_Bool): veri tek değer ya da iç içe
// listedir; öğeler doğrudan okunur (float32'den geçmez, 2²⁴'ün üstü yuvarlanmaz). Başka cinsten
// öğe hatadır; ileti ilk öğenin konumunu söyler.
func kindData(fn codes.Ref, kind codes.Code, dt tensor.DType, v interp.Value) (interp.Value, error) {
	var data []int64
	var bad interp.Value
	badAt := 0
	leaf := func(x interp.Value) error {
		switch e := x.(type) {
		case *interp.IntVal:
			if dt == tensor.Int64 {
				data = append(data, e.Val)
				return nil
			}
		case *interp.BoolVal:
			if dt == tensor.Bool {
				data = append(data, b2i(e.Val))
				return nil
			}
		}
		if bad == nil {
			bad, badAt = x, len(data)
		}
		data = append(data, 0) // konum sayılır; tensör kurulmaz
		return nil
	}
	var sh []int
	var err error
	if l, isList := v.(*interp.ListVal); isList {
		sh, err = walkList(fn, l, func(n int) { data = make([]int64, 0, n) }, leaf)
	} else {
		err = leaf(v)
	}
	if bad != nil {
		index := ""
		if len(sh) > 0 {
			index = positionText(badAt, sh)
		}
		return nil, messages.New(messages.H6974, fn, index, interp.Shown(bad), codes.NameRef(kind))
	}
	if err != nil {
		return nil, err
	}
	t, err := safe(func() *tensor.Tensor { return tensor.Int64sOn(tensor.CPU, dt, data, sh...) })
	if err != nil {
		return nil, err
	}
	return New(t), nil
}

// hasNamed adlı argüman verilmiş mi?
func hasNamed(named map[string]interp.Value, a bind.Named) bool {
	_, given := bind.GetNamed(named, a)
	return given
}
