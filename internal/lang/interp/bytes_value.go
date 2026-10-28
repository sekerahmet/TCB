package interp

import (
	"bytes"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// TR_Bytes (3.3a): değiştirilemez bayt dizisi (dosya, ağ, görüntü, ses verisi). b[i] 0–255 arası bir
// TR_Int, b[a..c] bir TR_Bytes verir; indeks ve dilim metindeki gibidir. Metinle bayt kendiliğinden
// dönüşmez: TR_Bytes(m) UTF-8 kodlar, TR_String(b) UTF-8 okur.

// ValBytes is the type name of BytesVal.
var ValBytes = TypeName(codes.TypeBytes)

// BytesVal is an immutable byte array; b is never changed after it is built.
type BytesVal struct{ b []byte }

// NewBytes is a TR_Bytes holding b (the caller gives up b).
func NewBytes(b []byte) *BytesVal { return &BytesVal{b: b} }

// Data is the bytes (read only).
func (b *BytesVal) Data() []byte { return b.b }

func (b *BytesVal) Type() ValueType { return ValBytes }
func (b *BytesVal) IsTruthy() bool  { return true }

// bytesTextLimit metnin bütün baytları yazdığı en uzun dizidir; daha uzun dizinin metni boyunu ve
// ilk baytlarını verir.
const bytesTextLimit = 64

// Inspect is the value's text: the call that builds it again, without the identity prefix
// (Bytes([104, 105]); copied as TR_Bytes([104, 105]) it reads back). A longer array gives its length
// and first bytes (Bytes(length 1024, [137, 80, …])).
func (b *BytesVal) Inspect() string {
	var sb strings.Builder
	sb.WriteString(codes.ValueNames().Name(codes.TypeBytes))
	sb.WriteByte('(')
	n := len(b.b)
	shown := b.b
	if n > bytesTextLimit {
		sb.WriteString("length " + strconv.Itoa(n) + ", ")
		shown = b.b[:16]
	}
	sb.WriteByte('[')
	for i, c := range shown {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(strconv.Itoa(int(c)))
	}
	if n > bytesTextLimit {
		sb.WriteString(", …")
	}
	sb.WriteString("])")
	return sb.String()
}

// Equal is byte-by-byte equality (4.5).
func (b *BytesVal) Equal(o Value) bool {
	x, ok := o.(*BytesVal)
	return ok && bytes.Equal(b.b, x.b)
}

// Index is b[i] (an Int 0–255; negative from the end; out of range an error) and b[a..c] (Bytes,
// clipped like a text slice).
func (b *BytesVal) Index(idx Value) (Value, error) {
	switch x := idx.(type) {
	case *IntVal:
		pos, ok := resolveIndex(x.Val, len(b.b))
		if !ok {
			return nil, messages.New(messages.H2515, ValBytes, x.Val, len(b.b), codes.NameRef(codes.MemGet))
		}
		return NewInt(int64(b.b[pos])), nil
	case *RangeVal:
		start, end := SliceBounds(x, len(b.b))
		return &BytesVal{b: b.b[start:end:end]}, nil
	}
	return nil, messages.New(messages.H2400, ValBytes, ValInt)
}

// IterLen and IterItem visit the bytes as Ints.
func (b *BytesVal) IterLen() (int, error)         { return len(b.b), nil }
func (b *BytesVal) IterItem(i int) (Value, error) { return NewInt(int64(b.b[i])), nil }

// GetProperty gives .length and .get(i) (none out of range).
func (b *BytesVal) GetProperty(kd codes.Code, name string) (Value, error) {
	switch kd {
	case codes.MemLength:
		return NewInt(int64(len(b.b))), nil
	case codes.MemGet:
		return &BuiltinVal{Name: codes.Name(kd), NoNamedArgs: true, Fn: func(_ *VM, args []Value, _ map[string]Value) (Value, error) {
			if len(args) != 1 {
				return nil, ArgCountError(kd, 1, len(args))
			}
			i, ok := args[0].(*IntVal)
			if !ok {
				return nil, ArgTypeError(kd, 1, args[0], codes.TypeInt)
			}
			if pos, ok := resolveIndex(i.Val, len(b.b)); ok {
				return NewInt(int64(b.b[pos])), nil
			}
			return Null, nil
		}}, nil
	}
	return nil, messages.New(messages.H2428, ValBytes, name, nil)
}

// Operator is '+' (two Bytes joined) and '*' (Bytes repeated by an Int; zero or less gives empty
// bytes; 4.10).
func (b *BytesVal) Operator(op string, other Value, onRight bool) (Value, bool, error) {
	switch op {
	case "+":
		x, ok := other.(*BytesVal)
		if !ok {
			return nil, false, nil
		}
		l, r := b.b, x.b
		if onRight {
			l, r = r, l
		}
		out := make([]byte, 0, len(l)+len(r))
		return &BytesVal{b: append(append(out, l...), r...)}, true, nil
	case "*":
		n, ok := other.(*IntVal)
		if !ok {
			return nil, false, nil
		}
		if n.Val <= 0 {
			return &BytesVal{b: []byte{}}, true, nil
		}
		if !mulFits(int64(len(b.b)), n.Val) || int64(len(b.b))*n.Val > maxRepeatBytes {
			return nil, true, messages.New(messages.H2610, n.Val, maxRepeatBytes)
		}
		return &BytesVal{b: bytes.Repeat(b.b, int(n.Val))}, true, nil
	}
	return nil, false, nil
}

// bytesCall is TR_Bytes(x): the UTF-8 bytes of a text, the bytes of a list of Ints 0–255 (3.3a).
func bytesCall(_ *VM, a Value) (Value, error) {
	switch x := a.(type) {
	case *BytesVal:
		return x, nil
	case *StringVal:
		return &BytesVal{b: []byte(x.Val)}, nil
	case *ListVal:
		out := make([]byte, len(x.Elements))
		for i, e := range x.Elements {
			n, ok := e.(*IntVal)
			if !ok || n.Val < 0 || n.Val > 255 {
				return nil, messages.New(messages.H2655, i, TypeAndSummary(e))
			}
			out[i] = byte(n.Val)
		}
		return &BytesVal{b: out}, nil
	}
	return nil, ArgTypeError(codes.TypeBytes, 1, a, codes.TypeString, codes.TypeList)
}

// bytesText is TR_String(b): the bytes read as UTF-8; invalid UTF-8 is an error (3.3a).
func bytesText(b *BytesVal) (Value, error) {
	if !utf8.Valid(b.b) {
		pos := 0
		for pos < len(b.b) {
			r, n := utf8.DecodeRune(b.b[pos:])
			if r == utf8.RuneError && n <= 1 {
				break
			}
			pos += n
		}
		return nil, messages.New(messages.H2656, pos)
	}
	return &StringVal{Val: string(b.b)}, nil
}

// Compile-time check: the interpreter finds these by type assertion.
var (
	_ Equaler        = (*BytesVal)(nil)
	_ Indexable      = (*BytesVal)(nil)
	_ Iterable       = (*BytesVal)(nil)
	_ PropertyGetter = (*BytesVal)(nil)
	_ OperatorHook   = (*BytesVal)(nil)
)
