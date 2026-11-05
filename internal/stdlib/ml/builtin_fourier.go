package ml

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/detmath"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Fourier dönüşümü, pencere ve STFT (23.1–23.3). Hesap tensör çekirdeğindedir; burada argümanlar
// denetlenir. Karmaşık sonuç (gerçek, sanal) demetidir.

var (
	nameN        = bind.NamedArg(codes.ArgN)
	namePeriodic = bind.NamedArg(codes.ArgPeriodic)
	nameHop      = bind.NamedArg(codes.ArgHopLength)
	nameCenter   = bind.NamedArg(codes.ArgCenter)
	namePadMode  = bind.NamedArg(codes.ArgPadMode)
)

// registerFourier yerleşikleri bağlar.
func registerFourier(v *interp.VM) {
	wrap(v, codes.FnRfft, newBuiltin(rfftBuiltin))
	wrap(v, codes.FnHannWindow, newBuiltin(hannBuiltin))
	wrap(v, codes.FnStft, newBuiltin(stftBuiltin))
}

// fourierTrig köklerin platformdan bağımsız sinüs ve kosinüsüdür.
var fourierTrig = tensor.Trig{Sin: detmath.Sin, Cos: detmath.Cos}

// fourierCall çekirdeğin çağrı bilgisidir: GPU iletisi tensörü CPU'ya taşımayı önerir (23.5).
func fourierCall(fn any) tensor.FourierCall {
	return tensor.FourierCall{Fn: fn, Move: "x." + codes.Spelling(codes.MemToDevice) + `("cpu")`, Trig: fourierTrig}
}

// complexResult (gerçek, sanal) demetidir.
func complexResult(re, im *tensor.Tensor) interp.Value {
	return &interp.TupleVal{Elements: []interp.Value{New(re), New(im)}}
}

// signalArg x argümanını denetler: ondalık tensördür, son ekseni vardır.
func signalArg(fn any, v interp.Value) (*tensor.Tensor, error) {
	d, ok := v.(*TensorValue)
	switch {
	case !ok:
		return nil, messages.New(messages.H7076, fn, typeName(v), "float", floatFix())
	case d.t.DType().IsQuantized():
		return nil, messages.New(messages.H6687, d.t.DType().Name())
	case d.t.Elem() != tensor.ElemFloat:
		return nil, messages.New(messages.H7076, fn, d.t.DType().Name(), "float", floatFix())
	case d.t.Dim() == 0:
		return nil, messages.New(messages.H7076, fn, tensor.ShapeText(d.t.Shape()), "scalar", "x."+codes.Spelling(codes.MemReshape)+"([1])")
	}
	return d.t, nil
}

// positiveInt v'nin pozitif tam sayı olmasını ister.
func positiveInt(fn, name any, v interp.Value) (int, error) {
	n, ok := v.(*interp.IntVal)
	if !ok || n.Val < 1 {
		return 0, messages.New(messages.H5530, fn, name, interp.Shown(v))
	}
	return int(n.Val), nil
}

// rfftBuiltin TR_rfft(x, TR_n: n).
func rfftBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	kd := codes.FnRfft
	fn := codes.NameRef(kd)
	if err := bind.CheckNamed(fn.Called(), named, nameN); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, interp.ArgCountError(kd, 1, len(args))
	}
	x, err := signalArg(fn, args[0])
	if err != nil {
		return nil, err
	}
	last := x.Shape()[x.Dim()-1]
	n := last
	if v, given := bind.GetNamed(named, nameN); given {
		if n, err = positiveInt(fn, nameN, v); err != nil {
			return nil, err
		}
	} else if n == 0 {
		return nil, messages.New(messages.H5530, fn, nameN, 0)
	}
	if last > n {
		return nil, messages.New(messages.H7078, fn, last, n, "long")
	}
	if err := refuseGrad(fn, x); err != nil {
		return nil, err
	}
	return safe(func() interp.Value {
		re, im := tensor.Rfft(x, n, fourierCall(fn))
		return complexResult(re, im)
	})
}

// refuseGrad türev isteyen girdiyi açıkça reddeder: sonuç sessizce kopmaz.
func refuseGrad(fn any, ts ...*tensor.Tensor) error {
	for _, t := range ts {
		if t.RequiresGrad() && tensor.GradEnabled() {
			return messages.New(messages.H7076, fn, codes.Spelling(codes.ArgRequiresGrad), "grad", codes.Spelling(codes.FnNoGrad)+" { ... }")
		}
	}
	return nil
}

// hannBuiltin TR_hann_window(n, TR_periodic: b).
func hannBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	kd := codes.FnHannWindow
	fn := codes.NameRef(kd)
	if err := bind.CheckNamed(fn.Called(), named, namePeriodic); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, interp.ArgCountError(kd, 1, len(args))
	}
	n, err := positiveInt(fn, "n", args[0])
	if err != nil {
		return nil, err
	}
	v, given := bind.GetNamed(named, namePeriodic)
	if !given {
		return nil, messages.New(messages.H7052, fn, namePeriodic, "23.2")
	}
	periodic, err := bind.Bool(fn, namePeriodic, v)
	if err != nil {
		return nil, err
	}
	return safe(func() interp.Value { return New(tensor.HannWindow(n, periodic, fourierTrig)) })
}

// stftOptions adlı argümanları denetler.
func stftOptions(fn any, n int, named map[string]interp.Value) (tensor.StftOptions, *tensor.Tensor, error) {
	o := tensor.StftOptions{N: n}
	hop, given := bind.GetNamed(named, nameHop)
	if !given {
		return o, nil, messages.New(messages.H7052, fn, nameHop, "23.3")
	}
	var err error
	if o.Hop, err = positiveInt(fn, nameHop, hop); err != nil {
		return o, nil, err
	}
	wv, given := bind.GetNamed(named, nameWindow)
	if !given {
		return o, nil, messages.New(messages.H7052, fn, nameWindow, "23.3")
	}
	w, ok := wv.(*TensorValue)
	if !ok || w.t.Elem() != tensor.ElemFloat || w.t.DType().IsQuantized() || w.t.Dim() != 1 || w.t.Shape()[0] != n {
		return o, nil, messages.New(messages.H7076, fn, windowText(wv), "window", hannFix(n))
	}
	if o.Center, err = bind.OptionalBool(fn, named, nameCenter, true); err != nil {
		return o, nil, err
	}
	mode, given := bind.GetNamed(named, namePadMode)
	switch {
	case !o.Center && given:
		return o, nil, messages.New(messages.H7079, fn, "pad_given", interp.Shown(mode), "", "")
	case o.Center && !given:
		return o, nil, messages.New(messages.H7052, fn, namePadMode, "23.3")
	case o.Center:
		s, isText := mode.(*interp.StringVal)
		if !isText || s.Val != "reflect" && s.Val != "constant" {
			return o, nil, messages.New(messages.H7077, fn, namePadMode, `"reflect", "constant"`, interp.Shown(mode), "23.3")
		}
		o.Reflect = s.Val == "reflect"
	}
	return o, w.t, nil
}

// floatFix tensörü ondalığa çevirmenin yazımıdır.
func floatFix() string {
	return codes.Spelling(codes.TypeTensor) + "[" + codes.Spelling(codes.TypeFloat) + "](x)"
}

// hannFix n uzunlukta pencerenin yazımıdır.
func hannFix(n int) string {
	return fmt.Sprintf("%s(%d, %s: %s)", codes.Spelling(codes.FnHannWindow), n, codes.Spelling(codes.ArgPeriodic), codes.Spelling(codes.KwTrue))
}

// windowText iletide pencerenin kendisini anlatır.
func windowText(v interp.Value) any {
	if w, ok := v.(*TensorValue); ok {
		return tensor.ShapeText(w.t.Shape())
	}
	return typeName(v)
}

// stftBuiltin TR_stft(x, n, TR_hop_length: h, TR_window: w, TR_center: b, TR_pad_mode: kip).
func stftBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	kd := codes.FnStft
	fn := codes.NameRef(kd)
	if err := bind.CheckNamed(fn.Called(), named, nameHop, nameWindow, nameCenter, namePadMode); err != nil {
		return nil, err
	}
	if len(args) != 2 {
		return nil, interp.ArgCountError(kd, 2, len(args))
	}
	x, err := signalArg(fn, args[0])
	if err != nil {
		return nil, err
	}
	n, err := positiveInt(fn, "n", args[1])
	if err != nil {
		return nil, err
	}
	o, w, err := stftOptions(fn, n, named)
	if err != nil {
		return nil, err
	}
	last := x.Shape()[x.Dim()-1]
	if o.Center && o.Reflect && last <= n/2 {
		return nil, messages.New(messages.H7078, fn, last, n/2, "reflect")
	}
	if padded, _ := tensor.StftFrames(last, o); padded < n {
		return nil, messages.New(messages.H7078, fn, padded, n, "short")
	}
	if err := refuseGrad(fn, x, w); err != nil {
		return nil, err
	}
	return safe(func() interp.Value {
		re, im := tensor.Stft(x, w, o, fourierCall(fn))
		return complexResult(re, im)
	})
}
