package ml

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// log Σ exp (15.18). Hesap tensör çekirdeğindedir; burada argümanlar denetlenir.

// registerLogSumExp yerleşiği bağlar.
func registerLogSumExp(v *interp.VM) {
	wrap(v, codes.FnLogsumexp, newBuiltin(logSumExpBuiltin))
}

// toCPUFix tensörü CPU'ya taşıyan yazımdır.
func toCPUFix() string { return "x." + codes.Spelling(codes.MemToDevice) + `("cpu")` }

// cpuFloatArg x argümanını denetler: CPU'da ondalık tensördür (15.18, 25.9).
func cpuFloatArg(fn any, x *tensor.Tensor) error {
	switch {
	case x.DType().IsQuantized():
		return messages.New(messages.H6687, x.DType().Name())
	case x.Elem() != tensor.ElemFloat:
		return messages.New(messages.H7081, fn, x.DType().Name(), "float", floatFix())
	case x.Device() != tensor.CPU:
		return messages.New(messages.H7081, fn, x.Device().String(), "gpu", toCPUFix())
	}
	return nil
}

// lseAxes TR_dim:'i eksen listesine çevirir; verilmemişse sonuç nil'dir (bütün öğeler).
func lseAxes(fn any, kd codes.Code, shape []int, named map[string]interp.Value) ([]int, error) {
	v, given := bind.GetNamed(named, nameAxis)
	if !given || v == interp.Null {
		return nil, nil
	}
	list, isList := v.(*interp.ListVal)
	if !isList {
		e, ok := intNumber(v)
		if !ok {
			return nil, messages.New(messages.H6106, fn, interp.Shown(v))
		}
		e, err := parseAxis(fn, e, shape)
		return []int{e}, err
	}
	if len(list.Elements) == 0 {
		return nil, messages.New(messages.H7082, fn, codes.Spelling(kd)+"(x)")
	}
	axes := make([]int, 0, len(list.Elements))
	twice := false
	for _, a := range list.Elements {
		e, ok := intNumber(a)
		if !ok {
			return nil, messages.New(messages.H6106, fn, interp.Shown(a))
		}
		e, err := parseAxis(fn, e, shape)
		if err != nil {
			return nil, err
		}
		if slices.Contains(axes, e) {
			twice = true
			continue
		}
		axes = append(axes, e)
	}
	if twice {
		fix := fmt.Sprintf("%s(x, %s: %s)", codes.Spelling(kd), codes.Spelling(codes.ArgDim), strings.ReplaceAll(fmt.Sprint(axes), " ", ", "))
		return nil, messages.New(messages.H7081, fn, interp.Shown(v), "twice", fix)
	}
	return axes, nil
}

// logSumExpBuiltin TR_logsumexp(t, TR_dim: k, TR_keepdim: b).
func logSumExpBuiltin(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	kd := codes.FnLogsumexp
	fn := codes.NameRef(kd)
	if err := bind.CheckNamed(fn.Called(), named, nameAxis, nameKeepDim); err != nil {
		return nil, err
	}
	if len(args) == 2 {
		e, _ := intNumber(args[1]) // sayı değilse ilk eksen önerilir
		return nil, messages.New(messages.H7086, fn, fmt.Sprintf("%s(x, %s: %d)", codes.Spelling(kd), codes.Spelling(codes.ArgDim), e))
	}
	if len(args) != 1 {
		return nil, interp.ArgCountError(kd, 1, len(args))
	}
	d, ok := args[0].(*TensorValue)
	if !ok {
		return nil, messages.New(messages.H7081, fn, typeName(args[0]), "float", floatFix())
	}
	if err := cpuFloatArg(fn, d.t); err != nil {
		return nil, err
	}
	keep, err := bind.OptionalBool(fn, named, nameKeepDim, false)
	if err != nil {
		return nil, err
	}
	axes, err := lseAxes(fn, kd, d.t.Shape(), named)
	if err != nil {
		return nil, err
	}
	t, err := highPrecision(m, d.t) // karışık hassasiyet bloğunda float32 verir (15.4)
	if err != nil {
		return nil, err
	}
	r, err := safe(func() *tensor.Tensor { return tensor.LogSumExp(t, axes, keep) })
	if err != nil {
		return nil, err
	}
	return New(r), nil
}
