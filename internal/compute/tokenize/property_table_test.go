package tokenize

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Unicode özellik sınıfları ve büyük/küçük harf duyarsız eşleşme başvuru
// uygulamasıyla aynı (onun Unicode sürümü ve sınıf tanımları). Beklenen
// değerler başvuru uygulamasının çıktısıdır: her karakter için desen o tek
// karakteri eşliyor mu (1) ya da eşlemiyor mu (0).
func TestPropertyClassesMatchReference(t *testing.T) {
	cases := []struct{ pattern, chars, expected string }{
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
		{`[[:word:]]`, "_\u200c", "10"},
		{`[[:graph:]]`, "  a", "001"},
	}
	for _, d := range cases {
		expr, err := CompileRegex(d.pattern)
		if err != nil {
			t.Errorf("%q: %v", d.pattern, err)
			continue
		}
		var got strings.Builder
		for _, r := range d.chars {
			s := string(r)
			_, e, ok, err := expr.Find(s, 0)
			if err != nil {
				t.Fatal(err)
			}
			if ok && e == len(s) {
				got.WriteByte('1')
			} else {
				got.WriteByte('0')
			}
		}
		if got.String() != d.expected {
			t.Errorf("%q %+q: %s, beklenen %s", d.pattern, d.chars, got.String(), d.expected)
		}
	}
	// Başvurunun tanımadığı adlar (ve iç tablo adları) tanınmaz.
	for _, d := range []string{`\p{Bosluk}`, `\p{Hrkt}`, `\p{Katakana_Or_Hiragana}`} {
		if _, err := CompileRegex(d); !messages.Has(err, messages.H5268) {
			t.Errorf("%q reddedilmeliydi: %v", d, err)
		}
	}
}
