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
	t.Setenv(LanguageEnvVar, "xx")
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
