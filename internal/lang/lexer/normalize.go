package lexer

import (
	"slices"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Form is a Unicode normalization form (UAX #15); its value is the form's name.
type Form string

const (
	FormNFC  Form = "NFC"
	FormNFD  Form = "NFD"
	FormNFKC Form = "NFKC"
	FormNFKD Form = "NFKD"
)

// Forms are the four forms of UAX #15.
var Forms = []Form{FormNFC, FormNFD, FormNFKC, FormNFKD}

// formTable bir biçimin x/text'teki karşılığıdır: kendisi, yalnız ayıran yarısı ve birleştirip
// birleştirmediği.
type formTable struct {
	form, decomposed norm.Form
	composes         bool
}

var formTables = map[Form]formTable{
	FormNFC:  {norm.NFC, norm.NFD, true},
	FormNFD:  {norm.NFD, norm.NFD, false},
	FormNFKC: {norm.NFKC, norm.NFKD, true},
	FormNFKD: {norm.NFKD, norm.NFKD, false},
}

// Normalize returns s in the form f, one of Forms (language rules 2.3 and 13.2). The result is the
// one the standard gives: x/text alone differs from it in three ways (see faithful), and then the
// form is built from the tables of x/text, character by character. Invalid UTF-8 is left to x/text.
func Normalize(f Form, s string) string {
	t := formTables[f]
	out := t.form.String(s)
	if t.faithful(s, out) || !utf8.ValidString(s) {
		return out
	}
	return t.exact(s)
}

// faithful x/text'in sonucunun (out) standardın biçimi olduğudur. x/text v0.39.0 üç yerde ayrılır:
//   - otuzdan çok ardışık birleşen işaretin arasına U+034F koyar (Stream-Safe biçimi);
//   - birleştirme anahtarını 16 bite kırpar: U+20041 U+0300 "À", U+05D2 U+0307 U+105C9 olur (sonucun
//     ayrılmış biçimi girdininkinden başkadır);
//   - Hangıldan sonra gelen Gurung Khema harflerini birleştirmez (U+1100 U+1161 U+1611E U+1611E);
//     ayrılmış biçimleri aynıdır, bunu composesAbove16Bits yakalar.
func (t formTable) faithful(s, out string) bool {
	if strings.Count(out, norm.GraphemeJoiner) != strings.Count(s, norm.GraphemeJoiner) {
		return false
	}
	if !t.composes {
		return true
	}
	return !composesAbove16Bits(s) && (out == s || t.decomposed.String(out) == t.decomposed.String(s))
}

// composesAbove16Bits 0xFFFF'in üstündeki bir karakterin komşusuyla aynı birleşim parçasında
// olduğudur: x/text'in kırpılan anahtarının ve Hangıl kısa yolunun yanıldığı yer.
func composesAbove16Bits(s string) bool {
	above := false
	for i, r := range s {
		was := above
		above = r > 0xFFFF
		if (above || was) && !norm.NFKC.PropertiesString(s[i:]).BoundaryBefore() {
			return true
		}
	}
	return false
}

// codePoint bir karakter ve birleşim sınıfıdır.
type codePoint struct {
	r     rune
	class uint8
}

// exact standardın algoritmasıdır: her karakter ayrılır, birleşen işaretler sınıflarına göre kararlı
// sıralanır, birleştiren biçimde ilk öğeyle arasında engel olmayan karakter ona katılır.
func (t formTable) exact(s string) string {
	chars := t.decompose(s)
	for i := 0; i < len(chars); i++ {
		j := i
		for j < len(chars) && chars[j].class != 0 {
			j++
		}
		slices.SortStableFunc(chars[i:j], func(a, b codePoint) int { return int(a.class) - int(b.class) })
		i = j
	}
	if t.composes {
		chars = compose(chars)
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, c := range chars {
		b.WriteRune(c.r)
	}
	return b.String()
}

func (t formTable) decompose(s string) []codePoint {
	chars := make([]codePoint, 0, utf8.RuneCountInString(s))
	for i := 0; i < len(s); {
		_, width := utf8.DecodeRuneInString(s[i:])
		parts := t.decomposed.String(s[i : i+width])
		i += width
		for j, r := range parts {
			chars = append(chars, codePoint{r, t.decomposed.PropertiesString(parts[j:]).CCC()})
		}
	}
	return chars
}

// compose sıralanmış karakterleri yerinde birleştirir. Bir karakter son ilk öğeye, ona bitişikse ya da
// aradaki işaretlerin sınıfı kendininkinden küçükse katılır.
func compose(chars []codePoint) []codePoint {
	out := chars[:0]
	starter := -1
	for _, c := range chars {
		if starter >= 0 && (len(out) == starter+1 || out[len(out)-1].class < c.class) {
			if r, ok := composed(out[starter].r, c.r); ok {
				out[starter].r = r
				continue
			}
		}
		if c.class == 0 {
			starter = len(out)
		}
		out = append(out, c)
	}
	return out
}

// composed iki karakterin birleşiğidir; birleşiğin ayrılmış biçimi ikisininki değilse x/text'in
// kırpılan anahtarı başka bir çiftin birleşiğini vermiştir.
func composed(a, b rune) (rune, bool) {
	pair := string([]rune{a, b})
	second := pair[utf8.RuneLen(a):]
	if norm.NFC.PropertiesString(second).BoundaryBefore() {
		return 0, false // öncekiyle hiç birleşmez
	}
	c := norm.NFC.String(pair)
	r, width := utf8.DecodeRuneInString(c)
	if width != len(c) || norm.NFD.String(c) != norm.NFD.String(pair) {
		return 0, false
	}
	return r, true
}
