package turkish

import (
	"strings"
	"unicode/utf8"
)

// foldASCII Türkçe karakterleri ASCII karşılıklarına indirger ve küçük harfe çevirir;
// böylece 'degisken_adi' ile 'değişken_adı' aynı sözcük sayılır.
func foldASCII(s string) string { return ASCII(Lower(s)) }

// asciiFolder Türkçe harfleri ASCII karşılıklarına çevirir; büyük/küçük harf korunur.
var asciiFolder = strings.NewReplacer(
	"ı", "i", "İ", "I", "ş", "s", "Ş", "S", "ğ", "g", "Ğ", "G",
	"ü", "u", "Ü", "U", "ö", "o", "Ö", "O", "ç", "c", "Ç", "C",
)

// ASCII returns the name with its Turkish letters replaced by their ASCII counterparts
// (öğrenme_oranı → ogrenme_orani); other characters and letter case are unchanged.
// ASCII-only names are returned as they are without calling the replacer (hot path of code lookups).
func ASCII(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return asciiFolder.Replace(s)
		}
	}
	return s
}

// HasTurkishLetter reports whether s contains a letter specific to Turkish (ı İ ş Ş ğ Ğ ü Ü ö Ö ç Ç).
func HasTurkishLetter(s string) bool {
	return strings.ContainsAny(s, "ıİşŞğĞüÜöÖçÇ")
}

// distance iki sözcük arasındaki düzenleme (Levenshtein) uzaklığıdır.
func distance(a, b string) int {
	x, y := []rune(a), []rune(b)
	prev := make([]int, len(y)+1)
	curr := make([]int, len(y)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(x); i++ {
		curr[0] = i
		for j := 1; j <= len(y); j++ {
			cost := 1
			if x[i-1] == y[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(y)]
}

// Closest suggests the candidate closest to the searched word ("Bunu mu demek istediniz?",
// "Did you mean?"). A Turkish/ASCII letter difference (degisken ↔ değişken) is not counted
// as an error. Returns "" when no candidate is close enough. Ties go to the alphabetically
// first candidate (deterministic).
func Closest(wanted string, candidates []string) string {
	target := foldASCII(wanted)
	limit := len([]rune(wanted)) / 3
	if limit < 1 {
		limit = 1
	}
	if limit > 3 {
		limit = 3
	}
	typesTurkish := HasTurkishLetter(wanted)
	best, farthest, bestMatch := "", limit+1, false
	for _, a := range candidates {
		if a == wanted || a == "" {
			continue
		}
		u := distance(target, foldASCII(a))
		// Eşit uzaklıkta kullanıcının yazım tercihine (Türkçe / ASCII) uyan aday seçilir:
		// 'yazdr' → 'yazdir', 'yazdırr' → 'yazdır'
		sameScript := HasTurkishLetter(a) == typesTurkish
		if u < farthest || (u == farthest && sameScript && !bestMatch) || (u == farthest && sameScript == bestMatch && a < best) {
			best, farthest, bestMatch = a, u, sameScript
		}
	}
	if farthest > limit {
		return ""
	}
	return best
}
