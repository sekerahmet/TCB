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

// Distance iki metnin düzenleme uzaklığıdır, kod noktası olarak: silme, ekleme, değiştirme ve iki komşu harfin yer
// değiştirmesi birer düzeltmedir (OSA, sınırlı Damerau-Levenshtein: pritn ~ print 1; Python'un önerisi gibi).
// Uzaklığın bound'u aşacağı belli olunca bound+1 verir (aday taramasında erken çıkış; bir satırın en küçüğü sınırı
// aşınca sonraki satırlar da aşar). Gerçek uzaklık için bound en az max(len(a), len(b)) olmalıdır. Dilin önerileri
// (Closest), codeindex'in ve codegraph'ın yakın ad önerileri bu tek uygulamayı kullanır.
func Distance(a, b string, bound int) int {
	ra, rb := []rune(a), []rune(b)
	if d := len(ra) - len(rb); d > bound || -d > bound {
		return bound + 1
	}
	before := make([]int, len(rb)+1) // i-2. satır (yer değiştirme için)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		rowMin := cur[0]
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				cur[j] = min(cur[j], before[j-2]+1)
			}
			rowMin = min(rowMin, cur[j])
		}
		if rowMin > bound {
			return bound + 1
		}
		before, prev, cur = prev, cur, before
	}
	return prev[len(rb)]
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
		fa := foldASCII(a)
		u := Distance(target, fa, max(len(target), len(fa))) // sınırsız: gerçek uzaklık (bayt sayısı kod noktası sayısından az değildir)
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
