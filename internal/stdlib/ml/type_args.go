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
// sonucu E'nin hassasiyetine (TR_dtype: ya da varsayılan) çevirir. Çeviri kesindir: sığmayan ya
// da NaN öğe hatadır (15.3); TR_Float'a çeviri en yakına yuvarlar. Türev yalnız TR_Float'tadır.

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
		kind := args[0].Code
		if _, ok := kindDTypes[kind]; !ok || args[0].Struct != nil || args[0].Elems != nil {
			return nil, messages.New(messages.H6971, fn, interp.Shown(args[0]), codes.Spelling(k))
		}
		if k == codes.FnRandn && kind != codes.TypeFloat {
			// İletinin bütün adları kod örneğindedir: kimlik yazımıyla.
			return nil, messages.New(messages.H6972, codes.Spelling(k), codes.Spelling(kind))
		}
		call := func(m *interp.VM, a []interp.Value, named map[string]interp.Value) (interp.Value, error) {
			if kind == codes.TypeFloat && k != codes.TypeTensor {
				return base(m, a, named) // cinssiz kurucu zaten TR_Float'tır
			}
			return kindCall(m, fn, k, kind, base, a, named)
		}
		return &interp.BuiltinVal{Name: codes.Name(k), Fn: guarded(call), Pure: true}, nil
	}
}

// kindCall k[kind](a...)'yı yapar: cinssiz kurucuyla (ya da TR_Tensor'ün verisinde doğrudan)
// kurar, kind'ın hassasiyetine çevirir, türevi en sonda açar.
func kindCall(m *interp.VM, fn codes.Ref, k, kind codes.Code, base interp.BuiltinFunc, a []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	dt, err := constructorDType(fn, named, kind)
	if err != nil {
		return nil, err
	}
	if err := gradKindError(fn, named, dt); err != nil {
		return nil, err
	}
	var positional interp.Value
	if k == codes.TypeTensor && len(a) == 2 {
		positional, a = a[1], a[:1] // TR_Tensor(veri, türev) yazımı
	}
	watch, err := requiresGrad(fn, named, positional, false)
	if err != nil {
		return nil, err
	}
	rest := withoutNamed(named, nameRequiresGrad)
	var t *tensor.Tensor
	src, isTensor := a[0].(*TensorValue)
	if k == codes.TypeTensor && len(a) == 1 && isTensor && kindOf(src.t.DType()) != kind {
		// Cins dönüşümü (15.3) doğrudan cihazdadır, ara kopya yok: TR_replay kaydında konağa okumaz.
		if err := bind.CheckNamed(fn.Called(), named, nameRequiresGrad, nameDType); err != nil {
			return nil, err
		}
		if (src.t.Elem() == tensor.ElemBool) != (kind == codes.TypeBool) {
			return nil, messages.New(messages.H6978, fn, codes.NameRef(kindOf(src.t.DType())), codes.NameRef(kind))
		}
		t = src.t
	} else if k == codes.TypeTensor && len(a) == 1 && !isTensor {
		if err := bind.CheckNamed(fn.Called(), named, nameRequiresGrad, nameDType); err != nil {
			return nil, err
		}
		t, err = kindData(fn, kind, dt, a[0])
	} else {
		// TR_dtype: hedef hassasiyettir (yukarıda okundu); cinssiz kurucuya gitmez.
		var r interp.Value
		if r, err = base(m, a, withoutNamed(rest, nameDType)); err == nil {
			t = r.(*TensorValue).t
		}
		if err == nil && isTensor && (src.t.Elem() == tensor.ElemBool) != (kind == codes.TypeBool) {
			// Mantık ile sayı arasında dönüşüm yoktur (3.12, 15.3).
			err = messages.New(messages.H6978, fn, codes.NameRef(kindOf(src.t.DType())), codes.NameRef(kind))
		}
	}
	if err != nil {
		return nil, err
	}
	if t, err = convertKind(fn, t, dt); err != nil {
		return nil, err
	}
	if watch {
		t.SetRequiresGrad(true)
	}
	return New(t), nil
}

// kindData TR_Tensor[E](veri)'nin verisini kurar: TR_Int ve TR_Bool'da öğeler o cinsten olmalı
// (doğrudan okunur, float32'den geçmez); TR_Float'ta sayılar en yakına yuvarlanır (tam sayı
// listesinden ondalık tensör, açık cins dönüşümü), mantık değeri hatadır.
func kindData(fn codes.Ref, kind codes.Code, dt tensor.DType, v interp.Value) (*tensor.Tensor, error) {
	d, err := readData(fn, v)
	if err != nil {
		return nil, err
	}
	if kind != codes.TypeFloat {
		return d.exact(fn, kind)
	}
	for i, x := range d.leaves {
		if _, number := interp.ToFloat(x); !number {
			return nil, messages.New(messages.H6974, fn, d.position(i), interp.Shown(x), codes.NameRef(kind))
		}
	}
	return d.float(dt)
}
