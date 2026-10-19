package lexer

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Metinler
// ---------------------------------------------------------------------------

func TestSimpleStrings(t *testing.T) {
	tests := []struct {
		src, lit string
	}{
		{`"merhaba"`, "merhaba"},
		{`'merhaba'`, "merhaba"},
		{`""`, ""},
		{`"çğışöü İĞŞ"`, "çğışöü İĞŞ"},
		{`"tek ' tırnak"`, "tek ' tırnak"},
		{`'çift " tırnak'`, `çift " tırnak`},
		{`"a}b"`, "a}b"}, // eşi olmayan '}' düz karakterdir
	}
	for _, tt := range tests {
		tokenlariDogrula(t, tt.src, []expected{{TOKEN_STRING, tt.lit}, {TOKEN_EOF, ""}})
	}
}
func TestStringEscapes(t *testing.T) {
	tests := []struct {
		src, lit string
	}{
		{`"a\nb"`, "a\nb"},
		{`"a\tb"`, "a\tb"},
		{`"a\rb"`, "a\rb"},
		{`"a\"b"`, `a"b`},
		{`'a\'b'`, "a'b"},
		{`"a\\b"`, `a\b`},
		// Kaçışlı süslü parantezler özel kullanım alanı karakterleriyle taşınır
		{`"\{düz\}"`, string(EscapedLBrace) + "düz" + string(EscapedRBrace)},
		// '\\{' ise ters bölü + interpolasyon başlangıcıdır
		{`"\\{x}"`, `\{x}`},
	}
	for _, tt := range tests {
		tokenlariDogrula(t, tt.src, []expected{{TOKEN_STRING, tt.lit}, {TOKEN_EOF, ""}})
	}
}

// İnterpolasyon bölümleri ayrıştırıcıya olduğu gibi aktarılır.
func TestStringInterpolation(t *testing.T) {
	tests := []struct {
		src, lit string
	}{
		{`"Merhaba {ad}!"`, "Merhaba {ad}!"},
		{`"{a} + {b} = {a + b}"`, "{a} + {b} = {a + b}"},
		// İnterpolasyon içindeki alt metinlerde aynı tırnak kullanılabilir
		{`"{p["ad"]}"`, `{p["ad"]}`},
		{`"{p['ad']}"`, `{p['ad']}`},
		// Alt metin içindeki '}' interpolasyonu kapatmaz
		{`"{f("}")}"`, `{f("}")}`},
		// İç içe süslü parantez (sözlük sabiti)
		{`"{ {'a': 1}['a'] }"`, `{ {'a': 1}['a'] }`},
		// Kaçışlı parantez ile interpolasyon bir arada
		{`"\{{x}\}"`, string(EscapedLBrace) + "{x}" + string(EscapedRBrace)},
	}
	for _, tt := range tests {
		tokenlariDogrula(t, tt.src, []expected{{TOKEN_STRING, tt.lit}, {TOKEN_EOF, ""}})
	}
}

// Metin birden fazla satıra yayılabilir; sonraki token'ın satırı doğru olmalı.
func TestMultilineString(t *testing.T) {
	toks := tokensNoErrors(t, "\"bir\niki\nüç\" x")
	if toks[0].Type != TOKEN_STRING || toks[0].Literal != "bir\niki\nüç" {
		t.Fatalf("çok satırlı metin yanlış okundu: %v", toks[0])
	}
	if toks[0].Line != 1 || toks[0].Column != 1 {
		t.Errorf("metin konumu 1:1 bekleniyordu, %d:%d geldi", toks[0].Line, toks[0].Column)
	}
	if toks[1].Line != 3 || toks[1].Column != 5 {
		t.Errorf("'x' konumu 3:5 bekleniyordu, %d:%d geldi", toks[1].Line, toks[1].Column)
	}
}
func containsError(errs []string, parts ...string) bool {
	for _, e := range errs {
		all := true
		for _, p := range parts {
			if !strings.Contains(e, p) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}
func TestUnterminatedString(t *testing.T) {
	tests := []struct {
		name, src, srcPos string
	}{
		{"tek satır", `"merhaba`, "Satır 1, Sütun 1:"},
		{"tek tırnak", `'merhaba`, "Satır 1, Sütun 1:"},
		{"sonraki satırlarda", "değişken a = 1\n  yazdır(\"merhaba)\nb = 2\n", "Satır 2, Sütun 10:"},
		{"ters bölü ile biten", `"abc\`, "Satır 1, Sütun 1:"},
		{"kaçışlı tırnak ile biten", `x = "abc\"`, "Satır 1, Sütun 5:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toks, errs := allTokens(t, tt.src)
			if len(errs) != 1 {
				t.Fatalf("tek bir hata bekleniyordu, gelen: %v", errs)
			}
			if !containsError(errs, tt.srcPos, "kapatılmamış metin") {
				t.Errorf("hata %q ve 'kapatılmamış metin' içermeli, gelen: %q", tt.srcPos, errs[0])
			}
			if toks[len(toks)-1].Type != TOKEN_EOF {
				t.Errorf("son token DOSYA_SONU olmalı: %v", toks)
			}
		})
	}
}
func TestUnterminatedInterpolation(t *testing.T) {
	tests := []struct {
		name, src, srcPos string
	}{
		{"dosya sonu", `"merhaba {ad"`, "Satır 1, Sütun 10:"},
		{"satır sonu", "yazdır(\"x {a\")\nb = 1", "Satır 1, Sütun 11:"},
		{"ikinci satırda", "a = 1\n\"ok\" + \"{ {b}\"", "Satır 2, Sütun 9:"},
		{"CRLF satır sonu", "a = 1\r\n\"x {a\r\nb\"", "Satır 2, Sütun 4:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := allTokens(t, tt.src)
			if len(errs) == 0 {
				t.Fatalf("hata bekleniyordu")
			}
			if !containsError(errs, tt.srcPos, "'{' kapatılmamış") {
				t.Errorf("hata %q ve \"'{' kapatılmamış\" içermeli, gelen: %v", tt.srcPos, errs)
			}
		})
	}
}

// Hatalı bir metinden sonra sözcük çözümleyici okumaya devam etmeli ve sonraki hataları da bildirmeli.
func TestMultipleLexerErrors(t *testing.T) {
	_, errs := allTokens(t, "\"{a\n\"tamam\"\n'son")
	if len(errs) != 2 {
		t.Fatalf("iki hata bekleniyordu, gelen: %v", errs)
	}
	if !containsError(errs[:1], "Satır 1, Sütun 2:", "'{' kapatılmamış") {
		t.Errorf("ilk hata yanlış: %q", errs[0])
	}
	if !containsError(errs[1:], "Satır 3, Sütun 1:", "kapatılmamış metin") {
		t.Errorf("ikinci hata yanlış: %q", errs[1])
	}
}

// CRLF satır sonlu dosyada çok satırlı metnin değeri '\r' içermez: satır sonu kaynağın
// biçiminden bağımsız olarak '\n'dir ('\r' yalnızca kaçışla, \r, yazılır).
func TestCRLFInString(t *testing.T) {
	toks := tokensNoErrors(t, "\"a\r\nb\" + 'c\r\nd' + \"e\\rf\"")
	if toks[0].Literal != "a\nb" || toks[2].Literal != "c\nd" || toks[4].Literal != "e\rf" {
		t.Fatalf("metin değerleri: %q %q %q", toks[0].Literal, toks[2].Literal, toks[4].Literal)
	}
}
