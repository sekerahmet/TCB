package vm

import (
	"fmt"
	"math"
)

func (v *VM) evalInfix(op string, left Value, right Value, line int) (Value, error) {
	// Matris / Tensör operatörleri
	if mop, ok := left.(MatrixOperator); ok {
		switch op {
		case "+":
			return mop.Add(right)
		case "-":
			return mop.Sub(right)
		case "*":
			return mop.Mul(right)
		case "/":
			return mop.Div(right)
		case "@":
			return mop.MatMul(right)
		}
	}
	if mop, ok := right.(MatrixOperator); ok {
		switch op {
		case "+":
			return mop.Add(left)
		case "*":
			return mop.Mul(left)
		}
	}

	switch op {
	case "==":
		if ValuesEqual(left, right) {
			return TRUE, nil
		}
		return FALSE, nil
	case "!=":
		if !ValuesEqual(left, right) {
			return TRUE, nil
		}
		return FALSE, nil
	}

	// Tam sayı işlemleri
	if li, ok1 := left.(*IntVal); ok1 {
		if ri, ok2 := right.(*IntVal); ok2 {
			switch op {
			case "+":
				sonuc := li.Val + ri.Val
				if (sonuc > li.Val) != (ri.Val > 0) {
					return nil, tasmaHatasi(line, li.Val, op, ri.Val)
				}
				return NewInt(sonuc), nil
			case "-":
				sonuc := li.Val - ri.Val
				if (sonuc < li.Val) != (ri.Val > 0) {
					return nil, tasmaHatasi(line, li.Val, op, ri.Val)
				}
				return NewInt(sonuc), nil
			case "*":
				if !mulFits(li.Val, ri.Val) {
					return nil, tasmaHatasi(line, li.Val, op, ri.Val)
				}
				return NewInt(li.Val * ri.Val), nil
			case "/":
				if ri.Val == 0 {
					return nil, fmt.Errorf("Satır %d: Sıfıra bölme hatası", line)
				}
				if li.Val%ri.Val == 0 {
					return NewInt(li.Val / ri.Val), nil
				}
				return &FloatVal{Val: float64(li.Val) / float64(ri.Val)}, nil
			case "%":
				if ri.Val == 0 {
					return nil, fmt.Errorf("Satır %d: Sıfıra göre mod alma hatası", line)
				}
				return NewInt(li.Val % ri.Val), nil
			case "**":
				if ri.Val < 0 {
					return &FloatVal{Val: math.Pow(float64(li.Val), float64(ri.Val))}, nil
				}
				res, ok := intPow(li.Val, ri.Val)
				if !ok {
					return nil, fmt.Errorf("Satır %d: Tam sayı taşması (%d ** %d); ondalık sonuç için örn. %d.0 ** %d yazın", line, li.Val, ri.Val, li.Val, ri.Val)
				}
				return NewInt(res), nil
			case "<":
				return BoolToVal(li.Val < ri.Val), nil
			case ">":
				return BoolToVal(li.Val > ri.Val), nil
			case "<=":
				return BoolToVal(li.Val <= ri.Val), nil
			case ">=":
				return BoolToVal(li.Val >= ri.Val), nil
			}
		}
	}

	// Ondalık (veya Tam + Ondalık) sayı işlemleri
	lf, ok1 := ToFloat(left)
	rf, ok2 := ToFloat(right)
	if ok1 && ok2 {
		switch op {
		case "+":
			return &FloatVal{Val: lf + rf}, nil
		case "-":
			return &FloatVal{Val: lf - rf}, nil
		case "*":
			return &FloatVal{Val: lf * rf}, nil
		case "/":
			if rf == 0 {
				return nil, fmt.Errorf("Satır %d: Sıfıra bölme hatası", line)
			}
			return &FloatVal{Val: lf / rf}, nil
		case "%":
			if rf == 0 {
				return nil, fmt.Errorf("Satır %d: Sıfıra göre mod alma hatası", line)
			}
			return &FloatVal{Val: math.Mod(lf, rf)}, nil
		case "**":
			return &FloatVal{Val: math.Pow(lf, rf)}, nil
		case "<":
			return BoolToVal(lf < rf), nil
		case ">":
			return BoolToVal(lf > rf), nil
		case "<=":
			return BoolToVal(lf <= rf), nil
		case ">=":
			return BoolToVal(lf >= rf), nil
		}
	}

	// Metin birleştirme
	if ls, ok := left.(*StringVal); ok && op == "+" {
		return &StringVal{Val: ls.Val + right.Inspect()}, nil
	}
	if rs, ok := right.(*StringVal); ok && op == "+" {
		return &StringVal{Val: left.Inspect() + rs.Val}, nil
	}

	// Liste birleştirme
	if ll, ok1 := left.(*ListVal); ok1 {
		if rl, ok2 := right.(*ListVal); ok2 && op == "+" {
			combined := make([]Value, 0, len(ll.Elements)+len(rl.Elements))
			combined = append(combined, ll.Elements...)
			combined = append(combined, rl.Elements...)
			return &ListVal{Elements: combined}, nil
		}
	}

	return nil, fmt.Errorf("Satır %d: %s ve %s tipleri arasında %q işlemi desteklenmiyor", line, left.Type(), right.Type(), op)
}

// tasmaHatasi int64 sınırını aşan tam sayı işlemleri için hata üretir (sessizce sarmak yerine).
func tasmaHatasi(line int, a int64, op string, b int64) error {
	return fmt.Errorf("Satır %d: Tam sayı taşması (%d %s %d); ondalık sonuç için örn. %d.0 %s %d yazın", line, a, op, b, a, op, b)
}

func ToFloat(v Value) (float64, bool) {
	switch n := v.(type) {
	case *IntVal:
		return float64(n.Val), true
	case *FloatVal:
		return n.Val, true
	default:
		return 0, false
	}
}

func BoolToVal(b bool) Value {
	if b {
		return TRUE
	}
	return FALSE
}

func ValuesEqual(a, b Value) bool {
	if a.Type() == VAL_NULL && b.Type() == VAL_NULL {
		return true
	}
	if af, ok1 := ToFloat(a); ok1 {
		if bf, ok2 := ToFloat(b); ok2 {
			return af == bf
		}
	}
	if a.Type() != b.Type() {
		return false
	}
	return a.Inspect() == b.Inspect()
}

// intPow tam sayı üssünü kare-çarp yöntemiyle hesaplar; int64 taşmasında ok=false döner.
func intPow(base, exp int64) (int64, bool) {
	result := int64(1)
	for exp > 0 {
		if exp&1 == 1 {
			if !mulFits(result, base) {
				return 0, false
			}
			result *= base
		}
		exp >>= 1
		if exp > 0 {
			if !mulFits(base, base) {
				return 0, false
			}
			base *= base
		}
	}
	return result, true
}

func mulFits(a, b int64) bool {
	if a == 0 || b == 0 {
		return true
	}
	c := a * b
	return c/b == a && !(a == -1 && b == math.MinInt64) && !(b == -1 && a == math.MinInt64)
}
