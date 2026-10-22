package messages

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"
)

// Kodlu hata metnini etkin dilde kurar; sarmaladığı hata errors.Is/As ile görülür; Has kodu
// zincirin derinliğinde de bulur.
func TestErrorChain(t *testing.T) {
	inner := New(H7001, fs.ErrNotExist)
	outer := fmt.Errorf("Satır 3: %w", New(H2000, "json_metin", inner))
	if !Has(outer, H2000) || !Has(outer, H7001) || Has(outer, H7003) || Has(nil, H2000) {
		t.Fatal("Has zinciri izlemiyor")
	}
	if !errors.Is(outer, fs.ErrNotExist) {
		t.Fatal("sarmalanan hata errors.Is ile görülmeli")
	}
	if m, ok := As(outer); !ok || m.Code != H2000 {
		t.Fatalf("As: %v %v", m, ok)
	}
	if got := outer.Error(); !strings.HasPrefix(got, "Satır 3: json_metin: JSON çözümleme hatası: ") {
		t.Fatalf("metin: %s", got)
	}
}

// İç hata tek ortak iletidir; ayrıntı geliştirici içindir.
func TestInternal(t *testing.T) {
	err := Internal("vector width must be 2 or 4")
	if !Has(err, H9000) || !strings.Contains(err.Error(), "TR'nin hatası") || !strings.HasSuffix(err.Error(), "vector width must be 2 or 4") {
		t.Fatalf("iç hata: %v", err)
	}
}

// Metni olmayan bir dil seçilirse ileti Türkçe gösterilir.
func TestUnknownLanguageFallsBack(t *testing.T) {
	useLanguage(t, "xx")
	if got := Text(H7004, "x"); got != "CSV okuma hatası: x" {
		t.Fatalf("metin: %s", got)
	}
}

func TestParseCode(t *testing.T) {
	if c, err := Parse("H7003"); err != nil || c != H7003 || c.String() != "H7003" {
		t.Fatalf("Parse: %v %v", c, err)
	}
	for _, bad := range []string{"H700", "G7003", "H70a3", "H-001"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("%q kabul edildi", bad)
		}
	}
}

func TestInternalfAndHasAny(t *testing.T) {
	err := Internalf("bad %s %d", "x", 3)
	if !Has(err, H9000) || !strings.Contains(err.Error(), "bad x 3") {
		t.Fatalf("Internalf: %v", err)
	}
	wrapped := New(H2000, "f", New(H2010, "a", "b"))
	if !HasAny(wrapped, H9000, H2010) || HasAny(wrapped, H9000, H2001) || HasAny(nil, H2000) {
		t.Fatal("HasAny zinciri yanlış okuyor")
	}
}

// useLanguage etkin dili test süresince değiştirir (süreç dili LanguageEnvVar'dan bir kez okunur).
func useLanguage(t *testing.T, lang string) {
	t.Helper()
	old := activeLanguage
	activeLanguage = func() string { return lang }
	t.Cleanup(func() { activeLanguage = old })
}

// withoutTranslation bir kodun çevirisini test süresince katalogdan çıkarır: en.json tamamlandığı
// için çevrilmemiş kod kalmadı, yedek davranışı bu yolla sınanır.
func withoutTranslation(t *testing.T, lang string, code Code) {
	t.Helper()
	load()
	c := catalogs[lang]
	saved, ok := c.templates[code]
	if !ok {
		t.Fatalf("%s: %s zaten çevrilmemiş", lang, code)
	}
	delete(c.templates, code)
	t.Cleanup(func() { c.templates[code] = saved })
}

// Bir ileti bütünüyle tek dilde gösterilir: içindeki kodlu hatalardan biri etkin dilde
// çevrilmemişse çerçeve de kaynak dilde (Türkçe) kalır; yarı İngilizce yarı Türkçe satır olmaz.
func TestFallbackKeepsOneLanguage(t *testing.T) {
	useLanguage(t, "en")
	withoutTranslation(t, "en", H7004)
	if got := At(1, 2, New(H7004, "x")).Error(); got != "Satır 1, Sütun 2: CSV okuma hatası: x" {
		t.Errorf("çevrilmemiş iç ileti: %q", got)
	}
	if got := At(1, 2, New(H2000, "f", "x")).Error(); got != "Line 1, column 2: f: x" {
		t.Errorf("çevrilmiş iç ileti: %q", got)
	}
	if got := Text(H2000, "f", New(H7004, "x")); got != "f: CSV okuma hatası: x" {
		t.Errorf("Text: %q", got)
	}
	if got := Text(H7004, At(1, 0, New(H2000, "f", "x"))); got != "CSV okuma hatası: Satır 1: f: x" {
		t.Errorf("çerçeve Türkçeyken iç ileti de Türkçe olmalı: %q", got)
	}
}

// Süreç dili bir kez okunur (codes paketi de öyle): ortam sonradan değişse de iletiler ve adlar
// aynı dilde kalır.
func TestLanguageReadOnce(t *testing.T) {
	first := ActiveLanguage()
	t.Setenv(LanguageEnvVar, "en")
	if ActiveLanguage() != first {
		t.Fatalf("dil ortamdan yeniden okundu: %s → %s", first, ActiveLanguage())
	}
}

// Kural: Texts her hatanın metnini sırayla verir; boş girdi boş dizi verir (nil değil).
// Neden: lexer, parser ve denetleyicinin Errors yöntemleri aynı dönüşümü kendi içinde tekrarlıyordu.
func TestTextsRendersEveryErrorInOrder(t *testing.T) {
	errs := []error{New(H0003), fmt.Errorf("düz hata")}
	got := Texts(errs)
	if len(got) != 2 || got[0] != errs[0].Error() || got[1] != "düz hata" {
		t.Fatalf("Texts = %q", got)
	}
	if got := Texts(nil); got == nil || len(got) != 0 {
		t.Fatalf("Texts(nil) = %#v, boş dizi bekleniyordu", got)
	}
}
