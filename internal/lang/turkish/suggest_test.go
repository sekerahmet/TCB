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
