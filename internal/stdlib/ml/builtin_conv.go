package ml

import (
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Bir boyutlu evrişim (15.23c). Hesap tensör çekirdeğindedir; burada argümanlar denetlenir.

var (
	nameStride   = bind.NamedArg(codes.ArgStride)
	namePadding  = bind.NamedArg(codes.ArgPadding)
	nameDilation = bind.NamedArg(codes.ArgDilation)
	nameGroups   = bind.NamedArg(codes.ArgNumGroups)
)

// registerConv yerleşiği bağlar.
func registerConv(v *interp.VM) {
	wrap(v, codes.FnConv1d, newBuiltin(conv1dBuiltin))
}

// convTensor x ya da w argümanını denetler.
func convTensor(fn any, v interp.Value) (*tensor.Tensor, error) {
	d, ok := v.(*TensorValue)
	if !ok {
		return nil, messages.New(messages.H7081, fn, typeName(v), "float", floatFix())
	}
	return d.t, cpuFloatArg(fn, d.t)
}

// convOptions adlı argümanları okur.
func convOptions(fn any, named map[string]interp.Value) (o tensor.Conv1dOptions, err error) {
	o = tensor.Conv1dOptions{Stride: 1, Dilation: 1, Groups: 1}
	for _, a := range []struct {
		name bind.Named
		to   *int
	}{{nameStride, &o.Stride}, {nameDilation, &o.Dilation}, {nameGroups, &o.Groups}} {
		if v, given := bind.GetNamed(named, a.name); given {
			if *a.to, err = positiveInt(fn, a.name, v); err != nil {
				return o, err
			}
		}
	}
	if v, given := bind.GetNamed(named, namePadding); given {
		n, ok := v.(*interp.IntVal)
		if !ok || n.Val < 0 {
			return o, messages.New(messages.H7085, fn, namePadding, interp.Shown(v))
		}
		o.Padding = int(min(n.Val, math.MaxInt/8)) // sınırı aşan dolgu bellek denetimine takılır
	}
	return o, nil
}

// convShapes şekilleri denetler: uymayan her şeyi ileti söyler (15.23c).
func convShapes(fn any, x, w *tensor.Tensor, o tensor.Conv1dOptions) error {
	xs, ws := x.Shape(), w.Shape()
	fail := func(c, fix string) error {
		return messages.New(messages.H7083, fn, tensor.ShapeText(xs), tensor.ShapeText(ws), c, o.Groups, fix)
	}
	if len(xs) != 3 || len(ws) != 3 || ws[2] == 0 {
		return fail("rank", rankFix(xs))
	}
	cin, cout, cg, k := xs[1], ws[0], ws[1], ws[2]
	if cin%o.Groups != 0 || cout%o.Groups != 0 {
		return fail("groups", "")
	}
	if cg*o.Groups != cin {
		return fail("channels", groupsFix(cin, cout, cg))
	}
	padded := xs[2] + 2*o.Padding
	if k > 1 && o.Dilation > (padded-1)/(k-1) || padded < k {
		return fail("other", paddingFix(xs[2], o, k))
	}
	return nil
}

// rankFix üç boyuta getiren yazımdır; yalnız bir ya da iki boyutlu girdide vardır.
func rankFix(xs []int) string {
	reshape := codes.Spelling(codes.MemReshape)
	switch len(xs) {
	case 1:
		return fmt.Sprintf(": `x.%s([1, 1, %d])`", reshape, xs[0])
	case 2:
		return fmt.Sprintf(": `x.%s([1, %d, %d])`", reshape, xs[0], xs[1])
	}
	return ""
}

// groupsFix kanalları tutan grup sayısının yazımıdır; varsa.
func groupsFix(cin, cout, cg int) string {
	if cg == 0 || cin%cg != 0 || cout%(cin/cg) != 0 {
		return ""
	}
	return fmt.Sprintf(": `%s: %d`", codes.Spelling(codes.ArgNumGroups), cin/cg)
}

// paddingFix çekirdeği sığdıran dolgunun yazımıdır; genişleme taşırıyorsa yoktur.
func paddingFix(l int, o tensor.Conv1dOptions, k int) string {
	if k > 1 && o.Dilation > math.MaxInt/8/(k-1) {
		return ""
	}
	return fmt.Sprintf(": `%s: %d`", codes.Spelling(codes.ArgPadding), (o.Span(k)-l+1)/2)
}

// convBias sapmayı denetler; TR_none ise nil'dir.
func convBias(fn any, v interp.Value, cout int) (*tensor.Tensor, error) {
	if v == interp.Null {
		return nil, nil
	}
	b, err := convTensor(fn, v)
	if err != nil {
		return nil, err
	}
	if s := b.Shape(); len(s) != 1 || s[0] != cout {
		return nil, messages.New(messages.H7084, fn, tensor.ShapeText(s), cout, codes.Spelling(codes.FnZeros)+fmt.Sprintf("([%d])", cout))
	}
	return b, nil
}

// conv1dBuiltin TR_conv1d(x, w, b, TR_stride: s, TR_padding: p, TR_dilation: d, TR_num_groups: g).
func conv1dBuiltin(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	kd := codes.FnConv1d
	fn := codes.NameRef(kd)
	if err := bind.CheckNamed(fn.Called(), named, nameStride, namePadding, nameDilation, nameGroups); err != nil {
		return nil, err
	}
	if len(args) != 3 {
		return nil, interp.ArgCountError(kd, 3, len(args))
	}
	x, err := convTensor(fn, args[0])
	if err != nil {
		return nil, err
	}
	w, err := convTensor(fn, args[1])
	if err != nil {
		return nil, err
	}
	o, err := convOptions(fn, named)
	if err != nil {
		return nil, err
	}
	if err = convShapes(fn, x, w, o); err != nil {
		return nil, err
	}
	b, err := convBias(fn, args[2], w.Shape()[0])
	if err != nil {
		return nil, err
	}
	mixed := mixedDType(m)
	r, err := safe(func() *tensor.Tensor {
		tensor.CheckSize([]int{x.Shape()[0], o.Length(x.Shape()[2], w.Shape()[2]), x.Shape()[1] * w.Shape()[2]}, tensor.Float32)
		return tensor.Conv1d(x, w, b, o, func(a, c *tensor.Tensor) *tensor.Tensor {
			if mixed != tensor.Float32 {
				return matMulOn(m, a.CastShared(mixed), c.CastShared(mixed), mixed)
			}
			return matMulOn(m, a, c, tensor.MatMulType(a, c))
		})
	})
	if err != nil {
		return nil, err
	}
	return New(r), nil
}
