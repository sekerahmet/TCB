package lexer

import (
	"os"
	"regexp"
	"testing"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kural (2.3, 2.1a): adın biçimi ve benzerliği üç Unicode tablosuna dayanır: Go'nun unicode paketi
// (harf ve rakam sınıfları), x/text'in NFC'si ve benzerlik çizelgesi (confusables_generated.go). Üçü
// aynı Unicode sürümünden olmalıdır: daha yeni bir Go ile derlenen trlang aynı kaynağı başka okursa
// bu sessiz kalırdı (DENETİM 586 N1). Çizelgenin sürümü üretilen dosyanın başlığındadır.
func TestUnicodeTablesHaveOneVersion(t *testing.T) {
	data, err := os.ReadFile(reporoot.Path("internal/lang/lexer/confusables_generated.go"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`\(Unicode ([0-9.]+), UTS #39\)`).FindSubmatch(data)
	if m == nil {
		t.Fatal("confusables_generated.go'nun başlığında Unicode sürümü yok")
	}
	chart := string(m[1])
	for table, version := range map[string]string{"Go'nun unicode paketi": unicode.Version, "x/text'in NFC'si": norm.Version} {
		if version != chart {
			t.Errorf("Unicode sürümleri ayrı: %s %s, benzerlik çizelgesi %s; üçü aynı sürüm olmalı (go.mod ve tools/gen/confusables)",
				table, version, chart)
		}
	}
}
