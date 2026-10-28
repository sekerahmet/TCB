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
		"TR_var a = 1 # yorum\u202e gizli\n",
		"TR_var a = \"metin \u2066 içinde\"\n",
		"TR_var\u200f a = 1\n",
	} {
		errs := lexErrors(src)
		if len(errs) != 1 || !messages.Has(errs[0], messages.H0011) || !hasLine(errs[0], 1) {
			t.Errorf("%q: %v", src, errs)
		}
	}
}

// Invisible filler letters in a name are rejected (2.1a). A name that mixes scripts is a warning
// of the checker (H1019; MixedScripts here); a name in one script (Turkish, Cyrillic, Greek) is fine.
func TestConfusableNames(t *testing.T) {
	if errs := lexErrors("TR_var ㅤ = 1\n"); len(errs) != 1 || !messages.Has(errs[0], messages.H0012) {
		t.Errorf("filler letter: H0012 expected, got %v", errs)
	}
	for name, mixed := range map[string]bool{
		"yetkі": true, "аdmin": true, "θ_eski": true,
		"öğrenme": false, "привет": false, "αβγ": false, "toplam_2024": false,
	} {
		if errs := lexErrors("TR_var " + name + " = 1\n"); len(errs) > 0 {
			t.Errorf("%q: the lexer gives no error, got %v", name, errs)
		}
		if _, _, got := MixedScripts(name); got != mixed {
			t.Errorf("MixedScripts(%q) = %v, want %v", name, got, mixed)
		}
	}
}

// hasLine, hatanın verilen satırda olduğunu söyler.
func hasLine(err error, line int) bool {
	l, _, ok := messages.Pos(err)
	return ok && l == line
}
