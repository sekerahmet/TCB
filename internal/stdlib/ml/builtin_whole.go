package ml

import (
	"slices"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// ----------------------------------------------------------------------------
// Bütün tensör hakkında soru (15.18a) ve tek öğe (13.8a)
// ----------------------------------------------------------------------------
//
// TR_all ve TR_equal TR_Bool verir; GPU'da sonucu konağa okur (15.6'nın bekleyen işlemleri).
// TR_all'ın eksenli biçimi ve TR_any henüz yok.

// registerWhole hepsi ve aynı yerleşiklerini bağlar.
func registerWhole(v *interp.VM) {
	wrap(v, codes.FnAll, newBuiltin(allBuiltin))
	wrap(v, codes.FnEqual, newBuiltin(equalBuiltin))
}

// allBuiltin TR_all(m): mantık tensörünün bütün öğeleri doğruysa doğru (boşta doğru).
func allBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(codes.FnAll)
	if err := checkWholeArgs(codes.FnAll, args, named, 1); err != nil {
		return nil, err
	}
	m, isTensor := args[0].(*TensorValue)
	if !isTensor {
		return nil, messages.New(messages.H6979, fn, "bool", typeName(args[0]))
	}
	if m.t.Elem() != tensor.ElemBool {
		return nil, messages.New(messages.H6979, fn, "bool", codes.NameRef(kindOf(m.t.DType())))
	}
	return safe(func() interp.Value { return interp.BoolToVal(allTrue(m.t)) })
}

// allTrue mantık tensöründe sıfır öğe yoksa doğrudur (öğeler konağa okunur).
func allTrue(t *tensor.Tensor) bool {
	for _, x := range t.Int64s() {
		if x == 0 {
			return false
		}
	}
	return true
}

// equalBuiltin TR_equal(a, b): şekil ve bütün öğeler aynıysa doğru. Cinsler aynı olmalıdır;
// hassasiyet farkı karşılaştırmaya engel değildir. NaN hiçbir şeye eşit değildir, −0.0 0.0'a
// eşittir (4.5). Karşılaştırma CPU'da yapılır: sonuç zaten konağa okunur.
func equalBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(codes.FnEqual)
	if err := checkWholeArgs(codes.FnEqual, args, named, 2); err != nil {
		return nil, err
	}
	a, aOK := args[0].(*TensorValue)
	b, bOK := args[1].(*TensorValue)
	switch {
	case !aOK:
		return nil, messages.New(messages.H6979, fn, "tensor", typeName(args[0]))
	case !bOK:
		return nil, messages.New(messages.H6979, fn, "tensor", typeName(args[1]))
	case a.t.Elem() != b.t.Elem():
		return nil, kindMismatch(fn, a.t.DType(), b.t.DType())
	}
	if !slices.Equal(a.t.Shape(), b.t.Shape()) {
		return interp.False, nil
	}
	return safe(func() interp.Value {
		x, y := a.t.To(tensor.CPU), b.t.To(tensor.CPU)
		eq := tensor.CompareTyped(x, y, tensor.CmpEq)
		r := allTrue(eq)
		eq.Drop()
		dropTemp(a.t, x)
		dropTemp(b.t, y)
		return interp.BoolToVal(r)
	})
}

// checkWholeArgs argüman sayısını denetler; adlı argüman yoktur.
func checkWholeArgs(kd codes.Code, args []interp.Value, named map[string]interp.Value, n int) error {
	if len(named) > 0 {
		return bind.CheckNamed(codes.NameRef(kd).Called(), named)
	}
	if len(args) != n {
		return interp.ArgCountError(kd, n, len(args))
	}
	return nil
}

// item .TR_item(): tek öğeli tensörün öğesi; öğe tipi tensörün cinsidir (13.8a, 15.3).
func (d *TensorValue) item(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
	if len(args) != 0 {
		return nil, interp.ArgCountError(memberItem, 0, len(args))
	}
	if d.t.Numel() != 1 {
		return nil, messages.New(messages.H3068, d.t.Shape())
	}
	return d.scalarValue(), nil
}
