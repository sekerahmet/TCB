package codes

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural: kurulamayan Türkçe liste (bozuk ya da yarım değişmiş tr.json) paketi başlatırken
// paniklemez: adlar kimliklere düşer, hata saklanır. İki yol: liste okunamaz ve liste bir kodu
// eksik taşır (Name eksik kodda paniklerdi; sözcük çözümleyici adları başlatmada ister).
func TestABrokenTurkishListFallsBackToIdentities(t *testing.T) {
	t.Skip("ARŞİV (iç biçim kod, 2026-10-26): test Türkçe metni dil belirtmeden okuyor ya da içerinin Türkçe olduğunu varsayıyor; yeniden yazılacak")
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
	t.Skip("ARŞİV (iç biçim kod, 2026-10-26): test Türkçe metni dil belirtmeden okuyor ya da içerinin Türkçe olduğunu varsayıyor; yeniden yazılacak")
	d, a, displayErr, activeErr := startLanguages("xx", languages)
	if activeErr != nil || displayErr == nil || d != a || a.Name(KwIf) == KwIf.String() {
		t.Fatalf("active error %v, display error %v, display %s, if %q", activeErr, displayErr, d.Code, a.Name(KwIf))
	}
}

// TestProgramRunsOnIdentities program içeride kimliklerle çalışır; hiçbir insan dili içerinin dili
// değildir (Ahmet, 2026-10-26: "türkçe ve ingilizce sadece gösterim dilidir. disk ve programla dili
// hep kodlardan çalışır"). İçerinin tablosu yeniden bir yazım dili yapılırsa bu test kalır.
func TestProgramRunsOnIdentities(t *testing.T) {
	if Active().Code == SourceLanguage {
		t.Fatalf("içerinin dili %q: program içeride bir yazım diliyle çalışmamalı", Active().Code)
	}
	for _, k := range Active().Codes() {
		if got := Active().Name(k); got != k.String() {
			t.Errorf("%s içeride %q adıyla duruyor; içeride ad kimliktir", k, got)
		}
	}
	if Source().Code != SourceLanguage || Source().Name(FnPrint) == FnPrint.String() {
		t.Fatalf("Türkçe bir yazım dili olarak yüklenmeli: %s, %q", Source().Code, Source().Name(FnPrint))
	}
}

// Kural: iletiye giren bir kimlik (TR_mem_append, liste.TR_mem_append) iletinin dilinde yazılır;
// kimliklerden kurulmuş bir yol (ağırlık anahtarı "TR_mem_embedding.TR_mem_table") ise bir
// değerdir ve olduğu gibi kalır: Python'un ve Swift'in değer gösterimi gibi veri çevrilmez,
// yarım çeviri ("TR_mem_embedding.tablo") ne kod ne ad olurdu.
func TestIdentityNamesLeaveAnIdentityPathAsItIs(t *testing.T) {
	for text, want := range map[string]string{
		"TR_mem_append":                 "append",
		"liste.TR_mem_append":           "liste.append",
		"TR_mem_append()":               "append()",
		"TR_mem_embedding.TR_mem_table": "TR_mem_embedding.TR_mem_table",
	} {
		if got := messages.IdentityNames(text, "en"); got != want {
			t.Errorf("IdentityNames(%q) = %q, beklenen %q", text, got, want)
		}
	}
}
