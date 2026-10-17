package vm

import (
	"fmt"
	"strings"
	"tr_dili/pkg/ast"
)

func (v *VM) evalExpression(expr ast.Expression, env *Environment) (Value, error) {
	val, _, err := v.evalExpressionWithSignal(expr, env)
	return val, err
}

func (v *VM) evalExpressionWithSignal(expr ast.Expression, env *Environment) (Value, flowSignal, error) {
	if expr == nil {
		return NULL, signalNone, nil
	}
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return &IntVal{Val: e.Value}, signalNone, nil
	case *ast.FloatLiteral:
		return &FloatVal{Val: e.Value}, signalNone, nil
	case *ast.StringLiteral:
		return &StringVal{Val: e.Value}, signalNone, nil
	case *ast.InterpolatedString:
		var sb strings.Builder
		for _, part := range e.Parts {
			val, err := v.evalExpression(part, env)
			if err != nil {
				return nil, signalNone, err
			}
			sb.WriteString(val.Inspect())
		}
		return &StringVal{Val: sb.String()}, signalNone, nil
	case *ast.BooleanLiteral:
		if e.Value {
			return TRUE, signalNone, nil
		}
		return FALSE, signalNone, nil
	case *ast.NullLiteral:
		return NULL, signalNone, nil
	case *ast.Identifier:
		val, ok := env.Get(e.Value)
		if !ok {
			return nil, signalNone, fmt.Errorf("Satır %d, Sütun %d: '%s' adında bir değişken ya da fonksiyon yok%s",
				e.Line(), e.Token.Column, e.Value, suggestName(e.Value, env))
		}
		return val, signalNone, nil

	case *ast.ListLiteral:
		elems := make([]Value, 0, len(e.Elements))
		for _, el := range e.Elements {
			val, err := v.evalExpression(el, env)
			if err != nil {
				return nil, signalNone, err
			}
			elems = append(elems, val)
		}
		return &ListVal{Elements: elems}, signalNone, nil

	case *ast.ListComprehension:
		val, err := v.evalListComprehension(e, env)
		return val, signalNone, err

	case *ast.DictLiteral:
		dict := NewDictVal()
		for _, pair := range e.Pairs {
			kVal, err := v.evalExpression(pair.Key, env)
			if err != nil {
				return nil, signalNone, err
			}
			vVal, err := v.evalExpression(pair.Value, env)
			if err != nil {
				return nil, signalNone, err
			}
			dict.SetValue(kVal, vVal)
		}
		return dict, signalNone, nil

	case *ast.RangeExpression:
		startVal, err := v.evalExpression(e.Start, env)
		if err != nil {
			return nil, signalNone, err
		}
		endVal, err := v.evalExpression(e.End, env)
		if err != nil {
			return nil, signalNone, err
		}
		sInt, ok1 := startVal.(*IntVal)
		eInt, ok2 := endVal.(*IntVal)
		if !ok1 || !ok2 {
			return nil, signalNone, fmt.Errorf("Satır %d: Aralık ('..') uçları Tam sayı olmalıdır", e.Line())
		}
		return &RangeVal{
			Start:     sInt.Val,
			End:       eInt.Val,
			Inclusive: e.Inclusive,
		}, signalNone, nil

	case *ast.PrefixExpression:
		right, err := v.evalExpression(e.Right, env)
		if err != nil {
			return nil, signalNone, err
		}
		if e.Operator == "!" {
			if right.IsTruthy() {
				return FALSE, signalNone, nil
			}
			return TRUE, signalNone, nil
		}
		if e.Operator == "-" {
			switch rv := right.(type) {
			case *IntVal:
				return &IntVal{Val: -rv.Val}, signalNone, nil
			case *FloatVal:
				return &FloatVal{Val: -rv.Val}, signalNone, nil
			}
			return nil, signalNone, fmt.Errorf("Satır %d: '-' operatörü %s tipine uygulanamaz", e.Line(), right.Type())
		}

	case *ast.InfixExpression:
		// Kısa devre ve özel operatörler (&&, ||, ??, |>)
		if e.Operator == "&&" {
			left, err := v.evalExpression(e.Left, env)
			if err != nil {
				return nil, signalNone, err
			}
			if !left.IsTruthy() {
				return FALSE, signalNone, nil
			}
			right, err := v.evalExpression(e.Right, env)
			if err != nil {
				return nil, signalNone, err
			}
			if right.IsTruthy() {
				return TRUE, signalNone, nil
			}
			return FALSE, signalNone, nil
		}
		if e.Operator == "||" {
			left, err := v.evalExpression(e.Left, env)
			if err != nil {
				return nil, signalNone, err
			}
			if left.IsTruthy() {
				return TRUE, signalNone, nil
			}
			right, err := v.evalExpression(e.Right, env)
			if err != nil {
				return nil, signalNone, err
			}
			if right.IsTruthy() {
				return TRUE, signalNone, nil
			}
			return FALSE, signalNone, nil
		}
		if e.Operator == "??" {
			left, err := v.evalExpression(e.Left, env)
			if err != nil {
				return nil, signalNone, err
			}
			if left.Type() != VAL_NULL {
				return left, signalNone, nil
			}
			val, err := v.evalExpression(e.Right, env)
			return val, signalNone, err
		}
		if e.Operator == "|>" {
			val, err := v.evalPipe(e.Left, e.Right, env)
			return val, signalNone, err
		}

		left, err := v.evalExpression(e.Left, env)
		if err != nil {
			return nil, signalNone, err
		}
		right, err := v.evalExpression(e.Right, env)
		if err != nil {
			return nil, signalNone, err
		}
		val, err := v.evalInfix(e.Operator, left, right, e.Line())
		return val, signalNone, err

	case *ast.IfExpression:
		cond, err := v.evalExpression(e.Condition, env)
		if err != nil {
			return nil, signalNone, err
		}
		if cond.IsTruthy() {
			return v.execStatement(e.Consequence, env)
		}
		for _, elif := range e.ElseIfs {
			elifCond, err := v.evalExpression(elif.Condition, env)
			if err != nil {
				return nil, signalNone, err
			}
			if elifCond.IsTruthy() {
				return v.execStatement(elif.Consequence, env)
			}
		}
		if e.Alternative != nil {
			return v.execStatement(e.Alternative, env)
		}
		return NULL, signalNone, nil

	case *ast.MatchExpression:
		return v.evalMatch(e, env)

	case *ast.FunctionLiteral:
		fn := &FuncVal{
			Name:       e.Name,
			Parameters: e.Parameters,
			Body:       e.Body,
			Env:        env,
		}
		if e.Name != "" {
			env.Define(e.Name, fn, false)
		}
		return fn, signalNone, nil

	case *ast.CallExpression:
		val, err := v.evalCall(e, env)
		return val, signalNone, err

	case *ast.IndexExpression:
		val, err := v.evalIndex(e, env)
		return val, signalNone, err

	case *ast.MemberExpression:
		val, err := v.evalMember(e, env)
		return val, signalNone, err
	}

	return NULL, signalNone, nil
}

func (v *VM) evalPipe(left ast.Expression, right ast.Expression, env *Environment) (Value, error) {
	leftVal, err := v.evalExpression(left, env)
	if err != nil {
		return nil, err
	}

	// Sağ taraf bir fonksiyon çağrısı ise (örn: veri |> filtrele((x) => x > 2))
	if callExpr, ok := right.(*ast.CallExpression); ok {
		fnVal, err := v.evalExpression(callExpr.Function, env)
		if err != nil {
			return nil, err
		}
		args := []Value{leftVal}
		for _, a := range callExpr.Arguments {
			av, err := v.evalExpression(a, env)
			if err != nil {
				return nil, err
			}
			args = append(args, av)
		}
		named := make(map[string]Value)
		for _, na := range callExpr.NamedArgs {
			nv, err := v.evalExpression(na.Value, env)
			if err != nil {
				return nil, err
			}
			named[na.Name] = nv
		}
		if err := checkArity(fnVal, len(args), named, callExpr.Line()); err != nil {
			return nil, err
		}
		return v.ApplyFunction(fnVal, args, named, callExpr.Line())
	}

	// Sağ taraf doğrudan bir fonksiyon ise (örn: veri |> sırala)
	fnVal, err := v.evalExpression(right, env)
	if err != nil {
		return nil, err
	}
	if err := checkArity(fnVal, 1, nil, right.Line()); err != nil {
		return nil, err
	}
	return v.ApplyFunction(fnVal, []Value{leftVal}, nil, right.Line())
}

func (v *VM) evalMatch(e *ast.MatchExpression, env *Environment) (Value, flowSignal, error) {
	subj, err := v.evalExpression(e.Subject, env)
	if err != nil {
		return nil, signalNone, err
	}

	for _, mc := range e.Cases {
		matched := mc.IsDefault
		if !matched {
			for _, pat := range mc.Patterns {
				patVal, err := v.evalExpression(pat, env)
				if err != nil {
					return nil, signalNone, err
				}
				if rng, ok := patVal.(*RangeVal); ok {
					if iv, ok := subj.(*IntVal); ok {
						if rng.Inclusive && iv.Val >= rng.Start && iv.Val <= rng.End {
							matched = true
							break
						} else if !rng.Inclusive && iv.Val >= rng.Start && iv.Val < rng.End {
							matched = true
							break
						}
					}
				} else if ValuesEqual(subj, patVal) {
					matched = true
					break
				}
			}
		}

		if matched {
			if mc.Body != nil {
				return v.execStatement(mc.Body, env)
			}
			if mc.ValueExpr != nil {
				val, err := v.evalExpression(mc.ValueExpr, env)
				return val, signalNone, err
			}
			return NULL, signalNone, nil
		}
	}

	return NULL, signalNone, nil
}

func (v *VM) evalListComprehension(lc *ast.ListComprehension, env *Environment) (Value, error) {
	iterVal, err := v.evalExpression(lc.Iterable, env)
	if err != nil {
		return nil, err
	}

	var items []Value
	switch it := iterVal.(type) {
	case *RangeVal:
		end := it.End
		if it.Inclusive {
			end++
		}
		step := int64(1)
		if it.Start > it.End {
			step = -1
			if it.Inclusive {
				end = it.End - 1
			} else {
				end = it.End
			}
		}
		for i := it.Start; (step > 0 && i < end) || (step < 0 && i > end); i += step {
			items = append(items, &IntVal{Val: i})
		}
	case *ListVal:
		items = it.Elements
	case *MatrixVal:
		for i := 0; i < it.Rows; i++ {
			row := make([]Value, it.Cols)
			for j := 0; j < it.Cols; j++ {
				row[j] = &FloatVal{Val: it.Data[i][j]}
			}
			items = append(items, &ListVal{Elements: row})
		}
	case *DictVal:
		for _, k := range it.Order {
			items = append(items, it.KeyValue(k))
		}
	case *StringVal:
		for _, ch := range []rune(it.Val) {
			items = append(items, &StringVal{Val: string(ch)})
		}
	default:
		return nil, fmt.Errorf("Satır %d: Liste üretecinde %s tipi üzerinde döngü kurulamaz", lc.Line(), iterVal.Type())
	}

	var results []Value
	for _, item := range items {
		compEnv := NewEnvironment(env)
		compEnv.Define(lc.VarName, item, false)
		if lc.Condition != nil {
			condVal, err := v.evalExpression(lc.Condition, compEnv)
			if err != nil {
				return nil, err
			}
			if !condVal.IsTruthy() {
				continue
			}
		}
		outVal, err := v.evalExpression(lc.Output, compEnv)
		if err != nil {
			return nil, err
		}
		results = append(results, outVal)
	}

	return &ListVal{Elements: results}, nil
}

func (v *VM) evalIndex(e *ast.IndexExpression, env *Environment) (Value, error) {
	left, err := v.evalExpression(e.Left, env)
	if err != nil {
		return nil, err
	}
	idx, err := v.evalExpression(e.Index, env)
	if err != nil {
		return nil, err
	}

	switch container := left.(type) {
	case *ListVal:
		// Dilimleme (Slicing): liste[1..4]
		if rng, ok := idx.(*RangeVal); ok {
			start := int(rng.Start)
			end := int(rng.End)
			if rng.Inclusive {
				end++
			}
			if start < 0 {
				start = 0
			}
			if end > len(container.Elements) {
				end = len(container.Elements)
			}
			if start > end {
				return &ListVal{Elements: []Value{}}, nil
			}
			sliced := make([]Value, end-start)
			copy(sliced, container.Elements[start:end])
			return &ListVal{Elements: sliced}, nil
		}
		iv, ok := idx.(*IntVal)
		if !ok {
			return nil, fmt.Errorf("Satır %d: Liste indeksi Tam sayı olmalıdır", e.Line())
		}
		pos := int(iv.Val)
		if pos < 0 {
			pos = len(container.Elements) + pos
		}
		if pos < 0 || pos >= len(container.Elements) {
			return NULL, nil
		}
		return container.Elements[pos], nil

	case *MatrixVal:
		iv, ok := idx.(*IntVal)
		if !ok {
			return nil, fmt.Errorf("Satır %d: Matris indeksi Tam sayı olmalıdır", e.Line())
		}
		pos := int(iv.Val)
		if pos < 0 {
			pos = container.Rows + pos
		}
		if pos < 0 || pos >= container.Rows {
			return NULL, nil
		}
		rowElems := make([]Value, container.Cols)
		for j := 0; j < container.Cols; j++ {
			rowElems[j] = &FloatVal{Val: container.Data[pos][j]}
		}
		return &ListVal{Elements: rowElems}, nil

	case *DictVal:
		val, exists := container.GetValue(idx)
		if !exists {
			return NULL, nil
		}
		return val, nil

	case *StringVal:
		runes := []rune(container.Val)
		if rng, ok := idx.(*RangeVal); ok {
			start := int(rng.Start)
			end := int(rng.End)
			if rng.Inclusive {
				end++
			}
			if start < 0 {
				start = 0
			}
			if end > len(runes) {
				end = len(runes)
			}
			if start > end {
				return &StringVal{Val: ""}, nil
			}
			return &StringVal{Val: string(runes[start:end])}, nil
		}
		iv, ok := idx.(*IntVal)
		if !ok {
			return nil, fmt.Errorf("Satır %d: Metin indeksi Tam sayı olmalıdır", e.Line())
		}
		pos := int(iv.Val)
		if pos < 0 {
			pos = len(runes) + pos
		}
		if pos < 0 || pos >= len(runes) {
			return NULL, nil
		}
		return &StringVal{Val: string(runes[pos])}, nil
	}

	return nil, fmt.Errorf("Satır %d: %s tipi indekslenemez", e.Line(), left.Type())
}

func (v *VM) evalMember(e *ast.MemberExpression, env *Environment) (Value, error) {
	obj, err := v.evalExpression(e.Object, env)
	if err != nil {
		return nil, err
	}
	if obj.Type() == VAL_NULL {
		if e.Safe {
			return NULL, nil
		}
		return nil, fmt.Errorf("Satır %d: 'yok' (null) değerin %q özelliğine erişilemez ('?.' güvenli erişim kullanabilirsiniz)", e.Line(), e.Property)
	}

	// Matris, Tensör, SinirAğı, WebSunucu, GörselArayüz ve Veritabanı özellikleri
	if pg, ok := obj.(PropertyGetter); ok {
		return pg.GetProperty(e.Property)
	}

	switch inst := obj.(type) {
	case *StructInstanceVal:
		if val, ok := inst.Fields[e.Property]; ok {
			return val, nil
		}
		if method, ok := inst.Def.Methods[e.Property]; ok {
			return &FuncVal{
				Name:       method.Name,
				Parameters: method.Parameters,
				Body:       method.Body,
				Env:        method.Env,
				BoundSelf:  inst,
			}, nil
		}
		if e.Safe {
			return NULL, nil
		}
		return nil, fmt.Errorf("Satır %d: %s yapısında %q alanı veya metodu bulunamadı", e.Line(), inst.Def.Name, e.Property)

	case *DictVal, *ListVal, *StringVal:
		return v.evalBuiltinMember(obj, e.Property, e.Safe, e.Line())
	}

	return nil, fmt.Errorf("Satır %d: %s tipinde %q özelliği bulunamadı", e.Line(), obj.Type(), e.Property)
}
