package interp

import (
	"fmt"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/hint"
)

// sabitKod her zaman aynı (değişmez) değeri veren kod üretir; sabitler bir kez kutulanır.
func constCode(d Value) exprCode {
	return func(v *VM, env *Environment) (Value, error) { return d, nil }
}

var nullCode = constCode(NULL)

func (c *compiler) expressions(k *compilerScope, exprs []ast.Expression) []exprCode {
	codeList := make([]exprCode, len(exprs))
	for i, e := range exprs {
		codeList[i] = c.expression(k, e)
	}
	return codeList
}

// ifadeSinyal deyim konumundaki bir ifadeyi derler. 'eğer' ve 'eşle' dallarında çalışan
// 'sonuç', 'kır', 'devam' sinyalleri yukarı iletilir; diğer ifadeler sinyal üretmez.
func (c *compiler) exprSignal(k *compilerScope, expr ast.Expression) statementCode {
	switch e := expr.(type) {
	case *ast.IfExpression:
		return c.ifExpr(k, e)
	case *ast.MatchExpression:
		return c.matchExpr(k, e)
	}
	codeVal := c.expression(k, expr)
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		val, err := codeVal(v, env)
		if err != nil {
			return nil, signalNone, err
		}
		return val, signalNone, nil
	}
}

// sinyalsiz deyim konumu dışında kullanılan 'eğer'/'eşle' için sinyali düşürür.
func noSignal(d statementCode) exprCode {
	return func(v *VM, env *Environment) (Value, error) {
		val, _, err := d(v, env)
		return val, err
	}
}

func (c *compiler) expression(k *compilerScope, expr ast.Expression) exprCode {
	switch e := expr.(type) {
	case nil:
		return nullCode
	case *ast.IntegerLiteral:
		return constCode(NewInt(e.Value))
	case *ast.FloatLiteral:
		return constCode(&FloatVal{Val: e.Value})
	case *ast.StringLiteral:
		return constCode(&StringVal{Val: e.Value})
	case *ast.BooleanLiteral:
		if e.Value {
			return constCode(TRUE)
		}
		return constCode(FALSE)
	case *ast.NullLiteral:
		return nullCode

	case *ast.InterpolatedString:
		pieces := c.expressions(k, e.Parts)
		return func(v *VM, env *Environment) (Value, error) {
			var sb strings.Builder
			for _, p := range pieces {
				val, err := p(v, env)
				if err != nil {
					return nil, err
				}
				sb.WriteString(val.Inspect())
			}
			return &StringVal{Val: sb.String()}, nil
		}

	case *ast.Identifier:
		return c.readName(k, e)

	case *ast.ListLiteral:
		elements := c.expressions(k, e.Elements)
		return func(v *VM, env *Environment) (Value, error) {
			elems := make([]Value, 0, len(elements))
			for _, el := range elements {
				val, err := el(v, env)
				if err != nil {
					return nil, err
				}
				elems = append(elems, val)
			}
			return &ListVal{Elements: elems}, nil
		}

	case *ast.ListComprehension:
		return c.listComprehension(k, e)

	case *ast.DictLiteral:
		keyList := make([]exprCode, len(e.Pairs))
		values := make([]exprCode, len(e.Pairs))
		for i, p := range e.Pairs {
			keyList[i] = c.expression(k, p.Key)
			values[i] = c.expression(k, p.Value)
		}
		return func(v *VM, env *Environment) (Value, error) {
			dict := NewDictVal()
			for i := range keyList {
				kVal, err := keyList[i](v, env)
				if err != nil {
					return nil, err
				}
				vVal, err := values[i](v, env)
				if err != nil {
					return nil, err
				}
				dict.SetValue(kVal, vVal)
			}
			return dict, nil
		}

	case *ast.RangeExpression:
		begin := c.expression(k, e.Start)
		last := c.expression(k, e.End)
		line := e.Line()
		inclusive := e.Inclusive
		return func(v *VM, env *Environment) (Value, error) {
			startVal, err := begin(v, env)
			if err != nil {
				return nil, err
			}
			endVal, err := last(v, env)
			if err != nil {
				return nil, err
			}
			sInt, ok1 := startVal.(*IntVal)
			eInt, ok2 := endVal.(*IntVal)
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("Satır %d: %s ('..') uçları %s sayı olmalıdır", line, VAL_RANGE, VAL_INT)
			}
			return &RangeVal{Start: sInt.Val, End: eInt.Val, Inclusive: inclusive}, nil
		}

	case *ast.PrefixExpression:
		return c.prefixText(k, e)

	case *ast.InfixExpression:
		return c.binary(k, e)

	case *ast.IfExpression:
		return noSignal(c.ifExpr(k, e))

	case *ast.MatchExpression:
		return noSignal(c.matchExpr(k, e))

	case *ast.FunctionLiteral:
		fk := c.function(k, e.Parameters, e.Body, false)
		var definition func(*Environment, Value, bool)
		if e.Name != "" {
			definition = identifier(k, e.Name)
		}
		return func(v *VM, env *Environment) (Value, error) {
			fn := &FuncVal{
				Name:       e.Name,
				Parameters: e.Parameters,
				Body:       e.Body,
				Env:        env,
				codeVal:    fk,
			}
			if definition != nil {
				definition(env, fn, false)
			}
			return fn, nil
		}

	case *ast.CallExpression:
		codeVal := c.call(k, e)
		// Gölgelenmiş bir yerleşiğin çağrısı: hata mesajı gölgelemeyi söyler (shadow.go).
		if id, ok := e.Function.(*ast.Identifier); ok && builtinName(id.Value) {
			if g, ok := k.findShadow(id.Value); ok {
				return shadowedCall(codeVal, c.expression(k, id), id.Value, g, e.Line())
			}
		}
		return codeVal

	case *ast.IndexExpression:
		leftFn := c.expression(k, e.Left)
		index := c.expression(k, e.Index)
		line := e.Line()
		return func(v *VM, env *Environment) (Value, error) {
			left, err := leftFn(v, env)
			if err != nil {
				return nil, err
			}
			idx, err := index(v, env)
			if err != nil {
				return nil, err
			}
			return v.indexValue(left, idx, line)
		}

	case *ast.MemberExpression:
		object := c.expression(k, e.Object)
		u := resolveMember(e.Property) // ad → kod derleme anında bir kez çözülür
		return func(v *VM, env *Environment) (Value, error) {
			obj, err := object(v, env)
			if err != nil {
				return nil, err
			}
			return v.memberValue(obj, e, u)
		}
	}
	return nullCode
}

// --- DEĞİŞKEN OKUMA ---

// adOku bir adın değerini okuyan kodu üretir. Derleme anında çözülen adlar yuvadan okunur;
// yuva boşsa (ya da ad çözülemediyse) arama adla yapılır.
func (c *compiler) readName(k *compilerScope, e *ast.Identifier) exprCode {
	r := k.resolve(e.Value)
	i := r.slot
	switch r.kind {
	case resolveNotYet:
		return func(v *VM, env *Environment) (Value, error) {
			return nil, fmt.Errorf("Satır %d, Sütun %d: '%s' parametresi henüz tanımlanmadı; varsayılan değer yalnızca kendisinden önceki parametreleri görür", e.Line(), e.Token.Column, e.Value)
		}
	case resolveGlobal:
		g := r.global
		return func(v *VM, env *Environment) (Value, error) {
			if i < len(g.vals) {
				if x := g.vals[i]; x != nil {
					return x, nil
				}
			}
			return readByName(env, e)
		}
	case resolveLocal:
		switch r.nestDepth {
		case 0:
			return func(v *VM, env *Environment) (Value, error) {
				if i < len(env.vals) {
					if x := env.vals[i]; x != nil {
						return x, nil
					}
				}
				return readByName(env, e)
			}
		case 1:
			return func(v *VM, env *Environment) (Value, error) {
				o := env.outer
				if i < len(o.vals) {
					if x := o.vals[i]; x != nil {
						return x, nil
					}
				}
				return readByName(env, e)
			}
		default:
			d := r.nestDepth
			return func(v *VM, env *Environment) (Value, error) {
				o := env
				for j := 0; j < d; j++ {
					o = o.outer
				}
				if i < len(o.vals) {
					if x := o.vals[i]; x != nil {
						return x, nil
					}
				}
				return readByName(env, e)
			}
		}
	}
	return func(v *VM, env *Environment) (Value, error) {
		return readByName(env, e)
	}
}

func readByName(env *Environment, e *ast.Identifier) (Value, error) {
	val, ok := env.Get(e.Value)
	if !ok {
		if h := hint.MovedGlobalNameError(e.Value); h != "" {
			return nil, fmt.Errorf("Satır %d, Sütun %d: %s", e.Line(), e.Token.Column, h)
		}
		return nil, fmt.Errorf("Satır %d, Sütun %d: '%s' adında bir değişken ya da fonksiyon yok%s",
			e.Line(), e.Token.Column, e.Value, suggestName(e.Value, env))
	}
	return val, nil
}

// yuvaOrtami çözümün işaret ettiği ortamı döndürür (yalnızca cozYerel/cozGenel için).
func slotEnv(r resolution, env *Environment) *Environment {
	if r.kind == resolveGlobal {
		return r.global
	}
	o := env
	for j := 0; j < r.nestDepth; j++ {
		o = o.outer
	}
	return o
}
