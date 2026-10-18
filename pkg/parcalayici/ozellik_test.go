package parcalayici

import (
	"strings"
	"testing"
)

// Unicode özellik sınıfları ve büyük/küçük harf duyarsız eşleşme başvuru
// uygulamasıyla aynı (onun Unicode sürümü ve sınıf tanımları). Beklenen
// değerler başvuru uygulamasının çıktısıdır: her karakter için desen o tek
// karakteri eşliyor mu (1) ya da eşlemiyor mu (0).
func TestOzellikSiniflariBasvuruyla(t *testing.T) {
	durumlar := []struct{ desen, karakterler, beklenen string }{
		// [[:punct:]] = P ve ASCII simgeleri; \p{Punct} yalnızca P.
		{`[[:punct:]]`, "<=>$+^|~`!", "1111111111"},
		{`\p{Punct}`, "<!", "01"},
		// Go tablolarından daha yeni Unicode sürümündeki karakterler.
		{`\p{Han}`, "\U0002EBF0", "1"},
		{`\p{Alphabetic}`, "ͣͤ", "11"},
		{`\p{Latin}`, "Ɤ", "1"},
		{`\p{Arabic}`, "ࢗ", "1"},
		{`\p{Common}`, "␧", "1"},
		{`\p{Garay}`, "\U00010D50", "1"},
		{`(?i)\x{10D70}`, "\U00010D50", "1"},
		{`(?i)[[:upper:]]`, "ꟍ\U00010D70", "11"},
		// Köşeli parantezsiz sınıf (?i) ile katlanmaz; köşeli parantezli katlanır
		// ve olumsuzlama katlamadan sonra uygulanır.
		{`(?i)\p{Lu}`, "aA", "01"},
		{`(?i)[\p{Lu}]`, "aA", "11"},
		{`(?i)[^a-z]`, "AK1", "001"},
		{`(?i)k`, "KK", "11"},
		{`[[:word:]]`, "_‌", "10"},
		{`[[:graph:]]`, "  a", "001"},
	}
	for _, d := range durumlar {
		ifade, err := DuzIfadeDerle(d.desen)
		if err != nil {
			t.Errorf("%q: %v", d.desen, err)
			continue
		}
		var got strings.Builder
		for _, r := range d.karakterler {
			s := string(r)
			_, e, ok, err := ifade.Bul(s, 0)
			if err != nil {
				t.Fatal(err)
			}
			if ok && e == len(s) {
				got.WriteByte('1')
			} else {
				got.WriteByte('0')
			}
		}
		if got.String() != d.beklenen {
			t.Errorf("%q %+q: %s, beklenen %s", d.desen, d.karakterler, got.String(), d.beklenen)
		}
	}
	// Başvurunun tanımadığı adlar (ve iç tablo adları) tanınmaz.
	for _, d := range []string{`\p{Bosluk}`, `\p{Hrkt}`, `\p{Katakana_Or_Hiragana}`} {
		if _, err := DuzIfadeDerle(d); err == nil || !strings.Contains(err.Error(), "bilinmeyen Unicode özelliği") {
			t.Errorf("%q reddedilmeliydi: %v", d, err)
		}
	}
}
