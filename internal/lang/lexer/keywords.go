package lexer

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Anahtar kelimelerin adları kod listesindedir (internal/lang/codes, tr.json'daki A0xx kodları). Bu
// dosya yalnızca hangi kodun hangi token türünü ürettiğini söyler; adı değiştirmek için
// sözlükteki satır değişir, sözcük çözümleyici, ayrıştırıcı ve yorumlayıcı kodu değişmez. Kabul edilen
// yazımlar kod.Yazimlar'dan gelir (yalnızca Türkçe biçim; bkz. kod.ASCIIKabul).

// Keyword binds a dictionary code to the token type it produces.
type Keyword struct {
	Code codes.Code
	Type TokenType
}

// anahtarKelimeler sözcük çözümleyicide ayrılmış bütün anahtar kelimelerdir.
var anahtarKelimeler = []Keyword{
	// Koşul ve eşleştirme
	{codes.A001, TOKEN_IF},
	{codes.A002, TOKEN_ELSE},
	{codes.A003, TOKEN_MATCH},
	{codes.A004, TOKEN_DEFAULT},

	// Döngüler ve akış kontrolü
	{codes.A010, TOKEN_FOR},
	{codes.A011, TOKEN_IN},
	{codes.A012, TOKEN_WHILE},
	{codes.A013, TOKEN_BREAK},
	{codes.A014, TOKEN_CONTINUE},

	// Fonksiyon ve dönüş
	{codes.A020, TOKEN_FUNCTION},
	{codes.A021, TOKEN_RETURN},

	// Değişken ve sabit
	{codes.A030, TOKEN_VAR},
	{codes.A031, TOKEN_CONST},

	// Mantıksal ve boş değerler
	{codes.A040, TOKEN_TRUE},
	{codes.A041, TOKEN_FALSE},
	{codes.A042, TOKEN_NIL},
	{codes.A043, TOKEN_KW_AND},
	{codes.A044, TOKEN_KW_OR},
	{codes.A045, TOKEN_KW_NOT},

	// Yapı ve kendi
	{codes.A050, TOKEN_STRUCT},
	{codes.A051, TOKEN_SELF},

	// Hata yönetimi, modül, eşzamanlılık, ertele, kesme noktası
	{codes.A060, TOKEN_TRY},
	{codes.A061, TOKEN_CATCH},
	{codes.A062, TOKEN_THROW},
	{codes.A070, TOKEN_IMPORT},
	{codes.A080, TOKEN_CONCURRENT},
	{codes.A081, TOKEN_DEFER},
	{codes.A090, TOKEN_BREAKPOINT},
}

// keywords yazım → token türü, turKodlari token türü → sözlük kodu haritasıdır.
var keywords, turKodlari = anahtarKelimeHaritasi(anahtarKelimeler)

// anahtarKelimeHaritasi tabloyu sözlükteki yazımlarla haritaya çevirir. Aynı yazım iki
// token türüne ya da aynı tür iki koda bağlanmışsa bu bir geliştirici hatasıdır ve
// program başlarken durur.
func anahtarKelimeHaritasi(tablo []Keyword) (map[string]TokenType, map[TokenType]codes.Code) {
	harita := make(map[string]TokenType)
	kodlar := make(map[TokenType]codes.Code)
	for _, ak := range tablo {
		if !ak.Code.IsKeyword() {
			panic(fmt.Sprintf("anahtar kelime tablosu: %s bir anahtar kelime kodu (A0xx) değil", ak.Code))
		}
		if onceki, ok := kodlar[ak.Type]; ok {
			panic(fmt.Sprintf("anahtar kelime tablosu: %s hem %s hem %s", ak.Type, onceki, ak.Code))
		}
		kodlar[ak.Type] = ak.Code
		for _, yazim := range codes.Spellings(ak.Code) {
			if onceki, ok := harita[yazim]; ok && onceki != ak.Type {
				panic(fmt.Sprintf("anahtar kelime sözlüğü: %q hem %s hem %s olamaz", yazim, onceki, ak.Type))
			}
			harita[yazim] = ak.Type
		}
	}
	return harita, kodlar
}

// KeywordName returns the dictionary name of a keyword token type (e.g. "sonuç"), or the
// empty string for other types.
func KeywordName(t TokenType) string {
	if k, ok := turKodlari[t]; ok {
		return codes.Name(k)
	}
	return ""
}

// LookupIdent reports whether a word is a keyword or an identifier.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return TOKEN_IDENT
}
