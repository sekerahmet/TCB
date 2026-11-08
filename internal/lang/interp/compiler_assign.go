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
	if id, ok := target.(*ast.Identifier); ok && id.Value == "_" && codeVal == opAssign {
		// '_' bir ad değildir: değer atılır (5.3)
		return func(*VM, *Environment, Value) error { return nil }
	}
	return c.assignTo(k, target, codeVal, line)
}

// assignTo bir ada, indekse ya da alana atamayı derler (assignTarget).
func (c *compiler) assignTo(k *compilerScope, target ast.Expression, codeVal uint8, line int) func(v *VM, env *Environment, rhs Value) error {
	compound := codeVal != opAssign
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
				cur := o.vals[i]
				if compound && cur != nil {
					// Yerinde atama bağı değiştirmez: sabitte de geçerli (5.4, assignInPlace).
					if done, err := v.assignInPlace(codeVal, cur, rhs, line); done {
						return err
					}
				}
				if cur != nil && !o.isConstant(i) {
					if !compound {
						o.store(i, rhs)
						return nil
					}
					if li, ok := cur.(*IntVal); ok && fast != nil {
						if ri, ok := rhs.(*IntVal); ok {
							if x, ok := fast(li.Val, ri.Val); ok {
								o.vals[i] = x
								return nil
							}
						}
					}
					fresh, err := v.infix(codeVal, opText, cur, rhs, line)
					if err != nil {
						return err
					}
					o.store(i, fresh)
					return nil
				}
			}
			return v.assignByName(env, t, codeVal, rhs, line)
		}

	case *ast.IndexExpression, *ast.MemberExpression:
		place, write := c.assignPlace(k, target, codeVal, line)
		return func(v *VM, env *Environment, rhs Value) error {
			owner, key, err := place(v, env)
			if err != nil {
				return err
			}
			return write(v, owner, key, rhs)
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
			return messages.At(line, t.Token.Column, messages.New(messages.H2437, t.Value, v.suggestName(t.Value, env)))
		}
		if _, done, err := v.listInPlace(codeVal, cur, rightVal, line); done {
			return err // yerinde: bağ değişmez (sabit listede de)
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
			return messages.At(line, t.Token.Column, messages.New(messages.H2438, err, v.suggestName(t.Value, env)))
		}
		return messages.At(line, 0, err)
	}
	return nil
}

// placeFunc atama hedefinin yerini hesaplar: indekste kap ve indeks, alanda alanın sahibi (key nil).
type placeFunc func(v *VM, env *Environment) (owner, key Value, err error)

// writeFunc yeri hesaplanmış hedefe sağ yanın değerini yazar (bileşik atamada önce işlem yapar).
type writeFunc func(v *VM, owner, key, rhs Value) error

// assignPlace indeks ve alan hedefinin yerini ve yazmasını ayrı derler: bileşik atamada yer sağ
// yandan önce, bir kez hesaplanır (5.4). A[i][j] = değer, A[i] okumasının sonucuna [j] ile atamadır
// (tensör satırı veriyi paylaşan bir görünümdür; bkz. IndeksAtanabilir). Modül değerine köşeli
// ayraçla atanmaz (11.2). Ad hedefinin yeri yoktur: place nil.
func (c *compiler) assignPlace(k *compilerScope, target ast.Expression, codeVal uint8, line int) (placeFunc, writeFunc) {
	switch t := target.(type) {
	case *ast.IndexExpression:
		leftFn := c.expression(k, t.Left)
		index := c.expression(k, t.Index)
		place := func(v *VM, env *Environment) (Value, Value, error) {
			left, err := leftFn(v, env)
			if err != nil {
				return nil, nil, err
			}
			idx, err := index(v, env)
			return left, idx, err
		}
		return place, func(v *VM, left, idx, rhs Value) error {
			if d, ok := left.(*DictVal); ok {
				if err := moduleIndexError(d, t.Left, idx); err != nil {
					return messages.At(line, 0, err)
				}
			}
			return v.setIndex(left, idx, codeVal, rhs, line)
		}
	case *ast.MemberExpression:
		object := c.expression(k, t.Object)
		prop := t.Property
		u := c.member(prop)
		place := func(v *VM, env *Environment) (Value, Value, error) {
			obj, err := object(v, env)
			return obj, nil, err
		}
		return place, func(v *VM, obj, _, rhs Value) error {
			return v.setMember(obj, prop, u, codeVal, rhs, line)
		}
	}
	return nil, nil
}
