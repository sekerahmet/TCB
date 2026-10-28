package interp

import (
	"math"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Tip çağrısı (3.12): tip çağrılınca değerini kurar ya da verileni o tipe çevirir (TR_Int("42"),
// TR_Float(n), TR_List(0..3), TR_Tensor([1, 2])). Çekirdeğin tipleri burada, kitaplığın tipleri
// kendi motorlarında kaydedilir (RegisterTypeCall). Yapının tipi yapının kendisi gibi çağrılır.

// RegisterTypeCall binds the call of type k (3.12) to fn: TR_Tensor([1, 2]) runs it. fn is the
// type's own builtin (its arguments, named arguments and purity); the call is named after the type
// in messages.
func (v *VM) RegisterTypeCall(k codes.Code, fn *BuiltinVal) {
	if k.Namespace() != codes.NamespaceType {
		panic("RegisterTypeCall: " + k.String() + " is not a type code")
	}
	if v.typeCalls == nil {
		v.typeCalls = map[codes.Code]*BuiltinVal{}
	}
	b := *fn
	b.Name = codes.Name(k)
	v.typeCalls[k] = &b
}

// registerTypeCalls çekirdeğin tiplerinin çağrısını kaydeder.
func (v *VM) registerTypeCalls() {
	for _, c := range []struct {
		k  codes.Code
		fn func(*VM, Value) (Value, error)
	}{
		{codes.TypeInt, intCall},
		{codes.TypeFloat, floatCall},
		{codes.TypeString, stringCall},
		{codes.TypeBool, boolCall},
		{codes.TypeList, listCall},
		{codes.TypeType, typeCall},
		{codes.TypeJSON, jsonCall},
		{codes.TypeBytes, bytesCall},
	} {
		k, single := c.k, c.fn
		v.RegisterTypeCall(k, &BuiltinVal{NoNamedArgs: true, Single: single,
			Fn: func(y *VM, args []Value, _ map[string]Value) (Value, error) {
				if len(args) != 1 {
					return nil, ArgCountError(k, 1, len(args), messages.Text(messages.H2301))
				}
				return single(y, args[0])
			}})
	}
}

// callType runs the call of type value t: a struct's type builds an instance like the struct, a
// type with a registered call runs it; any other type is not called.
func (v *VM) callType(t *TypeVal, args []Value, named map[string]Value, line int) (Value, error) {
	if t.Struct != nil {
		return v.apply(t.Struct, args, named, line)
	}
	b := v.typeCalls[t.Code]
	if b == nil && t.Code == codes.TypeRange {
		return nil, messages.At(line, 0, messages.New(messages.H2657)) // aralık a..b yazılır (3.8)
	}
	if b == nil {
		return nil, messages.At(line, 0, messages.New(messages.H2645, codes.NameRef(t.Code)))
	}
	if b.Single != nil && len(args) == 1 && len(named) == 0 {
		res, err := b.Single(v, args[0])
		if err != nil {
			return nil, lineSuffix(line, err)
		}
		return res, nil
	}
	return v.callBuiltin(b, args, named, line)
}

// intCall is TR_Int(x): a decimal is cut toward zero, a text is read in 2.4's decimal spelling.
func intCall(_ *VM, a Value) (Value, error) {
	switch x := a.(type) {
	case *JSONVal:
		if n, ok := x.v.(*IntVal); ok {
			return n, nil
		}
		return nil, jsonKindError(codes.TypeInt, x)
	case *IntVal:
		return x, nil
	case *FloatVal:
		n, err := floatToInt(CallPrefix(codes.TypeInt), x.Val)
		if err != nil {
			return nil, err
		}
		return NewInt(n), nil
	case *StringVal:
		return parseIntText(x.Val)
	}
	return nil, ArgTypeError(codes.TypeInt, 1, a, codes.TypeFloat, codes.TypeString)
}

// floatCall is TR_Float(x): an integer becomes a decimal, a text is read in 2.4's or 2.5's spelling
// or as "inf", "-inf", "nan".
func floatCall(_ *VM, a Value) (Value, error) {
	switch x := a.(type) {
	case *JSONVal:
		switch n := x.v.(type) {
		case *FloatVal:
			return n, nil
		case *IntVal:
			return &FloatVal{Val: float64(n.Val)}, nil
		}
		return nil, jsonKindError(codes.TypeFloat, x)
	case *FloatVal:
		return x, nil
	case *IntVal:
		return &FloatVal{Val: float64(x.Val)}, nil
	case *StringVal:
		f, ok := parseFloatText(x.Val)
		if !ok {
			return nil, messages.New(messages.H2642, x.Val)
		}
		return &FloatVal{Val: f}, nil
	}
	return nil, ArgTypeError(codes.TypeFloat, 1, a, codes.TypeInt, codes.TypeString)
}

// stringCall is TR_String(x): the value's text (3.14).
func stringCall(y *VM, a Value) (Value, error) {
	if s, ok := a.(*StringVal); ok {
		return s, nil
	}
	if j, ok := a.(*JSONVal); ok {
		// JSON değerinde tip çağrısı değeri açar (3.12, 3.7a): yalnız JSON metni metindir.
		if s, ok := j.v.(*StringVal); ok {
			return s, nil
		}
		return nil, jsonKindError(codes.TypeString, j)
	}
	if b, ok := a.(*BytesVal); ok {
		return bytesText(b)
	}
	return &StringVal{Val: y.textOf(a)}, nil
}

// boolCall is TR_Bool(m): only the texts "true" and "false" (3.12); no truth value is made from a
// number or a list (3.10).
func boolCall(_ *VM, a Value) (Value, error) {
	switch x := a.(type) {
	case *JSONVal:
		if b, ok := x.v.(*BoolVal); ok {
			return b, nil
		}
		return nil, jsonKindError(codes.TypeBool, x)
	case *BoolVal:
		return x, nil
	case *StringVal:
		switch x.Val {
		case "true":
			return True, nil
		case "false":
			return False, nil
		}
		return nil, messages.New(messages.H2643, x.Val)
	}
	return nil, messages.New(messages.H2644, TypeAndSummary(a))
}

// listCall is TR_List(x): the integers of a range, a new list with a list's elements, or a JSON
// array's elements as a list of JSON values (its kind is List[JSON], 3.7a).
func listCall(_ *VM, a Value) (Value, error) {
	switch x := a.(type) {
	case *JSONVal:
		l, ok := x.v.(*ListVal)
		if !ok {
			return nil, jsonKindError(codes.TypeList, x)
		}
		elems := make([]Value, len(l.Elements))
		for i, e := range l.Elements {
			elems[i] = &JSONVal{v: e}
		}
		return &ListVal{Elements: elems}, nil
	case *RangeVal:
		return rangeList(x)
	case *ListVal:
		return &ListVal{Elements: append([]Value{}, x.Elements...)}, nil
	}
	return nil, ArgTypeError(codes.TypeList, 1, a, codes.TypeRange, codes.TypeList)
}

// typeCall is TR_Type(x): the value's type (3.11); for a JSON value its JSON kind (3.7a).
func typeCall(_ *VM, a Value) (Value, error) { return TypeOfValue(a), nil }

// jsonCall is TR_JSON(x): a JSON value built from x (3.7a).
func jsonCall(_ *VM, a Value) (Value, error) { return MakeJSON(a) }

// rangeList is the list of a range's integers; a range longer than MaxCollectionSize is an error.
func rangeList(r *RangeVal) (Value, error) {
	first, last, step, empty := rangeSteps(r)
	if empty {
		return &ListVal{Elements: []Value{}}, nil
	}
	if n := uint64(last-first)*uint64(step) + 1; n > MaxCollectionSize || n == 0 {
		return nil, messages.New(messages.H2435, CallPrefix(codes.TypeList), MaxCollectionSize)
	}
	elems := make([]Value, 0, uint64(last-first)*uint64(step)+1)
	for i := first; ; i += step {
		elems = append(elems, NewInt(i))
		if i == last {
			break
		}
	}
	return &ListVal{Elements: elems}, nil
}

// parseIntText reads s in 2.4's decimal spelling: an optional sign, digits with '_' only between
// digits, no spaces and no locale.
func parseIntText(s string) (Value, error) {
	digits, ok := numberDigits(s, false)
	if !ok {
		return nil, messages.New(messages.H2640, s)
	}
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return nil, messages.New(messages.H2641, s)
	}
	return NewInt(n), nil
}

// parseFloatText reads s in 2.4's or 2.5's spelling with an optional sign, or as "inf", "-inf",
// "nan"; ok is false for anything else and for a number too large for a decimal.
func parseFloatText(s string) (float64, bool) {
	inf, nan := codes.ValueNames().Name(codes.ValInf), codes.ValueNames().Name(codes.ValNan)
	switch s {
	case inf:
		return math.Inf(1), true
	case "-" + inf:
		return math.Inf(-1), true
	case nan:
		return math.NaN(), true
	}
	digits, ok := numberDigits(s, true)
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(digits, 64)
	if err != nil || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// numberDigits checks s against the number spelling of 2.4 (decimal integer) or, with decimal,
// 2.5 (a dot with digits on both sides, an exponent) after an optional sign, and returns it
// without '_'.
func numberDigits(s string, decimal bool) (string, bool) {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	i, ok := digitRun(s, i)
	if !ok {
		return "", false
	}
	if decimal && i < len(s) && s[i] == '.' {
		if i, ok = digitRun(s, i+1); !ok {
			return "", false
		}
	}
	if decimal && i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		if i, ok = digitRun(s, i); !ok {
			return "", false
		}
	}
	if i != len(s) {
		return "", false
	}
	return strings.ReplaceAll(s, "_", ""), true
}

// digitRun reads at least one digit from i, with single '_' only between digits; it returns the
// index after the run.
func digitRun(s string, i int) (int, bool) {
	start := i
	for i < len(s) {
		switch {
		case s[i] >= '0' && s[i] <= '9':
			i++
		case s[i] == '_' && i > start && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9':
			i++
		default:
			return i, i > start
		}
	}
	return i, i > start
}
