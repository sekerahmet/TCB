package interp

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// MessageVal is a catalog message built in TR code with TR_Message(code, values): its code and the
// values in the order of the catalog definition. Its text is the call that builds it (3.14):
// Message("H6903", -1), the same for every reader; the message's text in the reader's language is
// built only when the error is shown (8.5, 8.6). TR_throw and TR_assert throw it as the message
// itself, so the library's errors are catalog messages in every language.
type MessageVal struct {
	Code messages.Code
	Args []Value
	// source is the run-time error the message was taken from (a caught error's TR_value, 8.3):
	// throwing it again shows that error's text.
	source *messages.Error
}

// ValMessage is the type name of MessageVal.
var ValMessage = TypeName(codes.TypeMessage)

func (m *MessageVal) Type() ValueType { return ValMessage }
func (m *MessageVal) IsTruthy() bool  { return true }

// Inspect is the value's text (3.14): the call that builds it, with the type's name;
// `TR_Message("H6903", -1)` is written Message("H6903", -1).
func (m *MessageVal) Inspect() string {
	var sb strings.Builder
	sb.WriteString(codes.ValueNames().Name(codes.TypeMessage))
	sb.WriteByte('(')
	sb.WriteString(QuoteText(m.Code.String()))
	for _, a := range m.Args {
		sb.WriteString(", ")
		sb.WriteString(writeElement(a, nil, nil))
	}
	sb.WriteByte(')')
	return sb.String()
}

// Error is the message as an error: the catalog code with its values.
func (m *MessageVal) Error() *messages.Error {
	if m.source != nil {
		return m.source
	}
	return messages.New(m.Code, m.params()...)
}

// Count is the value as a {name, plural, ...} number; nil when it is not a number.
func (s shown) Count() any {
	switch n := s.v.(type) {
	case *IntVal:
		return n.Val
	case *FloatVal:
		return n.Val
	}
	return nil
}

// params are the values as message parameters; each is written in the message's language.
func (m *MessageVal) params() []any {
	out := make([]any, len(m.Args))
	for i, a := range m.Args {
		out[i] = Shown(a)
	}
	return out
}

// registerMessageBuiltins registers the call of İleti and hata_kodu.
func (v *VM) registerMessageBuiltins() {
	// İleti(kod, değerler...): katalog iletisini kurar; kod tanımlı olmalı, değer sayısı tanımla aynı.
	v.RegisterTypeCall(codes.TypeMessage, &BuiltinVal{NoNamedArgs: true, Fn: func(y *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) < 1 {
			return nil, ArgCountError(codes.TypeMessage, AtLeast(1), len(args))
		}
		text, ok := args[0].(*StringVal)
		if !ok {
			return nil, messages.New(messages.H2528, codes.NameRef(codes.TypeMessage), Shown(args[0]))
		}
		code, err := messages.Parse(text.Val)
		if err != nil {
			return nil, messages.New(messages.H2528, codes.NameRef(codes.TypeMessage), text.Val)
		}
		n, defined := messages.ParamCount(code)
		if !defined {
			return nil, messages.New(messages.H2528, codes.NameRef(codes.TypeMessage), text.Val)
		}
		if len(args)-1 != n {
			return nil, messages.New(messages.H2529, codes.NameRef(codes.TypeMessage), text.Val, n, len(args)-1)
		}
		return &MessageVal{Code: code, Args: append([]Value(nil), args[1:]...)}, nil
	}})

	// hata_kodu(f): f'yi çalıştırır; hatanın kodunu (TR_code, Metin) verir, hata yoksa yok. Hatanın
	// kodu yoksa nedenler zincirinde dıştan içe ilk kodu (8.5); zincirde de kod yoksa (düz metin,
	// durumlu tip değeri) hata koda dönmez, olduğu gibi yeniden fırlatılır. Durdurma ve TR_exit hata
	// değildir (1.5, 1.10), geçer.
	v.RegisterNoNamed(codes.FnErrorCode, func(y *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgCountError(codes.FnErrorCode, 1, len(args))
		}
		_, err := y.ApplyFunction(args[0], nil, nil, 0)
		if err == nil {
			return Null, nil
		}
		if _, coded := messages.As(err); !coded || uncatchable(err) {
			return nil, err
		}
		for e := errorValue(err); e != nil; e = e.cause {
			if code := e.code(); code != Null {
				return code, nil
			}
		}
		return nil, err
	})
}
