package ml

import (
	"math"
	"slices"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Öğe öğe en büyük ve en küçük (15.16a) ve yakınlık sınaması (15.18a).

var (
	nameRtol = bind.NamedArg(codes.ArgRtol)
	nameAtol = bind.NamedArg(codes.ArgAtol)
)

// registerExtremes TR_maximum, TR_minimum ve TR_all_close'u bağlar.
func registerExtremes(v *interp.VM) {
	wrap(v, codes.FnMaximum, newBuiltin(extremeBuiltin(codes.FnMaximum, tensor.Maximum, false)))
	wrap(v, codes.FnMinimum, newBuiltin(extremeBuiltin(codes.FnMinimum, tensor.Minimum, true)))
	wrap(v, codes.FnAllClose, newBuiltin(allCloseBuiltin))
}

// extremeBuiltin iki tensörün ya da tensörle sayının öğe öğe seçimidir; cins aynı olmalıdır.
func extremeBuiltin(kd codes.Code, pick func(a, b *tensor.Tensor) *tensor.Tensor, lower bool) interp.BuiltinFunc {
	ad := codes.NameRef(kd)
	return func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		if err := checkWholeArgs(kd, args, named, 2); err != nil {
			return nil, err
		}
		for _, a := range args {
			if err := listError(ad, a, messages.New(messages.H6131), tensorCallExample(ad, ", x")); err != nil {
				return nil, err
			}
		}
		if !isTensor(args) {
			return extremeNumbers(ad, args[0], args[1], lower)
		}
		a, b, err := extremeOperands(ad, args)
		if err != nil {
			return nil, err
		}
		if err := checkBroadcast(m, ad, a, b); err != nil {
			return nil, err
		}
		if err := checkDevice(ad, a, b); err != nil {
			return nil, err
		}
		return safe(func() interp.Value { return New(onePrecision(pick(a, b), args)) })
	}
}

// onePrecision sayı işlenenli sonucu tensörün hassasiyetine bir kez yuvarlar (15.6a).
func onePrecision(r *tensor.Tensor, args []interp.Value) *tensor.Tensor {
	if isTensorValue(args[0]) && isTensorValue(args[1]) {
		return r
	}
	for _, v := range args {
		if d, ok := v.(*TensorValue); ok && d.t.DType() != r.DType() {
			return r.Cast(d.t.DType())
		}
	}
	return r
}

func isTensorValue(v interp.Value) bool {
	_, ok := v.(*TensorValue)
	return ok
}

// floatOperand ondalık yolun işlenenidir: sayı daraltılmaz, en az float32'de kurulur (15.6a).
func floatOperand(ad any, v interp.Value, ref *tensor.Tensor) (*tensor.Tensor, error) {
	if d, ok := v.(*TensorValue); ok {
		return d.t, nil
	}
	f, ok := interp.ToFloat(v)
	if !ok {
		return nil, messages.New(messages.H6204, ad, typeName(v))
	}
	return safe(func() *tensor.Tensor {
		if ref.DType() == tensor.Float64 {
			return tensor.Float64sOn(ref.Device(), []float64{f})
		}
		return tensor.FullOn(ref.Device(), float32(f))
	})
}

// extremeOperands iki işleneni aynı cinsten tensörlere çevirir. Sayı tensörün cinsini alır
// (15.6a); mantık tensöründe sıralama yoktur, nicemlenmiş tensör açılmaz (15.53).
func extremeOperands(ad any, args []interp.Value) (*tensor.Tensor, *tensor.Tensor, error) {
	var ref *tensor.Tensor
	for _, v := range args {
		if d, ok := v.(*TensorValue); ok {
			switch {
			case d.t.DType().IsQuantized():
				return nil, nil, messages.New(messages.H6687, d.t.DType().Name())
			case d.t.Elem() == tensor.ElemBool:
				return nil, nil, messages.New(messages.H6967, ad, d.t.DType().Name(), "bool_order")
			}
			if ref == nil {
				ref = d.t
			}
		}
	}
	if ref.Elem() == tensor.ElemFloat {
		for _, v := range args {
			if d, ok := v.(*TensorValue); ok && d.t.Elem() != tensor.ElemFloat {
				return nil, nil, kindMismatch(ad, ref.DType(), d.t.DType())
			}
		}
		a, err := floatOperand(ad, args[0], ref)
		if err != nil {
			return nil, nil, err
		}
		b, err := floatOperand(ad, args[1], ref)
		if err != nil {
			return nil, nil, err
		}
		return a, b, nil
	}
	own := &TensorValue{t: ref}
	a, err := own.typedOperandFor(ad, "", args[0], false)
	if err != nil {
		return nil, nil, err
	}
	b, err := own.typedOperandFor(ad, "", args[1], false)
	if err != nil {
		return nil, nil, err
	}
	if a.Elem() != b.Elem() {
		return nil, nil, kindMismatch(ad, a.DType(), b.DType())
	}
	return a, b, nil
}

// extremeNumbers iki sayının seçimidir: iki TR_Int TR_Int, biri ondalıksa TR_Float verir (4.2).
func extremeNumbers(ad any, a, b interp.Value, lower bool) (interp.Value, error) {
	x, xInt := a.(*interp.IntVal)
	y, yInt := b.(*interp.IntVal)
	if xInt && yInt {
		if (x.Val < y.Val) == lower {
			return x, nil
		}
		return y, nil
	}
	p, ok1 := interp.ToFloat(a)
	q, ok2 := interp.ToFloat(b)
	if !ok1 || !ok2 {
		return nil, messages.New(messages.H6118, ad, typeName(a), typeName(b))
	}
	if lower {
		return &interp.FloatVal{Val: math.Min(p, q)}, nil
	}
	return &interp.FloatVal{Val: math.Max(p, q)}, nil
}

// closeTolerances hassasiyetin varsayılan toleransları (15.18a).
var closeTolerances = map[tensor.DType][2]float64{
	tensor.Float64:  {1e-7, 1e-7},
	tensor.Float32:  {1.3e-6, 1e-5},
	tensor.Float16:  {1e-3, 1e-5},
	tensor.BFloat16: {1.6e-2, 1e-5},
}

// allCloseBuiltin TR_all_close(a, b): şekiller aynıysa ve her öğe yakınsa doğru. Karşılaştırma
// konakta float64'tedir: sonuç zaten konağa okunur.
func allCloseBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(codes.FnAllClose)
	if err := bind.CheckNamed(fn.Called(), named, nameRtol, nameAtol); err != nil {
		return nil, err
	}
	if len(args) != 2 {
		return nil, interp.ArgCountError(codes.FnAllClose, 2, len(args))
	}
	var ts [2]*tensor.Tensor
	for i, v := range args {
		d, ok := v.(*TensorValue)
		switch {
		case !ok:
			return nil, messages.New(messages.H6979, fn, "tensor", typeName(v))
		case d.t.DType().IsQuantized():
			return nil, messages.New(messages.H6687, d.t.DType().Name())
		case d.t.Elem() != tensor.ElemFloat:
			return nil, messages.New(messages.H7050, fn, codes.NameRef(kindOf(d.t.DType())))
		}
		ts[i] = d.t
	}
	rtol, atol, err := closeBounds(fn, ts[0].DType(), ts[1].DType(), named)
	if err != nil {
		return nil, err
	}
	if !slices.Equal(ts[0].Shape(), ts[1].Shape()) {
		return interp.False, nil
	}
	return safe(func() interp.Value {
		x, y := ts[0].Float64s(), ts[1].Float64s()
		for i := range x {
			if !nearFloats(x[i], y[i], rtol, atol) {
				return interp.False
			}
		}
		return interp.True
	})
}

// closeBounds toleransları kurar: varsayılan daha az hassas işlenenindir, adlı argüman üstüne yazar.
func closeBounds(fn any, a, b tensor.DType, named map[string]interp.Value) (float64, float64, error) {
	ta, tb := closeTolerances[a], closeTolerances[b]
	bound := [2]float64{max(ta[0], tb[0]), max(ta[1], tb[1])}
	for i, name := range []bind.Named{nameRtol, nameAtol} {
		v, ok := bind.GetNamed(named, name)
		if !ok {
			continue
		}
		f, isNumber := interp.ToFloat(v)
		if !isNumber {
			return 0, 0, messages.New(messages.H6517, fn, name, typeName(v))
		}
		bound[i] = f
	}
	return bound[0], bound[1], nil
}

// nearFloats iki öğenin yakınlığıdır (15.18a): sonsuz yalnız aynı işaretli sonsuza yakındır, NaN
// hiçbir şeye yakın değildir.
func nearFloats(x, y, rtol, atol float64) bool {
	if math.IsInf(x, 0) || math.IsInf(y, 0) {
		return x == y
	}
	scaled := float64(rtol * max(math.Abs(x), math.Abs(y))) // açık yuvarlama: FMA makineye bağlı sonuç verir
	return math.Abs(x-y) <= atol+scaled
}
