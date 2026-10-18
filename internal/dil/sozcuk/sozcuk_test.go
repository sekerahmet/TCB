package sozcuk

import (
	"strings"
	"testing"
)

// tumTokenlar girdinin tüm sözcük birimlerini (DOSYA_SONU dahil) ve sözcük çözümleyici hatalarını döndürür.
func tumTokenlar(t *testing.T, input string) ([]Token, []string) {
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

// hatasizTokenlar hata beklenmeyen girdiler içindir.
func hatasizTokenlar(t *testing.T, input string) []Token {
	t.Helper()
	toks, errs := tumTokenlar(t, input)
	if len(errs) > 0 {
		t.Fatalf("%q için beklenmeyen sözcük çözümleyici hataları: %v", input, errs)
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

// Anahtar kelimelerin yalnızca Türkçe yazımı geçerlidir (her işin tek yazımı); ASCII
// yazım sıradan bir addır ve hata mesajında Türkçe biçim önerilir. Bu liste, tablodan
// bir yazımın yanlışlıkla silinmesini de yakalar.
func TestAnahtarKelimeYalnizcaTurkceYazim(t *testing.T) {
	ciftler := []struct {
		turkce, ascii string
		tur           TokenType
	}{
		{"değişken", "degisken", TOKEN_DEGISKEN},
		{"sonuç", "sonuc", TOKEN_SONUC},
		{"eğer", "eger", TOKEN_EGER},
		{"değilse", "degilse", TOKEN_DEGILSE},
		{"eşle", "esle", TOKEN_ESLE},
		{"varsayılan", "varsayilan", TOKEN_VARSAYILAN},
		{"döngü", "dongu", TOKEN_DONGU},
		{"içinde", "icinde", TOKEN_ICINDE},
		{"çık", "cik", TOKEN_CIK},
		{"doğru", "dogru", TOKEN_DOGRU},
		{"yanlış", "yanlis", TOKEN_YANLIS},
		{"değil", "degil", TOKEN_DEGIL},
		{"yapı", "yapi", TOKEN_YAPI},
		{"fırlat", "firlat", TOKEN_FIRLAT},
		{"eşzamanlı", "eszamanli", TOKEN_ESZAMANLI},
		{"kesme_noktası", "kesme_noktasi", TOKEN_KESME_NOKTASI},
	}
	for _, c := range ciftler {
		if got := LookupIdent(c.turkce); got != c.tur {
			t.Errorf("LookupIdent(%q) = %s, beklenen %s", c.turkce, got, c.tur)
		}
		if got := LookupIdent(c.ascii); got != TOKEN_IDENT {
			t.Errorf("LookupIdent(%q) = %s; ASCII yazım sıradan bir ad olmalı", c.ascii, got)
		}
		want := " ('" + c.ascii + "' TR'de anahtar kelime değil; '" + c.turkce + "' yazmak mı istediniz?)"
		if got := KaldirilanYazimIpucu(c.ascii); got != want {
			t.Errorf("KaldirilanYazimIpucu(%q) = %q, %q bekleniyordu", c.ascii, got, want)
		}
	}

	// Tek yazımı olan anahtar kelimeler
	tekler := map[string]TokenType{
		"sabit": TOKEN_SABIT, "fonksiyon": TOKEN_FONKSIYON, "iken": TOKEN_IKEN,
		"devam": TOKEN_DEVAM, "yok": TOKEN_YOK, "ve": TOKEN_VE, "veya": TOKEN_VEYA,
		"kendi": TOKEN_KENDI, "dene": TOKEN_DENE, "yakala": TOKEN_YAKALA,
		"aktar": TOKEN_AKTAR, "ertele": TOKEN_ERTELE,
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

// Bilimsel yazım: üs harfi (e/E), isteğe bağlı işaret ve en az bir rakam; sonuç Ondalık'tır.
func TestBilimselSayilar(t *testing.T) {
	testler := []struct{ girdi, lit string }{
		{"1e-5", "1e-5"},
		{"3.2E+4", "3.2E+4"},
		{"2e10", "2e10"},
		{"6.02e2_3", "6.02e23"},
	}
	for _, tt := range testler {
		tokenlariDogrula(t, tt.girdi, []beklenen{{TOKEN_FLOAT, tt.lit}, {TOKEN_EOF, ""}})
	}
	// Üsten sonra rakam yoksa 'e' sayıya ait değildir
	tokenlariDogrula(t, "2e", []beklenen{{TOKEN_INT, "2"}, {TOKEN_IDENT, "e"}, {TOKEN_EOF, ""}})
	tokenlariDogrula(t, "2e+x", []beklenen{
		{TOKEN_INT, "2"}, {TOKEN_IDENT, "e"}, {TOKEN_PLUS, "+"}, {TOKEN_IDENT, "x"}, {TOKEN_EOF, ""},
	})
	tokenlariDogrula(t, "1e3..5", []beklenen{
		{TOKEN_FLOAT, "1e3"}, {TOKEN_RANGE, ".."}, {TOKEN_INT, "5"}, {TOKEN_EOF, ""},
	})
}

// Mevcut davranış: sayıya bitişik harfler ayrı bir tanimlayıcı olur; sözcük çözümleyici hata vermez
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
	// '`' ham metin başlatır (2.6a); geçersiz karakter değildir.
	for _, s := range []string{"&", "|", "$", "^", "~", "\\", "😀"} {
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
// Bütünleşik örnek
// ---------------------------------------------------------------------------

func TestTamProgram(t *testing.T) {
	girdi := `fonksiyon topla(a: Tam, b: Tam) -> Tam {
	sonuç a + b
}
sabit liste = [1, 2.5, "üç"]
döngü i, x içinde liste { eğer x == yok ve değil doğru { çık } }
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
		{TOKEN_LBRACE, "{"}, {TOKEN_CIK, "çık"}, {TOKEN_RBRACE, "}"}, {TOKEN_RBRACE, "}"},
		{TOKEN_EOF, ""},
	})
}

// Kaldırılan eş anlamlılar artık sıradan adlardır.
func TestKaldirilanEsAnlamlilarAddir(t *testing.T) {
	for _, ad := range []string{"işlev", "islev", "döndür", "dondur", "kır", "kir", "sınıf", "sinif",
		"dur_noktası", "dur_noktasi", "durdurma_noktası", "durdurma_noktasi"} {
		if tur := LookupIdent(ad); tur != TOKEN_IDENT {
			t.Errorf("%q artık ad olmalı, %s", ad, tur)
		}
		if KaldirilanYazimIpucu(ad) == "" {
			t.Errorf("%q için ipucu bekleniyordu", ad)
		}
	}
	// Eski adın ASCII biçimi de Türkçe adı önerir (ASCII biçim önerilmez).
	for ad, want := range map[string]string{"islev": "'fonksiyon'", "dondur": "'sonuç'", "kir": "'çık'"} {
		if got := KaldirilanYazimIpucu(ad); !strings.Contains(got, want) {
			t.Errorf("KaldirilanYazimIpucu(%q) = %q, %s önermeli", ad, got, want)
		}
	}
}
