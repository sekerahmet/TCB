package interp

import (
	"math"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// registerMath taban, tavan, işaret ve sonlu_mu yerleşiklerinin sayı yolunu ve pi sabitini
// kaydeder. Tensör yolu internal/stdlib/ml'dadır; o, bu yerleşikleri sarar (karekök
// gibi). Tam girdide sonuç Tam, Ondalık girdide Ondalık'tır.
func (v *VM) registerMath() {
	v.RegisterNoNamed(codes.G406, roundingBuiltin(codes.G406, math.Floor))
	v.RegisterNoNamed(codes.G407, roundingBuiltin(codes.G407, math.Ceil))
	v.RegisterNoNamed(codes.G408, signBuiltin)
	v.RegisterNoNamed(codes.G415, isFiniteBuiltin)
	v.DefineConstant(codes.G414, &FloatVal{Val: math.Pi})
}

// roundingBuiltin taban ve tavan: Tam olduğu gibi, Ondalık f ile yuvarlanmış Ondalık döner
// (NaN ve sonsuzlar korunur).
func roundingBuiltin(kd codes.Code, f func(float64) float64) BuiltinFunc {
	return func(y *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgCountError(kd, 1, len(args), messages.Text(messages.H2305))
		}
		switch a := args[0].(type) {
		case *IntVal:
			return a, nil
		case *FloatVal:
			return &FloatVal{Val: f(a.Val)}, nil
		}
		return nil, ArgTypeError(kd, 1, args[0], numberTypes...)
	}
}

// signBuiltin işaret(): pozitifte 1, negatifte -1, sıfırda 0; Tam girdide Tam, Ondalık
// girdide Ondalık (NaN ve işaretli sıfır korunur).
func signBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.G408, 1, len(args), messages.Text(messages.H2305))
	}
	switch a := args[0].(type) {
	case *IntVal:
		switch {
		case a.Val > 0:
			return NewInt(1), nil
		case a.Val < 0:
			return NewInt(-1), nil
		}
		return NewInt(0), nil
	case *FloatVal:
		switch {
		case a.Val > 0:
			return &FloatVal{Val: 1}, nil
		case a.Val < 0:
			return &FloatVal{Val: -1}, nil
		}
		return a, nil
	}
	return nil, ArgTypeError(codes.G408, 1, args[0], numberTypes...)
}

// isFiniteBuiltin sonlu_mu(): Ondalık NaN ve ±sonsuzda yanlış, öbür sayılarda (her Tam) doğru.
func isFiniteBuiltin(y *VM, args []Value, named map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, ArgCountError(codes.G415, 1, len(args), messages.Text(messages.H2305))
	}
	switch a := args[0].(type) {
	case *IntVal:
		return True, nil
	case *FloatVal:
		return BoolToVal(!math.IsNaN(a.Val) && !math.IsInf(a.Val, 0)), nil
	}
	return nil, ArgTypeError(codes.G415, 1, args[0], numberTypes...)
}
