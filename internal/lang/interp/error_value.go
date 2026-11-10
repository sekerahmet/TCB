package interp

import (
	"errors"
	"fmt"
	"slices"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ErrorVal is TR_Error (8.3): the error a TR_catch binds. It carries the thrown value (a text, a
// TR_Message or a value of a type with variants; a run-time error's message), the error it was
// built from (TR_cause), the errors deferred work gave while it left a scope (TR_suppressed, 9.4)
// and the error as it was raised: TR_throw e raises that again, with its code, value, cause,
// suppressed errors and the place it was first thrown.
type ErrorVal struct {
	thrown     Value
	cause      *ErrorVal
	suppressed []*ErrorVal
	err        error
}

// ValError is the type name of ErrorVal.
var ValError = TypeName(codes.TypeError)

var _ PropertyGetter = (*ErrorVal)(nil)

func (e *ErrorVal) Type() ValueType { return ValError }
func (e *ErrorVal) IsTruthy() bool  { return true }

// Inspect is the error's text: the text of the thrown value (3.14, 8.3), the same for every
// reader; the reader's language is used only for a shown error (8.5, 8.6).
func (e *ErrorVal) Inspect() string { return writeItem(e.thrown, nil, nil) }

// code is TR_code: the message's code, or none for a plain text or a value of a type with variants.
func (e *ErrorVal) code() Value {
	if m, ok := e.thrown.(*MessageVal); ok {
		return &StringVal{Val: m.Code.String()}
	}
	return Null
}

// GetProperty gives the members of 8.3: TR_code, TR_value, TR_cause, TR_suppressed and the method
// TR_text.
func (e *ErrorVal) GetProperty(kd codes.Code, name string) (Value, error) {
	switch kd {
	case codes.MemCode:
		return e.code(), nil
	case codes.MemValue:
		return e.thrown, nil
	case codes.MemCause:
		if e.cause == nil {
			return Null, nil
		}
		return e.cause, nil
	case codes.MemSuppressed:
		list := make([]Value, len(e.suppressed))
		for i, s := range e.suppressed {
			list[i] = s
		}
		return &ListVal{Elements: list}, nil
	case codes.MemText:
		return textMethod(codes.TypeError, func(lang string) string {
			if m, ok := e.thrown.(*MessageVal); ok {
				return m.textIn(lang)
			}
			return e.Inspect() // düz metin ve durumlu tip değeri: TR_String(e)
		}), nil
	}
	return nil, messages.New(messages.H2407, ValError, name)
}

// textMethod is .TR_text("tr") of an error or a message (8.3, 8.5): the readable text in the language
// whose code is given; a language without a message catalog is H2550.
func textMethod(owner codes.Code, text func(lang string) string) *BuiltinVal {
	fullName := codes.MemberRef(owner, codes.MemText)
	return &BuiltinVal{Name: codes.Name(codes.MemText), NoNamedArgs: true, Fn: func(_ *VM, args []Value, _ map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, messages.New(messages.H2551, len(args))
		}
		lang, ok := args[0].(*StringVal)
		if !ok {
			return nil, argTypeErrorByName(fullName, 1, TypeNames(codes.TypeString), args[0])
		}
		langs := messages.CatalogLanguages()
		if !slices.Contains(langs, lang.Val) {
			list := make([]Value, len(langs))
			for i, l := range langs {
				list[i] = &StringVal{Val: l}
			}
			return nil, messages.New(messages.H2550, lang.Val, Shown(&ListVal{Elements: list}))
		}
		return &StringVal{Val: text(lang.Val)}, nil
	}}
}

// throwable reports whether v may be thrown (8.2): a text, a message, a value of a type with
// variants (7.7) or an error.
func throwable(v Value) bool {
	switch x := v.(type) {
	case *StringVal, *MessageVal, *ErrorVal:
		return true
	case *StructInstanceVal:
		return x.Variant != nil
	}
	return false
}

// raise is the Go error of throwing e at line: e.err if e was raised before (TR_throw e raises it
// again as it was, 8.2), otherwise a new thrown error that carries e.
func (e *ErrorVal) raise(line int) error {
	if e.err != nil {
		return e.err
	}
	thrown := &thrownError{caught: e}
	if m, ok := e.thrown.(*MessageVal); ok {
		thrown.value = m.Error() // katalog iletisi kendi koduyla
	} else {
		thrown.text = writeItem(e.thrown, nil, nil) // düz metin, durumlu tip değeri: kodsuz (8.6)
	}
	e.err = messages.At(line, 0, thrown)
	return e.err
}

// errorValue is the TR_Error a TR_catch binds for err (8.3): a thrown error gives the value that
// was thrown; a run-time error (and a failed TR_assert) gives its message, with the code of its
// innermost cause (messages.Root, the code TR_error_code gives, 8.5); errors of deferred work are
// the error's TR_suppressed (9.4).
func errorValue(err error) *ErrorVal {
	var deferred *deferError
	if errors.As(err, &deferred) {
		e := errorValue(deferred.mainSrc)
		ev := &ErrorVal{thrown: e.thrown, cause: e.cause, suppressed: append([]*ErrorVal(nil), e.suppressed...), err: err}
		for _, x := range deferred.extras {
			ev.suppressed = append(ev.suppressed, suppressedOf(x)...)
		}
		return ev
	}
	var thrown *thrownError
	if errors.As(err, &thrown) && thrown.caught != nil {
		return thrown.caught
	}
	if m, ok := messages.Root(err); ok {
		return &ErrorVal{thrown: messageValue(m), err: err}
	}
	return &ErrorVal{thrown: messageValue(messages.Internal(err.Error())), err: err}
}

// suppressedOf is the errors of deferred work in run order: a scope's deferred work joins its own
// errors into one (Environment.RunDefers), which is opened here.
func suppressedOf(err error) []*ErrorVal {
	d, ok := err.(*deferError)
	if !ok {
		return []*ErrorVal{errorValue(err)}
	}
	list := suppressedOf(d.mainSrc)
	for _, x := range d.extras {
		list = append(list, suppressedOf(x)...)
	}
	return list
}

// messageValue is the TR_Message of a coded Go error: its code and its values as TR values. The
// message keeps the error, so throwing it again shows the same text.
func messageValue(m *messages.Error) *MessageVal {
	args := make([]Value, len(m.Args))
	for i, a := range m.Args {
		args[i] = argValue(a)
	}
	return &MessageVal{Code: m.Code, Args: args, source: m}
}

// argValue is a message parameter as a TR value: values as they are, numbers and texts as numbers
// and texts, a coded error as its message, a name as its identity (the same in every language).
func argValue(a any) Value {
	switch x := a.(type) {
	case nil:
		return Null
	case *messages.Error:
		if x == nil {
			return Null
		}
		return messageValue(x)
	case Value:
		return x
	case shown:
		return x.v
	case shownList:
		return x.v
	case ValueType:
		if t := typeValueByName[x]; t != nil {
			return &StringVal{Val: t.Inspect()}
		}
		return &StringVal{Val: string(x)}
	case string:
		return &StringVal{Val: x}
	case int:
		return NewInt(int64(x))
	case int64:
		return NewInt(x)
	case float64:
		return &FloatVal{Val: x}
	case bool:
		return BoolToVal(x)
	case codes.Code:
		return &StringVal{Val: codes.Spelling(x)}
	case codes.Ref:
		return &StringVal{Val: codes.Spelling(x.Code)}
	case error:
		if m, ok := messages.As(x); ok && m != nil {
			return messageValue(m)
		}
		return &StringVal{Val: x.Error()}
	case messages.Localized:
		return &StringVal{Val: x.MessageText(messages.ValueLanguage)}
	}
	return &StringVal{Val: fmt.Sprint(a)}
}

// registerErrorType registers the call of TR_Error (8.2): TR_Error(v, TR_cause: e) builds a new
// error whose code, text and value are v's and whose cause is e.
func (v *VM) registerErrorType() {
	v.RegisterTypeCall(codes.TypeError, &BuiltinVal{Fn: func(y *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, ArgCountError(codes.TypeError, 1, len(args))
		}
		if !throwable(args[0]) {
			return nil, messages.New(messages.H2911, codes.NameRef(codes.TypeError), TypeAndSummary(args[0]), "")
		}
		e := &ErrorVal{thrown: args[0]}
		if inner, ok := args[0].(*ErrorVal); ok {
			e.thrown = inner.thrown
		}
		for name, val := range named {
			if !isSpelling(codes.ArgCause, name) {
				return nil, messages.New(messages.H2410, codes.NameRef(codes.TypeError), name, "")
			}
			switch c := val.(type) {
			case *ErrorVal:
				e.cause = c
			case *NullVal:
			default:
				return nil, messages.New(messages.H2912, TypeAndSummary(val))
			}
		}
		return e, nil
	}})
}

// isSpelling reports whether name is an accepted spelling of k.
func isSpelling(k codes.Code, name string) bool {
	for _, s := range codes.Spellings(k) {
		if s == name {
			return true
		}
	}
	return false
}
