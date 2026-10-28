package interp

import (
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// MessageVal is a catalog message built in TR code with ileti(H6903, values): its code and the
// values in the order of the catalog definition. It has no text of its own; the text is built
// from the catalog when it is shown, in the language it is shown in (a program's text of it
// follows its file's language, a displayed error the person's). fırlat and doğrula throw it as
// the message itself, so the library's errors are catalog messages in every language.
type MessageVal struct {
	Code messages.Code
	Args []Value
}

// ValMessage is the type name of MessageVal.
var ValMessage = TypeName(codes.TypeMessage)

var _ LanguageText = (*MessageVal)(nil)

func (m *MessageVal) Type() ValueType { return ValMessage }
func (m *MessageVal) IsTruthy() bool  { return true }

// Inspect is the value's text (3.14): the message in the fixed catalog of values' text
// (messages.ValueLanguage), like a caught error; shown to a person it is in their language.
func (m *MessageVal) Inspect() string { return m.Text(codes.ValueNames()) }

// Text is the message in lang.
func (m *MessageVal) Text(lang *codes.Language) string {
	return messages.TextIn(lang.Code, m.Code, m.params()...)
}

// Error is the message as an error: the catalog code with its values.
func (m *MessageVal) Error() *messages.Error { return messages.New(m.Code, m.params()...) }

// params are the values as message parameters; each is written in the message's language.
func (m *MessageVal) params() []any {
	out := make([]any, len(m.Args))
	for i, a := range m.Args {
		out[i] = Shown(a)
	}
	return out
}

// registerMessageBuiltins registers ileti and hata_kodu.
func (v *VM) registerMessageBuiltins() {
	// ileti(kod, değerler...): katalog iletisini kurar; kod tanımlı olmalı, değer sayısı tanımla aynı.
	v.RegisterNoNamed(codes.FnMessage, func(y *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) < 1 {
			return nil, ArgCountError(codes.FnMessage, AtLeast(1), len(args))
		}
		text, ok := args[0].(*StringVal)
		if !ok {
			return nil, messages.New(messages.H2528, codes.NameRef(codes.FnMessage), Shown(args[0]))
		}
		code, err := messages.Parse(text.Val)
		if err != nil {
			return nil, messages.New(messages.H2528, codes.NameRef(codes.FnMessage), text.Val)
		}
		n, defined := messages.ParamCount(code)
		if !defined {
			return nil, messages.New(messages.H2528, codes.NameRef(codes.FnMessage), text.Val)
		}
		if len(args)-1 != n {
			return nil, messages.New(messages.H2529, codes.NameRef(codes.FnMessage), text.Val, n, len(args)-1)
		}
		return &MessageVal{Code: code, Args: append([]Value(nil), args[1:]...)}, nil
	})
	v.RegisterTypeCall(codes.TypeMessage, v.Builtin(codes.FnMessage))

	// hata_kodu(f): f'yi çalıştırır; hatanın en içteki nedeninin kodunu (Metin) verir, hata
	// yoksa yok.
	v.RegisterNoNamed(codes.FnErrorCode, func(y *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgCountError(codes.FnErrorCode, 1, len(args))
		}
		_, err := y.ApplyFunction(args[0], nil, nil, 0)
		if err == nil {
			return Null, nil
		}
		if coded, ok := messages.Root(err); ok {
			return &StringVal{Val: coded.Code.String()}, nil
		}
		return nil, err
	})
}
