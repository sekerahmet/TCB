package turkish

import "testing"

func TestNumberSuffix(t *testing.T) {
	cases := []struct {
		n           int64
		grammarCase Case
		want        string
	}{
		{3, Dative, "3'e"}, {6, Dative, "6'ya"}, {9, Dative, "9'a"}, {10, Dative, "10'a"},
		{40, Dative, "40'a"}, {100, Dative, "100'e"}, {2000, Dative, "2000'e"}, {1, Dative, "1'e"},
		{2, Dative, "2'ye"}, {0, Dative, "0'a"}, {7, Dative, "7'ye"}, {60, Dative, "60'a"},
		{3, Locative, "3'te"}, {6, Locative, "6'da"}, {4, Locative, "4'te"}, {5, Locative, "5'te"},
		{12, Locative, "12'de"}, {40, Locative, "40'ta"}, {100, Locative, "100'de"}, {70, Locative, "70'te"},
		{6, Ablative, "6'dan"}, {3, Ablative, "3'ten"}, {50, Ablative, "50'den"},
		{6, Accusative, "6'yı"}, {3, Accusative, "3'ü"}, {4, Accusative, "4'ü"}, {10, Accusative, "10'u"}, {1, Accusative, "1'i"},
		{6, Genitive, "6'nın"}, {3, Genitive, "3'ün"}, {10, Genitive, "10'un"}, {2, Genitive, "2'nin"},
		{-6, Dative, "-6'ya"},
	}
	for _, c := range cases {
		if got := NumberSuffix(c.n, c.grammarCase); got != c.want {
			t.Errorf("SayiEk(%d, %d) = %q, %q bekleniyordu", c.n, c.grammarCase, got, c.want)
		}
	}
}

func TestSuffixWord(t *testing.T) {
	cases := map[string]string{
		Suffix("Ankara", Locative):   "Ankara'da",
		Suffix("İstanbul", Locative): "İstanbul'da",
		Suffix("Ahmet", Dative):      "Ahmet'e",
		Suffix("Türkiye", Dative):    "Türkiye'ye",
		Suffix("Paris", Ablative):    "Paris'ten",
		Suffix("Mehmet", Genitive):   "Mehmet'in",
		Suffix("Ali", Genitive):      "Ali'nin",
		Suffix("Konya", Accusative):  "Konya'yı",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("%q bekleniyordu, %q alındı", want, got)
		}
	}
}
