package interp

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (8.2, ileti standardı): fırlatılamayan değerin iletisi programın kendi adıyla
// `TR_throw TR_String(ad)`'ı önerir (eskiden sabit `TR_String(x)`'ti; ad sayı ya da x olunca
// kopyalanan öneri tanımsız ad veriyordu). Önerilen yazım çalışır: değerin metniyle fırlar.
func TestNotThrowableSuggestsTheProgramsSpelling(t *testing.T) {
	for _, name := range []string{"sayı", "x", "son_değer"} {
		_, _, err := runTROut(t, "TR_var "+name+" = 5\nTR_throw "+name+"\n")
		suggestion := "TR_throw TR_String(" + name + ")"
		if !messages.Has(err, messages.H2911) {
			t.Fatalf("%s: H2911 bekleniyordu: %v", name, err)
		}
		for _, lang := range []string{"tr", "en"} {
			if text := messages.ErrorIn(err, lang); !strings.Contains(text, "`"+suggestion+"`") {
				t.Errorf("%s (%s): ileti %q önerisini içermiyor: %q", name, lang, suggestion, text)
			}
		}
		out, _, err := runTROut(t, "TR_var "+name+" = 5\nTR_try { "+suggestion+" } TR_catch e { TR_print(e, e.TR_code) }\n")
		if err != nil || out != "5 none\n" {
			t.Errorf("%s: önerilen yazım: çıktı %q, hata %v", name, out, err)
		}
	}
}
