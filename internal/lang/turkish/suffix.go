// Package turkish provides grammar helpers for producing correct Turkish in TR's messages:
// case suffixes by vowel harmony, suffix choice by how numbers are read, Turkish letter
// case, spelling suggestions, user-facing file error texts and source code frames.
//
// Layer: language core (internal/lang); imports only internal/lang packages.
package turkish

import (
	"strconv"
	"strings"
)

// Case is the grammatical case suffix a noun takes.
type Case int

const (
	Accusative Case = iota // -ı, -i, -u, -ü   (6'yı, kitabı)
	Dative                 // -a, -e          (6'ya, eve)
	Locative               // -da, -de, -ta, -te (6'da, 3'te)
	Ablative               // -dan, -den, -tan, -ten (6'dan, 3'ten)
	Genitive               // -ın, -in, -un, -ün (6'nın, 3'ün)
)

const (
	backVowels     = "aıou"
	frontVowels    = "eiöü"
	hardConsonants = "çfhkpsşt"
)

// Suffix appends the case suffix with an apostrophe: Suffix("Ankara", Locative) = "Ankara'da".
// The apostrophe is used for proper names, numbers and names in code.
// Note: loanwords that break vowel harmony (Kemal'in, saat'e) are not supported yet.
func Suffix(word string, grammarCase Case) string {
	return word + "'" + findSuffix(pronounce(word), grammarCase)
}

// NumberSuffix writes a number with the suffix that matches its pronunciation:
// 6 → "6'ya", 3 → "3'e", 40 → "40'a".
func NumberSuffix(n int64, grammarCase Case) string {
	return Suffix(strconv.FormatInt(n, 10), grammarCase)
}

// pronounce ekin seçimine esas olan sözcüğü döndürür. Sayılar okunuşlarıyla değerlendirilir
// (6 = "altı"); diğer sözcükler kendileridir.
func pronounce(s string) string {
	if w, ok := lastNumberWord(s); ok {
		return w
	}
	return s
}

// findSuffix sözcüğün son ünlüsüne ve son sesine göre eki üretir.
func findSuffix(word string, grammarCase Case) string {
	w := []rune(Lower(word))
	if len(w) == 0 {
		return ""
	}
	lastVowel := 'e'
	for i := len(w) - 1; i >= 0; i-- {
		if strings.ContainsRune(backVowels+frontVowels, w[i]) {
			lastVowel = w[i]
			break
		}
	}
	last := w[len(w)-1]
	endsWithVowel := strings.ContainsRune(backVowels+frontVowels, last)
	endsHard := strings.ContainsRune(hardConsonants, last)
	backVowel := strings.ContainsRune(backVowels, lastVowel)

	// İki yönlü (a/e) ve dört yönlü (ı/i/u/ü) ünlü uyumu
	a := "e"
	if backVowel {
		a = "a"
	}
	var i string
	switch lastVowel {
	case 'a', 'ı':
		i = "ı"
	case 'e', 'i':
		i = "i"
	case 'o', 'u':
		i = "u"
	default: // ö, ü
		i = "ü"
	}
	d := "d"
	if endsHard {
		d = "t"
	}

	switch grammarCase {
	case Accusative:
		if endsWithVowel {
			return "y" + i
		}
		return i
	case Dative:
		if endsWithVowel {
			return "y" + a
		}
		return a
	case Locative:
		return d + a
	case Ablative:
		return d + a + "n"
	case Genitive:
		if endsWithVowel {
			return "n" + i + "n"
		}
		return i + "n"
	}
	return ""
}

var ones = []string{"sıfır", "bir", "iki", "üç", "dört", "beş", "altı", "yedi", "sekiz", "dokuz"}
var tens = []string{"", "on", "yirmi", "otuz", "kırk", "elli", "altmış", "yetmiş", "seksen", "doksan"}

// lastNumberWord yalnızca ASCII rakamlardan oluşan (başında '-' olabilen) bir sayının
// okunuşundaki son sözcüğü verir: 46 → "altı", 40 → "kırk", 2000 → "bin". Sayı değil,
// rakam dizisi olarak sondan okunur: int64'e sığmayan uzunlukta da taşma olmaz. Sayı
// değilse ok false'tur (başka yazıların rakamları, ör. "٦", sayı sayılmaz).
func lastNumberWord(s string) (word string, ok bool) {
	digits := strings.TrimPrefix(s, "-")
	if digits == "" {
		return "", false
	}
	for i := 0; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			return "", false
		}
	}
	// sondaki sıfırlar: kaçıncı basamakta sıfır olmayan bir rakam var?
	zeros := 0
	for zeros < len(digits) && digits[len(digits)-1-zeros] == '0' {
		zeros++
	}
	if zeros == len(digits) {
		return "sıfır", true
	}
	d := digits[len(digits)-1-zeros] - '0'
	switch {
	case zeros == 0:
		return ones[d], true
	case zeros == 1:
		return tens[d], true
	case zeros == 2:
		return "yüz", true
	case zeros < 6:
		return "bin", true
	case zeros < 9:
		return "milyon", true
	case zeros < 12:
		return "milyar", true
	default:
		return "trilyon", true
	}
}
