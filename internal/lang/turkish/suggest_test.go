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
