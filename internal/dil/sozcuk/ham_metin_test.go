package sozcuk

import (
	"strings"
	"testing"
)

// Bilinmeyen kaçış dizisi sözdizimi hatasıdır (2.6): ters bölü sessizce atılmaz. Hata
// ters bölünün konumunu gösterir ve ham metni önerir.
func TestBilinmeyenKacisHatadir(t *testing.T) {
	testler := []struct {
		girdi, konum, dizi string
	}{
		{`"\d+"`, "Satır 1, Sütun 2:", `'\d'`},
		{`x = "C:\veri"`, "Satır 1, Sütun 8:", `'\v'`},
		{`'a\qb'`, "Satır 1, Sütun 3:", `'\q'`},
		{"\"a\\\nb\"", "Satır 1, Sütun 3:", `'\' ve satır sonu`},
		{"\"a\\\r\nb\"", "Satır 1, Sütun 3:", `'\' ve satır sonu`},
		{"\"{f(\\u)}\"", "Satır 1, Sütun 5:", `'\u'`},
	}
	for _, tt := range testler {
		l := New(tt.girdi)
		for l.NextToken().Type != TOKEN_EOF {
		}
		h := l.Errors()
		if len(h) != 1 || !strings.HasPrefix(h[0], tt.konum) || !strings.Contains(h[0], "bilinmeyen kaçış dizisi "+tt.dizi) ||
			!strings.Contains(h[0], "ham metin") {
			t.Errorf("%q: %s konumunda %s hatası bekleniyordu, alınan: %v", tt.girdi, tt.konum, tt.dizi, h)
		}
	}
	// Bilinen kaçışlar hata değildir
	for _, g := range []string{`"\n\t\r\"\'\\\{\}"`, `'\''`} {
		hatasizTokenlar(t, g)
	}
}

// Ham metin (2.6a): kaçış ve yerleştirme yoktur; süslü parantezler '\{' '\}' işaretçileriyle
// taşınır (ayrıştırıcı yerleştirme saymaz), satır sonu kaynaktan bağımsız olarak '\n'dir.
func TestHamMetin(t *testing.T) {
	L, R := string(EscapedLBrace), string(EscapedRBrace)
	testler := []struct {
		girdi, lit string
	}{
		{"`C:\\veri\\yeni`", `C:\veri\yeni`},
		{"`\\d{3}`", `\d` + L + "3" + R},
		{"`{ad}`", L + "ad" + R},
		{"``", ""},
		{"`\"'`", `"'`},
		{"`a\nb`", "a\nb"},
		{"`a\r\nb`", "a\nb"},
		{"`a\rb`", "a\rb"},
		{"`\\`", `\`},
	}
	for _, tt := range testler {
		tokenlariDogrula(t, tt.girdi, []beklenen{{TOKEN_STRING, tt.lit}, {TOKEN_EOF, ""}})
	}
	// Ham metinden sonra sözcük birimleri ve konumlar sürer
	l := New("x = `a\nb` + 1")
	var son Token
	for tok := l.NextToken(); tok.Type != TOKEN_EOF; tok = l.NextToken() {
		son = tok
	}
	if son.Literal != "1" || son.Line != 2 || son.Column != 6 {
		t.Errorf("ham metinden sonraki konum yanlış: %+v", son)
	}
}

func TestKapanmamisHamMetin(t *testing.T) {
	l := New("x = `açık\nmetin")
	for l.NextToken().Type != TOKEN_EOF {
	}
	h := l.Errors()
	if len(h) != 1 || !strings.Contains(h[0], "Satır 1, Sütun 5: kapatılmamış ham metin") {
		t.Errorf("kapatılmamış ham metin hatası bekleniyordu: %v", h)
	}
}
