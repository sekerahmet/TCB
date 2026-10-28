package interp

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// TR_JSON (3.7a): yapısı önceden bilinmeyen veri. Değer JSON'un bir değerini çekirdeğin değerleriyle
// tutar (yok, mantık değeri, Tam, Ondalık, metin, liste, metin anahtarlı sözlük); okumak yine TR_JSON
// verir, asıl değere tip çağrısıyla geçilir (TR_Int(j) ...). Değer değiştirilemez: kurarken kopyalanır.

// ValJSON is the type name of JSONVal.
var ValJSON = TypeName(codes.TypeJSON)

// JSONVal is a TR_JSON value: v is one JSON value made of core values (Null, BoolVal, IntVal,
// FloatVal, StringVal, ListVal and DictVal with string keys, all JSON again).
type JSONVal struct{ v Value }

// NewJSON wraps a JSON value that is already made of core values (JSONToTR's result).
func NewJSON(v Value) *JSONVal { return &JSONVal{v: v} }

// Unwrap is the JSON value as core values (data formats write it).
func (j *JSONVal) Unwrap() Value { return j.v }

func (j *JSONVal) Type() ValueType { return ValJSON }
func (j *JSONVal) IsTruthy() bool  { return true }

// Inspect is the value's text (3.14, 3.7a): a JSON string is its text, any other JSON value its
// JSON text.
func (j *JSONVal) Inspect() string {
	if s, ok := j.v.(*StringVal); ok {
		return s.Val
	}
	return j.JSONText()
}

// JSONText is the JSON text of the value (TR_json_string): compact, keys in their order; a number
// keeps its integer or decimal spelling (3.7a: 2 and 2.0 stay apart).
func (j *JSONVal) JSONText() string {
	var b []byte
	return string(appendJSON(b, j.v))
}

func appendJSON(b []byte, v Value) []byte {
	switch x := v.(type) {
	case *NullVal:
		return append(b, "null"...)
	case *BoolVal:
		return strconv.AppendBool(b, x.Val)
	case *IntVal:
		return strconv.AppendInt(b, x.Val, 10)
	case *FloatVal:
		return append(b, FloatText(x.Val)...)
	case *StringVal:
		return appendJSONString(b, x.Val)
	case *ListVal:
		b = append(b, '[')
		for i, e := range x.Elements {
			if i > 0 {
				b = append(b, ',')
			}
			b = appendJSON(b, e)
		}
		return append(b, ']')
	case *DictVal:
		b = append(b, '{')
		for i, k := range x.Order {
			if i > 0 {
				b = append(b, ',')
			}
			b = appendJSONString(b, x.KeyText(k))
			b = append(b, ':')
			b = appendJSON(b, x.Pairs[k])
		}
		return append(b, '}')
	}
	return b
}

// appendJSONString metni JSON metin olarak yazar (HTML kaçışı yok).
func appendJSONString(b []byte, s string) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return append(b, bytes.TrimSuffix(buf.Bytes(), []byte("\n"))...)
}

// Equal is JSON equality (4.5): the same JSON value, numbers by 4.5's number rule, an object's keys in
// any order.
func (j *JSONVal) Equal(b Value) bool {
	o, ok := b.(*JSONVal)
	return ok && ValuesEqual(j.v, o.v)
}

// kind is the type of the JSON value (TR_Type(j), 3.7a): NoneType, Bool, Int, Float, String, List, Dict.
func (j *JSONVal) kind() *TypeVal { return TypeOfValue(j.v) }

// kindName is the JSON value's kind as a message parameter.
func (j *JSONVal) kindName() codes.Ref { return codes.NameRef(j.kind().Code) }

// Index is j["k"] on an object and j[i] on an array; the result is JSON again. A missing key and an
// index out of range are errors (3.7a).
func (j *JSONVal) Index(idx Value) (Value, error) {
	switch x := j.v.(type) {
	case *DictVal:
		k, ok := idx.(*StringVal)
		if !ok {
			return nil, messages.New(messages.H2650, j.kindName(), TypeAndSummary(idx))
		}
		v, ok := x.GetValue(k)
		if !ok {
			return nil, messages.New(messages.H2649, QuoteText(k.Val))
		}
		return &JSONVal{v: v}, nil
	case *ListVal:
		i, ok := idx.(*IntVal)
		if !ok {
			return nil, messages.New(messages.H2650, j.kindName(), TypeAndSummary(idx))
		}
		pos, in := resolveIndex(i.Val, len(x.Elements))
		if !in {
			return nil, messages.New(messages.H2515, ValJSON, i.Val, len(x.Elements), codes.NameRef(codes.MemGet))
		}
		return &JSONVal{v: x.Elements[pos]}, nil
	}
	return nil, messages.New(messages.H2650, j.kindName(), TypeAndSummary(idx))
}

// GetProperty gives .length (object and array), .keys() (object) and .get(k) (object: a key; array:
// an index; missing: none).
func (j *JSONVal) GetProperty(kd codes.Code, name string) (Value, error) {
	switch kd {
	case codes.MemLength:
		switch x := j.v.(type) {
		case *DictVal:
			return NewInt(int64(len(x.Order))), nil
		case *ListVal:
			return NewInt(int64(len(x.Elements))), nil
		case *StringVal:
			return nil, messages.New(messages.H2666) // metnin uzunluğu açılınca: TR_String(j).TR_length
		}
		return nil, messages.New(messages.H2651, codes.NameRef(kd), j.kindName())
	case codes.MemKeys:
		d, ok := j.v.(*DictVal)
		if !ok {
			return nil, messages.New(messages.H2651, codes.NameRef(kd), j.kindName())
		}
		return &BuiltinVal{Name: codes.Name(kd), NoNamedArgs: true, Fn: func(_ *VM, args []Value, _ map[string]Value) (Value, error) {
			if len(args) != 0 {
				return nil, ArgCountError(kd, 0, len(args))
			}
			keys := make([]Value, len(d.Order))
			for i, k := range d.Order {
				keys[i] = &StringVal{Val: d.KeyText(k)}
			}
			return &ListVal{Elements: keys}, nil
		}}, nil
	case codes.MemGet:
		return &BuiltinVal{Name: codes.Name(kd), NoNamedArgs: true, Fn: func(_ *VM, args []Value, _ map[string]Value) (Value, error) {
			if len(args) != 1 {
				return nil, ArgCountError(kd, 1, len(args))
			}
			v, err := j.Index(args[0])
			if err != nil && (messages.Has(err, messages.H2649) || messages.Has(err, messages.H2515)) {
				return Null, nil
			}
			return v, err
		}}, nil
	}
	return nil, messages.New(messages.H2428, ValJSON, name, messages.New(messages.H2658, name))
}

// jsonKindError tip çağrısının JSON değerini açamadığı hatadır (3.7a): değerin türü tutmuyor.
func jsonKindError(want codes.Code, j *JSONVal) error {
	return messages.New(messages.H2652, codes.NameRef(want), j.kindName())
}

// MakeJSON builds a TR_JSON from a value (TR_JSON(x)): text, a number (not inf or nan), a truth
// value, none, a list and a dict with text keys of these, or JSON. The value is copied.
func MakeJSON(v Value) (*JSONVal, error) {
	c, err := jsonCopy(v, 0)
	if err != nil {
		return nil, err
	}
	return &JSONVal{v: c}, nil
}

func jsonCopy(v Value, depth int) (Value, error) {
	if depth > maxJSONDepth {
		return nil, messages.New(messages.H2500, maxJSONDepth)
	}
	switch x := v.(type) {
	case *NullVal, *BoolVal, *IntVal, *StringVal:
		return v, nil
	case *FloatVal:
		if math.IsNaN(x.Val) || math.IsInf(x.Val, 0) {
			return nil, messages.New(messages.H2653, x.Inspect())
		}
		return v, nil
	case *JSONVal:
		return x.v, nil
	case *ListVal:
		elems := make([]Value, len(x.Elements))
		for i, e := range x.Elements {
			c, err := jsonCopy(e, depth+1)
			if err != nil {
				return nil, err
			}
			elems[i] = c
		}
		return &ListVal{Elements: elems}, nil
	case *DictVal:
		d := NewDictVal()
		for _, k := range x.Order {
			if _, text := x.KeyValue(k).(*StringVal); !text {
				return nil, messages.New(messages.H2654, TypeAndSummary(x.KeyValue(k)))
			}
			c, err := jsonCopy(x.Pairs[k], depth+1)
			if err != nil {
				return nil, err
			}
			d.Set(k, c)
		}
		return d, nil
	}
	return nil, messages.New(messages.H2654, TypeAndSummary(v))
}

// Compile-time check: the interpreter finds these by type assertion.
var (
	_ Equaler        = (*JSONVal)(nil)
	_ Indexable      = (*JSONVal)(nil)
	_ PropertyGetter = (*JSONVal)(nil)
)
