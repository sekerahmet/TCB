package lexer

import (
	"fmt"
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Anahtar kelimelerin adları kod listesindedir (internal/lang/codes, tr.json'daki kw kodları). Bu
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
	{codes.KwIf, TokIf},
	{codes.KwElse, TokElse},
	{codes.KwMatch, TokMatch},

	// Döngüler ve akış kontrolü
	{codes.KwFor, TokFor},
	{codes.KwIn, TokIn},
	{codes.KwBreak, TokBreak},
	{codes.KwContinue, TokContinue},

	// Fonksiyon ve dönüş
	{codes.KwFunc, TokFunction},
	{codes.KwReturn, TokReturn},

	// Değişken ve sabit
	{codes.KwVar, TokVar},
	{codes.KwConst, TokConst},

	// Mantıksal ve boş değerler
	{codes.KwTrue, TokTrue},
	{codes.KwFalse, TokFalse},
	{codes.KwNone, TokNil},
	{codes.KwAnd, TokKwAnd},
	{codes.KwOr, TokKwOr},
	{codes.KwNot, TokKwNot},

	// Yapı ve kendi
	{codes.KwStruct, TokStruct},
	{codes.KwSelf, TokSelf},

	// Hata yönetimi, modül, eşzamanlılık, ertele, kesme noktası
	{codes.KwTry, TokTry},
	{codes.KwCatch, TokCatch},
	{codes.KwThrow, TokThrow},
	{codes.KwImport, TokImport},
	{codes.KwAsync, TokConcurrent},
	{codes.KwDefer, TokDefer},
	{codes.KwBreakpoint, TokBreakpoint},
}

// keywords kimlik listesinde (codes.Active) yazım → token türü, typeCodes token türü → sözlük kodu haritasıdır.
var keywords, typeCodes = keywordMap(keywordTable, codes.Spellings)

// otherKeywords yazım dillerinin anahtar kelime haritalarıdır (dil kodu → harita;
// keywordsOf gerektiğinde bir kez kurar).
var otherKeywords sync.Map

// keywordsOf dilin yazım → token türü haritasıdır.
func keywordsOf(lang *codes.Language) map[string]TokenType {
	if lang.Code == codes.Active().Code {
		return keywords
	}
	if m, ok := otherKeywords.Load(lang.Code); ok {
		return m.(map[string]TokenType)
	}
	m, _ := keywordMap(keywordTable, lang.Spellings)
	m2, _ := otherKeywords.LoadOrStore(lang.Code, m)
	return m2.(map[string]TokenType)
}

// lookupIdent kökün dilinde (NewBodyIn) bir sözcüğün anahtar kelime mi ad mı olduğudur.
func (l *Lexer) lookupIdent(ident string) TokenType {
	if l.words == nil {
		return LookupIdent(ident)
	}
	if tok, ok := l.words[ident]; ok {
		return tok
	}
	return TokIdent
}

// keywordMap tabloyu bir dilin yazımlarıyla haritaya çevirir. Aynı yazım iki
// token türüne ya da aynı tür iki koda bağlanmışsa bu bir geliştirici hatasıdır ve
// program başlarken durur.
func keywordMap(table []Keyword, spellings func(codes.Code) []string) (map[string]TokenType, map[TokenType]codes.Code) {
	posMap := make(map[string]TokenType)
	codeList := make(map[TokenType]codes.Code)
	for _, kw := range table {
		if !kw.Code.IsKeyword() {
			panic(fmt.Sprintf("keyword table: %s is not a keyword code (kw)", kw.Code))
		}
		if prev, ok := codeList[kw.Type]; ok {
			panic(fmt.Sprintf("anahtar kelime tablosu: %s hem %s hem %s", kw.Type, prev, kw.Code))
		}
		codeList[kw.Type] = kw.Code
		for _, spelling := range spellings(kw.Code) {
			if prev, ok := posMap[spelling]; ok && prev != kw.Type {
				panic(fmt.Sprintf("keyword dictionary: %q cannot be both %s and %s", spelling, prev, kw.Type))
			}
			posMap[spelling] = kw.Type
		}
	}
	return posMap, codeList
}

// LookupIdent reports whether a word is a keyword or an identifier.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return TokIdent
}
