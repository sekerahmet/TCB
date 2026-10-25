package interp

import (
	"math"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// --- İŞLEÇLER ---

func (c *compiler) prefixText(k *compilerScope, e *ast.PrefixExpression) exprCode {
	rightFn := c.expression(k, e.Right)
	line := e.Line()
	switch e.Operator {
	case "!":
		return func(v *VM, env *Environment) (Value, error) {
			right, err := rightFn(v, env)
			if err != nil {
				return nil, err
			}
			if right.IsTruthy() {
				return False, nil
			}
			return True, nil
		}
	case "-":
		return func(v *VM, env *Environment) (Value, error) {
			right, err := rightFn(v, env)
			if err != nil {
				return nil, err
			}
			switch rv := right.(type) {
			case *IntVal:
				if rv.Val == math.MinInt64 {
					return nil, messages.At(line, 0, messages.New(messages.H2444, ValInt, rv.Val))
				}
				return NewInt(-rv.Val), nil
			case *FloatVal:
				return &FloatVal{Val: -rv.Val}, nil
			case Negatable:
				newOp()
				val, err := rv.Negate()
				if err == nil {
					v.adopt(val)
					v.releaseTemp(right, val)
				}
				return val, lineSuffix(line, err)
			}
			return nil, messages.At(line, 0, messages.New(messages.H2445, right.Type()))
		}
	}
	return func(v *VM, env *Environment) (Value, error) {
		if _, err := rightFn(v, env); err != nil {
			return nil, err
		}
		return Null, nil
	}
}

// intFast iki Tam sayı arasındaki işlemin hızlı yoludur; ok=false ise (taşma, sıfıra
// bölme, desteklenmeyen işleç) genel yola (evalInfixKod) düşülür ve hata orada üretilir.
type intFast func(a, b int64) (Value, bool)

func intFastFunc(codeVal uint8) intFast {
	switch codeVal {
	case opAdd:
		return func(a, b int64) (Value, bool) {
			s := a + b
			if (s > a) != (b > 0) {
				return nil, false
			}
			return NewInt(s), true
		}
	case opSub:
		return func(a, b int64) (Value, bool) {
			s := a - b
			if (s < a) != (b > 0) {
				return nil, false
			}
			return NewInt(s), true
		}
	case opMul:
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
	case opDiv:
		return func(a, b int64) (Value, bool) {
			if b == 0 || (a == math.MinInt64 && b == -1) {
				return nil, false
			}
			if a%b == 0 {
				return NewInt(a / b), true
			}
			return &FloatVal{Val: float64(a) / float64(b)}, true
		}
	case opLt:
		return func(a, b int64) (Value, bool) { return BoolToVal(a < b), true }
	case opGt:
		return func(a, b int64) (Value, bool) { return BoolToVal(a > b), true }
	case opLe:
		return func(a, b int64) (Value, bool) { return BoolToVal(a <= b), true }
	case opGe:
		return func(a, b int64) (Value, bool) { return BoolToVal(a >= b), true }
	case opEq:
		return func(a, b int64) (Value, bool) { return BoolToVal(a == b), true }
	case opNe:
		return func(a, b int64) (Value, bool) { return BoolToVal(a != b), true }
	}
	return nil
}
func (c *compiler) binary(k *compilerScope, e *ast.InfixExpression) exprCode {
	codeVal := opCode(e.Operator)
	leftFn := c.expression(k, e.Left)
	rightFn := c.expression(k, e.Right)
	op := e.Operator
	line := e.Line()

	// Kısa devre ve özel operatörler (&&, ||, ??, |>)
	switch codeVal {
	case opAnd:
		return func(v *VM, env *Environment) (Value, error) {
			left, err := leftFn(v, env)
			if err != nil {
				return nil, err
			}
			if !isTrue(left) {
				return False, nil
			}
			right, err := rightFn(v, env)
			if err != nil {
				return nil, err
			}
			return BoolToVal(isTrue(right)), nil
		}
	case opOr:
		return func(v *VM, env *Environment) (Value, error) {
			left, err := leftFn(v, env)
			if err != nil {
				return nil, err
			}
			if isTrue(left) {
				return True, nil
			}
			right, err := rightFn(v, env)
			if err != nil {
				return nil, err
			}
			return BoolToVal(isTrue(right)), nil
		}
	case opCoalesce:
		return func(v *VM, env *Environment) (Value, error) {
			left, err := leftFn(v, env)
			if err != nil {
				return nil, err
			}
			if left.Type() != ValNull {
				return left, nil
			}
			return rightFn(v, env)
		}
	case opPipe:
		return c.pipe(k, e.Left, e.Right)
	}

	fast := intFastFunc(codeVal)
	if fast != nil {
		// Sağ taraf Tam sayı sabitiyse (i < 1000, x % 7) sağ kod hiç çağrılmaz
		if lit, ok := e.Right.(*ast.IntegerLiteral); ok {
			b := lit.Value
			rightValue := NewInt(b)
			// Sol taraf yerel (derinlik 0) ya da genel bir değişkense yuvası doğrudan okunur
			if id, ok := e.Left.(*ast.Identifier); ok {
				if r := k.resolve(id.Value); r.kind == resolveGlobal || (r.kind == resolveLocal && r.nestDepth == 0) {
					i, g := r.slot, r.global
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
							if x, ok := fast(li.Val, b); ok {
								return x, nil
							}
						}
						if left == nil {
							var err error
							if left, err = leftFn(v, env); err != nil {
								return nil, err
							}
						}
						return v.infix(codeVal, op, left, rightValue, line)
					}
				}
			}
			return func(v *VM, env *Environment) (Value, error) {
				left, err := leftFn(v, env)
				if err != nil {
					return nil, err
				}
				if li, ok := left.(*IntVal); ok {
					if x, ok := fast(li.Val, b); ok {
						return x, nil
					}
				}
				return v.infix(codeVal, op, left, rightValue, line)
			}
		}
		return func(v *VM, env *Environment) (Value, error) {
			left, err := leftFn(v, env)
			if err != nil {
				return nil, err
			}
			right, err := rightFn(v, env)
			if err != nil {
				return nil, err
			}
			if li, ok := left.(*IntVal); ok {
				if ri, ok := right.(*IntVal); ok {
					if x, ok := fast(li.Val, ri.Val); ok {
						return x, nil
					}
				}
			}
			return v.infix(codeVal, op, left, right, line)
		}
	}
	return func(v *VM, env *Environment) (Value, error) {
		left, err := leftFn(v, env)
		if err != nil {
			return nil, err
		}
		right, err := rightFn(v, env)
		if err != nil {
			return nil, err
		}
		return v.infix(codeVal, op, left, right, line)
	}
}

// pipe 'veri |> f(...)' ve 'veri |> f' ifadelerini derler.
func (c *compiler) pipe(k *compilerScope, left ast.Expression, right ast.Expression) exprCode {
	leftFn := c.expression(k, left)
	// Sağ taraf bir fonksiyon çağrısı ise (örn: veri |> filtrele((x) => x > 2))
	if callExpr, ok := right.(*ast.CallExpression); ok {
		fun := c.expression(k, callExpr.Function)
		argCode := c.expressions(k, callExpr.Arguments)
		namedCode := c.compileNamedArgs(k, callExpr.NamedArgs)
		line := callExpr.Line()
		return func(v *VM, env *Environment) (Value, error) {
			leftVal, err := leftFn(v, env)
			if err != nil {
				return nil, err
			}
			fnVal, err := fun(v, env)
			if err != nil {
				return nil, err
			}
			args := make([]Value, 1, len(argCode)+1)
			args[0] = leftVal
			for _, a := range argCode {
				av, err := a(v, env)
				if err != nil {
					return nil, err
				}
				args = append(args, av)
			}
			named, err := namedCode.eval(v, env)
			if err != nil {
				return nil, err
			}
			if err := checkArity(fnVal, nil, len(args), named, line); err != nil {
				return nil, err
			}
			return v.applyCompiled(fnVal, args, named, line)
		}
	}

	// Sağ taraf doğrudan bir fonksiyon ise (örn: veri |> sırala)
	fun := c.expression(k, right)
	line := right.Line()
	return func(v *VM, env *Environment) (Value, error) {
		leftVal, err := leftFn(v, env)
		if err != nil {
			return nil, err
		}
		fnVal, err := fun(v, env)
		if err != nil {
			return nil, err
		}
		if err := checkArity(fnVal, nil, 1, nil, line); err != nil {
			return nil, err
		}
		return v.applyCompiled(fnVal, []Value{leftVal}, nil, line)
	}
}
