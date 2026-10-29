package lexer

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// allTokens girdinin tüm sözcük birimlerini (DOSYA_SONU dahil) ve sözcük çözümleyici hatalarını döndürür.
func allTokens(t *testing.T, input string) ([]Token, []error) {
	t.Helper()
	return lexTokens(t, New(input), input)
}

// lexTokens l'nin tüm sözcük birimlerini (DOSYA_SONU dahil) ve hatalarını döndürür; input yalnızca
// hata iletisi içindir.
func lexTokens(t *testing.T, l *Lexer, input string) ([]Token, []error) {
	t.Helper()
	var toks []Token
	for i := 0; ; i++ {
		if i > 10000 {
			t.Fatalf("sözcük çözümleyici sonsuz döngüye girdi: %q", input)
		}
		tok := l.NextToken()
		toks = append(toks, tok)
		if tok.Type == TokEOF {
			break
		}
	}
	return toks, l.Errs()
}

// hasCode, hatalar arasında verilen ileti koduna sahip olan var mı diye bakar (konuma bakmaz).
func hasCode(errs []error, code messages.Code) bool {
	for _, e := range errs {
		if messages.Has(e, code) {
			return true
		}
	}
	return false
}

// hasErr, hatalar arasında verilen ileti kodunu verilen satır ve sütunda taşıyan var mı diye bakar.
func hasErr(errs []error, line, col int, code messages.Code) bool {
	for _, e := range errs {
		l, c, ok := messages.Pos(e)
		if ok && l == line && c == col && messages.Has(e, code) {
			return true
		}
	}
	return false
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
	compareTokens(t, input, tokensNoErrors(t, input), want)
}

func compareTokens(t *testing.T, input string, toks []Token, want []expected) {
	t.Helper()
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

// Sözlükteki her anahtar kelime kimlik yazımıyla (TR_if, 2.3a) tek başına doğru türde token üretmeli.
func TestKeywordTableComplete(t *testing.T) {
	for _, e := range keywordTable {
		word, kind := codes.Spelling(e.Code), e.Type
		toks := tokensNoErrors(t, word)
		if len(toks) != 2 {
			t.Errorf("%q: tek token + DOSYA_SONU bekleniyordu, gelen: %v", word, toks)
			continue
		}
		if toks[0].Type != kind || toks[0].Literal != e.Code.String() { // içeride kod (TR_kw_if)
			t.Errorf("%q: %s bekleniyordu, %s %q geldi", word, kind, toks[0].Type, toks[0].Literal)
		}
		if toks[0].Line != 1 || toks[0].Column != 1 {
			t.Errorf("%q: konum 1:1 bekleniyordu, %d:%d geldi", word, toks[0].Line, toks[0].Column)
		}
	}
}

// Türkçe yazılmış metinde anahtar kelimelerin yalnızca Türkçe yazımı geçerlidir (her işin tek
// yazımı); ASCII yazım sıradan bir addır ve hata iletisinde Türkçe biçim önerilir. Bu liste,
// Türkçe listeden bir yazımın yanlışlıkla silinmesini de yakalar. Dil açıkça verilir: program
// içeride kimliklerle okunur (LookupIdent kimlik listesindedir).
func TestKeywordTurkishSpellingOnly(t *testing.T) {
	pairs := []struct {
		turkishSpelling, ascii string
		kind                   TokenType
	}{
		{"değişken", "degisken", TokVar},
		{"sonuç", "sonuc", TokReturn},
		{"eğer", "eger", TokIf},
		{"değilse", "degilse", TokElse},
		{"eşle", "esle", TokMatch},
		{"döngü", "dongu", TokFor},
		{"içinde", "icinde", TokIn},
		{"çık", "cik", TokBreak},
		{"doğru", "dogru", TokTrue},
		{"yanlış", "yanlis", TokFalse},
		{"değil", "degil", TokKwNot},
		{"yapı", "yapi", TokStruct},
		{"fırlat", "firlat", TokThrow},
		{"eşzamanlı", "eszamanli", TokConcurrent},
		{"kesme_noktası", "kesme_noktasi", TokBreakpoint},
	}
	tr := keywordsOf(codes.Source())
	for _, c := range pairs {
		if got := tr[c.turkishSpelling]; got != c.kind {
			t.Errorf("Türkçe %q = %s, beklenen %s", c.turkishSpelling, got, c.kind)
		}
		if got, ok := tr[c.ascii]; ok {
			t.Errorf("Türkçe %q = %s; ASCII yazım sıradan bir ad olmalı", c.ascii, got)
		}
		want := " ('" + c.ascii + "' TR'de anahtar kelime değil; '" + c.turkishSpelling + "' yazmak mı istediniz?)"
		if got := hint.KeywordSpellingHint(c.ascii, codes.Source()).Error(); got != want {
			t.Errorf("hint.KeywordSpellingHint(%q) = %q, %q bekleniyordu", c.ascii, got, want)
		}
	}

	// Tek yazımı olan anahtar kelimeler
	singles := map[string]TokenType{
		"sabit": TokConst, "fonksiyon": TokFunction,
		"devam": TokContinue, "yok": TokNil, "ve": TokKwAnd, "veya": TokKwOr,
		"kendi": TokSelf, "dene": TokTry, "yakala": TokCatch,
		"aktar": TokImport, "ertele": TokDefer,
	}
	for word, kind := range singles {
		if got := tr[word]; got != kind {
			t.Errorf("Türkçe %q = %s, beklenen %s", word, got, kind)
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
		if len(toks) != 2 || toks[0].Type != TokIdent || toks[0].Literal != s {
			t.Errorf("%q: tek bir TANIMLAYICI bekleniyordu, gelen: %v", s, toks)
		}
	}
}

// İleti kodu biçimindeki yazım (H ve dört rakam) sıradan bir addır (eski 2.12 kalktı); kod bir
// metindir: TR_Message("H6903", x). Koşullu döngü ve varsayılan kolun ayrı sözcüğü yoktur (2.9).
func TestNoMessageCodeOrRemovedWordTokens(t *testing.T) {
	for _, s := range []string{"H6903", "H690", "H69030", "h6903", "iken", "varsayılan"} {
		toks := tokensNoErrors(t, s)
		if len(toks) != 2 || toks[0].Type != TokIdent || toks[0].Literal != s {
			t.Errorf("%q: tek bir TANIMLAYICI bekleniyordu, gelen: %v", s, toks)
		}
	}
	checkTokens(t, "ileti(\"H6903\", x)", []expected{
		{TokIdent, "ileti"}, {TokLParen, "("}, {TokString, "H6903"}, {TokComma, ","}, {TokIdent, "x"}, {TokRParen, ")"}, {TokEOF, ""},
	})
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
		if len(toks) != 2 || toks[0].Type != TokIdent || toks[0].Literal != s {
			t.Errorf("%q: tek bir TANIMLAYICI bekleniyordu, gelen: %v", s, toks)
		}
	}
}

func TestIdentifierAndOperatorBoundaries(t *testing.T) {
	checkTokens(t, "öğrenci.yaş+çarpan", []expected{
		{TokIdent, "öğrenci"},
		{TokDot, "."},
		{TokIdent, "yaş"},
		{TokPlus, "+"},
		{TokIdent, "çarpan"},
		{TokEOF, ""},
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
		{"0", TokInt, "0"},
		{"42", TokInt, "42"},
		{"007", TokInt, "007"},
		{"1_000_000", TokInt, "1000000"},
		{"3.14", TokFloat, "3.14"},
		{"0.5", TokFloat, "0.5"},
		{"1_000.25", TokFloat, "1000.25"},
		{"3.141_592", TokFloat, "3.141592"},
	}
	for _, tt := range tests {
		checkTokens(t, tt.src, []expected{{tt.kind, tt.lit}, {TokEOF, ""}})
	}
}

func TestNumberAndRangeSplit(t *testing.T) {
	// '1..10' ondalık sayı değil, aralıktır
	checkTokens(t, "1..10", []expected{
		{TokInt, "1"}, {TokRange, ".."}, {TokInt, "10"}, {TokEOF, ""},
	})
	checkTokens(t, "0..=5", []expected{
		{TokInt, "0"}, {TokRangeIncl, "..="}, {TokInt, "5"}, {TokEOF, ""},
	})
	// Noktadan sonra rakam yoksa sayı tam sayı olarak biter (üye erişimi için)
	checkTokens(t, "5.kare", []expected{
		{TokInt, "5"}, {TokDot, "."}, {TokIdent, "kare"}, {TokEOF, ""},
	})
	checkTokens(t, "1.5..2.5", []expected{
		{TokFloat, "1.5"}, {TokRange, ".."}, {TokFloat, "2.5"}, {TokEOF, ""},
	})
}

// Üyeden sonraki rakamlar demetin öğe indeksidir (3.6a): 't.0.1' iki indekstir, (t.0).1; ondalık
// okunmaz. Ondalık ve aralık okuması üye dışında aynı kalır.
func TestElementIndexSplit(t *testing.T) {
	dot, end := expected{TokDot, "."}, expected{TokEOF, ""}
	checkTokens(t, "t.0.1", []expected{{TokIdent, "t"}, dot, {TokInt, "0"}, dot, {TokInt, "1"}, end})
	checkTokens(t, "t.10", []expected{{TokIdent, "t"}, dot, {TokInt, "10"}, end})
	checkTokens(t, "t.0.1.2", []expected{{TokIdent, "t"}, dot, {TokInt, "0"}, dot, {TokInt, "1"}, dot, {TokInt, "2"}, end})
	checkTokens(t, "(1, 2).1", []expected{
		{TokLParen, "("}, {TokInt, "1"}, {TokComma, ","}, {TokInt, "2"}, {TokRParen, ")"}, dot, {TokInt, "1"}, end,
	})
	checkTokens(t, "l[0].1", []expected{{TokIdent, "l"}, {TokLBracket, "["}, {TokInt, "0"}, {TokRBracket, "]"}, dot, {TokInt, "1"}, end})
	checkTokens(t, "t.0..2", []expected{{TokIdent, "t"}, dot, {TokInt, "0"}, {TokRange, ".."}, {TokInt, "2"}, end})
	// üye dışında ondalık ve aralık
	checkTokens(t, "1.5", []expected{{TokFloat, "1.5"}, end})
	checkTokens(t, "1..2", []expected{{TokInt, "1"}, {TokRange, ".."}, {TokInt, "2"}, end})
	checkTokens(t, "x = 1.5 + t.1", []expected{
		{TokIdent, "x"}, {TokAssign, "="}, {TokFloat, "1.5"}, {TokPlus, "+"}, {TokIdent, "t"}, dot, {TokInt, "1"}, end,
	})
	// indeks yalnız rakamdır: üs, ayırıcı ve baştaki sıfır hatadır
	for _, src := range []string{"x.0e1", "t.1_0", "t.01"} {
		if _, errs := allTokens(t, src); !hasCode(errs, messages.H0036) {
			t.Errorf("%q: H0036 bekleniyordu, hatalar: %v", src, errs)
		}
	}
	// işlenenden sonra gelmeyen '.5' yine noktayla başlayan sayının hatasıdır
	if _, errs := allTokens(t, "x = .5"); !hasCode(errs, messages.H0032) {
		t.Errorf("'x = .5': H0032 bekleniyordu, hatalar: %v", errs)
	}
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
		checkTokens(t, tt.src, []expected{{TokFloat, tt.lit}, {TokEOF, ""}})
	}
	// Üsten sonra rakam yoksa 'e' sayıya ait değildir
	checkTokens(t, "2e", []expected{{TokInt, "2"}, {TokIdent, "e"}, {TokEOF, ""}})
	checkTokens(t, "2e+x", []expected{
		{TokInt, "2"}, {TokIdent, "e"}, {TokPlus, "+"}, {TokIdent, "x"}, {TokEOF, ""},
	})
	checkTokens(t, "1e3..5", []expected{
		{TokFloat, "1e3"}, {TokRange, ".."}, {TokInt, "5"}, {TokEOF, ""},
	})
}

// Mevcut davranış: sayıya bitişik harfler ayrı bir tanimlayıcı olur; sözcük çözümleyici hata vermez
// (ayrıştırıcı ardışık ifadeyi ayrıca değerlendirir).
func TestLetterAfterNumber(t *testing.T) {
	checkTokens(t, "12abc", []expected{
		{TokInt, "12"}, {TokIdent, "abc"}, {TokEOF, ""},
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
		{"=", TokAssign}, {":=", TokWalrus}, {"+", TokPlus}, {"-", TokMinus},
		{"*", TokAsterisk}, {"/", TokSlash}, {"%", TokPercent}, {"**", TokPower},
		{"@", TokMatmul}, {"!", TokBang}, {"==", TokEq}, {"!=", TokNotEq},
		{"<", TokLt}, {">", TokGt}, {"<=", TokLte}, {">=", TokGte},
		{"&&", TokAnd}, {"||", TokOr}, {"|>", TokPipe}, {"??", TokNullCoalesc},
		{"?.", TokSafeDot}, {"=>", TokArrow}, {"..", TokRange}, {"..=", TokRangeIncl},
		{"+=", TokPlusAssign}, {"-=", TokMinusAssign}, {"*=", TokMulAssign},
		{"/=", TokDivAssign}, {"//=", TokFloorDivAssign}, {"%=", TokModAssign}, {"**=", TokPowAssign},
		{"&=", TokBitAndAssign}, {"|=", TokBitOrAssign}, {"^=", TokBitXorAssign}, {"<<=", TokShlAssign},
		{">>=", TokShrAssign},
		{"&", TokBitAnd}, {"|", TokBitOr}, {"^", TokBitXor}, {"~", TokTilde},
		{"<<", TokShl}, {">>", TokShr},
		{",", TokComma}, {";", TokSemicolon}, {":", TokColon}, {".", TokDot},
		{"?", TokQuestion}, {"(", TokLParen}, {")", TokRParen}, {"{", TokLBrace},
		{"}", TokRBrace}, {"[", TokLBracket}, {"]", TokRBracket},
	}
	for _, tt := range tests {
		// Hem tek başına hem de iki tanımlayıcı arasında doğru okunmalı; satır başındaki '//' yorum
		// hatasıdır (2.2), '//=' yalnız iki ad arasında sınanır.
		if tt.kind != TokFloorDivAssign {
			checkTokens(t, tt.src, []expected{{tt.kind, tt.src}, {TokEOF, ""}})
		}
		checkTokens(t, "a"+tt.src+"b", []expected{
			{TokIdent, "a"}, {tt.kind, tt.src}, {TokIdent, "b"}, {TokEOF, ""},
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
		{"->", []expected{{TokMinus, "-"}, {TokGt, ">"}}},
		{"**=", []expected{{TokPowAssign, "**="}}},
		{"**==", []expected{{TokPowAssign, "**="}, {TokAssign, "="}}},
		{"===", []expected{{TokEq, "=="}, {TokAssign, "="}}},
		{"==>", []expected{{TokEq, "=="}, {TokGt, ">"}}},
		{"...", []expected{{TokRange, ".."}, {TokDot, "."}}},
		{"???", []expected{{TokNullCoalesc, "??"}, {TokQuestion, "?"}}},
		{"!!", []expected{{TokBang, "!"}, {TokBang, "!"}}},
		{"<<=", []expected{{TokShlAssign, "<<="}}},
		{"<<<", []expected{{TokShl, "<<"}, {TokLt, "<"}}},
		{"<=<", []expected{{TokLte, "<="}, {TokLt, "<"}}},
		{">>>", []expected{{TokShr, ">>"}, {TokGt, ">"}}},
		{"|||", []expected{{TokOr, "||"}, {TokBitOr, "|"}}},
		{"&&&", []expected{{TokAnd, "&&"}, {TokBitAnd, "&"}}},
		{"~~", []expected{{TokTilde, "~"}, {TokTilde, "~"}}},
		{"x?.y??z", []expected{
			{TokIdent, "x"}, {TokSafeDot, "?."}, {TokIdent, "y"},
			{TokNullCoalesc, "??"}, {TokIdent, "z"},
		}},
		{"liste |> filtrele", []expected{
			{TokIdent, "liste"}, {TokPipe, "|>"}, {TokIdent, "filtrele"},
		}},
		{"(x) => x ** 2", []expected{
			{TokLParen, "("}, {TokIdent, "x"}, {TokRParen, ")"}, {TokArrow, "=>"},
			{TokIdent, "x"}, {TokPower, "**"}, {TokInt, "2"},
		}},
	}
	for _, tt := range tests {
		checkTokens(t, tt.src, append(tt.want, expected{TokEOF, ""}))
	}
}

func TestIllegalCharacters(t *testing.T) {
	// '`' ham metin başlatır (2.6a); geçersiz karakter değildir.
	for _, s := range []string{"$", "\\", "😀"} {
		toks := tokensNoErrors(t, s)
		if len(toks) != 2 || toks[0].Type != TokIllegal || toks[0].Literal != s {
			t.Errorf("%q: tek bir GEÇERSİZ token bekleniyordu, gelen: %v", s, toks)
		}
	}
	// Geçersiz karakterden sonra okumaya devam edilmeli
	checkTokens(t, "a $ b", []expected{
		{TokIdent, "a"}, {TokIllegal, "$"}, {TokIdent, "b"}, {TokEOF, ""},
	})
}

// ---------------------------------------------------------------------------
// Bütünleşik örnek
// ---------------------------------------------------------------------------

func TestFullProgram(t *testing.T) {
	src := `TR_func topla(a: TR_Int, b: TR_Int) -> TR_Int {
	TR_return a + b
}
TR_const liste = [1, 2.5, "üç"]
TR_for i, x TR_in liste { TR_if x == TR_none TR_and TR_not TR_true { TR_break } }
`
	// Dosya kodlu okunur (NewBody): dilin sözcüğünün metni iç kimliğidir.
	toks, errs := lexTokens(t, NewBody(src, true), src)
	if len(errs) > 0 {
		t.Fatalf("beklenmeyen sözcük çözümleyici hataları: %v", errs)
	}
	compareTokens(t, src, toks, []expected{
		{TokFunction, "TR_kw_func"}, {TokIdent, "topla"}, {TokLParen, "("},
		{TokIdent, "a"}, {TokColon, ":"}, {TokIdent, "TR_type_Int"}, {TokComma, ","},
		{TokIdent, "b"}, {TokColon, ":"}, {TokIdent, "TR_type_Int"}, {TokRParen, ")"},
		{TokMinus, "-"}, {TokGt, ">"}, {TokIdent, "TR_type_Int"}, {TokLBrace, "{"},
		{TokReturn, "TR_kw_return"}, {TokIdent, "a"}, {TokPlus, "+"}, {TokIdent, "b"},
		{TokRBrace, "}"},
		{TokConst, "TR_kw_const"}, {TokIdent, "liste"}, {TokAssign, "="},
		{TokLBracket, "["}, {TokInt, "1"}, {TokComma, ","}, {TokFloat, "2.5"},
		{TokComma, ","}, {TokString, "üç"}, {TokRBracket, "]"},
		{TokFor, "TR_kw_for"}, {TokIdent, "i"}, {TokComma, ","}, {TokIdent, "x"},
		{TokIn, "TR_kw_in"}, {TokIdent, "liste"}, {TokLBrace, "{"},
		{TokIf, "TR_kw_if"}, {TokIdent, "x"}, {TokEq, "=="}, {TokNil, "TR_kw_none"},
		{TokKwAnd, "TR_kw_and"}, {TokKwNot, "TR_kw_not"}, {TokTrue, "TR_kw_true"},
		{TokLBrace, "{"}, {TokBreak, "TR_kw_break"}, {TokRBrace, "}"}, {TokRBrace, "}"},
		{TokEOF, ""},
	})
}

// Türkçe yazılmış metinde eş anlamlılar (her işin tek adı vardır, 2.9a) sıradan adlardır;
// anahtar kelimenin ASCII biçimi de sıradan addır, ipucu Türkçe adı önerir. Dil açıkça verilir.
func TestSynonymsAreNames(t *testing.T) {
	tr := codes.Source()
	for _, name := range []string{"işlev", "döndür", "kır", "sınıf", "degisken", "eger"} {
		if kind, ok := keywordsOf(tr)[name]; ok {
			t.Errorf("%q ad olmalı, %s", name, kind)
		}
	}
	for name, want := range map[string]string{"degisken": "'değişken'", "eger": "'eğer'"} {
		if got := hint.KeywordSpellingHint(name, tr).Error(); !strings.Contains(got, want) {
			t.Errorf("hint.KeywordSpellingHint(%q) = %q, %s önermeli", name, got, want)
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

// positionsOf words'ün src'de sırayla geçtiği yerlerdir: satır 1'den, sütun karakter (rune) sayısıyla
// 1'den (2.1); beklenen konumlar böylece elle değil programdan hesaplanır. Tırnaklı sözcüğün metni
// tırnaksızdır, konumu açılış tırnağıdır; "" dosya sonudur (son karakterden sonraki yer).
// "yazım|metin" dilin sözcüğüdür: kaynakta yazımıyla aranır, metni iç kimliğidir (TR_var|TR_kw_var).
func positionsOf(t *testing.T, src string, words ...string) []srcPos {
	t.Helper()
	pos := func(off int) (int, int) {
		before := src[:off]
		return strings.Count(before, "\n") + 1, len([]rune(before[strings.LastIndex(before, "\n")+1:])) + 1
	}
	var out []srcPos
	at := 0
	for _, w := range words {
		if w == "" {
			line, col := pos(len(src))
			out = append(out, srcPos{"", line, col})
			continue
		}
		w, lit, coded := strings.Cut(w, "|")
		if !coded {
			lit = strings.Trim(w, `"`)
		}
		j := strings.Index(src[at:], w)
		if j < 0 {
			t.Fatalf("%q: %q yok", src[at:], w)
		}
		line, col := pos(at + j)
		out = append(out, srcPos{lit, line, col})
		at += j + len(w)
	}
	return out
}

// Sütunlar 1 tabanlıdır ve bayt değil rune (karakter) sayar; sekme tek sütun sayılır.
func TestLineAndColumnNumbers(t *testing.T) {
	src := "TR_var ğüş = 1\nTR_if ğüş >= 10 {\n\tTR_print(\"çok\")\n}"
	checkPositions(t, src, positionsOf(t, src,
		"TR_var|TR_kw_var", "ğüş", "=", "1", "TR_if|TR_kw_if", "ğüş", ">=", "10", "{", "TR_print|TR_fn_print", "(", `"çok"`, ")", "}", ""))
}
func TestBlankLinesAndEOFPosition(t *testing.T) {
	checkPositions(t, "\n\n   x\n", []srcPos{{"x", 3, 4}, {"", 4, 1}})
	checkPositions(t, "", []srcPos{{"", 1, 1}})
}
func TestCRLFLineEndings(t *testing.T) {
	src := "TR_var a = 1\r\na += 2 # yorum\r\n\r\nTR_print(\"x\")\r\n"
	checkPositions(t, src, positionsOf(t, src,
		"TR_var|TR_kw_var", "a", "=", "1", "a", "+=", "2", "TR_print|TR_fn_print", "(", `"x"`, ")", ""))

	// '\r' hiçbir yerde GEÇERSİZ token üretmemeli
	for _, tok := range tokensNoErrors(t, "a\r\nb\rc") {
		if tok.Type == TokIllegal {
			t.Errorf("'\\r' için GEÇERSİZ token üretildi: %v", tok)
		}
	}
}

// Windows'ta bazı editörler dosyanın başına UTF-8 BOM (U+FEFF) ekler; sözcük çözümleyici bunu atlamalı.
func TestUTF8BOM(t *testing.T) {
	bom := string(rune(0xFEFF))
	const src = "TR_var x = 1"
	checkPositions(t, bom+src, positionsOf(t, src, "TR_var|TR_kw_var", "x", "=", "1", ""))
	// Yalnızca dosya başındaki BOM atlanır; metnin ortasındakine dokunulmaz
	toks := tokensNoErrors(t, "\"a"+bom+"b\"")
	if toks[0].Literal != "a"+bom+"b" {
		t.Errorf("metin içindeki U+FEFF korunmalı, gelen: %q", toks[0].Literal)
	}
}
