package turkce

import "testing"

func TestSayiEk(t *testing.T) {
	cases := []struct {
		n    int64
		hal  Hal
		want string
	}{
		{3, Yonelme, "3'e"}, {6, Yonelme, "6'ya"}, {9, Yonelme, "9'a"}, {10, Yonelme, "10'a"},
		{40, Yonelme, "40'a"}, {100, Yonelme, "100'e"}, {2000, Yonelme, "2000'e"}, {1, Yonelme, "1'e"},
		{2, Yonelme, "2'ye"}, {0, Yonelme, "0'a"}, {7, Yonelme, "7'ye"}, {60, Yonelme, "60'a"},
		{3, Bulunma, "3'te"}, {6, Bulunma, "6'da"}, {4, Bulunma, "4'te"}, {5, Bulunma, "5'te"},
		{12, Bulunma, "12'de"}, {40, Bulunma, "40'ta"}, {100, Bulunma, "100'de"}, {70, Bulunma, "70'te"},
		{6, Ayrilma, "6'dan"}, {3, Ayrilma, "3'ten"}, {50, Ayrilma, "50'den"},
		{6, Belirtme, "6'yı"}, {3, Belirtme, "3'ü"}, {4, Belirtme, "4'ü"}, {10, Belirtme, "10'u"}, {1, Belirtme, "1'i"},
		{6, Tamlayan, "6'nın"}, {3, Tamlayan, "3'ün"}, {10, Tamlayan, "10'un"}, {2, Tamlayan, "2'nin"},
		{-6, Yonelme, "-6'ya"},
	}
	for _, c := range cases {
		if got := SayiEk(c.n, c.hal); got != c.want {
			t.Errorf("SayiEk(%d, %d) = %q, %q bekleniyordu", c.n, c.hal, got, c.want)
		}
	}
}

func TestEkSozcuk(t *testing.T) {
	cases := map[string]string{
		Ek("Ankara", Bulunma):   "Ankara'da",
		Ek("İstanbul", Bulunma): "İstanbul'da",
		Ek("Ahmet", Yonelme):    "Ahmet'e",
		Ek("Türkiye", Yonelme):  "Türkiye'ye",
		Ek("Paris", Ayrilma):    "Paris'ten",
		Ek("Mehmet", Tamlayan):  "Mehmet'in",
		Ek("Ali", Tamlayan):     "Ali'nin",
		Ek("Konya", Belirtme):   "Konya'yı",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("%q bekleniyordu, %q alındı", want, got)
		}
	}
}

func TestEnYakin(t *testing.T) {
	adaylar := []string{"toplam", "topla", "yazdır", "yazdir", "değişken_adı", "sayılar", "uzunluk"}
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
	for aranan, want := range cases {
		if got := EnYakin(aranan, adaylar); got != want {
			t.Errorf("EnYakin(%q) = %q, %q bekleniyordu", aranan, got, want)
		}
	}
}
