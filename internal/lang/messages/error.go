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
	cat, ok := catalogs[lang]
	if !ok || !cat.translates(c, args) {
		cat = catalogs["tr"]
	}
	return cat.render(c, args)
}

func (e *Error) Error() string { return Text(e.Code, e.Args...) }

// errorText hatanın metnidir (nil için boş).
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Unwrap returns the errors among the parameter values, so errors.Is and errors.As see the
// wrapped cause as they did with fmt.Errorf's %w.
func (e *Error) Unwrap() []error {
	var out []error
	for _, a := range e.Args {
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
