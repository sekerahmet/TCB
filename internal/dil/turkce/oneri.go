package turkce

import "strings"

// asciiKatla Türkçe karakterleri ASCII karşılıklarına indirger ve küçük harfe çevirir;
// böylece 'degisken_adi' ile 'değişken_adı' aynı sözcük sayılır.
func asciiKatla(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch r {
		case 'ı', 'I', 'İ':
			sb.WriteRune('i')
		case 'ş', 'Ş':
			sb.WriteRune('s')
		case 'ğ', 'Ğ':
			sb.WriteRune('g')
		case 'ü', 'Ü':
			sb.WriteRune('u')
		case 'ö', 'Ö':
			sb.WriteRune('o')
		case 'ç', 'Ç':
			sb.WriteRune('c')
		default:
			sb.WriteString(strings.ToLower(string(r)))
		}
	}
	return sb.String()
}

// asciiCevirici Türkçe harfleri ASCII karşılıklarına çevirir; büyük/küçük harf korunur.
var asciiCevirici = strings.NewReplacer(
	"ı", "i", "İ", "I", "ş", "s", "Ş", "S", "ğ", "g", "Ğ", "G",
	"ü", "u", "Ü", "U", "ö", "o", "Ö", "O", "ç", "c", "Ç", "C",
)

// ASCII adın Türkçe harflerini ASCII karşılıklarına çevrilmiş biçimini verir
// (öğrenme_oranı → ogrenme_orani); diğer karakterler ve harf büyüklüğü değişmez.
func ASCII(s string) string {
	return asciiCevirici.Replace(s)
}

func turkceHarfVar(s string) bool {
	return strings.ContainsAny(s, "ıİşŞğĞüÜöÖçÇ")
}

// uzaklik iki sözcük arasındaki düzenleme (Levenshtein) uzaklığıdır.
func uzaklik(a, b string) int {
	x, y := []rune(a), []rune(b)
	onceki := make([]int, len(y)+1)
	simdi := make([]int, len(y)+1)
	for j := range onceki {
		onceki[j] = j
	}
	for i := 1; i <= len(x); i++ {
		simdi[0] = i
		for j := 1; j <= len(y); j++ {
			maliyet := 1
			if x[i-1] == y[j-1] {
				maliyet = 0
			}
			simdi[j] = min(onceki[j]+1, simdi[j-1]+1, onceki[j-1]+maliyet)
		}
		onceki, simdi = simdi, onceki
	}
	return onceki[len(y)]
}

// EnYakin adaylar arasından aranan sözcüğe en yakın olanı önerir ("Bunu mu demek istediniz?").
// Türkçe/ASCII karakter farkı (degisken ↔ değişken) hata sayılmaz. Yeterince yakın aday
// yoksa boş döner. Eşit uzaklıkta alfabetik olarak ilk aday seçilir (kararlı sonuç).
func EnYakin(aranan string, adaylar []string) string {
	hedef := asciiKatla(aranan)
	sinir := len([]rune(aranan)) / 3
	if sinir < 1 {
		sinir = 1
	}
	if sinir > 3 {
		sinir = 3
	}
	turkceYaziyor := turkceHarfVar(aranan)
	en, enUzak, enUyumlu := "", sinir+1, false
	for _, a := range adaylar {
		if a == aranan || a == "" {
			continue
		}
		u := uzaklik(hedef, asciiKatla(a))
		// Eşit uzaklıkta kullanıcının yazım tercihine (Türkçe / ASCII) uyan aday seçilir:
		// 'yazdr' → 'yazdir', 'yazdırr' → 'yazdır'
		uyumlu := turkceHarfVar(a) == turkceYaziyor
		if u < enUzak || (u == enUzak && uyumlu && !enUyumlu) || (u == enUzak && uyumlu == enUyumlu && a < en) {
			en, enUzak, enUyumlu = a, u, uyumlu
		}
	}
	if enUzak > sinir {
		return ""
	}
	return en
}
