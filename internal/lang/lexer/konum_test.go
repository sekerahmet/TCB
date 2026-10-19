package lexer

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Satır ve sütun numaraları
// ---------------------------------------------------------------------------

type konum struct {
	lit       string
	line, col int
}

func konumlariDogrula(t *testing.T, input string, want []konum) {
	t.Helper()
	toks := hatasizTokenlar(t, input)
	if len(toks) != len(want) {
		t.Fatalf("%q: %d token bekleniyordu, %d geldi: %v", input, len(want), len(toks), toks)
	}
	for i, w := range want {
		if toks[i].Literal != w.lit || toks[i].Line != w.line || toks[i].Column != w.col {
			t.Errorf("token[%d] = %q @%d:%d, beklenen %q @%d:%d",
				i, toks[i].Literal, toks[i].Line, toks[i].Column, w.lit, w.line, w.col)
		}
	}
}

// Sütunlar 1 tabanlıdır ve bayt değil rune (karakter) sayar.
func TestSatirVeSutunNumaralari(t *testing.T) {
	girdi := "değişken ğüş = 1\n" +
		"eğer ğüş >= 10 {\n" +
		"\tyazdır(\"çok\")\n" +
		"}"
	konumlariDogrula(t, girdi, []konum{
		{"değişken", 1, 1}, {"ğüş", 1, 10}, {"=", 1, 14}, {"1", 1, 16},
		{"eğer", 2, 1}, {"ğüş", 2, 6}, {">=", 2, 10}, {"10", 2, 13}, {"{", 2, 16},
		{"yazdır", 3, 2}, {"(", 3, 8}, {"çok", 3, 9}, {")", 3, 14}, // sekme tek sütun sayılır
		{"}", 4, 1},
		{"", 4, 2},
	})
}
func TestBosSatirlarVeEOFKonumu(t *testing.T) {
	konumlariDogrula(t, "\n\n   x\n", []konum{{"x", 3, 4}, {"", 4, 1}})
	konumlariDogrula(t, "", []konum{{"", 1, 1}})
}
func TestCRLFSatirSonlari(t *testing.T) {
	girdi := "değişken a = 1\r\n" +
		"a += 2 // yorum\r\n" +
		"\r\n" +
		"yazdır(\"x\")\r\n"
	konumlariDogrula(t, girdi, []konum{
		{"değişken", 1, 1}, {"a", 1, 10}, {"=", 1, 12}, {"1", 1, 14},
		{"a", 2, 1}, {"+=", 2, 3}, {"2", 2, 6},
		{"yazdır", 4, 1}, {"(", 4, 7}, {"x", 4, 8}, {")", 4, 11},
		{"", 5, 1},
	})

	// '\r' hiçbir yerde GEÇERSİZ token üretmemeli
	for _, tok := range hatasizTokenlar(t, "a\r\nb\rc") {
		if tok.Type == TOKEN_ILLEGAL {
			t.Errorf("'\\r' için GEÇERSİZ token üretildi: %v", tok)
		}
	}
}

// Windows'ta bazı editörler dosyanın başına UTF-8 BOM (U+FEFF) ekler; sözcük çözümleyici bunu atlamalı.
func TestUTF8BOM(t *testing.T) {
	bom := string(rune(0xFEFF))
	konumlariDogrula(t, bom+"değişken x = 1", []konum{
		{"değişken", 1, 1}, {"x", 1, 10}, {"=", 1, 12}, {"1", 1, 14}, {"", 1, 15},
	})
	// Yalnızca dosya başındaki BOM atlanır; metnin ortasındakine dokunulmaz
	toks := hatasizTokenlar(t, "\"a"+bom+"b\"")
	if toks[0].Literal != "a"+bom+"b" {
		t.Errorf("metin içindeki U+FEFF korunmalı, gelen: %q", toks[0].Literal)
	}
}

// Token.String hata ayıklama çıktısında konum, tür ve değeri gösterir.
func TestTokenString(t *testing.T) {
	tok := Token{Type: TOKEN_IDENT, Literal: "ğ", Line: 3, Column: 7}
	if got, want := tok.String(), `[3:7] TANIMLAYICI ("ğ")`; got != want {
		t.Errorf("Token.String() = %q, beklenen %q", got, want)
	}
}
