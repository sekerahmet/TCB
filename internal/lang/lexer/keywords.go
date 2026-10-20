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

// keywordTable sözcük çözümleyicide ayrılmış bütün anahtar kelimelerdir.
var keywordTable = []Keyword{
	// Koşul ve eşleştirme
	{codes.A001, TokIf},
	{codes.A002, TokElse},
	{codes.A003, TokMatch},
	{codes.A004, TokDefault},

	// Döngüler ve akış kontrolü
	{codes.A010, TokFor},
	{codes.A011, TokIn},
	{codes.A012, TokWhile},
	{codes.A013, TokBreak},
	{codes.A014, TokContinue},

	// Fonksiyon ve dönüş
	{codes.A020, TokFunction},
	{codes.A021, TokReturn},

	// Değişken ve sabit
	{codes.A030, TokVar},
	{codes.A031, TokConst},

	// Mantıksal ve boş değerler
	{codes.A040, TokTrue},
	{codes.A041, TokFalse},
	{codes.A042, TokNil},
	{codes.A043, TokKwAnd},
	{codes.A044, TokKwOr},
	{codes.A045, TokKwNot},

	// Yapı ve kendi
	{codes.A050, TokStruct},
	{codes.A051, TokSelf},

	// Hata yönetimi, modül, eşzamanlılık, ertele, kesme noktası
	{codes.A060, TokTry},
	{codes.A061, TokCatch},
	{codes.A062, TokThrow},
	{codes.A070, TokImport},
	{codes.A080, TokConcurrent},
	{codes.A081, TokDefer},
	{codes.A090, TokBreakpoint},
}

// keywords yazım → token türü, turKodlari token türü → sözlük kodu haritasıdır.
var keywords, typeCodes = keywordMap(keywordTable)

// keywordMap tabloyu sözlükteki yazımlarla haritaya çevirir. Aynı yazım iki
// token türüne ya da aynı tür iki koda bağlanmışsa bu bir geliştirici hatasıdır ve
// program başlarken durur.
func keywordMap(table []Keyword) (map[string]TokenType, map[TokenType]codes.Code) {
	posMap := make(map[string]TokenType)
	codeList := make(map[TokenType]codes.Code)
	for _, kw := range table {
		if !kw.Code.IsKeyword() {
			panic(fmt.Sprintf("anahtar kelime tablosu: %s bir anahtar kelime kodu (A0xx) değil", kw.Code))
		}
		if prev, ok := codeList[kw.Type]; ok {
			panic(fmt.Sprintf("anahtar kelime tablosu: %s hem %s hem %s", kw.Type, prev, kw.Code))
		}
		codeList[kw.Type] = kw.Code
		for _, spelling := range codes.Spellings(kw.Code) {
			if prev, ok := posMap[spelling]; ok && prev != kw.Type {
				panic(fmt.Sprintf("anahtar kelime sözlüğü: %q hem %s hem %s olamaz", spelling, prev, kw.Type))
			}
			posMap[spelling] = kw.Type
		}
	}
	return posMap, codeList
}

// KeywordName returns the dictionary name of a keyword token type (e.g. "sonuç"), or the
// empty string for other types.
func KeywordName(t TokenType) string {
	if k, ok := typeCodes[t]; ok {
		return codes.Name(k)
	}
	return ""
}

// LookupIdent reports whether a word is a keyword or an identifier.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return TokIdent
}
