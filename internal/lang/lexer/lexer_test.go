package lexer

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/hint"
)

// allTokens girdinin tüm sözcük birimlerini (DOSYA_SONU dahil) ve sözcük çözümleyici hatalarını döndürür.
func allTokens(t *testing.T, input string) ([]Token, []string) {
	t.Helper()
	l := New(input)
	var toks []Token
	for i := 0; ; i++ {
		if i > 10000 {
			t.Fatalf("sözcük çözümleyici sonsuz döngüye girdi: %q", input)
		}
		tok := l.NextToken()
		toks = append(toks, tok)
		if tok.Type == TOKEN_EOF {
			break
		}
	}
	return toks, l.Errors()
}

// tokensNoErrors hata beklenmeyen girdiler içindir.
func tokensNoErrors(t *testing.T, input string) []Token {
	t.Helper()
	toks, errs := allTokens(t, input)
	if len(errs) > 0 {
		t.Fatalf("%q için beklenmeyen sözcük çözümleyici hataları: %v", input, errs)
	}
	return toks
}

type expected struct {
	typ TokenType
	lit string
}

func checkTokens(t *testing.T, input string, want []expected) {
	t.Helper()
	toks := tokensNoErrors(t, input)
	if len(toks) != len(want) {
		t.Fatalf("%q: %d token bekleniyordu, %d geldi: %v", input, len(want), len(toks), toks)
	}
	for i, w := range want {
		if toks[i].Type != w.typ || toks[i].Literal != w.lit {
			t.Errorf("%q: token[%d] = %s %q, beklenen %s %q", input, i, toks[i].Type, toks[i].Literal, w.typ, w.lit)
		}
	}
}

// ---------------------------------------------------------------------------
// Anahtar kelimeler
// ---------------------------------------------------------------------------

// Sözlükteki her anahtar kelime tek başına doğru türde token üretmeli.
func TestKeywordTableComplete(t *testing.T) {
	for word, kind := range keywords {
		toks := tokensNoErrors(t, word)
		if len(toks) != 2 {
			t.Errorf("%q: tek token + DOSYA_SONU bekleniyordu, gelen: %v", word, toks)
			continue
		}
		if toks[0].Type != kind || toks[0].Literal != word {
			t.Errorf("%q: %s bekleniyordu, %s %q geldi", word, kind, toks[0].Type, toks[0].Literal)
		}
		if toks[0].Line != 1 || toks[0].Column != 1 {
			t.Errorf("%q: konum 1:1 bekleniyordu, %d:%d geldi", word, toks[0].Line, toks[0].Column)
		}
	}
}

// Anahtar kelimelerin yalnızca Türkçe yazımı geçerlidir (her işin tek yazımı); ASCII
// yazım sıradan bir addır ve hata mesajında Türkçe biçim önerilir. Bu liste, tablodan
// bir yazımın yanlışlıkla silinmesini de yakalar.
func TestKeywordTurkishSpellingOnly(t *testing.T) {
	pairs := []struct {
		turkishSpelling, ascii string
		kind                   TokenType
	}{
		{"değişken", "degisken", TOKEN_VAR},
		{"sonuç", "sonuc", TOKEN_RETURN},
		{"eğer", "eger", TOKEN_IF},
		{"değilse", "degilse", TOKEN_ELSE},
		{"eşle", "esle", TOKEN_MATCH},
		{"varsayılan", "varsayilan", TOKEN_DEFAULT},
		{"döngü", "dongu", TOKEN_FOR},
		{"içinde", "icinde", TOKEN_IN},
		{"çık", "cik", TOKEN_BREAK},
		{"doğru", "dogru", TOKEN_TRUE},
		{"yanlış", "yanlis", TOKEN_FALSE},
		{"değil", "degil", TOKEN_KW_NOT},
		{"yapı", "yapi", TOKEN_STRUCT},
		{"fırlat", "firlat", TOKEN_THROW},
		{"eşzamanlı", "eszamanli", TOKEN_CONCURRENT},
		{"kesme_noktası", "kesme_noktasi", TOKEN_BREAKPOINT},
	}
	for _, c := range pairs {
		if got := LookupIdent(c.turkishSpelling); got != c.kind {
			t.Errorf("LookupIdent(%q) = %s, beklenen %s", c.turkishSpelling, got, c.kind)
		}
		if got := LookupIdent(c.ascii); got != TOKEN_IDENT {
			t.Errorf("LookupIdent(%q) = %s; ASCII yazım sıradan bir ad olmalı", c.ascii, got)
		}
		want := " ('" + c.ascii + "' TR'de anahtar kelime değil; '" + c.turkishSpelling + "' yazmak mı istediniz?)"
		if got := hint.KeywordSpellingHint(c.ascii); got != want {
			t.Errorf("ipucu.KaldirilanYazimIpucu(%q) = %q, %q bekleniyordu", c.ascii, got, want)
		}
	}

	// Tek yazımı olan anahtar kelimeler
	singles := map[string]TokenType{
		"sabit": TOKEN_CONST, "fonksiyon": TOKEN_FUNCTION, "iken": TOKEN_WHILE,
		"devam": TOKEN_CONTINUE, "yok": TOKEN_NIL, "ve": TOKEN_KW_AND, "veya": TOKEN_KW_OR,
		"kendi": TOKEN_SELF, "dene": TOKEN_TRY, "yakala": TOKEN_CATCH,
		"aktar": TOKEN_IMPORT, "ertele": TOKEN_DEFER,
	}
	for word, kind := range singles {
		if got := LookupIdent(word); got != kind {
			t.Errorf("LookupIdent(%q) = %s, beklenen %s", word, got, kind)
		}
	}
}

// Anahtar kelimeler büyük/küçük harfe duyarlıdır ve yalnızca tam kelime olarak eşleşir.
func TestNonKeywords(t *testing.T) {
	for _, s := range []string{
		"Eğer", "EĞER", "Değişken", "DOĞRU", // büyük harfli yazımlar tanımlayıcıdır
		"eğerki", "değişkenler", "dongu2", "_eger", "sabitler", // anahtar kelime önekli tanımlayıcılar
		"egér",
		"break_point", // eski İngilizce yazım kaldırıldı; artık sıradan bir ad
	} {
		toks := tokensNoErrors(t, s)
		if len(toks) != 2 || toks[0].Type != TOKEN_IDENT || toks[0].Literal != s {
			t.Errorf("%q: tek bir TANIMLAYICI bekleniyordu, gelen: %v", s, toks)
		}
	}
}

// ---------------------------------------------------------------------------
// Tanımlayıcılar
// ---------------------------------------------------------------------------

func TestTurkishIdentifiers(t *testing.T) {
	for _, s := range []string{
		"ışık", "İstanbul", "şeker", "ağaç", "üzüm", "göz", "çiçek",
		"IĞDIR", "öğrenci_sayısı", "_gizli", "x1", "değer2", "ÇÖĞŞÜİı",
	} {
		toks := tokensNoErrors(t, s)
		if len(toks) != 2 || toks[0].Type != TOKEN_IDENT || toks[0].Literal != s {
			t.Errorf("%q: tek bir TANIMLAYICI bekleniyordu, gelen: %v", s, toks)
		}
	}
}

func TestIdentifierAndOperatorBoundaries(t *testing.T) {
	checkTokens(t, "öğrenci.yaş+çarpan", []expected{
		{TOKEN_IDENT, "öğrenci"},
		{TOKEN_DOT, "."},
		{TOKEN_IDENT, "yaş"},
		{TOKEN_PLUS, "+"},
		{TOKEN_IDENT, "çarpan"},
		{TOKEN_EOF, ""},
	})
}

// ---------------------------------------------------------------------------
// Sayılar
// ---------------------------------------------------------------------------

func TestNumbers(t *testing.T) {
	tests := []struct {
		src  string
		kind TokenType
		lit  string
	}{
		{"0", TOKEN_INT, "0"},
		{"42", TOKEN_INT, "42"},
		{"007", TOKEN_INT, "007"},
		{"1_000_000", TOKEN_INT, "1000000"},
		{"3.14", TOKEN_FLOAT, "3.14"},
		{"0.5", TOKEN_FLOAT, "0.5"},
		{"1_000.25", TOKEN_FLOAT, "1000.25"},
		{"3.141_592", TOKEN_FLOAT, "3.141592"},
		// Mevcut davranış: alt çizgiler konumdan bağımsız atılır (sonda veya art arda olsa da).
		{"1_", TOKEN_INT, "1"},
		{"1__0", TOKEN_INT, "10"},
	}
	for _, tt := range tests {
		checkTokens(t, tt.src, []expected{{tt.kind, tt.lit}, {TOKEN_EOF, ""}})
	}
}

func TestNumberAndRangeSplit(t *testing.T) {
	// '1..10' ondalık sayı değil, aralıktır
	checkTokens(t, "1..10", []expected{
		{TOKEN_INT, "1"}, {TOKEN_RANGE, ".."}, {TOKEN_INT, "10"}, {TOKEN_EOF, ""},
	})
	checkTokens(t, "0..=5", []expected{
		{TOKEN_INT, "0"}, {TOKEN_RANGE_INCL, "..="}, {TOKEN_INT, "5"}, {TOKEN_EOF, ""},
	})
	// Noktadan sonra rakam yoksa sayı tam sayı olarak biter (üye erişimi için)
	checkTokens(t, "5.kare", []expected{
		{TOKEN_INT, "5"}, {TOKEN_DOT, "."}, {TOKEN_IDENT, "kare"}, {TOKEN_EOF, ""},
	})
	checkTokens(t, "1.5..2.5", []expected{
		{TOKEN_FLOAT, "1.5"}, {TOKEN_RANGE, ".."}, {TOKEN_FLOAT, "2.5"}, {TOKEN_EOF, ""},
	})
}

// Bilimsel yazım: üs harfi (e/E), isteğe bağlı işaret ve en az bir rakam; sonuç Ondalık'tır.
func TestScientificNumbers(t *testing.T) {
	tests := []struct{ src, lit string }{
		{"1e-5", "1e-5"},
		{"3.2E+4", "3.2E+4"},
		{"2e10", "2e10"},
		{"6.02e2_3", "6.02e23"},
	}
	for _, tt := range tests {
		checkTokens(t, tt.src, []expected{{TOKEN_FLOAT, tt.lit}, {TOKEN_EOF, ""}})
	}
	// Üsten sonra rakam yoksa 'e' sayıya ait değildir
	checkTokens(t, "2e", []expected{{TOKEN_INT, "2"}, {TOKEN_IDENT, "e"}, {TOKEN_EOF, ""}})
	checkTokens(t, "2e+x", []expected{
		{TOKEN_INT, "2"}, {TOKEN_IDENT, "e"}, {TOKEN_PLUS, "+"}, {TOKEN_IDENT, "x"}, {TOKEN_EOF, ""},
	})
	checkTokens(t, "1e3..5", []expected{
		{TOKEN_FLOAT, "1e3"}, {TOKEN_RANGE, ".."}, {TOKEN_INT, "5"}, {TOKEN_EOF, ""},
	})
}

// Mevcut davranış: sayıya bitişik harfler ayrı bir tanimlayıcı olur; sözcük çözümleyici hata vermez
// (ayrıştırıcı ardışık ifadeyi ayrıca değerlendirir).
func TestLetterAfterNumber(t *testing.T) {
	checkTokens(t, "12abc", []expected{
		{TOKEN_INT, "12"}, {TOKEN_IDENT, "abc"}, {TOKEN_EOF, ""},
	})
}

// ---------------------------------------------------------------------------
// Operatörler ve ayırıcılar
// ---------------------------------------------------------------------------

func TestAllOperators(t *testing.T) {
	tests := []struct {
		src  string
		kind TokenType
	}{
		{"=", TOKEN_ASSIGN}, {":=", TOKEN_WALRUS}, {"+", TOKEN_PLUS}, {"-", TOKEN_MINUS},
		{"*", TOKEN_ASTERISK}, {"/", TOKEN_SLASH}, {"%", TOKEN_PERCENT}, {"**", TOKEN_POWER},
		{"@", TOKEN_MATMUL}, {"!", TOKEN_BANG}, {"==", TOKEN_EQ}, {"!=", TOKEN_NOT_EQ},
		{"<", TOKEN_LT}, {">", TOKEN_GT}, {"<=", TOKEN_LTE}, {">=", TOKEN_GTE},
		{"&&", TOKEN_AND}, {"||", TOKEN_OR}, {"|>", TOKEN_PIPE}, {"??", TOKEN_NULL_COALESC},
		{"?.", TOKEN_SAFE_DOT}, {"=>", TOKEN_ARROW}, {"..", TOKEN_RANGE}, {"..=", TOKEN_RANGE_INCL},
		{"+=", TOKEN_PLUS_ASSIGN}, {"-=", TOKEN_MINUS_ASSIGN}, {"*=", TOKEN_MUL_ASSIGN},
		{"/=", TOKEN_DIV_ASSIGN},
		{",", TOKEN_COMMA}, {";", TOKEN_SEMICOLON}, {":", TOKEN_COLON}, {".", TOKEN_DOT},
		{"?", TOKEN_QUESTION}, {"(", TOKEN_LPAREN}, {")", TOKEN_RPAREN}, {"{", TOKEN_LBRACE},
		{"}", TOKEN_RBRACE}, {"[", TOKEN_LBRACKET}, {"]", TOKEN_RBRACKET},
	}
	for _, tt := range tests {
		// Hem tek başına hem de iki tanımlayıcı arasında doğru okunmalı
		checkTokens(t, tt.src, []expected{{tt.kind, tt.src}, {TOKEN_EOF, ""}})
		checkTokens(t, "a"+tt.src+"b", []expected{
			{TOKEN_IDENT, "a"}, {tt.kind, tt.src}, {TOKEN_IDENT, "b"}, {TOKEN_EOF, ""},
		})
	}
}

// En uzun eşleşme kuralı ve iki token'a bölünen birleşimler.
func TestOperatorCombinations(t *testing.T) {
	tests := []struct {
		src  string
		want []expected
	}{
		// '->' tek token değildir; ayrıştırıcı '-' ve '>' çiftini dönüş tipi oku olarak tanır
		{"->", []expected{{TOKEN_MINUS, "-"}, {TOKEN_GT, ">"}}},
		{"**=", []expected{{TOKEN_POWER, "**"}, {TOKEN_ASSIGN, "="}}},
		{"===", []expected{{TOKEN_EQ, "=="}, {TOKEN_ASSIGN, "="}}},
		{"==>", []expected{{TOKEN_EQ, "=="}, {TOKEN_GT, ">"}}},
		{"...", []expected{{TOKEN_RANGE, ".."}, {TOKEN_DOT, "."}}},
		{"???", []expected{{TOKEN_NULL_COALESC, "??"}, {TOKEN_QUESTION, "?"}}},
		{"!!", []expected{{TOKEN_BANG, "!"}, {TOKEN_BANG, "!"}}},
		{"<<", []expected{{TOKEN_LT, "<"}, {TOKEN_LT, "<"}}},
		{"|||", []expected{{TOKEN_OR, "||"}, {TOKEN_ILLEGAL, "|"}}},
		{"x?.y??z", []expected{
			{TOKEN_IDENT, "x"}, {TOKEN_SAFE_DOT, "?."}, {TOKEN_IDENT, "y"},
			{TOKEN_NULL_COALESC, "??"}, {TOKEN_IDENT, "z"},
		}},
		{"liste |> filtrele", []expected{
			{TOKEN_IDENT, "liste"}, {TOKEN_PIPE, "|>"}, {TOKEN_IDENT, "filtrele"},
		}},
		{"(x) => x ** 2", []expected{
			{TOKEN_LPAREN, "("}, {TOKEN_IDENT, "x"}, {TOKEN_RPAREN, ")"}, {TOKEN_ARROW, "=>"},
			{TOKEN_IDENT, "x"}, {TOKEN_POWER, "**"}, {TOKEN_INT, "2"},
		}},
	}
	for _, tt := range tests {
		checkTokens(t, tt.src, append(tt.want, expected{TOKEN_EOF, ""}))
	}
}

func TestIllegalCharacters(t *testing.T) {
	// '`' ham metin başlatır (2.6a); geçersiz karakter değildir.
	for _, s := range []string{"&", "|", "$", "^", "~", "\\", "😀"} {
		toks := tokensNoErrors(t, s)
		if len(toks) != 2 || toks[0].Type != TOKEN_ILLEGAL || toks[0].Literal != s {
			t.Errorf("%q: tek bir GEÇERSİZ token bekleniyordu, gelen: %v", s, toks)
		}
	}
	// Geçersiz karakterden sonra okumaya devam edilmeli
	checkTokens(t, "a $ b", []expected{
		{TOKEN_IDENT, "a"}, {TOKEN_ILLEGAL, "$"}, {TOKEN_IDENT, "b"}, {TOKEN_EOF, ""},
	})
}

// ---------------------------------------------------------------------------
// Bütünleşik örnek
// ---------------------------------------------------------------------------

func TestFullProgram(t *testing.T) {
	src := `fonksiyon topla(a: Tam, b: Tam) -> Tam {
	sonuç a + b
}
sabit liste = [1, 2.5, "üç"]
döngü i, x içinde liste { eğer x == yok ve değil doğru { çık } }
`
	checkTokens(t, src, []expected{
		{TOKEN_FUNCTION, "fonksiyon"}, {TOKEN_IDENT, "topla"}, {TOKEN_LPAREN, "("},
		{TOKEN_IDENT, "a"}, {TOKEN_COLON, ":"}, {TOKEN_IDENT, "Tam"}, {TOKEN_COMMA, ","},
		{TOKEN_IDENT, "b"}, {TOKEN_COLON, ":"}, {TOKEN_IDENT, "Tam"}, {TOKEN_RPAREN, ")"},
		{TOKEN_MINUS, "-"}, {TOKEN_GT, ">"}, {TOKEN_IDENT, "Tam"}, {TOKEN_LBRACE, "{"},
		{TOKEN_RETURN, "sonuç"}, {TOKEN_IDENT, "a"}, {TOKEN_PLUS, "+"}, {TOKEN_IDENT, "b"},
		{TOKEN_RBRACE, "}"},
		{TOKEN_CONST, "sabit"}, {TOKEN_IDENT, "liste"}, {TOKEN_ASSIGN, "="},
		{TOKEN_LBRACKET, "["}, {TOKEN_INT, "1"}, {TOKEN_COMMA, ","}, {TOKEN_FLOAT, "2.5"},
		{TOKEN_COMMA, ","}, {TOKEN_STRING, "üç"}, {TOKEN_RBRACKET, "]"},
		{TOKEN_FOR, "döngü"}, {TOKEN_IDENT, "i"}, {TOKEN_COMMA, ","}, {TOKEN_IDENT, "x"},
		{TOKEN_IN, "içinde"}, {TOKEN_IDENT, "liste"}, {TOKEN_LBRACE, "{"},
		{TOKEN_IF, "eğer"}, {TOKEN_IDENT, "x"}, {TOKEN_EQ, "=="}, {TOKEN_NIL, "yok"},
		{TOKEN_KW_AND, "ve"}, {TOKEN_KW_NOT, "değil"}, {TOKEN_TRUE, "doğru"},
		{TOKEN_LBRACE, "{"}, {TOKEN_BREAK, "çık"}, {TOKEN_RBRACE, "}"}, {TOKEN_RBRACE, "}"},
		{TOKEN_EOF, ""},
	})
}

// Kaldırılan eş anlamlılar artık sıradan adlardır.
func TestRemovedSynonymsAreNames(t *testing.T) {
	for _, name := range []string{"işlev", "islev", "döndür", "dondur", "kır", "kir", "sınıf", "sinif",
		"dur_noktası", "dur_noktasi", "durdurma_noktası", "durdurma_noktasi"} {
		if kind := LookupIdent(name); kind != TOKEN_IDENT {
			t.Errorf("%q artık ad olmalı, %s", name, kind)
		}
		if hint.KeywordSpellingHint(name) == "" {
			t.Errorf("%q için ipucu bekleniyordu", name)
		}
	}
	// Eski adın ASCII biçimi de Türkçe adı önerir (ASCII biçim önerilmez).
	for name, want := range map[string]string{"islev": "'fonksiyon'", "dondur": "'sonuç'", "kir": "'çık'"} {
		if got := hint.KeywordSpellingHint(name); !strings.Contains(got, want) {
			t.Errorf("ipucu.KaldirilanYazimIpucu(%q) = %q, %s önermeli", name, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Satır ve sütun numaraları
// ---------------------------------------------------------------------------

type srcPos struct {
	lit       string
	line, col int
}

func checkPositions(t *testing.T, input string, want []srcPos) {
	t.Helper()
	toks := tokensNoErrors(t, input)
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
func TestLineAndColumnNumbers(t *testing.T) {
	src := "değişken ğüş = 1\n" +
		"eğer ğüş >= 10 {\n" +
		"\tyazdır(\"çok\")\n" +
		"}"
	checkPositions(t, src, []srcPos{
		{"değişken", 1, 1}, {"ğüş", 1, 10}, {"=", 1, 14}, {"1", 1, 16},
		{"eğer", 2, 1}, {"ğüş", 2, 6}, {">=", 2, 10}, {"10", 2, 13}, {"{", 2, 16},
		{"yazdır", 3, 2}, {"(", 3, 8}, {"çok", 3, 9}, {")", 3, 14}, // sekme tek sütun sayılır
		{"}", 4, 1},
		{"", 4, 2},
	})
}
func TestBlankLinesAndEOFPosition(t *testing.T) {
	checkPositions(t, "\n\n   x\n", []srcPos{{"x", 3, 4}, {"", 4, 1}})
	checkPositions(t, "", []srcPos{{"", 1, 1}})
}
func TestCRLFLineEndings(t *testing.T) {
	src := "değişken a = 1\r\n" +
		"a += 2 // yorum\r\n" +
		"\r\n" +
		"yazdır(\"x\")\r\n"
	checkPositions(t, src, []srcPos{
		{"değişken", 1, 1}, {"a", 1, 10}, {"=", 1, 12}, {"1", 1, 14},
		{"a", 2, 1}, {"+=", 2, 3}, {"2", 2, 6},
		{"yazdır", 4, 1}, {"(", 4, 7}, {"x", 4, 8}, {")", 4, 11},
		{"", 5, 1},
	})

	// '\r' hiçbir yerde GEÇERSİZ token üretmemeli
	for _, tok := range tokensNoErrors(t, "a\r\nb\rc") {
		if tok.Type == TOKEN_ILLEGAL {
			t.Errorf("'\\r' için GEÇERSİZ token üretildi: %v", tok)
		}
	}
}

// Windows'ta bazı editörler dosyanın başına UTF-8 BOM (U+FEFF) ekler; sözcük çözümleyici bunu atlamalı.
func TestUTF8BOM(t *testing.T) {
	bom := string(rune(0xFEFF))
	checkPositions(t, bom+"değişken x = 1", []srcPos{
		{"değişken", 1, 1}, {"x", 1, 10}, {"=", 1, 12}, {"1", 1, 14}, {"", 1, 15},
	})
	// Yalnızca dosya başındaki BOM atlanır; metnin ortasındakine dokunulmaz
	toks := tokensNoErrors(t, "\"a"+bom+"b\"")
	if toks[0].Literal != "a"+bom+"b" {
		t.Errorf("metin içindeki U+FEFF korunmalı, gelen: %q", toks[0].Literal)
	}
}
