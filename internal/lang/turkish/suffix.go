// Package turkish provides grammar helpers for producing correct Turkish in TR's messages:
// case suffixes by vowel harmony, suffix choice by how numbers are read, Turkish letter
// case, spelling suggestions, user-facing file error texts and source code frames.
//
// Layer: language core (internal/lang); imports only internal/lang packages.
package turkish

import (
	"strings"
	"unicode"
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
	kalinUnluler = "aıou"
	inceUnluler  = "eiöü"
	sertUnsuzler = "çfhkpsşt"
)

// Suffix appends the case suffix with an apostrophe: Suffix("Ankara", Locative) = "Ankara'da".
// The apostrophe is used for proper names, numbers and names in code.
// Note: loanwords that break vowel harmony (Kemal'in, saat'e) are not supported yet.
func Suffix(sozcuk string, hal Case) string {
	return sozcuk + "'" + ekBul(okunus(sozcuk), hal)
}

// NumberSuffix writes a number with the suffix that matches its pronunciation:
// 6 → "6'ya", 3 → "3'e", 40 → "40'a".
func NumberSuffix(n int64, hal Case) string {
	return Suffix(itoa(n), hal)
}

// okunus ekin seçimine esas olan sözcüğü döndürür. Sayılar okunuşlarıyla değerlendirilir
// (6 = "altı"); diğer sözcükler kendileridir.
func okunus(s string) string {
	if n, ok := parseInt(s); ok {
		return sayiSonSozcuk(n)
	}
	return s
}

// ekBul sözcüğün son ünlüsüne ve son sesine göre eki üretir.
func ekBul(sozcuk string, hal Case) string {
	w := []rune(strings.ToLower(sozcuk))
	if len(w) == 0 {
		return ""
	}
	sonUnlu := 'e'
	for i := len(w) - 1; i >= 0; i-- {
		if strings.ContainsRune(kalinUnluler+inceUnluler, w[i]) {
			sonUnlu = w[i]
			break
		}
	}
	son := w[len(w)-1]
	unluyleBiter := strings.ContainsRune(kalinUnluler+inceUnluler, son)
	sertleBiter := strings.ContainsRune(sertUnsuzler, son)
	kalin := strings.ContainsRune(kalinUnluler, sonUnlu)

	// İki yönlü (a/e) ve dört yönlü (ı/i/u/ü) ünlü uyumu
	a := "e"
	if kalin {
		a = "a"
	}
	var i string
	switch sonUnlu {
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
	if sertleBiter {
		d = "t"
	}

	switch hal {
	case Accusative:
		if unluyleBiter {
			return "y" + i
		}
		return i
	case Dative:
		if unluyleBiter {
			return "y" + a
		}
		return a
	case Locative:
		return d + a
	case Ablative:
		return d + a + "n"
	case Genitive:
		if unluyleBiter {
			return "n" + i + "n"
		}
		return i + "n"
	}
	return ""
}

var birler = []string{"sıfır", "bir", "iki", "üç", "dört", "beş", "altı", "yedi", "sekiz", "dokuz"}
var onlar = []string{"", "on", "yirmi", "otuz", "kırk", "elli", "altmış", "yetmiş", "seksen", "doksan"}

// sayiSonSozcuk bir sayının okunuşundaki son sözcüğü verir: 46 → "altı", 40 → "kırk", 2000 → "bin".
func sayiSonSozcuk(n int64) string {
	if n < 0 {
		n = -n
	}
	switch {
	case n == 0:
		return "sıfır"
	case n%10 != 0:
		return birler[n%10]
	case n%100 != 0:
		return onlar[(n%100)/10]
	case n%1000 != 0:
		return "yüz"
	case n%1_000_000 != 0:
		return "bin"
	case n%1_000_000_000 != 0:
		return "milyon"
	case n%1_000_000_000_000 != 0:
		return "milyar"
	default:
		return "trilyon"
	}
}

func parseInt(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	var n int64
	neg := false
	for i, r := range s {
		if i == 0 && r == '-' && len(s) > 1 {
			neg = true
			continue
		}
		if !unicode.IsDigit(r) {
			return 0, false
		}
		n = n*10 + int64(r-'0')
	}
	if neg {
		n = -n
	}
	return n, true
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
