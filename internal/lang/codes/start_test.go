package codes

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural: kurulamayan Türkçe liste (bozuk ya da yarım değişmiş tr.json) paketi başlatırken
// paniklemez: adlar kimliklere düşer, hata saklanır. İki yol: liste okunamaz ve liste bir kodu
// eksik taşır (Name eksik kodda paniklerdi; sözcük çözümleyici adları başlatmada ister).
func TestABrokenTurkishListFallsBackToIdentities(t *testing.T) {
	var short []string // TR_kw_if satırı silinmiş liste
	for _, line := range strings.Split(string(trJSON), "\n") {
		if !strings.Contains(line, `"TR_kw_if":`) {
			short = append(short, line)
		}
	}
	for name, tr := range map[string]string{"unreadable": `{ "TR_kw_if": `, "incomplete": strings.Join(short, "\n")} {
		langs := map[string][]byte{SourceLanguage: []byte(tr)}
		t.Run(name, func(t *testing.T) {
			var d, a *Language
			var displayErr, activeErr error
			if err := testkit.PanicError(func() { d, a, displayErr, activeErr = startLanguages(SourceLanguage, langs) }); err != nil {
				t.Fatal(err)
			}
			if activeErr == nil || displayErr != nil || d != a || a.Code != SourceLanguage {
				t.Fatalf("active error %v, display error %v, display %p, active %p (%s)", activeErr, displayErr, d, a, a.Code)
			}
			for _, k := range Active().Codes() {
				if got := a.Name(k); got != k.String() {
					t.Errorf("%s falls back to %q, want its identity", k, got)
				}
				if got, ok := a.Find(k.Namespace(), k.String()); !ok || got != k {
					t.Errorf("Find(%s) = %s %v", k, got, ok)
				}
			}
		})
	}
}

// Türkçe liste geçerliyse kurulamayan bir gösterim dili (geçersiz TR_DIL) yalnız gösterim hatasıdır.
func TestAnInvalidDisplayLanguageKeepsTheTurkishList(t *testing.T) {
	d, a, displayErr, activeErr := startLanguages("xx", languages)
	if activeErr != nil || displayErr == nil || d != a || a.Name(KwIf) == KwIf.String() {
		t.Fatalf("active error %v, display error %v, display %s, if %q", activeErr, displayErr, d.Code, a.Name(KwIf))
	}
}
