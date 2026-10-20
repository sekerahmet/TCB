package interp

import (
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
// builtins (aralık), with the receiver for members (liste.filtrele; see codes.MemberName).
func CallName(kd codes.Code) string {
	if kd.Namespace() == codes.NamespaceMember {
		return codes.MemberName(kd)
	}
	return codes.Name(kd)
}

// CallPrefix is the error-message prefix of a code: "aralık()", "liste.filtrele()".
func CallPrefix(kd codes.Code) string { return CallName(kd) + "()" }

// Usage writes the valid call forms of a code; each form is a parameter list:
// Usage(codes.G305, "bitiş", "başlangıç, bitiş") → "aralık(bitiş) ya da aralık(başlangıç, bitiş)".
func Usage(kd codes.Code, forms ...string) string {
	return usageByName(CallName(kd), forms)
}

func usageByName(name string, forms []string) string {
	s := ""
	for i, b := range forms {
		piece := name + "(" + b + ")"
		if i == 0 {
			s = piece
			continue
		}
		s = messages.Text(messages.H2010, s, piece) // "a ya da b"
	}
	return s
}

// TypeNames writes the names of type codes as a list of alternatives:
// TypeNames(codes.A200, codes.A201, codes.A202) → "Tam, Ondalık ya da Metin".
func TypeNames(typesOf ...codes.Code) string {
	switch len(typesOf) {
	case 0:
		return ""
	case 1:
		return codes.Name(typesOf[0])
	}
	nameList := codes.Names(typesOf...)
	return strings.Join(nameList[:len(nameList)-1], ", ") + " ya da " + nameList[len(nameList)-1]
}

// ArgTypeError reports that argument number sira of builtin kd (or its member) is not one of
// the expected types; the expected values are type codes (A2xx).
func ArgTypeError(kd codes.Code, order int, given Value, expected ...codes.Code) error {
	return argTypeErrorByName(CallName(kd), order, TypeNames(expected...), given)
}

func argTypeErrorByName(name string, order int, expected string, given Value) error {
	return messages.New(messages.H2423, name, order, expected, TypeAndSummary(given))
}

// ArgCountError reports that builtin kd (or its member) was called with the wrong number of
// arguments. beklenen is a count phrase such as "1", "en az 2", "1 ya da 2"; when kullanim
// is given, the valid call forms (parameter lists, see Usage) are appended.
func ArgCountError(kd codes.Code, expected string, given int, usage ...string) error {
	name := CallName(kd)
	k := ""
	if len(usage) > 0 {
		k = usageByName(name, usage)
	}
	return argCountErrorByName(name, expected, given, k)
}

func argCountErrorByName(name, expected string, given int, usage string) error {
	if usage != "" {
		return messages.New(messages.H2011, name, expected, given, usage)
	}
	return messages.New(messages.H2012, name, expected, given)
}

// Counts writes the accepted argument counts for ArgCountError in the active language:
// Counts(1, 2) → "1 ya da 2", Counts(0, 1, 2) → "0, 1 ya da 2".
func Counts(ns ...int) string {
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = strconv.Itoa(n)
	}
	if len(parts) < 2 {
		return strings.Join(parts, "")
	}
	return messages.Text(messages.H2010, strings.Join(parts[:len(parts)-1], ", "), parts[len(parts)-1])
}

// AtLeast writes "en az n" for ArgCountError in the active language.
func AtLeast(n int) string { return messages.Text(messages.H2016, n) }

// TypeAndSummary writes a value for an error message as "Metin ("a")".
func TypeAndSummary(v Value) string {
	if v == nil {
		return messages.Text(messages.H2013)
	}
	return messages.Text(messages.H2424, v.Type(), ValueSummary(v))
}

// summaryLimit hata mesajlarındaki değer özetinin en fazla karakter sayısıdır.
const summaryLimit = 40

// ValueSummary writes a value short enough for an error message: strings in quotes, long
// values cut with "…". Large lists are never printed in full.
func ValueSummary(v Value) string {
	var s string
	switch x := v.(type) {
	case nil:
		return codes.Name(codes.A042)
	case *StringVal:
		return strconv.Quote(shorten(x.Val, summaryLimit))
	case *ListVal:
		if len(x.Elements) > 5 {
			pieces := make([]string, 0, 4)
			for _, e := range x.Elements[:3] {
				pieces = append(pieces, ValueSummary(e))
			}
			return shorten("["+strings.Join(pieces, ", ")+", …]", summaryLimit) +
				messages.Text(messages.H2425, len(x.Elements))
		}
		s = x.Inspect()
	case *DictVal:
		if len(x.Pairs) > 5 {
			return messages.Text(messages.H2426, len(x.Pairs))
		}
		s = x.Inspect()
	default:
		s = v.Inspect()
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
