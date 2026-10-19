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
func Usage(kd codes.Code, bicimler ...string) string {
	return kullanimAdla(CallName(kd), bicimler)
}

func kullanimAdla(ad string, bicimler []string) string {
	parcalar := make([]string, len(bicimler))
	for i, b := range bicimler {
		parcalar[i] = ad + "(" + b + ")"
	}
	return strings.Join(parcalar, " ya da ")
}

// TypeNames writes the names of type codes as a list of alternatives:
// TypeNames(codes.A200, codes.A201, codes.A202) → "Tam, Ondalık ya da Metin".
func TypeNames(tipler ...codes.Code) string {
	switch len(tipler) {
	case 0:
		return ""
	case 1:
		return codes.Name(tipler[0])
	}
	adlar := codes.Names(tipler...)
	return strings.Join(adlar[:len(adlar)-1], ", ") + " ya da " + adlar[len(adlar)-1]
}

// ArgTypeError reports that argument number sira of builtin kd (or its member) is not one of
// the expected types; the expected values are type codes (A2xx).
func ArgTypeError(kd codes.Code, sira int, verilen Value, beklenen ...codes.Code) error {
	return argumanTipHatasiAdla(CallName(kd), sira, TypeNames(beklenen...), verilen)
}

func argumanTipHatasiAdla(ad string, sira int, beklenen string, verilen Value) error {
	return fmt.Errorf("%s() %d. argüman olarak %s bekler, %s verildi", ad, sira, beklenen, TypeAndSummary(verilen))
}

// ArgCountError reports that builtin kd (or its member) was called with the wrong number of
// arguments. beklenen is a count phrase such as "1", "en az 2", "1 ya da 2"; when kullanim
// is given, the valid call forms (parameter lists, see Usage) are appended.
func ArgCountError(kd codes.Code, beklenen string, verilen int, kullanim ...string) error {
	ad := CallName(kd)
	k := ""
	if len(kullanim) > 0 {
		k = kullanimAdla(ad, kullanim)
	}
	return argumanSayisiHatasiAdla(ad, beklenen, verilen, k)
}

func argumanSayisiHatasiAdla(ad, beklenen string, verilen int, kullanim string) error {
	if kullanim != "" {
		return fmt.Errorf("%s() %s argüman bekler, %d verildi (kullanım: %s)", ad, beklenen, verilen, kullanim)
	}
	return fmt.Errorf("%s() %s argüman bekler, %d verildi", ad, beklenen, verilen)
}

// TypeAndSummary writes a value for an error message as "Metin ("a")".
func TypeAndSummary(v Value) string {
	if v == nil {
		return "hiçbir değer"
	}
	return fmt.Sprintf("%s (%s)", v.Type(), ValueSummary(v))
}

// ozetSiniri hata mesajlarındaki değer özetinin en fazla karakter sayısıdır.
const ozetSiniri = 40

// ValueSummary writes a value short enough for an error message: strings in quotes, long
// values cut with "…". Large lists are never printed in full.
func ValueSummary(v Value) string {
	var s string
	switch x := v.(type) {
	case nil:
		return codes.Name(codes.A042)
	case *StringVal:
		return strconv.Quote(kisalt(x.Val, ozetSiniri))
	case *ListVal:
		if len(x.Elements) > 5 {
			parcalar := make([]string, 0, 4)
			for _, e := range x.Elements[:3] {
				parcalar = append(parcalar, ValueSummary(e))
			}
			return kisalt("["+strings.Join(parcalar, ", ")+", …]", ozetSiniri) +
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
	return kisalt(s, ozetSiniri)
}

// kisalt metni en fazla n karaktere (rune) indirir; kısaltılınca sonuna "…" ekler.
func kisalt(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}
