package lexer

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func lexErrors(src string) []error {
	l := New(src)
	for l.NextToken().Type != TokEOF {
	}
	return l.Errs()
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
		if len(errs) != 1 || !messages.Has(errs[0], messages.H0011) || !hasLine(errs[0], 1) {
			t.Errorf("%q: %v", src, errs)
		}
	}
}

// Invisible filler letters and names that mix look-alike scripts are rejected; a name in one
// script (Turkish, Cyrillic, Greek) is fine.
func TestConfusableNames(t *testing.T) {
	for src, want := range map[string]messages.Code{
		"değişken ㅤ = 1\n":           messages.H0012,
		"değişken yetkі = 1\n":       messages.H0013,
		"değişken аdmin = 1\n":       messages.H0013,
		"değişken öğrenme = 1\n":     0,
		"değişken привет = 1\n":      0,
		"değişken αβγ = 1\n":         0,
		"değişken toplam_2024 = 1\n": 0,
	} {
		errs := lexErrors(src)
		if want == 0 {
			if len(errs) > 0 {
				t.Errorf("%q: unexpected %v", src, errs)
			}
		} else if len(errs) != 1 || !messages.Has(errs[0], want) {
			t.Errorf("%q: %v expected, got %v", src, want, errs)
		}
	}
}

// hasLine, hatanın verilen satırda olduğunu söyler.
func hasLine(err error, line int) bool {
	l, _, ok := messages.Pos(err)
	return ok && l == line
}
