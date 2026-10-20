package turkish

import (
	"math"
	"testing"
)

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
		{1_000_000, Dative, "1000000'a"}, {3_000_000_000, Locative, "3000000000'da"}, {20_000, Ablative, "20000'den"},
		{2_000_000_000_000, Dative, "2000000000000'a"}, {1200, Locative, "1200'de"},
		// en küçük int64: eskiden n = -n negatif kalıp ones[-8] ile çöküyordu (…808 → sekiz)
		{math.MinInt64, Dative, "-9223372036854775808'e"},
		{math.MaxInt64, Dative, "9223372036854775807'ye"},
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
		// Büyük harfli sözcükte I ı'dır, İ i'dir (Türkçe büyük/küçük harf kuralı)
		Suffix("KAPI", Dative):    "KAPI'ya",
		Suffix("ADI", Genitive):   "ADI'nın",
		Suffix("SİNİR", Locative): "SİNİR'de",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("%q bekleniyordu, %q alındı", want, got)
		}
	}
}

// Sayı rakam dizisi olarak sondan okunur: int64'e sığmayan sayılar taşmaz ve son sözcük
// doğru bulunur; yalnızca ASCII rakamlar sayıdır.
func TestSuffixLongAndNonASCIINumbers(t *testing.T) {
	cases := map[string]string{
		Suffix("123456789012345678901234567896", Dative):    "123456789012345678901234567896'ya",   // ...altı
		Suffix("-99999999999999999999999999999993", Dative): "-99999999999999999999999999999993'e", // ...üç
		Suffix("100000000000000000000", Dative):             "100000000000000000000'a",             // eskiden taşıp 'ye
		Suffix("-9223372036854775808", Dative):              "-9223372036854775808'e",              // eskiden çöküyordu
		Suffix("0000", Locative):                            "0000'da",
		Suffix("007", Dative):                               "007'ye",
		// Arapça-Hint rakamı ٦ (altı) sayı değildir; ek sözcük kuralıyla seçilir ('ya değil)
		Suffix("٦", Dative): "٦'e",
		Suffix("-", Dative): "-'e",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("%q bekleniyordu, %q alındı", want, got)
		}
	}
}

// Türkçe ileti şablonundaki hâl biçimi ({ad|dat}) bu paketin ekini kullanır.
func TestTemplateInflector(t *testing.T) {
	for c, want := range map[string]string{"acc": "ileri'yi", "dat": "ileri'ye", "loc": "ileri'de", "abl": "ileri'den", "gen": "ileri'nin"} {
		if got := inflect("ileri", c); got != want {
			t.Errorf("%s: %q, %q bekleniyordu", c, got, want)
		}
	}
	if got := inflect("ileri", "xyz"); got != "ileri" {
		t.Errorf("bilinmeyen hâl: %q", got)
	}
}

// Ünlüsü olmayan ad (k, fn, tbmm) kısaltma gibi son harfinin okunuşuyla çekimlenir (TDK: k'ye,
// TBMM'ye, x'e). Milyar ve üstü sayılar da okunuşlarıyla: 1000000000'ı (milyarı).
func TestSuffixWithoutVowel(t *testing.T) {
	for _, d := range []struct {
		word string
		c    Case
		want string
	}{
		{"k", Accusative, "k'yi"}, {"k", Dative, "k'ye"}, {"k", Locative, "k'de"},
		{"fn", Dative, "fn'ye"}, {"TBMM", Dative, "TBMM'ye"}, {"x", Dative, "x'e"},
		{"x", Locative, "x'te"}, {"q", Accusative, "q'yu"}, {"st", Ablative, "st'den"},
		{"1000000000", Accusative, "1000000000'ı"}, {"1000000", Accusative, "1000000'u"},
	} {
		if got := Suffix(d.word, d.c); got != d.want {
			t.Errorf("Suffix(%q, %d) = %q, %q bekleniyordu", d.word, d.c, got, d.want)
		}
	}
}
