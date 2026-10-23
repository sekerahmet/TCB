package turkish

import "testing"

func TestClosest(t *testing.T) {
	candidates := []string{"toplam", "topla", "yazdır", "yazdir", "değişken_adı", "sayılar", "uzunluk"}
	cases := map[string]string{
		"toplm":        "topla",        // eşit uzaklıkta alfabetik ilk: topla (1) < toplam (1)
		"yazdr":        "yazdir",       // ASCII yazana ASCII öneri
		"yazdırr":      "yazdır",       // Türkçe yazana Türkçe öneri
		"degisken_adi": "değişken_adı", // yalnızca ASCII farkı
		"sayilar":      "sayılar",
		"uznluk":       "uzunluk",
		"xyz":          "",
		"bambaşka":     "",
	}
	for wanted, want := range cases {
		if got := Closest(wanted, candidates); got != want {
			t.Errorf("EnYakin(%q) = %q, %q bekleniyordu", wanted, got, want)
		}
	}
}

// Kural: foldASCII, Türkçe küçük harf kuralı (Lower) ve ASCII karşılıkları (ASCII) birleşimidir;
// I, İ, ı ve i dördü de i olur, öbür Türkçe harfler ASCII karşılığına iner.
// Neden: eski ayrı döngü bu iki kuralı yeniden yazıyordu ve I/İ/ı davranışı ayrışabilirdi.
func TestFoldASCIIFoldsTurkishLetters(t *testing.T) {
	for in, want := range map[string]string{
		"Iİıi":          "iiii",
		"IŞIK":          "isik",
		"İşler":         "isler",
		"ÂÎÛ":           "âîû",
		"değişken_adı":  "degisken_adi",
		"ÖĞRENME_ORANI": "ogrenme_orani",
		"a\xffb":        "a\uFFFDb",
		"":              "",
	} {
		if got := foldASCII(in); got != want {
			t.Errorf("foldASCII(%q) = %q, beklenen %q", in, got, want)
		}
	}
}

// oldDistance Distance dışa açılmadan önceki sınırsız uygulamadır; Closest'in davranışının değişmediğini sabitlemek
// için burada durur.
func oldDistance(a, b string) int {
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

// Kural: Distance kod noktası olarak Levenshtein uzaklığıdır; sınır en az iki uzunluğun büyüğüyse gerçek uzaklık, sınır
// aşılınca bound+1. Closest gerçek uzaklığı alır: dışa açılan sınırlı uygulama önerileri değiştirmez (sınırın üstünde
// birden çok aday olan durum dahil). codeindex ve codegraph aynı uygulamayı çağırır (bağımsız denetim T2).
func TestDistanceAndClosestUnchanged(t *testing.T) {
	for _, c := range []struct {
		a, b  string
		bound int
		want  int
	}{
		{"kare", "kare", 4, 0}, {"yaz_alan", "yaz_alam", 4, 1}, {"işık", "isik", 4, 2},
		{"abc", "abcdefgh", 2, 3}, {"katman", "kitap", 1, 2}, {"katman", "kitap", 6, 3},
	} {
		if got := Distance(c.a, c.b, c.bound); got != c.want {
			t.Errorf("Distance(%q, %q, %d) = %d, want %d", c.a, c.b, c.bound, got, c.want)
		}
	}
	words := []string{"toplam", "topla", "yazdır", "yazdir", "değişken_adı", "sayılar", "uzunluk", "kare", "katman",
		"kitap", "ağırlık", "agirlik", "öğrenme_oranı", "uzaklık", "x", "çok_uzun_bir_ad_burada", "baş", "bas", "başlık"}
	oldClosest := func(wanted string, candidates []string) string {
		target := foldASCII(wanted)
		limit := min(max(len([]rune(wanted))/3, 1), 3)
		typesTurkish := HasTurkishLetter(wanted)
		best, farthest, bestMatch := "", limit+1, false
		for _, a := range candidates {
			if a == wanted || a == "" {
				continue
			}
			u := oldDistance(target, foldASCII(a))
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
	for _, q := range append(words, "toplm", "yazdr", "katmn", "zzzz", "ağırlk", "uzunlk", "bşlık", "q") {
		for n := 1; n <= len(words); n++ {
			if got, want := Closest(q, words[:n]), oldClosest(q, words[:n]); got != want {
				t.Errorf("Closest(%q, first %d) = %q, before %q", q, n, got, want)
			}
			for a := range words {
				if got, want := Distance(foldASCII(q), foldASCII(words[a]), 64), oldDistance(foldASCII(q), foldASCII(words[a])); got != want {
					t.Errorf("Distance(%q, %q) = %d, unbounded %d", q, words[a], got, want)
				}
			}
		}
	}
}
