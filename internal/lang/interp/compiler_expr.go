package interp

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/hint"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// constCode her zaman aynı (değişmez) değeri veren kod üretir; sabitler bir kez kutulanır.
func constCode(d Value) exprCode {
	return func(v *VM, env *Environment) (Value, error) { return d, nil }
}

var nullCode = constCode(Null)

func (c *compiler) expressions(k *compilerScope, exprs []ast.Expression) []exprCode {
	codeList := make([]exprCode, len(exprs))
	for i, e := range exprs {
		codeList[i] = c.expression(k, e)
	}
	return codeList
}

// exprSignal deyim konumundaki bir ifadeyi derler. 'eğer' ve 'eşle' dallarında çalışan
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

// noSignal deyim konumu dışında kullanılan 'eğer'/'eşle' için sinyali düşürür.
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
			return constCode(True)
		}
		return constCode(False)
	case *ast.NullLiteral:
		return nullCode

	case *ast.InterpolatedString:
		return c.interpolation(c.expressions(k, e.Parts))

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
			l := &ListVal{Elements: elems}
			v.ownList(l)
			return l, nil
		}

	case *ast.ListComprehension:
		return c.listComprehension(k, e)

	case *ast.DictLiteral:
		return c.dictLiteral(k, e)

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
				return nil, messages.At(line, 0, messages.New(messages.H2439, ValRange, ValInt))
			}
			return &RangeVal{Start: sInt.Val, End: eInt.Val, Inclusive: inclusive}, nil
		}

	case *ast.PrefixExpression:
		return c.prefixText(k, e)

	case *ast.InfixExpression:
		return c.binary(k, e)

	case *ast.IfExpression:
		return nestedStatements(noSignal(c.ifExpr(k, e)))

	case *ast.MatchExpression:
		return nestedStatements(noSignal(c.matchExpr(k, e)))

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
		if id, ok := e.Function.(*ast.Identifier); ok && builtinName(id.Value, c.textLang) {
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
			newOp()
			res, err := v.indexValue(left, idx, line)
			if err == nil {
				v.afterIndex(res, left, idx)
			}
			return res, err
		}

	case *ast.MemberExpression:
		object := c.expression(k, e.Object)
		u := c.member(e.Property) // ad → kod derleme anında bir kez çözülür
		return func(v *VM, env *Environment) (Value, error) {
			obj, err := object(v, env)
			if err != nil {
				return nil, err
			}
			res, err := v.memberValue(obj, e, u)
			if _, method := res.(*BuiltinVal); method {
				escape(obj) // bağlı metot değeri alıcıyı tutar (ownership.go)
			}
			return res, err
		}
	}
	return nullCode
}

// dictLiteral sözlük yazımının kodudur: anahtar bir ifadedir (3.7; {k: 1}'de k değişkendir).
func (c *compiler) dictLiteral(k *compilerScope, e *ast.DictLiteral) exprCode {
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
			if err := DictKeyError(kVal); err != nil {
				return nil, messages.At(e.Line(), 0, err)
			}
			escape(vVal) // sözlüğe yazılan değer kaçar (ownership.go)
			dict.SetValue(kVal, vVal)
		}
		return dict, nil
	}
}

// interpolation metin yerleştirmesinin kodudur: parçaların değerleri derlenen dosyanın dilinde
// metne çevrilip birleştirilir (3; c.textLang).
func (c *compiler) interpolation(pieces []exprCode) exprCode {
	if lang := c.textLang; lang != nil {
		return func(v *VM, env *Environment) (Value, error) {
			var sb strings.Builder
			for _, p := range pieces {
				val, err := p(v, env)
				if err != nil {
					return nil, err
				}
				sb.WriteString(valueText(val, lang))
			}
			return &StringVal{Val: sb.String()}, nil
		}
	}
	// Türkçe dosya: çekirdek değerlerin metni Inspect'tir (Inspect bugün Türkçedir, codes.Source); Inspect'i görüntü
	// olan öbür değerler (fonksiyon, yerleşik, yapı, motor değerleri) valueText'e gider. Sıcak
	// yol: tip anahtarı somut.
	return func(v *VM, env *Environment) (Value, error) {
		var sb strings.Builder
		for _, p := range pieces {
			val, err := p(v, env)
			if err != nil {
				return nil, err
			}
			switch x := val.(type) {
			case *StringVal:
				sb.WriteString(x.Val)
			case *IntVal, *FloatVal, *BoolVal, *NullVal:
				sb.WriteString(val.Inspect())
			default:
				sb.WriteString(valueText(val, nil))
			}
		}
		return &StringVal{Val: sb.String()}, nil
	}
}

// --- DEĞİŞKEN OKUMA ---

// readName bir adın değerini okuyan kodu üretir. Derleme anında çözülen adlar yuvadan okunur;
// yuva boşsa (ya da ad çözülemediyse) arama adla yapılır.
func (c *compiler) readName(k *compilerScope, e *ast.Identifier) exprCode {
	read := c.readSlot(k, e)
	if !blockParams[e.Value] {
		return read
	}
	// Argümansız çağrılan sondaki bloğun TR_it'i değer almamıştır (6.7, geçiş).
	return func(v *VM, env *Environment) (Value, error) {
		x, err := read(v, env)
		if err == nil && x == unboundArg {
			return nil, messages.At(e.Line(), e.Token.Column, messages.New(messages.H2707))
		}
		return x, err
	}
}

// readSlot adın değerini yuvasından (ya da adla) okuyan koddur.
func (c *compiler) readSlot(k *compilerScope, e *ast.Identifier) exprCode {
	r := k.resolve(e.Value)
	i := r.slot
	switch r.kind {
	case resolveNotYet:
		return func(v *VM, env *Environment) (Value, error) {
			return nil, messages.At(e.Line(), e.Token.Column, messages.New(messages.H2440, e.Value))
		}
	case resolveGlobal:
		g := r.global
		return func(v *VM, env *Environment) (Value, error) {
			if i < len(g.vals) {
				if x := g.vals[i]; x != nil {
					return x, nil
				}
			}
			return v.readByName(env, e)
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
				return v.readByName(env, e)
			}
		case 1:
			return func(v *VM, env *Environment) (Value, error) {
				o := env.outer
				if i < len(o.vals) {
					if x := o.vals[i]; x != nil {
						return x, nil
					}
				}
				return v.readByName(env, e)
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
				return v.readByName(env, e)
			}
		}
	}
	return func(v *VM, env *Environment) (Value, error) {
		return v.readByName(env, e)
	}
}

func (v *VM) readByName(env *Environment, e *ast.Identifier) (Value, error) {
	val, ok := env.Get(e.Value)
	if !ok {
		// Program bir dosyadır, dili yoktur (2.1): öneri kimliğin yazımıdır (TR_print).
		return nil, messages.At(e.Line(), e.Token.Column, hint.UndefinedNameError(e.Value, env.Names(), nil))
	}
	return val, nil
}

// slotEnv çözümün işaret ettiği ortamı döndürür (yalnızca cozYerel/cozGenel için).
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
