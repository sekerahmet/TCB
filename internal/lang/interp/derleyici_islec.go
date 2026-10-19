package interp

import (
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"math"
)

// --- İŞLEÇLER ---

func (c *derleyici) onEk(k *derleyiciKapsam, e *ast.PrefixExpression) ifadeKodu {
	sag := c.ifade(k, e.Right)
	line := e.Line()
	switch e.Operator {
	case "!":
		return func(v *VM, env *Environment) (Value, error) {
			right, err := sag(v, env)
			if err != nil {
				return nil, err
			}
			if right.IsTruthy() {
				return FALSE, nil
			}
			return TRUE, nil
		}
	case "-":
		return func(v *VM, env *Environment) (Value, error) {
			right, err := sag(v, env)
			if err != nil {
				return nil, err
			}
			switch rv := right.(type) {
			case *IntVal:
				if rv.Val == math.MinInt64 {
					return nil, fmt.Errorf("Satır %d: %s sayı taşması (-(%d)); ondalık sonuç için örn. -(%d.0) yazın", line, VAL_INT, rv.Val, rv.Val)
				}
				return NewInt(-rv.Val), nil
			case *FloatVal:
				return &FloatVal{Val: -rv.Val}, nil
			case Olumsuzlanabilir:
				val, err := rv.Olumsuz()
				return val, satirEki(line, err)
			}
			return nil, fmt.Errorf("Satır %d: '-' operatörü %s tipine uygulanamaz", line, right.Type())
		}
	}
	return func(v *VM, env *Environment) (Value, error) {
		if _, err := sag(v, env); err != nil {
			return nil, err
		}
		return NULL, nil
	}
}

// tamHizli iki Tam sayı arasındaki işlemin hızlı yoludur; ok=false ise (taşma, sıfıra
// bölme, desteklenmeyen işleç) genel yola (evalInfixKod) düşülür ve hata orada üretilir.
type tamHizli func(a, b int64) (Value, bool)

func tamHizliIslev(kod uint8) tamHizli {
	switch kod {
	case opTopla:
		return func(a, b int64) (Value, bool) {
			s := a + b
			if (s > a) != (b > 0) {
				return nil, false
			}
			return NewInt(s), true
		}
	case opCikar:
		return func(a, b int64) (Value, bool) {
			s := a - b
			if (s < a) != (b > 0) {
				return nil, false
			}
			return NewInt(s), true
		}
	case opCarp:
		return func(a, b int64) (Value, bool) {
			if !mulFits(a, b) {
				return nil, false
			}
			return NewInt(a * b), true
		}
	case opMod:
		return func(a, b int64) (Value, bool) {
			if b == 0 {
				return nil, false
			}
			return NewInt(a % b), true
		}
	case opBol:
		return func(a, b int64) (Value, bool) {
			if b == 0 || (a == math.MinInt64 && b == -1) {
				return nil, false
			}
			if a%b == 0 {
				return NewInt(a / b), true
			}
			return &FloatVal{Val: float64(a) / float64(b)}, true
		}
	case opKucuk:
		return func(a, b int64) (Value, bool) { return BoolToVal(a < b), true }
	case opBuyuk:
		return func(a, b int64) (Value, bool) { return BoolToVal(a > b), true }
	case opKucukEsit:
		return func(a, b int64) (Value, bool) { return BoolToVal(a <= b), true }
	case opBuyukEsit:
		return func(a, b int64) (Value, bool) { return BoolToVal(a >= b), true }
	case opEsit:
		return func(a, b int64) (Value, bool) { return BoolToVal(a == b), true }
	case opEsitDegil:
		return func(a, b int64) (Value, bool) { return BoolToVal(a != b), true }
	}
	return nil
}
func (c *derleyici) ikili(k *derleyiciKapsam, e *ast.InfixExpression) ifadeKodu {
	kod := islecKodu(e.Operator)
	sol := c.ifade(k, e.Left)
	sag := c.ifade(k, e.Right)
	op := e.Operator
	line := e.Line()

	// Kısa devre ve özel operatörler (&&, ||, ??, |>)
	switch kod {
	case opVe:
		return func(v *VM, env *Environment) (Value, error) {
			left, err := sol(v, env)
			if err != nil {
				return nil, err
			}
			if !dogruMu(left) {
				return FALSE, nil
			}
			right, err := sag(v, env)
			if err != nil {
				return nil, err
			}
			return BoolToVal(dogruMu(right)), nil
		}
	case opVeya:
		return func(v *VM, env *Environment) (Value, error) {
			left, err := sol(v, env)
			if err != nil {
				return nil, err
			}
			if dogruMu(left) {
				return TRUE, nil
			}
			right, err := sag(v, env)
			if err != nil {
				return nil, err
			}
			return BoolToVal(dogruMu(right)), nil
		}
	case opYoksa:
		return func(v *VM, env *Environment) (Value, error) {
			left, err := sol(v, env)
			if err != nil {
				return nil, err
			}
			if left.Type() != VAL_NULL {
				return left, nil
			}
			return sag(v, env)
		}
	case opBoru:
		return c.boru(k, e.Left, e.Right)
	}

	hizli := tamHizliIslev(kod)
	if hizli != nil {
		// Sağ taraf Tam sayı sabitiyse (i < 1000, x % 7) sağ kod hiç çağrılmaz
		if lit, ok := e.Right.(*ast.IntegerLiteral); ok {
			b := lit.Value
			sagDeger := NewInt(b)
			// Sol taraf yerel (derinlik 0) ya da genel bir değişkense yuvası doğrudan okunur
			if id, ok := e.Left.(*ast.Identifier); ok {
				if r := k.coz(id.Value); r.tur == cozGenel || (r.tur == cozYerel && r.derinlik == 0) {
					i, g := r.yuva, r.genel
					return func(v *VM, env *Environment) (Value, error) {
						o := g
						if o == nil {
							o = env
						}
						var left Value
						if i < len(o.vals) {
							left = o.vals[i]
						}
						if li, ok := left.(*IntVal); ok {
							if x, ok := hizli(li.Val, b); ok {
								return x, nil
							}
						}
						if left == nil {
							var err error
							if left, err = sol(v, env); err != nil {
								return nil, err
							}
						}
						return v.evalInfixKod(kod, op, left, sagDeger, line)
					}
				}
			}
			return func(v *VM, env *Environment) (Value, error) {
				left, err := sol(v, env)
				if err != nil {
					return nil, err
				}
				if li, ok := left.(*IntVal); ok {
					if x, ok := hizli(li.Val, b); ok {
						return x, nil
					}
				}
				return v.evalInfixKod(kod, op, left, sagDeger, line)
			}
		}
		return func(v *VM, env *Environment) (Value, error) {
			left, err := sol(v, env)
			if err != nil {
				return nil, err
			}
			right, err := sag(v, env)
			if err != nil {
				return nil, err
			}
			if li, ok := left.(*IntVal); ok {
				if ri, ok := right.(*IntVal); ok {
					if x, ok := hizli(li.Val, ri.Val); ok {
						return x, nil
					}
				}
			}
			return v.evalInfixKod(kod, op, left, right, line)
		}
	}
	return func(v *VM, env *Environment) (Value, error) {
		left, err := sol(v, env)
		if err != nil {
			return nil, err
		}
		right, err := sag(v, env)
		if err != nil {
			return nil, err
		}
		return v.evalInfixKod(kod, op, left, right, line)
	}
}

// boru 'veri |> f(...)' ve 'veri |> f' ifadelerini derler.
func (c *derleyici) boru(k *derleyiciKapsam, left ast.Expression, right ast.Expression) ifadeKodu {
	sol := c.ifade(k, left)
	// Sağ taraf bir fonksiyon çağrısı ise (örn: veri |> filtrele((x) => x > 2))
	if callExpr, ok := right.(*ast.CallExpression); ok {
		fonk := c.ifade(k, callExpr.Function)
		argKod := c.ifadeler(k, callExpr.Arguments)
		adlar := make([]string, len(callExpr.NamedArgs))
		adliKod := make([]ifadeKodu, len(callExpr.NamedArgs))
		for i, na := range callExpr.NamedArgs {
			adlar[i] = na.Name
			adliKod[i] = c.ifade(k, na.Value)
		}
		line := callExpr.Line()
		return func(v *VM, env *Environment) (Value, error) {
			leftVal, err := sol(v, env)
			if err != nil {
				return nil, err
			}
			fnVal, err := fonk(v, env)
			if err != nil {
				return nil, err
			}
			args := make([]Value, 1, len(argKod)+1)
			args[0] = leftVal
			for _, a := range argKod {
				av, err := a(v, env)
				if err != nil {
					return nil, err
				}
				args = append(args, av)
			}
			named := make(map[string]Value)
			for i, na := range adliKod {
				nv, err := na(v, env)
				if err != nil {
					return nil, err
				}
				named[adlar[i]] = nv
			}
			if err := checkArity(fnVal, len(args), named, line); err != nil {
				return nil, err
			}
			return v.ApplyFunction(fnVal, args, named, line)
		}
	}

	// Sağ taraf doğrudan bir fonksiyon ise (örn: veri |> sırala)
	fonk := c.ifade(k, right)
	line := right.Line()
	return func(v *VM, env *Environment) (Value, error) {
		leftVal, err := sol(v, env)
		if err != nil {
			return nil, err
		}
		fnVal, err := fonk(v, env)
		if err != nil {
			return nil, err
		}
		if err := checkArity(fnVal, 1, nil, line); err != nil {
			return nil, err
		}
		return v.ApplyFunction(fnVal, []Value{leftVal}, nil, line)
	}
}
