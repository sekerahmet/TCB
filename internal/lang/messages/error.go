package messages

import (
	"errors"
	"fmt"
)

// Error is a coded message used as an error. Args are the parameter values in the order of
// the code's definition; the text is built in the active language when Error is called.
type Error struct {
	Code Code
	Args []any
}

// Localized is a parameter value whose text depends on the language of the message it is put
// in: a TR value, a type name. The catalog asks for it in the language it writes the message
// in (TR_DIL, or the source language when the message falls back), so a message and the values
// inside it are always in one language. Like Fluent's FluentType.toString(scope); messages does
// not know the values (interp implements it).
type Localized interface {
	MessageText(lang string) string
}

// New returns the message code with its parameter values as an error.
func New(c Code, args ...any) *Error { return &Error{Code: c, Args: args} }

// Text returns the message text of code c with its parameter values, for output that is not
// an error (warnings, notes). The whole text is in one language: if the active language lacks
// the code or any coded error among the values, it is all in the source language.
func Text(c Code, args ...any) string { return catalogFor(c, args).render(c, args) }

// TextIn is the message's text in the given language instead of the process's (the source
// language when that language has no text for it). A value's text follows the language of the
// file the program is written in, not TR_DIL (docs/spec/03_tipler.md): '<function f>' in an
// English file wherever it runs.
func TextIn(lang string, c Code, args ...any) string {
	load()
	cat, ok := catalogs[catalogLanguage(lang)]
	if !ok || !cat.translates(c, args) {
		cat = catalogs["tr"]
	}
	return cat.render(c, args)
}

func (e *Error) Error() string {
	if e == nil {
		return "" // yok olan ileti parçası (ör. öneri yoksa) boş yazılır
	}
	return Text(e.Code, e.Args...)
}

// errorText hatanın metnidir (nil için boş).
// ErrorIn is err's text in the given language instead of the process's: a coded error (Error,
// PosError, an error that is Localized) is written whole by that language's catalog (the
// source language when it lacks a text), so the error and the values inside it are in one
// language. Other errors keep their Error(). A caught error becomes the program's value in the
// language of the file that catches it (docs/spec/08_hata_yonetimi.md).
func ErrorIn(err error, lang string) string {
	load()
	cat, ok := catalogs[catalogLanguage(lang)]
	if !ok || !cat.translatesValue(err) {
		cat = catalogs["tr"]
	}
	if text, ok := cat.codedText(err); ok {
		return text
	}
	return err.Error()
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Unwrap returns the errors among the parameter values, so errors.Is and errors.As see the
// wrapped cause as they did with fmt.Errorf's %w.
func (e *Error) Unwrap() []error {
	if e == nil {
		return nil
	}
	var out []error
	for _, a := range e.Args {
		if part, ok := a.(*Error); ok && part == nil {
			continue // yok olan ileti parçası sarılmış bir hata değildir
		}
		if err, ok := a.(error); ok {
			out = append(out, err)
		}
	}
	return out
}

// Has reports whether err or any error it wraps is the message c. Tests check codes with it
// instead of message texts, so rewording a message does not break them.
func Has(err error, c Code) bool {
	if err == nil {
		return false
	}
	if m, ok := err.(*Error); ok && m.Code == c {
		return true
	}
	switch u := err.(type) {
	case interface{ Unwrap() error }:
		return Has(u.Unwrap(), c)
	case interface{ Unwrap() []error }:
		for _, e := range u.Unwrap() {
			if Has(e, c) {
				return true
			}
		}
	}
	return false
}

// Internal reports a state that only a bug in TR itself can reach. The user sees one shared
// message asking them to report it; detail is for developers and is written in English.
func Internal(detail string) *Error { return New(H9000, detail) }

// Internalf is Internal with a formatted English detail.
func Internalf(form string, a ...any) *Error { return Internal(fmt.Sprintf(form, a...)) }

// Texts returns the rendered text of every error, in order.
func Texts(errs []error) []string {
	out := make([]string, len(errs))
	for i, e := range errs {
		out[i] = e.Error()
	}
	return out
}

// HasAny reports whether err's chain carries any of the codes.
func HasAny(err error, codes ...Code) bool {
	for _, c := range codes {
		if Has(err, c) {
			return true
		}
	}
	return false
}

// As returns the first coded message in err's chain.
func As(err error) (*Error, bool) {
	var m *Error
	ok := errors.As(err, &m)
	return m, ok
}

// Root returns the message of err's innermost cause (the code TR_error_code gives): starting
// from the first coded message it descends only through cause slots (IsCauseSlot: "tensor
// operation failed: {reason}"), not into the message's parts. Precedent: Rust's Error::source()
// chain and anyhow's root_cause ("the lowest level cause"); Go's errors.As gives the outermost.
func Root(err error) (*Error, bool) {
	m, ok := As(err)
	for ok {
		inner, found := causeOf(m)
		if !found {
			break
		}
		m = inner
	}
	return m, ok
}

// causeOf is the first coded message in m's cause slots.
func causeOf(m *Error) (*Error, bool) {
	for i, a := range m.Args {
		if e, isErr := a.(error); isErr && IsCauseSlot(m.Code, i) {
			if inner, found := As(e); found {
				return inner, true
			}
		}
	}
	return nil, false
}
