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

// TR_assert_close (12.4a): iki ondalık sayıyı ya da iki TR_Float tensörü toleransla karşılaştırır;
// formül, varsayılan toleranslar, sonsuz ve NaN kuralı TR_all_close'unkidir (15.18a, nearFloats,
// closeBounds). Sayı 0 boyutlu bir tensör gibi okunur: sayıyla 0 boyutlu tensör karşılaştırılabilir,
// tolerans tensörün hassasiyetinden; iki sayıda "float64"ün satırı. Şekiller aynı olmalıdır, yayın
// yoktur. İleti en büyük farkı, yerini ve kullanılan toleransları söyler.

// closeOperand assert_close'un bir işleneni: düz değerleri, şekli ve hassasiyeti.
type closeOperand struct {
	vals  []float64
	shape []int
	dtype tensor.DType
}

// closeOperandOf değeri işlenene çevirir: TR_Float sayı ya da TR_Float tensör.
func closeOperandOf(fn any, order int, v interp.Value) (closeOperand, error) {
	switch x := v.(type) {
	case *interp.FloatVal:
		return closeOperand{vals: []float64{x.Val}, shape: []int{}, dtype: tensor.Float64}, nil
	case *TensorValue:
		switch {
		case x.t.DType().IsQuantized():
			return closeOperand{}, messages.New(messages.H6687, x.t.DType().Name())
		case x.t.Elem() == tensor.ElemFloat:
			vals, err := safe(x.t.Float64s)
			return closeOperand{vals: vals, shape: x.t.Shape(), dtype: x.t.DType()}, err
		}
	}
	return closeOperand{}, messages.New(messages.H2555, fn, order, typeName(v))
}

// assertCloseBuiltin TR_assert_close(beklenen, gerçek, [mesaj], TR_rtol:, TR_atol:).
func assertCloseBuiltin(y *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(codes.FnAssertClose)
	if err := bind.CheckNamed(fn.Called(), named, nameRtol, nameAtol); err != nil {
		return nil, err
	}
	if len(args) < 2 || len(args) > 3 {
		return nil, interp.ArgCountError(codes.FnAssertClose, interp.Counts(2, 3), len(args))
	}
	var ops [2]closeOperand
	for i := range ops {
		op, err := closeOperandOf(fn, i+1, args[i])
		if err != nil {
			return nil, err
		}
		ops[i] = op
	}
	rtol, atol, err := closeBounds(fn, ops[0].dtype, ops[1].dtype, named)
	if err != nil {
		return nil, err
	}
	message := ""
	if len(args) == 3 {
		message = y.TextOf(args[2]) // kullanıcının değeri, metni dilden bağımsız (3.14)
	}
	if !slices.Equal(ops[0].shape, ops[1].shape) {
		return nil, messages.New(messages.H2554, message, shapeText(ops[0].shape), shapeText(ops[1].shape))
	}
	worst, diff := -1, 0.0
	for i := range ops[0].vals {
		x, g := ops[0].vals[i], ops[1].vals[i]
		if nearFloats(x, g, rtol, atol) {
			continue
		}
		d := math.Abs(x - g)
		if math.IsNaN(d) {
			d = math.Inf(1) // NaN ve sonsuz fark en büyüğüdür
		}
		if worst < 0 || d > diff {
			worst, diff = i, d
		}
	}
	if worst < 0 {
		return interp.Null, nil
	}
	x, g := ops[0].vals[worst], ops[1].vals[worst]
	return nil, messages.New(messages.H2553, message, shownFloat(math.Abs(x-g)), placeText(ops[0].shape, worst),
		shownFloat(x), shownFloat(g), shownFloat(rtol), shownFloat(atol))
}

// shownFloat bir sayının iletideki değeridir (3.2'nin yazımıyla).
func shownFloat(x float64) any { return interp.Shown(&interp.FloatVal{Val: x}) }

// intList tam sayıların liste değeridir.
func intList(ns []int) *interp.ListVal {
	l := make([]interp.Value, len(ns))
	for i, n := range ns {
		l[i] = &interp.IntVal{Val: int64(n)}
	}
	return &interp.ListVal{Elements: l}
}

// shapeText şeklin liste yazımıdır ([2, 3]; sayıda []).
func shapeText(shape []int) any { return interp.Shown(intList(shape)) }

// placeText düz sıradaki öğenin çok boyutlu indeksidir ([1, 0]); sayıda ve 0 boyutlu tensörde boş.
func placeText(shape []int, flat int) string {
	if len(shape) == 0 {
		return ""
	}
	idx := make([]int, len(shape))
	for i := len(shape) - 1; i >= 0; i-- {
		idx[i] = flat % shape[i]
		flat /= shape[i]
	}
	return intList(idx).Inspect()
}
