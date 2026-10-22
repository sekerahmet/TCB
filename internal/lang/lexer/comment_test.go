package lexer

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ---------------------------------------------------------------------------
// Yorumlar
// ---------------------------------------------------------------------------

func TestComments(t *testing.T) {
	tests := []struct {
		name, src string
		want      []expected
	}{
		{"çift bölü", "a // yorum {\nb", []expected{{TokIdent, "a"}, {TokIdent, "b"}}},
		{"blok", "a /* çok\nsatırlı ' yorum */ b", []expected{{TokIdent, "a"}, {TokIdent, "b"}}},
		{"dosya sonunda", "a // son", []expected{{TokIdent, "a"}}},
		{"art arda", "// bir\n// iki\n/* üç */\nx", []expected{{TokIdent, "x"}}},
		{"blok içinde yıldız", "/* a * b ** c */x", []expected{{TokIdent, "x"}}},
		{"bölme yorum değildir", "a / b", []expected{{TokIdent, "a"}, {TokSlash, "/"}, {TokIdent, "b"}}},
		{"metin içindeki # yorum değildir", `"# değil" x`, []expected{{TokString, "# değil"}, {TokIdent, "x"}}},
		{"metin içindeki // yorum değildir", `"http://a" x`, []expected{{TokString, "http://a"}, {TokIdent, "x"}}},
		// Mevcut davranış: blok yorumlar iç içe geçmez; ilk '*/' yorumu bitirir
		{"iç içe blok yok", "/* a /* b */ c */", []expected{
			{TokIdent, "c"}, {TokAsterisk, "*"}, {TokSlash, "/"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkTokens(t, tt.src, append(tt.want, expected{TokEOF, ""}))
		})
	}
}

// '#' yorum değildir: açık bir hata verir, satırın geri kalanı atlanır (tek hata).
func TestHashIsNotComment(t *testing.T) {
	for _, c := range []struct {
		src, failed string
		col         int
		name        []string
	}{
		{"a # yorum ** \"\n b", "Satır 1, Sütun 3: '#' TR'de yorum değil; yorum için '//' yazın", 3, []string{"a", "b"}},
		{"# bir\nx", "Satır 1, Sütun 1: '#' TR'de yorum değil; yorum için '//' yazın", 1, []string{"x"}},
	} {
		toks, errList := allTokens(t, c.src)
		if len(errList) != 1 || !hasErr(errList, 1, c.col, messages.H0005) {
			t.Errorf("%q: tek hata (H0005, satır 1, sütun %d) bekleniyordu, gelen: %v", c.src, c.col, errList)
		}
		// Tam metin biçimi (Errors çıktısı) de sınanır.
		if got := messages.Texts(errList); len(got) != 1 || got[0] != c.failed {
			t.Errorf("%q: tam metin %q bekleniyordu, gelen: %v", c.src, c.failed, got)
		}
		var names []string
		for _, tk := range toks {
			if tk.Type == TokIdent {
				names = append(names, tk.Literal)
			}
		}
		if strings.Join(names, " ") != strings.Join(c.name, " ") {
			t.Errorf("%q: adlar %v, %v bekleniyordu", c.src, names, c.name)
		}
	}
}
func TestLineNumberAfterComment(t *testing.T) {
	toks := tokensNoErrors(t, "/* bir\niki\nüç */ x // yorum\n  y")
	if toks[0].Line != 3 || toks[0].Column != 7 {
		t.Errorf("'x' konumu 3:7 bekleniyordu, %d:%d geldi", toks[0].Line, toks[0].Column)
	}
	if toks[1].Line != 4 || toks[1].Column != 3 {
		t.Errorf("'y' konumu 4:3 bekleniyordu, %d:%d geldi", toks[1].Line, toks[1].Column)
	}
}
func TestUnterminatedBlockComment(t *testing.T) {
	toks, errs := allTokens(t, "a = 1\n  /* yorum\nb = 2\n")
	if len(toks) != 4 || toks[3].Type != TokEOF {
		t.Errorf("yorum dosya sonuna kadar sürmeli, gelen: %v", toks)
	}
	if len(errs) != 1 || !hasErr(errs, 2, 3, messages.H0006) {
		t.Errorf("H0006 kapatılmamış yorum hatası (satır 2, sütun 3) bekleniyordu, gelen: %v", errs)
	}

	// Yalnızca '/*' ve tek '*' ile biten girdiler de hata vermeli, çökmemeli
	for _, s := range []string{"/*", "/**", "/* *"} {
		_, errs := allTokens(t, s)
		if len(errs) != 1 {
			t.Errorf("%q: tek bir hata bekleniyordu, gelen: %v", s, errs)
		}
	}
	// '/**/' boş ama kapalı bir yorumdur
	checkTokens(t, "/**/x", []expected{{TokIdent, "x"}, {TokEOF, ""}})
}
