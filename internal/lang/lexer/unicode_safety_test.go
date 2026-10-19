package lexer

import (
	"strings"
	"testing"
)

func lexErrors(src string) []string {
	l := New(src)
	for l.NextToken().Type != TOKEN_EOF {
	}
	return l.Errors()
}

// Bidirectional controls reorder how a line is displayed ("Trojan Source"); they are rejected
// in code, comments and strings alike.
func TestBidiControlsRejected(t *testing.T) {
	for _, src := range []string{
		"değişken a = 1 // yorum ‮ gizli\n",
		"değişken a = \"metin ⁦ içinde\"\n",
		"değişken‏ a = 1\n",
	} {
		errs := lexErrors(src)
		if len(errs) != 1 || !strings.Contains(errs[0], "yazı yönünü değiştiren") || !strings.HasPrefix(errs[0], "Satır 1, Sütun ") {
			t.Errorf("%q: %v", src, errs)
		}
	}
}

// Invisible filler letters and names that mix look-alike scripts are rejected; a name in one
// script (Turkish, Cyrillic, Greek) is fine.
func TestConfusableNames(t *testing.T) {
	for src, want := range map[string]string{
		"değişken ㅤ = 1\n":           "görünmez bir harf",
		"değişken yetkі = 1\n":       "farklı yazı sistemlerinin",
		"değişken аdmin = 1\n":       "farklı yazı sistemlerinin",
		"değişken öğrenme = 1\n":     "",
		"değişken привет = 1\n":      "",
		"değişken αβγ = 1\n":         "",
		"değişken toplam_2024 = 1\n": "",
	} {
		errs := lexErrors(src)
		if want == "" {
			if len(errs) > 0 {
				t.Errorf("%q: unexpected %v", src, errs)
			}
		} else if len(errs) != 1 || !strings.Contains(errs[0], want) {
			t.Errorf("%q: %q expected, got %v", src, want, errs)
		}
	}
}
