package lexer

import (
	"strings"
	"testing"
)

// tumTokenlar girdinin tüm sözcük birimlerini (DOSYA_SONU dahil) ve lexer hatalarını döndürür.
func tumTokenlar(t *testing.T, input string) ([]Token, []string) {
	t.Helper()
	l := New(input)
	var toks []Token
	for i := 0; ; i++ {
		if i > 10000 {
			t.Fatalf("lexer sonsuz döngüye girdi: %q", input)
		}
		tok := l.NextToken()
		toks = append(toks, tok)
		if tok.Type == TOKEN_EOF {
			break
		}
	}
	return toks, l.Errors()
}

// hatasizTokenlar hata beklenmeyen girdiler içindir.
func hatasizTokenlar(t *testing.T, input string) []Token {
	t.Helper()
	toks, errs := tumTokenlar(t, input)
	if len(errs) > 0 {
		t.Fatalf("%q için beklenmeyen lexer hataları: %v", input, errs)
	}
	return toks
}

type beklenen struct {
	typ TokenType
	lit string
}

func tokenlariDogrula(t *testing.T, input string, want []beklenen) {
	t.Helper()
	toks := hatasizTokenlar(t, input)
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
func TestAnahtarKelimeTablosununTamami(t *testing.T) {
	for kelime, tur := range keywords {
		toks := hatasizTokenlar(t, kelime)
		if len(toks) != 2 {
			t.Errorf("%q: tek token + DOSYA_SONU bekleniyordu, gelen: %v", kelime, toks)
			continue
		}
		if toks[0].Type != tur || toks[0].Literal != kelime {
			t.Errorf("%q: %s bekleniyordu, %s %q geldi", kelime, tur, toks[0].Type, toks[0].Literal)
		}
		if toks[0].Line != 1 || toks[0].Column != 1 {
			t.Errorf("%q: konum 1:1 bekleniyordu, %d:%d geldi", kelime, toks[0].Line, toks[0].Column)
		}
	}
}

// Türkçe karakterli ve ASCII yazımların ikisi de aynı token türünü vermeli.
// Bu liste, tablodan bir yazımın yanlışlıkla silinmesini yakalar.
func TestAnahtarKelimeTurkceVeASCIIYazim(t *testing.T) {
	ciftler := []struct {
		turkce, ascii string
		tur           TokenType
	}{
		{"değişken", "degisken", TOKEN_DEGISKEN},
		{"işlev", "islev", TOKEN_FONKSIYON},
		{"sonuç", "sonuc", TOKEN_SONUC},
		{"döndür", "dondur", TOKEN_SONUC},
		{"eğer", "eger", TOKEN_EGER},
		{"değilse", "degilse", TOKEN_DEGILSE},
		{"eşle", "esle", TOKEN_ESLE},
		{"varsayılan", "varsayilan", TOKEN_VARSAYILAN},
		{"döngü", "dongu", TOKEN_DONGU},
		{"içinde", "icinde", TOKEN_ICINDE},
		{"çık", "cik", TOKEN_CIK},
		{"kır", "kir", TOKEN_CIK},
		{"doğru", "dogru", TOKEN_DOGRU},
		{"yanlış", "yanlis", TOKEN_YANLIS},
		{"değil", "degil", TOKEN_DEGIL},
		{"yapı", "yapi", TOKEN_YAPI},
		{"sınıf", "sinif", TOKEN_SINIF},
		{"fırlat", "firlat", TOKEN_FIRLAT},
		{"eşzamanlı", "eszamanli", TOKEN_ESZAMANLI},
		{"kesme_noktası", "kesme_noktasi", TOKEN_KESME_NOKTASI},
		{"dur_noktası", "dur_noktasi", TOKEN_KESME_NOKTASI},
	}
	for _, c := range ciftler {
		for _, yazim := range []string{c.turkce, c.ascii} {
			if got := LookupIdent(yazim); got != c.tur {
				t.Errorf("LookupIdent(%q) = %s, beklenen %s", yazim, got, c.tur)
			}
		}
	}

	// Tek yazımı olan anahtar kelimeler
	tekler := map[string]TokenType{
		"sabit": TOKEN_SABIT, "fonksiyon": TOKEN_FONKSIYON, "iken": TOKEN_IKEN,
		"devam": TOKEN_DEVAM, "yok": TOKEN_YOK, "ve": TOKEN_VE, "veya": TOKEN_VEYA,
		"kendi": TOKEN_KENDI, "dene": TOKEN_DENE, "yakala": TOKEN_YAKALA,
		"aktar": TOKEN_AKTAR, "ertele": TOKEN_ERTELE, "durdurma_noktası": TOKEN_KESME_NOKTASI,
		"durdurma_noktasi": TOKEN_KESME_NOKTASI,
	}
	for kelime, tur := range tekler {
		if got := LookupIdent(kelime); got != tur {
			t.Errorf("LookupIdent(%q) = %s, beklenen %s", kelime, got, tur)
		}
	}
}

// Anahtar kelimeler büyük/küçük harfe duyarlıdır ve yalnızca tam kelime olarak eşleşir.
func TestAnahtarKelimeOlmayanlar(t *testing.T) {
	for _, s := range []string{
		"Eğer", "EĞER", "Değişken", "DOĞRU", // büyük harfli yazımlar tanımlayıcıdır
		"eğerki", "değişkenler", "dongu2", "_eger", "sabitler", // anahtar kelime önekli tanımlayıcılar
		"egér",
		"break_point", // eski İngilizce yazım kaldırıldı; artık sıradan bir ad
	} {
		toks := hatasizTokenlar(t, s)
		if len(toks) != 2 || toks[0].Type != TOKEN_IDENT || toks[0].Literal != s {
			t.Errorf("%q: tek bir TANIMLAYICI bekleniyordu, gelen: %v", s, toks)
		}
	}
}

// ---------------------------------------------------------------------------
// Tanımlayıcılar
// ---------------------------------------------------------------------------

func TestTurkceKarakterliTanimlayicilar(t *testing.T) {
	for _, s := range []string{
		"ışık", "İstanbul", "şeker", "ağaç", "üzüm", "göz", "çiçek",
		"IĞDIR", "öğrenci_sayısı", "_gizli", "x1", "değer2", "ÇÖĞŞÜİı",
	} {
		toks := hatasizTokenlar(t, s)
		if len(toks) != 2 || toks[0].Type != TOKEN_IDENT || toks[0].Literal != s {
			t.Errorf("%q: tek bir TANIMLAYICI bekleniyordu, gelen: %v", s, toks)
		}
	}
}

func TestTanimlayiciVeOperatorSinirlari(t *testing.T) {
	tokenlariDogrula(t, "öğrenci.yaş+çarpan", []beklenen{
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

func TestSayilar(t *testing.T) {
	testler := []struct {
		girdi string
		tur   TokenType
		lit   string
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
	for _, tt := range testler {
		tokenlariDogrula(t, tt.girdi, []beklenen{{tt.tur, tt.lit}, {TOKEN_EOF, ""}})
	}
}

func TestSayiVeAralikAyrimi(t *testing.T) {
	// '1..10' ondalık sayı değil, aralıktır
	tokenlariDogrula(t, "1..10", []beklenen{
		{TOKEN_INT, "1"}, {TOKEN_RANGE, ".."}, {TOKEN_INT, "10"}, {TOKEN_EOF, ""},
	})
	tokenlariDogrula(t, "0..=5", []beklenen{
		{TOKEN_INT, "0"}, {TOKEN_RANGE_INCL, "..="}, {TOKEN_INT, "5"}, {TOKEN_EOF, ""},
	})
	// Noktadan sonra rakam yoksa sayı tam sayı olarak biter (üye erişimi için)
	tokenlariDogrula(t, "5.kare", []beklenen{
		{TOKEN_INT, "5"}, {TOKEN_DOT, "."}, {TOKEN_IDENT, "kare"}, {TOKEN_EOF, ""},
	})
	tokenlariDogrula(t, "1.5..2.5", []beklenen{
		{TOKEN_FLOAT, "1.5"}, {TOKEN_RANGE, ".."}, {TOKEN_FLOAT, "2.5"}, {TOKEN_EOF, ""},
	})
}

// Mevcut davranış: sayıya bitişik harfler ayrı bir tanimlayıcı olur; lexer hata vermez
// (ayrıştırıcı ardışık ifadeyi ayrıca değerlendirir).
func TestSayiyaBitisikHarf(t *testing.T) {
	tokenlariDogrula(t, "12abc", []beklenen{
		{TOKEN_INT, "12"}, {TOKEN_IDENT, "abc"}, {TOKEN_EOF, ""},
	})
}

// ---------------------------------------------------------------------------
// Operatörler ve ayırıcılar
// ---------------------------------------------------------------------------

func TestTumOperatorler(t *testing.T) {
	testler := []struct {
		girdi string
		tur   TokenType
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
	for _, tt := range testler {
		// Hem tek başına hem de iki tanımlayıcı arasında doğru okunmalı
		tokenlariDogrula(t, tt.girdi, []beklenen{{tt.tur, tt.girdi}, {TOKEN_EOF, ""}})
		tokenlariDogrula(t, "a"+tt.girdi+"b", []beklenen{
			{TOKEN_IDENT, "a"}, {tt.tur, tt.girdi}, {TOKEN_IDENT, "b"}, {TOKEN_EOF, ""},
		})
	}
}

// En uzun eşleşme kuralı ve iki token'a bölünen birleşimler.
func TestOperatorBirlesimleri(t *testing.T) {
	testler := []struct {
		girdi string
		want  []beklenen
	}{
		// '->' tek token değildir; ayrıştırıcı '-' ve '>' çiftini dönüş tipi oku olarak tanır
		{"->", []beklenen{{TOKEN_MINUS, "-"}, {TOKEN_GT, ">"}}},
		{"**=", []beklenen{{TOKEN_POWER, "**"}, {TOKEN_ASSIGN, "="}}},
		{"===", []beklenen{{TOKEN_EQ, "=="}, {TOKEN_ASSIGN, "="}}},
		{"==>", []beklenen{{TOKEN_EQ, "=="}, {TOKEN_GT, ">"}}},
		{"...", []beklenen{{TOKEN_RANGE, ".."}, {TOKEN_DOT, "."}}},
		{"???", []beklenen{{TOKEN_NULL_COALESC, "??"}, {TOKEN_QUESTION, "?"}}},
		{"!!", []beklenen{{TOKEN_BANG, "!"}, {TOKEN_BANG, "!"}}},
		{"<<", []beklenen{{TOKEN_LT, "<"}, {TOKEN_LT, "<"}}},
		{"|||", []beklenen{{TOKEN_OR, "||"}, {TOKEN_ILLEGAL, "|"}}},
		{"x?.y??z", []beklenen{
			{TOKEN_IDENT, "x"}, {TOKEN_SAFE_DOT, "?."}, {TOKEN_IDENT, "y"},
			{TOKEN_NULL_COALESC, "??"}, {TOKEN_IDENT, "z"},
		}},
		{"liste |> filtrele", []beklenen{
			{TOKEN_IDENT, "liste"}, {TOKEN_PIPE, "|>"}, {TOKEN_IDENT, "filtrele"},
		}},
		{"(x) => x ** 2", []beklenen{
			{TOKEN_LPAREN, "("}, {TOKEN_IDENT, "x"}, {TOKEN_RPAREN, ")"}, {TOKEN_ARROW, "=>"},
			{TOKEN_IDENT, "x"}, {TOKEN_POWER, "**"}, {TOKEN_INT, "2"},
		}},
	}
	for _, tt := range testler {
		tokenlariDogrula(t, tt.girdi, append(tt.want, beklenen{TOKEN_EOF, ""}))
	}
}

func TestGecersizKarakterler(t *testing.T) {
	for _, s := range []string{"&", "|", "$", "^", "~", "`", "\\", "😀"} {
		toks := hatasizTokenlar(t, s)
		if len(toks) != 2 || toks[0].Type != TOKEN_ILLEGAL || toks[0].Literal != s {
			t.Errorf("%q: tek bir GEÇERSİZ token bekleniyordu, gelen: %v", s, toks)
		}
	}
	// Geçersiz karakterden sonra okumaya devam edilmeli
	tokenlariDogrula(t, "a $ b", []beklenen{
		{TOKEN_IDENT, "a"}, {TOKEN_ILLEGAL, "$"}, {TOKEN_IDENT, "b"}, {TOKEN_EOF, ""},
	})
}

// ---------------------------------------------------------------------------
// Metinler
// ---------------------------------------------------------------------------

func TestBasitMetinler(t *testing.T) {
	testler := []struct {
		girdi, lit string
	}{
		{`"merhaba"`, "merhaba"},
		{`'merhaba'`, "merhaba"},
		{`""`, ""},
		{`"çğışöü İĞŞ"`, "çğışöü İĞŞ"},
		{`"tek ' tırnak"`, "tek ' tırnak"},
		{`'çift " tırnak'`, `çift " tırnak`},
		{`"a}b"`, "a}b"}, // eşi olmayan '}' düz karakterdir
	}
	for _, tt := range testler {
		tokenlariDogrula(t, tt.girdi, []beklenen{{TOKEN_STRING, tt.lit}, {TOKEN_EOF, ""}})
	}
}

func TestMetinKacisDizileri(t *testing.T) {
	testler := []struct {
		girdi, lit string
	}{
		{`"a\nb"`, "a\nb"},
		{`"a\tb"`, "a\tb"},
		{`"a\rb"`, "a\rb"},
		{`"a\"b"`, `a"b`},
		{`'a\'b'`, "a'b"},
		{`"a\\b"`, `a\b`},
		// Bilinmeyen kaçış: ters bölü atılır, karakter korunur (mevcut davranış)
		{`"a\qb"`, "aqb"},
		// Kaçışlı süslü parantezler özel kullanım alanı karakterleriyle taşınır
		{`"\{düz\}"`, string(EscapedLBrace) + "düz" + string(EscapedRBrace)},
		// '\\{' ise ters bölü + interpolasyon başlangıcıdır
		{`"\\{x}"`, `\{x}`},
	}
	for _, tt := range testler {
		tokenlariDogrula(t, tt.girdi, []beklenen{{TOKEN_STRING, tt.lit}, {TOKEN_EOF, ""}})
	}
}

// İnterpolasyon bölümleri ayrıştırıcıya olduğu gibi aktarılır.
func TestMetinInterpolasyonu(t *testing.T) {
	testler := []struct {
		girdi, lit string
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
	for _, tt := range testler {
		tokenlariDogrula(t, tt.girdi, []beklenen{{TOKEN_STRING, tt.lit}, {TOKEN_EOF, ""}})
	}
}

// Metin birden fazla satıra yayılabilir; sonraki token'ın satırı doğru olmalı.
func TestCokSatirliMetin(t *testing.T) {
	toks := hatasizTokenlar(t, "\"bir\niki\nüç\" x")
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

func hataIcerirMi(errs []string, parcalar ...string) bool {
	for _, e := range errs {
		hepsi := true
		for _, p := range parcalar {
			if !strings.Contains(e, p) {
				hepsi = false
				break
			}
		}
		if hepsi {
			return true
		}
	}
	return false
}

func TestKapatilmamisMetin(t *testing.T) {
	testler := []struct {
		ad, girdi, konum string
	}{
		{"tek satır", `"merhaba`, "Satır 1, Sütun 1:"},
		{"tek tırnak", `'merhaba`, "Satır 1, Sütun 1:"},
		{"sonraki satırlarda", "değişken a = 1\n  yazdır(\"merhaba)\nb = 2\n", "Satır 2, Sütun 10:"},
		{"ters bölü ile biten", `"abc\`, "Satır 1, Sütun 1:"},
		{"kaçışlı tırnak ile biten", `x = "abc\"`, "Satır 1, Sütun 5:"},
	}
	for _, tt := range testler {
		t.Run(tt.ad, func(t *testing.T) {
			toks, errs := tumTokenlar(t, tt.girdi)
			if len(errs) != 1 {
				t.Fatalf("tek bir hata bekleniyordu, gelen: %v", errs)
			}
			if !hataIcerirMi(errs, tt.konum, "kapatılmamış metin") {
				t.Errorf("hata %q ve 'kapatılmamış metin' içermeli, gelen: %q", tt.konum, errs[0])
			}
			if toks[len(toks)-1].Type != TOKEN_EOF {
				t.Errorf("son token DOSYA_SONU olmalı: %v", toks)
			}
		})
	}
}

func TestKapatilmamisInterpolasyon(t *testing.T) {
	testler := []struct {
		ad, girdi, konum string
	}{
		{"dosya sonu", `"merhaba {ad"`, "Satır 1, Sütun 10:"},
		{"satır sonu", "yazdır(\"x {a\")\nb = 1", "Satır 1, Sütun 11:"},
		{"ikinci satırda", "a = 1\n\"ok\" + \"{ {b}\"", "Satır 2, Sütun 9:"},
		{"CRLF satır sonu", "a = 1\r\n\"x {a\r\nb\"", "Satır 2, Sütun 4:"},
	}
	for _, tt := range testler {
		t.Run(tt.ad, func(t *testing.T) {
			_, errs := tumTokenlar(t, tt.girdi)
			if len(errs) == 0 {
				t.Fatalf("hata bekleniyordu")
			}
			if !hataIcerirMi(errs, tt.konum, "'{' kapatılmamış") {
				t.Errorf("hata %q ve \"'{' kapatılmamış\" içermeli, gelen: %v", tt.konum, errs)
			}
		})
	}
}

// Hatalı bir metinden sonra lexer okumaya devam etmeli ve sonraki hataları da bildirmeli.
func TestBirdenFazlaLexerHatasi(t *testing.T) {
	_, errs := tumTokenlar(t, "\"{a\n\"tamam\"\n'son")
	if len(errs) != 2 {
		t.Fatalf("iki hata bekleniyordu, gelen: %v", errs)
	}
	if !hataIcerirMi(errs[:1], "Satır 1, Sütun 2:", "'{' kapatılmamış") {
		t.Errorf("ilk hata yanlış: %q", errs[0])
	}
	if !hataIcerirMi(errs[1:], "Satır 3, Sütun 1:", "kapatılmamış metin") {
		t.Errorf("ikinci hata yanlış: %q", errs[1])
	}
}

// ---------------------------------------------------------------------------
// Yorumlar
// ---------------------------------------------------------------------------

func TestYorumlar(t *testing.T) {
	testler := []struct {
		ad, girdi string
		want      []beklenen
	}{
		{"diyez", "a # yorum ** \"\n b", []beklenen{{TOKEN_IDENT, "a"}, {TOKEN_IDENT, "b"}}},
		{"çift bölü", "a // yorum {\nb", []beklenen{{TOKEN_IDENT, "a"}, {TOKEN_IDENT, "b"}}},
		{"blok", "a /* çok\nsatırlı ' yorum */ b", []beklenen{{TOKEN_IDENT, "a"}, {TOKEN_IDENT, "b"}}},
		{"dosya sonunda", "a # son", []beklenen{{TOKEN_IDENT, "a"}}},
		{"art arda", "# bir\n// iki\n/* üç */\nx", []beklenen{{TOKEN_IDENT, "x"}}},
		{"blok içinde yıldız", "/* a * b ** c */x", []beklenen{{TOKEN_IDENT, "x"}}},
		{"bölme yorum değildir", "a / b", []beklenen{{TOKEN_IDENT, "a"}, {TOKEN_SLASH, "/"}, {TOKEN_IDENT, "b"}}},
		{"metin içindeki # yorum değildir", `"# değil" x`, []beklenen{{TOKEN_STRING, "# değil"}, {TOKEN_IDENT, "x"}}},
		{"metin içindeki // yorum değildir", `"http://a" x`, []beklenen{{TOKEN_STRING, "http://a"}, {TOKEN_IDENT, "x"}}},
		// Mevcut davranış: blok yorumlar iç içe geçmez; ilk '*/' yorumu bitirir
		{"iç içe blok yok", "/* a /* b */ c */", []beklenen{
			{TOKEN_IDENT, "c"}, {TOKEN_ASTERISK, "*"}, {TOKEN_SLASH, "/"},
		}},
	}
	for _, tt := range testler {
		t.Run(tt.ad, func(t *testing.T) {
			tokenlariDogrula(t, tt.girdi, append(tt.want, beklenen{TOKEN_EOF, ""}))
		})
	}
}

func TestYorumSonrasiSatirNumarasi(t *testing.T) {
	toks := hatasizTokenlar(t, "/* bir\niki\nüç */ x # yorum\n  y")
	if toks[0].Line != 3 || toks[0].Column != 7 {
		t.Errorf("'x' konumu 3:7 bekleniyordu, %d:%d geldi", toks[0].Line, toks[0].Column)
	}
	if toks[1].Line != 4 || toks[1].Column != 3 {
		t.Errorf("'y' konumu 4:3 bekleniyordu, %d:%d geldi", toks[1].Line, toks[1].Column)
	}
}

func TestKapatilmamisBlokYorum(t *testing.T) {
	toks, errs := tumTokenlar(t, "a = 1\n  /* yorum\nb = 2\n")
	if len(toks) != 4 || toks[3].Type != TOKEN_EOF {
		t.Errorf("yorum dosya sonuna kadar sürmeli, gelen: %v", toks)
	}
	if len(errs) != 1 || !hataIcerirMi(errs, "Satır 2, Sütun 3:", "kapatılmamış") {
		t.Errorf("'Satır 2, Sütun 3' konumlu kapatılmamış yorum hatası bekleniyordu, gelen: %v", errs)
	}

	// Yalnızca '/*' ve tek '*' ile biten girdiler de hata vermeli, çökmemeli
	for _, s := range []string{"/*", "/**", "/* *"} {
		_, errs := tumTokenlar(t, s)
		if len(errs) != 1 {
			t.Errorf("%q: tek bir hata bekleniyordu, gelen: %v", s, errs)
		}
	}
	// '/**/' boş ama kapalı bir yorumdur
	tokenlariDogrula(t, "/**/x", []beklenen{{TOKEN_IDENT, "x"}, {TOKEN_EOF, ""}})
}

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
		"a += 2 # yorum\r\n" +
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

// Windows'ta bazı editörler dosyanın başına UTF-8 BOM (U+FEFF) ekler; lexer bunu atlamalı.
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

// ---------------------------------------------------------------------------
// Bütünleşik örnek
// ---------------------------------------------------------------------------

func TestTamProgram(t *testing.T) {
	girdi := `fonksiyon topla(a: Tam, b: Tam) -> Tam {
	sonuç a + b
}
sabit liste = [1, 2.5, "üç"]
döngü i, x içinde liste { eğer x == yok ve değil doğru { kır } }
`
	tokenlariDogrula(t, girdi, []beklenen{
		{TOKEN_FONKSIYON, "fonksiyon"}, {TOKEN_IDENT, "topla"}, {TOKEN_LPAREN, "("},
		{TOKEN_IDENT, "a"}, {TOKEN_COLON, ":"}, {TOKEN_IDENT, "Tam"}, {TOKEN_COMMA, ","},
		{TOKEN_IDENT, "b"}, {TOKEN_COLON, ":"}, {TOKEN_IDENT, "Tam"}, {TOKEN_RPAREN, ")"},
		{TOKEN_MINUS, "-"}, {TOKEN_GT, ">"}, {TOKEN_IDENT, "Tam"}, {TOKEN_LBRACE, "{"},
		{TOKEN_SONUC, "sonuç"}, {TOKEN_IDENT, "a"}, {TOKEN_PLUS, "+"}, {TOKEN_IDENT, "b"},
		{TOKEN_RBRACE, "}"},
		{TOKEN_SABIT, "sabit"}, {TOKEN_IDENT, "liste"}, {TOKEN_ASSIGN, "="},
		{TOKEN_LBRACKET, "["}, {TOKEN_INT, "1"}, {TOKEN_COMMA, ","}, {TOKEN_FLOAT, "2.5"},
		{TOKEN_COMMA, ","}, {TOKEN_STRING, "üç"}, {TOKEN_RBRACKET, "]"},
		{TOKEN_DONGU, "döngü"}, {TOKEN_IDENT, "i"}, {TOKEN_COMMA, ","}, {TOKEN_IDENT, "x"},
		{TOKEN_ICINDE, "içinde"}, {TOKEN_IDENT, "liste"}, {TOKEN_LBRACE, "{"},
		{TOKEN_EGER, "eğer"}, {TOKEN_IDENT, "x"}, {TOKEN_EQ, "=="}, {TOKEN_YOK, "yok"},
		{TOKEN_VE, "ve"}, {TOKEN_DEGIL, "değil"}, {TOKEN_DOGRU, "doğru"},
		{TOKEN_LBRACE, "{"}, {TOKEN_CIK, "kır"}, {TOKEN_RBRACE, "}"}, {TOKEN_RBRACE, "}"},
		{TOKEN_EOF, ""},
	})
}
