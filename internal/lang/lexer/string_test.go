package lexer

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ---------------------------------------------------------------------------
// Metinler (2.6, 2.7)
// ---------------------------------------------------------------------------

func TestSimpleStrings(t *testing.T) {
	tests := []struct {
		src, lit string
	}{
		{`"merhaba"`, "merhaba"},
		{`""`, ""},
		{`"çğışöü İĞŞ"`, "çğışöü İĞŞ"},
		{`"tek ' tırnak"`, "tek ' tırnak"},
		{`"a}b{c"`, "a}b{c"}, // süslü parantez sıradan karakterdir
	}
	for _, tt := range tests {
		checkTokens(t, tt.src, []expected{{TokString, tt.lit}, {TokEOF, ""}})
	}
}

// Metin ve yorum dışında tek tırnak sözdizimi hatasıdır, ileti çift tırnağı önerir; okuma metin
// gibi sürer.
func TestSingleQuoteIsAnError(t *testing.T) {
	toks, errs := allTokens(t, `x = 'a' + 1`)
	if len(errs) != 1 || !hasErr(errs, 1, 5, messages.H0016) {
		t.Errorf("H0016 (satır 1, sütun 5) bekleniyordu: %v", errs)
	}
	if len(toks) != 6 || toks[2].Type != TokString || toks[2].Literal != "a" {
		t.Errorf("okuma metin gibi sürmeli: %v", toks)
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
		{`"a\\b"`, `a\b`},
		{`"\u{41}\u{1F600}"`, "A\U0001F600"},
		// '\\(' ters bölü ve düz paranteztir, yerleştirme değil
		{`"\\(x)"`, `\(x)`},
	}
	for _, tt := range tests {
		checkTokens(t, tt.src, []expected{{TokString, tt.lit}, {TokEOF, ""}})
	}
	// \u{...} bir ile altı onaltılık rakamlı geçerli bir Unicode karakteridir; hata ters bölüyü gösterir.
	for _, src := range []string{`"\u{}"`, `"\u{1234567}"`, `"\u{110000}"`, `"\u{D800}"`, `"\u41"`} {
		if errs := lexErrors(src); len(errs) != 1 || !hasErr(errs, 1, 2, messages.H0017) {
			t.Errorf("%q: H0017 (satır 1, sütun 2) bekleniyordu: %v", src, errs)
		}
	}
}

// Yerleştirme bölümleri ayrıştırıcıya olduğu gibi aktarılır: değer '\(' yerine '(' taşır, aralıkları
// Interpolations verir. Süslü parantez yerleştirme değildir.
func TestStringInterpolation(t *testing.T) {
	tests := []struct {
		src, lit string
		spans    int
	}{
		{`"Merhaba \(ad)!"`, "Merhaba (ad)!", 1},
		{`"\(a) + \(b) = \(a + b)"`, "(a) + (b) = (a + b)", 3},
		// Yerleştirme içindeki alt metinlerde aynı tırnak kullanılabilir
		{`"\(p["ad"])"`, `(p["ad"])`, 1},
		// Alt metin içindeki ')' yerleştirmeyi kapatmaz; iç parantezler eşleşir
		{`"\(f(")"))"`, `(f(")"))`, 1},
		{`"\((1 + 2) * 3)"`, "((1 + 2) * 3)", 1},
		{`"{\"n\": \(n)}"`, `{"n": (n)}`, 1},
		{`"{ad} ${ad}"`, "{ad} ${ad}", 0},
	}
	for _, tt := range tests {
		l := New(tt.src)
		toks := allOf(l)
		if len(l.Errors()) > 0 || len(toks) != 1 || toks[0].Type != TokString || toks[0].Literal != tt.lit {
			t.Errorf("%q: %q metni bekleniyordu: %v %v", tt.src, tt.lit, toks, l.Errors())
			continue
		}
		if got := len(l.Interpolations(toks[0])); got != tt.spans {
			t.Errorf("%q: %d yerleştirme, beklenen %d", tt.src, got, tt.spans)
		}
	}
}

// Metin birden fazla satıra yayılabilir; sonraki token'ın satırı doğru olmalı.
func TestMultilineString(t *testing.T) {
	toks := tokensNoErrors(t, "\"bir\niki\nüç\" x")
	if toks[0].Type != TokString || toks[0].Literal != "bir\niki\nüç" {
		t.Fatalf("çok satırlı metin yanlış okundu: %v", toks[0])
	}
	if toks[0].Line != 1 || toks[0].Column != 1 {
		t.Errorf("metin konumu 1:1 bekleniyordu, %d:%d geldi", toks[0].Line, toks[0].Column)
	}
	if toks[1].Line != 3 || toks[1].Column != 5 {
		t.Errorf("'x' konumu 3:5 bekleniyordu, %d:%d geldi", toks[1].Line, toks[1].Column)
	}
}

func TestUnterminatedString(t *testing.T) {
	tests := []struct {
		name, src string
		line, col int
	}{
		{"tek satır", `"merhaba`, 1, 1},
		{"sonraki satırlarda", "TR_var a = 1\n  TR_print(\"merhaba)\nb = 2\n", 2, 12},
		{"ters bölü ile biten", `"abc\`, 1, 1},
		{"kaçışlı tırnak ile biten", `x = "abc\"`, 1, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toks, errs := allTokens(t, tt.src)
			if len(errs) != 1 {
				t.Fatalf("tek bir hata bekleniyordu, gelen: %v", errs)
			}
			if !hasErr(errs, tt.line, tt.col, messages.H0010) {
				t.Errorf("H0010 (satır %d, sütun %d) bekleniyordu, gelen: %q", tt.line, tt.col, errs[0])
			}
			if toks[len(toks)-1].Type != TokEOF {
				t.Errorf("son token DOSYA_SONU olmalı: %v", toks)
			}
		})
	}
}

// Yerleştirme aynı satırda kapanmalıdır (2.7); hata '\(' konumunu gösterir, metin orada biter.
func TestUnterminatedInterpolation(t *testing.T) {
	tests := []struct {
		name, src string
		line, col int
	}{
		{"dosya sonu", `"merhaba \(ad"`, 1, 10},
		{"satır sonu", "TR_print(\"x \\(a\")\nb = 1", 1, 13},
		{"ikinci satırda", "a = 1\n\"ok\" + \"\\( (b)\"", 2, 9},
		{"CRLF satır sonu", "a = 1\r\n\"x \\(a\r\nb\"", 2, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := allTokens(t, tt.src)
			if len(errs) == 0 {
				t.Fatalf("hata bekleniyordu")
			}
			if !hasErr(errs, tt.line, tt.col, messages.H0009) {
				t.Errorf("H0009 (satır %d, sütun %d) bekleniyordu, gelen: %v", tt.line, tt.col, errs)
			}
		})
	}
}

// Hatalı bir metinden sonra sözcük çözümleyici okumaya devam etmeli ve sonraki hataları da bildirmeli.
func TestMultipleLexerErrors(t *testing.T) {
	toks, errs := allTokens(t, "\"\\(a\n\"tamam\"\n\"son")
	if len(errs) != 2 {
		t.Fatalf("iki hata bekleniyordu, gelen: %v", errs)
	}
	if !hasErr(errs[:1], 1, 2, messages.H0009) {
		t.Errorf("ilk hata yanlış: %q", errs[0])
	}
	if !hasErr(errs[1:], 3, 1, messages.H0010) {
		t.Errorf("ikinci hata yanlış: %q", errs[1])
	}
	if len(toks) != 4 || toks[1].Literal != "tamam" {
		t.Errorf("ikinci satırın metni kendi başına okunmalı: %v", toks)
	}
}

// CRLF satır sonlu dosyada çok satırlı metnin değeri '\r' içermez: satır sonu kaynağın
// biçiminden bağımsız olarak '\n'dir ('\r' yalnızca kaçışla, \r, yazılır).
func TestCRLFInString(t *testing.T) {
	toks := tokensNoErrors(t, "\"a\r\nb\" + \"e\\rf\"")
	if toks[0].Literal != "a\nb" || toks[2].Literal != "e\rf" {
		t.Fatalf("metin değerleri: %q %q", toks[0].Literal, toks[2].Literal)
	}
}
