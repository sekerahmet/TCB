package lexer

import "testing"

// Token.String hata ayıklama çıktısında konum, tür ve değeri gösterir.
func TestTokenString(t *testing.T) {
	tok := Token{Type: TokIdent, Literal: "ğ", Line: 3, Column: 7}
	if got, want := tok.String(), `[3:7] TANIMLAYICI ("ğ")`; got != want {
		t.Errorf("Token.String() = %q, beklenen %q", got, want)
	}
}
