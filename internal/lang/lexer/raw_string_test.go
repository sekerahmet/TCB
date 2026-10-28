package lexer

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Bilinmeyen kaçış dizisi sözdizimi hatasıdır (2.6): ters bölü sessizce atılmaz. Hata
// ters bölünün konumunu gösterir ve ham metni önerir. \{, \} ve tek tırnağın kaçışı kaçış değildir.
func TestUnknownEscapeIsError(t *testing.T) {
	tests := []struct {
		src    string
		col    int
		escape string // yalnızca durumu belgeler: hangi kaçış dizisi bilinmiyor
	}{
		{`"\d+"`, 2, `'\d'`},
		{`x = "C:\veri"`, 8, `'\v'`},
		{`"a\qb"`, 3, `'\q'`},
		{"\"a\\\nb\"", 3, `'\' ve satır sonu`},
		{"\"a\\\r\nb\"", 3, `'\' ve satır sonu`},
		{`"\{"`, 2, `'\{'`},
		{`"\'"`, 2, `'\''`},
	}
	for _, tt := range tests {
		h := lexErrors(tt.src)
		if len(h) != 1 || !hasErr(h, 1, tt.col, messages.H0008) {
			t.Errorf("%q: satır 1, sütun %d konumunda %s için H0008 bekleniyordu, alınan: %v", tt.src, tt.col, tt.escape, h)
		}
	}
	// Bilinen kaçışlar hata değildir
	checkTokens(t, `"\n\t\r\"\\\u{41}"`, []expected{{TokString, "\n\t\r\"\\A"}, {TokEOF, ""}})
}

// Ham metin (2.6a): kaçış ve yerleştirme yoktur, süslü parantez ve '\(' olduğu gibi kalır; satır
// sonu kaynaktan bağımsız olarak '\n'dir.
func TestRawString(t *testing.T) {
	tests := []struct {
		src, lit string
	}{
		{"`C:\\veri\\yeni`", `C:\veri\yeni`},
		{"`\\d{3}`", `\d{3}`},
		{"`{ad} \\(ad)`", `{ad} \(ad)`},
		{"``", ""},
		{"`\"'`", `"'`},
		{"`a\nb`", "a\nb"},
		{"`a\r\nb`", "a\nb"},
		{"`a\rb`", "a\rb"},
		{"`\\`", `\`},
	}
	for _, tt := range tests {
		checkTokens(t, tt.src, []expected{{TokString, tt.lit}, {TokEOF, ""}})
	}
	// Ham metinden sonra sözcük birimleri ve konumlar sürer
	l := New("x = `a\nb` + 1")
	var end Token
	for tok := l.NextToken(); tok.Type != TokEOF; tok = l.NextToken() {
		end = tok
	}
	if end.Literal != "1" || end.Line != 2 || end.Column != 6 {
		t.Errorf("ham metinden sonraki konum yanlış: %+v", end)
	}
}

func TestUnterminatedRawString(t *testing.T) {
	h := lexErrors("x = `açık\nmetin")
	if len(h) != 1 || !hasErr(h, 1, 5, messages.H0007) {
		t.Errorf("kapatılmamış ham metin hatası bekleniyordu: %v", h)
	}
}
