package interp

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (B2b): Liste ve Metin'de .dilim yoktur (Tensör'ün metodu); ileti dilimin köşeli
// parantezle alındığını söyler (H2526).
func TestSliceMethodOnListGetsRangeHint(t *testing.T) {
	for _, code := range []string{"[1, 2, 3].dilim(0, 1)", `"abc".dilim(0, 1)`} {
		_, err := New().Run(mustParseProgram(t, code+"\n"))
		if !messages.Has(err, messages.H2526) {
			t.Errorf("%s: %v", code, err)
		}
	}
}

// Kural (13.2): büyük_harf ve küçük_harf herkes için Unicode'un varsayılan harf dönüşümüdür
// (Python str.upper, Swift uppercased(), Go strings.ToUpper gibi); dosyanın dili sonucu değiştirmez:
// İngilizce ve Türkçe yazılmış aynı program aynı metni verir ("title" → "TITLE", "istanbul" →
// "ISTANBUL", "LIST" → "list").
func TestCaseConversionIsTheSameInEveryLanguage(t *testing.T) {
	want := "TITLE ISTANBUL list\n"
	for lang, text := range map[string]string{
		"en": `print("title".upper(), "istanbul".upper(), "LIST".lower())` + "\n",
		"tr": `TR_print("title".TR_upper(), "istanbul".TR_upper(), "LIST".TR_lower())` + "\n",
	} {
		if got := runCoded(t, text, lang, nil); got != want {
			t.Errorf("%s: %q, beklenen %q", lang, got, want)
		}
	}
}
