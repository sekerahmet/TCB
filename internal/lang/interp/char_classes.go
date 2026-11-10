package interp

import (
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// setupCharClassMethods metnin sınıf metotlarını kaydeder (13.2): .TR_is_alpha (Unicode harfi, genel
// kategori L), .TR_is_digit (ondalık rakam, Nd: "²" değil, "٣" rakam), .TR_is_space (.TR_split()'in
// boşluğu, unicode.IsSpace). Sınıf her yazıda aynıdır, okuyanın dilinden bağımsızdır.
func setupCharClassMethods() {
	for _, class := range []struct {
		k  codes.Code
		is func(rune) bool
	}{
		{codes.MemIsAlpha, unicode.IsLetter},
		{codes.MemIsDigit, unicode.IsDigit},
		{codes.MemIsSpace, unicode.IsSpace},
	} {
		addMethod(typeString, class.k, signature{}, func(y *VM, inst Value, args []Value, line int) (Value, error) {
			return BoolToVal(everyRune(inst.(*StringVal).Val, class.is)), nil
		})
	}
}

// everyRune metin boş değilse ve her karakteri is'ten geçiyorsa doğrudur (boş metin TR_false).
func everyRune(s string, is func(rune) bool) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !is(r) {
			return false
		}
	}
	return true
}
