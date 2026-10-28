package interp

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Yerleşik fonksiyonların argüman hataları tek biçimdedir; motor paketlerindeki
// yerleşikler de bu işlevleri kullanmalıdır. Dilin adları (yerleşik, metot, tip adları)
// mesaja koddan girer; Go metninde yazılmaz:
//
//	aralık() 1. argüman olarak Tam bekler, Metin ("a") verildi
//	aralık() 1 ya da 2 argüman bekler, 0 verildi (kullanım: aralık(bitiş) ya da aralık(başlangıç, bitiş))

// CallName is the name of a code as written in error messages: the name itself for
// builtins (aralık), with the receiver for members (liste.filtrele; see codes.CallNameRef).
// It is a message parameter (codes.Ref): the catalog writes it in the message's language.
func CallName(kd codes.Code) codes.Ref { return codes.CallNameRef(kd) }

// CallPrefix is the error-message prefix of a code: "aralık()", "liste.filtrele()" (codes.Ref).
func CallPrefix(kd codes.Code) codes.Ref { return codes.CallRef(kd) }

// Usage writes the valid call forms of a code; each form is a parameter list:
// Usage(codes.FnRound, "x", "x, basamak") → "yuvarla(x) ya da yuvarla(x, basamak)".
// A form that names a language element is a message part (codes.ReceiverRef(codes.TypeString)),
// written in the message's language like the name.
func Usage(kd codes.Code, forms ...any) messages.Localized {
	return usageByName(CallName(kd), forms)
}

func usageByName(name codes.Ref, forms []any) messages.Localized { return usage{name, forms} }

// usage geçerli çağrı biçimleridir, ileti parametresi olarak: iletinin dilinde yazılır
// ("aralık(bitiş) ya da aralık(başlangıç, bitiş)").
type usage struct {
	name  codes.Ref
	forms []any
}

func (u usage) MessageText(lang string) string {
	name := u.name.MessageText(lang)
	pieces := make([]string, len(u.forms))
	for i, b := range u.forms {
		form := fmt.Sprint(b)
		if l, ok := b.(messages.Localized); ok {
			form = l.MessageText(lang)
		}
		pieces[i] = name + "(" + form + ")"
	}
	return messages.OrListIn(lang, pieces...)
}

// TypeNames is the names of type codes as a list of alternatives, as a message parameter: it is
// written in the language of its message (messages.Localized), TypeNames(codes.TypeInt,
// codes.TypeFloat, codes.TypeString) → "Tam, Ondalık ya da Metin" / "Int, Float or String".
func TypeNames(typesOf ...codes.Code) messages.Localized { return typeNames(typesOf) }

type typeNames []codes.Code

func (t typeNames) MessageText(lang string) string {
	l := languageNamed(lang)
	names := make([]string, len(t))
	for i, k := range t {
		names[i] = l.Name(k)
	}
	return messages.OrListIn(l.Code, names...)
}

// languageNamed iletinin dilinin kod listesidir; bilinmeyen dilde bugün Türkçe (codes.Source; geçici istisna).
func languageNamed(lang string) *codes.Language {
	if l, err := codes.LoadLanguage(lang); err == nil {
		return l
	}
	return codes.Source()
}

// ArgTypeError reports that argument number sira of builtin kd (or its member) is not one of
// the expected types; the expected values are type codes (type).
func ArgTypeError(kd codes.Code, order int, given Value, expected ...codes.Code) error {
	if j, ok := given.(*JSONVal); ok && len(expected) > 0 {
		return jsonArgError(CallName(kd), order, j, expected)
	}
	return argTypeErrorByName(CallName(kd), order, TypeNames(expected...), given)
}

func argTypeErrorByName(name any, order int, expected any, given Value) error {
	return messages.New(messages.H2423, name, order, expected, TypeAndSummary(given))
}

// ArgCountError reports that builtin kd (or its member) was called with the wrong number of
// arguments. expected is the count as a number (1; the plural follows it: "takes 1 argument") or
// a count phrase written in the message's language (Counts(1, 2), AtLeast(2)); when usage is
// given, the valid call forms (parameter lists, see Usage) are appended.
func ArgCountError(kd codes.Code, expected any, given int, usage ...any) error {
	name := CallName(kd)
	if len(usage) > 0 {
		return messages.New(messages.H2011, name, expected, given, usageByName(name, usage))
	}
	return argCountErrorByName(name, expected, given)
}

func argCountErrorByName(name any, expected any, given int) error {
	return messages.New(messages.H2012, name, expected, given)
}

// Counts are the accepted argument counts for ArgCountError, a message part written in the
// message's language: Counts(1, 2) → "1 ya da 2" (en "1 or 2"), Counts(0, 1, 2) → "0, 1 ya da 2".
func Counts(ns ...int) messages.Localized { return counts(ns) }

type counts []int

func (c counts) MessageText(lang string) string {
	parts := make([]string, len(c))
	for i, n := range c {
		parts[i] = strconv.Itoa(n)
	}
	return messages.OrListIn(lang, parts...)
}

// AtLeast is "en az n" (en "n or more") for ArgCountError, a message part.
func AtLeast(n int) *messages.Error { return messages.New(messages.H2016, n) }

// TypeAndSummary is a value for an error message, "Metin ("a")": a message parameter written in
// the language of the message it goes into (the error's display, or the language of the file
// that catches it).
func TypeAndSummary(v Value) *messages.Error {
	if v == nil {
		return messages.New(messages.H2013)
	}
	return messages.New(messages.H2424, v.Type(), valueSummary{v})
}

// valueSummary bir değerin ileti için özetidir; iletinin dilinde yazılır (messages.Localized).
type valueSummary struct{ v Value }

func (s valueSummary) MessageText(lang string) string { return summaryIn(s.v, languageNamed(lang)) }

// summaryLimit hata mesajlarındaki değer özetinin en fazla karakter sayısıdır.
const summaryLimit = 40

// summaryIn writes a value short enough for an error message, in l: strings in quotes, long
// values cut with "…". Large lists are never printed in full.
func summaryIn(v Value, l *codes.Language) string {
	var s string
	switch x := v.(type) {
	case nil:
		return l.Name(codes.KwNone)
	case *StringVal:
		return strconv.Quote(shorten(x.Val, summaryLimit))
	case *ListVal:
		if len(x.Elements) > 5 {
			pieces := make([]string, 0, 4)
			for _, e := range x.Elements[:3] {
				pieces = append(pieces, summaryIn(e, l))
			}
			return messages.TextIn(l.Code, messages.H2425, shorten("["+strings.Join(pieces, ", ")+", …]", summaryLimit), len(x.Elements))
		}
		s = valueText(x, l)
	case *DictVal:
		if len(x.Pairs) > 5 {
			return messages.TextIn(l.Code, messages.H2426, len(x.Pairs))
		}
		s = valueText(x, l)
	default:
		s = valueText(v, l)
	}
	return shorten(s, summaryLimit)
}

// shorten metni en fazla n karaktere (rune) indirir; kısaltılınca sonuna "…" ekler.
func shorten(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}
