package interp

import (
	"errors"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// --- ATAMA ---

// assignTarget bir atama hedefini derler; dönen işlev sağ taraf değerini hedefe yazar
// (bileşik atamada önce mevcut değerle işlem yapar).
func (c *compiler) assignTarget(k *compilerScope, target ast.Expression, codeVal uint8, line int) func(v *VM, env *Environment, rhs Value) error {
	compound := codeVal == opAdd || codeVal == opSub || codeVal == opMul || codeVal == opDiv
	switch t := target.(type) {
	case *ast.Identifier:
		r := k.resolve(t.Value)
		if r.kind == resolveDynamic || r.kind == resolveNotYet {
			return func(v *VM, env *Environment, rhs Value) error {
				return v.assignByName(env, t, codeVal, rhs, line)
			}
		}
		i := r.slot
		fast := intFastFunc(codeVal)
		opText := assignOpText(codeVal)
		return func(v *VM, env *Environment, rhs Value) error {
			o := slotEnv(r, env)
			if i < len(o.vals) {
				if cur := o.vals[i]; cur != nil && !o.isConstant(i) {
					if !compound {
						o.vals[i] = rhs
						return nil
					}
					if li, ok := cur.(*IntVal); ok {
						if ri, ok := rhs.(*IntVal); ok {
							if x, ok := fast(li.Val, ri.Val); ok {
								o.vals[i] = x
								return nil
							}
						}
					}
					fresh, err := v.evalInfixKod(codeVal, opText, cur, rhs, line)
					if err != nil {
						return err
					}
					o.vals[i] = fresh
					return nil
				}
			}
			return v.assignByName(env, t, codeVal, rhs, line)
		}

	case *ast.IndexExpression:
		// A[i][j] = değer, A[i] okumasının sonucuna [j] ile atamadır (tensör satırı veriyi
		// paylaşan bir görünümdür; bkz. IndeksAtanabilir).
		leftFn := c.expression(k, t.Left)
		index := c.expression(k, t.Index)
		return func(v *VM, env *Environment, rhs Value) error {
			left, err := leftFn(v, env)
			if err != nil {
				return err
			}
			idx, err := index(v, env)
			if err != nil {
				return err
			}
			return v.setIndex(left, idx, codeVal, rhs, line)
		}

	case *ast.MemberExpression:
		object := c.expression(k, t.Object)
		prop := t.Property
		u := resolveMember(prop)
		return func(v *VM, env *Environment, rhs Value) error {
			obj, err := object(v, env)
			if err != nil {
				return err
			}
			return v.setMember(obj, prop, u, codeVal, rhs, line)
		}
	}
	return func(v *VM, env *Environment, rhs Value) error {
		return messages.At(line, 0, messages.New(messages.H2436))
	}
}

// assignByName bir ada (yuvası bilinmeden) atama yapar.
func (v *VM) assignByName(env *Environment, t *ast.Identifier, codeVal uint8, rightVal Value, line int) error {
	var finalVal Value = rightVal
	if codeVal != opAssign {
		cur, ok := env.Get(t.Value)
		if !ok {
			return messages.At(line, t.Token.Column, messages.New(messages.H2437, t.Value, suggestName(t.Value, env)))
		}
		var err error
		finalVal, err = v.assignOp(codeVal, cur, rightVal, line)
		if err != nil {
			return err
		}
	}
	if err := env.Assign(t.Value, finalVal); err != nil {
		var undefined *UndefinedVarError
		if errors.As(err, &undefined) {
			return messages.At(line, t.Token.Column, messages.New(messages.H2438, err, suggestName(t.Value, env)))
		}
		return messages.At(line, 0, err)
	}
	return nil
}
