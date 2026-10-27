package lexer

import (
	"fmt"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ---------------------------------------------------------------------------
// Yorumlar (2.2)
// ---------------------------------------------------------------------------

// Yorum metin dışındaki '#' ile başlar ve satır sonuna kadar sürer; '//' tam bölme işlecidir,
// metindeki '#' ve '//' yorum değildir.
func TestComments(t *testing.T) {
	tests := []struct {
		name, src string
		want      []expected
	}{
		{"satır sonu", "a # yorum {\nb", []expected{{TokIdent, "a"}, {TokIdent, "b"}}},
		{"dosya sonunda", "a # son", []expected{{TokIdent, "a"}}},
		{"art arda", "# bir\n# iki\nx", []expected{{TokIdent, "x"}}},
		{"başlık ve #! birer yorumdur", "#!/usr/bin/env tr\n# TR 4\nx", []expected{{TokIdent, "x"}}},
		{"bölme yorum değildir", "a / b", []expected{{TokIdent, "a"}, {TokSlash, "/"}, {TokIdent, "b"}}},
		{"satır içindeki // tam bölmedir", "a // b", []expected{{TokIdent, "a"}, {TokSlashSlash, "//"}, {TokIdent, "b"}}},
		{"metin içindeki # yorum değildir", `"# değil" x`, []expected{{TokString, "# değil"}, {TokIdent, "x"}}},
		{"metin içindeki // yorum değildir", `"http://a" x`, []expected{{TokString, "http://a"}, {TokIdent, "x"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkTokens(t, tt.src, append(tt.want, expected{TokEOF, ""}))
		})
	}
}

// Satır başındaki '//' ve '/*' (C ailesinin yorumu) sözdizimi hatasıdır: ileti '#'i önerir, satırın
// (ya da '*/'ya kadar yorumun) geri kalanı atlanır, tek hata verilir.
func TestCStyleCommentIsAnError(t *testing.T) {
	for _, c := range []struct {
		src, token string
		col        int
	}{
		{"a = 1\n// yorum ** \"\nb", "//", 1},
		{"a = 1\n  /* çok\nsatırlı */ b", "/*", 3},
	} {
		toks, errList := allTokens(t, c.src)
		if len(errList) != 1 || !hasErr(errList, 2, c.col, messages.H0015) {
			t.Fatalf("%q: tek hata (H0015, satır 2, sütun %d) bekleniyordu, gelen: %v", c.src, c.col, errList)
		}
		want := fmt.Sprintf("Line 2, column %d: '%s' is not a comment in TR; comments are written with '#': # note", c.col, c.token)
		if got := messages.ErrorIn(errList[0], "en"); got != want {
			t.Errorf("%q: ileti %q, beklenen %q", c.src, got, want)
		}
		var names []string
		for _, tk := range toks {
			if tk.Type == TokIdent {
				names = append(names, tk.Literal)
			}
		}
		if fmt.Sprint(names) != "[a b]" {
			t.Errorf("%q: adlar %v, [a b] bekleniyordu", c.src, names)
		}
	}
	// Kapanmayan '/*' dosyanın sonuna kadar atlanır; çökmez, tek hata verir.
	for _, s := range []string{"/*", "/**", "/* *"} {
		if _, errs := allTokens(t, s); len(errs) != 1 {
			t.Errorf("%q: tek bir hata bekleniyordu, gelen: %v", s, errs)
		}
	}
}

func TestLineNumberAfterComment(t *testing.T) {
	toks := tokensNoErrors(t, "# bir\n# iki\n# üç\n  x # yorum\n  y")
	if toks[0].Line != 4 || toks[0].Column != 3 {
		t.Errorf("'x' konumu 4:3 bekleniyordu, %d:%d geldi", toks[0].Line, toks[0].Column)
	}
	if toks[1].Line != 5 || toks[1].Column != 3 {
		t.Errorf("'y' konumu 5:3 bekleniyordu, %d:%d geldi", toks[1].Line, toks[1].Column)
	}
}
