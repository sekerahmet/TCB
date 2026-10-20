package tokenize

import "sort"

// charTable sıralı, çakışmayan [ilk, son] kod noktası aralıklarıdır.
// Tablolar karaktertablo.go'da başvuru uygulamasından çıkarılarak üretilir
// (bkz. testdata/generate_char_tables.py).
type charTable [][2]rune

func (t charTable) inside(r rune) bool {
	i := sort.Search(len(t), func(i int) bool { return t[i][1] >= r })
	return i < len(t) && t[i][0] <= r
}

// wordChar başvuru uygulamasının "sözcük karakteri" tanımıdır (Unicode
// UTS #18 \w: Alphabetic, M, Nd, Pc, Join_Control). Whitespace ön bölücüsü ve
// ek tokenların single_word seçeneği bunu kullanır.
func wordChar(r rune) bool {
	if r < 0x80 {
		return r == '_' || (r >= '0' && r <= '9') || (r|0x20 >= 'a' && r|0x20 <= 'z')
	}
	return tableWord.inside(r)
}

// exprWord düzenli ifadelerdeki \w sınıfıdır (L, M, N, Pc).
func exprWord(r rune) bool {
	if r < 0x80 {
		return r == '_' || (r >= '0' && r <= '9') || (r|0x20 >= 'a' && r|0x20 <= 'z')
	}
	return tableExprWord.inside(r)
}

// exprSpace düzenli ifadelerdeki \s sınıfıdır.
func exprSpace(r rune) bool {
	if r < 0x80 {
		return r == ' ' || (r >= '\t' && r <= '\r')
	}
	return exprSpaceTable.inside(r)
}

var (
	exprLetterTable = exprCategory["l"]
	exprDigitTable  = exprCategory["n"]
	exprSpaceTable  = exprCategory["bosluk"]
)
