package interp

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// Kural (13.2): bilinmeyen biçim H2671'dir. Yalnız harf büyüklüğü tutmayan biçimde ileti doğru yazımı
// verir ve öneri kopyalanınca çalışır; öbüründe öneri yoktur. Metinle biçim yer değiştirmişse verilen
// metin kısalır, gizli değer kesilmeden önce maskelenir.
func TestUnicodeNormalizeMessages(t *testing.T) {
	for given, want := range map[string]string{
		"nfc": "é 1\n", "nfd": "é 2\n", "nfkC": "é 1\n", "NfKd": "é 2\n",
		"utf8": "", "": "", "NFC ": "", strings.Repeat("uzun metin ", 8): "",
	} {
		_, _, err := runTROut(t, "TR_print(\"e\\u{301}\".TR_unicode_normalize(\""+given+"\"))\n")
		if !messages.Has(err, messages.H2671) {
			t.Fatalf("%q: H2671 beklenirdi, gelen %v", given, err)
		}
		suggestion := ".TR_unicode_normalize(\"" + strings.ToUpper(given) + "\")"
		for _, lang := range []string{"tr", "en"} {
			text := messages.ErrorIn(err, lang)
			if strings.Contains(text, "`"+suggestion+"`") != (want != "") || strings.Contains(text, "`") != (want != "") {
				t.Errorf("%q (%s): %s", given, lang, text)
			}
			if strings.Contains(text, given) != (len(given) < summaryLimit) {
				t.Errorf("%q (%s): verilen metin kısalmadı: %s", given, lang, text)
			}
		}
		if want == "" {
			continue
		}
		if out, _, err := runTROut(t, "TR_var m = \"e\\u{301}\""+suggestion+"\nTR_print(m, m.TR_length)\n"); err != nil || out != want {
			t.Errorf("%s: %q %v, beklenen %q", suggestion, out, err, want)
		}
	}
	m := New()
	key := "sk-bicim-anahtari-0123456789abcdefghijklmnop"
	m.AddSecret(key)
	_, err := m.Run(mustParseProgram(t, "\"NFC\".TR_unicode_normalize(\""+key+"\")\n"))
	if text := messages.ErrorIn(err, "tr"); !messages.Has(err, messages.H2671) || strings.Contains(text, key[:8]) {
		t.Errorf("gizli değerin başı iletide kalmamalı: %s", text)
	}
}

// Kural (13.2, 2.3a): sayı ve tip çalışmadan önce denetlenir, kullanım satırı kopyalanınca çalışır;
// Python'un yazımları metodu önerir.
func TestUnicodeNormalizeIsSuggested(t *testing.T) {
	errorsOf := func(code, lang string) string {
		r := source.Prepare(source.Header+"\n"+code+"\n", source.Options{TypeCheck: true})
		var lines []string
		for _, e := range r.Errors {
			lines = append(lines, messages.ErrorIn(e, lang))
		}
		return strings.Join(lines, "\n")
	}
	for code, want := range map[string]string{
		`TR_print("a".TR_unicode_normalize())`:            "`.TR_unicode_normalize(\"NFC\")`",
		`TR_print("a".TR_unicode_normalize("NFC", "a"))`:  "`.TR_unicode_normalize(\"NFC\")`",
		`TR_print("a".TR_unicode_normalize(1))`:           "TR_String",
		`TR_print(unicodedata.normalize("NFC", "a"))`:     "'x.TR_unicode_normalize'",
		`TR_print(normalize("NFC", "a"))`:                 "'x.TR_unicode_normalize'",
		`TR_print(unicode_normalize("a", "NFC"))`:         "'x.TR_unicode_normalize'",
		`TR_print(TR_normalize("a", "NFC"))`:              "'x.TR_unicode_normalize'",
		`TR_print(TR_unicode_normalize("a", "NFC"))`:      "x.TR_unicode_normalize",
		`TR_print(TR_ones([2]).TR_unicode_normalize(""))`: "TR_unicode_normalize",
	} {
		for _, lang := range []string{"tr", "en"} {
			if text := errorsOf(code, lang); !strings.Contains(text, want) {
				t.Errorf("%s (%s): %q, %q taşımalı", code, lang, text, want)
			}
		}
	}
	if text := errorsOf(`TR_var m: TR_String = "a".TR_unicode_normalize("NFC")`, "tr"); text != "" {
		t.Errorf("doğru çağrı denetimden geçmedi: %s", text)
	}
	_, err := runCodedErr(t, "TR_print(\"a\".normalize(\"NFC\"))\n", nil)
	if text := messages.ErrorIn(err, "tr"); !strings.Contains(text, "'TR_unicode_normalize'") {
		t.Errorf("\"a\".normalize(...): %s", text)
	}
}

// Kural (10.2): metnin biçimi karakter karakter kurulabilir; ASCII olmayan metinde kilit 64 KB'tan önce
// (unicodeFormBytes) bırakılır, ASCII metinde ve kısa metinde bırakılmaz.
func TestUnicodeNormalizeReleasesTheLockEarly(t *testing.T) {
	for text, want := range map[string]int{"ş": 1, "a": 0} {
		for _, n := range []int{unicodeFormBytes, unicodeFormBytes - 1} {
			prelude := fmt.Sprintf("TR_var s = %q\n", strings.Repeat(text, n/len(text)))
			expected := want
			if n < unicodeFormBytes {
				expected = 0
			}
			if got := lockTakes(t, prelude+"TR_var r = s.TR_unicode_normalize(\"NFC\")\n") - lockTakes(t, prelude); got != expected {
				t.Errorf("%d bayt %q: kilit %d kez bırakıldı, beklenen %d", n, text, got, expected)
			}
		}
	}
}
