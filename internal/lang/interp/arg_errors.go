package interp

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
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
	pieces := make([]string, len(forms))
	for i, b := range forms {
		pieces[i] = name + "(" + b + ")"
	}
	return strings.Join(pieces, " ya da ")
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
	return fmt.Errorf("%s() %d. argüman olarak %s bekler, %s verildi", name, order, expected, TypeAndSummary(given))
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
		return fmt.Errorf("%s() %s argüman bekler, %d verildi (kullanım: %s)", name, expected, given, usage)
	}
	return fmt.Errorf("%s() %s argüman bekler, %d verildi", name, expected, given)
}

// TypeAndSummary writes a value for an error message as "Metin ("a")".
func TypeAndSummary(v Value) string {
	if v == nil {
		return "hiçbir değer"
	}
	return fmt.Sprintf("%s (%s)", v.Type(), ValueSummary(v))
}

// ozetSiniri hata mesajlarındaki değer özetinin en fazla karakter sayısıdır.
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
				fmt.Sprintf(", %d eleman", len(x.Elements))
		}
		s = x.Inspect()
	case *DictVal:
		if len(x.Pairs) > 5 {
			return fmt.Sprintf("%d anahtarlı sözlük", len(x.Pairs))
		}
		s = x.Inspect()
	default:
		s = v.Inspect()
	}
	return shorten(s, summaryLimit)
}

// kisalt metni en fazla n karaktere (rune) indirir; kısaltılınca sonuna "…" ekler.
func shorten(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}
