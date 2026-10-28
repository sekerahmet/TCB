package codes

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural: kurulamayan Türkçe liste (bozuk ya da yarım değişmiş tr.json) paketi başlatırken
// paniklemez: hata saklanır, gösterim de içerisi gibi kimliklere düşer (içerisi her zaman
// kimliktir). İki yol: liste okunamaz ve liste bir kodu eksik taşır.
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
			var displayErr, sourceErr error
			if err := testkit.PanicError(func() { d, a, displayErr, sourceErr = startLanguages(SourceLanguage, langs) }); err != nil {
				t.Fatal(err)
			}
			if sourceErr == nil || displayErr != nil || d != a || a.Code != IdentityLanguage {
				t.Fatalf("source error %v, display error %v, display %s, internal %s", sourceErr, displayErr, d.Code, a.Code)
			}
			for _, k := range a.Codes() {
				if got := d.Name(k); got != k.String() {
					t.Errorf("%s falls back to %q, want its identity", k, got)
				}
				if got, ok := d.Find(k.Namespace(), k.String()); !ok || got != k {
					t.Errorf("Find(%s) = %s %v", k, got, ok)
				}
			}
		})
	}
}

// Kural: Türkçe liste geçerliyse kurulamayan bir gösterim dili (geçersiz TR_DIL) yalnız gösterim
// hatasıdır: gösterim Türkçe kalır, içerisi kimliktir.
func TestAnInvalidDisplayLanguageKeepsTheTurkishList(t *testing.T) {
	d, a, displayErr, sourceErr := startLanguages("xx", languages)
	if sourceErr != nil || displayErr == nil || d.Code != SourceLanguage || d.Name(KwIf) != "eğer" || a.Code != IdentityLanguage {
		t.Fatalf("source error %v, display error %v, display %s, if %q, internal %s", sourceErr, displayErr, d.Code, d.Name(KwIf), a.Code)
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
		"TR_append":             "append",
		"liste.TR_append":       "liste.append",
		"TR_append()":           "append()",
		"TR_embedding.TR_table": "TR_embedding.TR_table",
	} {
		if got := messages.IdentityNames(text, "en"); got != want {
			t.Errorf("IdentityNames(%q) = %q, beklenen %q", text, got, want)
		}
	}
}
