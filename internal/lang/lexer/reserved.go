package lexer

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// defines okunan ad, değişken adı tanımlanan bir yerde mi (2.3a: kullanıcının adı TR_ ile
// başlamaz): çoklu tanımın sonraki adları (TR_var a, b), döngü değişkenleri (TR_for i, x TR_in),
// yakalama adı (TR_catch h) ve ok fonksiyonunun parametreleri ((a, b) =>, a =>). Fonksiyonun
// parametresi kitaplıkta adlı argümandır (TR_func f(TR_lr)), burada tanım sayılmaz; öbür tanımların
// adı çözülemezse definesName'dedir, çözülürse kullanıcının dosyasını source.Prepare'in kapısı durdurur.
func (l *Lexer) defines() bool {
	s := &l.places
	top := s.top()
	switch {
	case s.prev == TokCatch, s.varDecl && s.prev == TokComma && len(s.stack) == s.declDepth:
		return true
	case s.forHeader && len(s.stack) == s.declDepth && (s.prev == TokFor || s.prev == TokComma):
		return l.aheadIs(",") || l.aheadWord(codes.Spelling(codes.KwIn))
	case top.params:
		return false
	case top.open == TokLParen && (s.prev == TokLParen || s.prev == TokComma) && l.closesBeforeArrow():
		return true
	case s.prev == TokAssign || s.prev == TokLParen || s.prev == TokComma && top.open == TokLParen:
		return l.aheadIs("=>")
	}
	return false
}

// definesName okunan, çözülemeyen ad bir tanımın adı mı: TR_var, TR_const, TR_func, TR_struct, TR_enum
// ve TR_interface'ten sonraki ad, takma ad (TR_as), fonksiyonun parametresi, yapı gövdesinin alanı ve
// durumun alanı. Burada en yakın kimliği önermek yanlış yola götürür: o da tanımlanamaz.
func (l *Lexer) definesName(place codes.Place) bool {
	s := &l.places
	switch s.prev {
	case TokVar, TokConst, TokFunction, TokStruct, TokEnum, TokInterface:
		return true
	case TokDot, TokSafeDot:
		return false
	}
	top := s.top()
	if s.importStep == 3 || place == codes.PlaceArg && top.params || place == codes.PlaceMember && top.structBody {
		return true
	}
	n := len(s.stack)
	return n > 1 && top.open == TokLParen && s.stack[n-2].structBody && (s.prev == TokLParen || s.prev == TokComma)
}

// ReservedName is the message of a user's name that starts with TR_ (2.3a, 17).
func ReservedName(lit string) error { return messages.New(messages.H0004, lit, UserName(lit)) }

// UserName is the name the message suggests for lit. Önek ASCII'dir; küçük harfli öneri
// strings.ToLower ile üretilir (TR_x → tr_x).
func UserName(lit string) string { return strings.ToLower(lit) }
