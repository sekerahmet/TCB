package turkish

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Printable returns s with control characters made visible (\x1b, \x07, \u009b ...), for
// writing errors and warnings to a terminal: data from outside (a file name, a JSON value) in a
// message cannot clear the screen or hide text. Newline and tab are kept; invalid UTF-8 becomes
// U+FFFD. Program output (yazdır) is not passed through this: colors there are the program's
// own choice.
func Printable(s string) string {
	if isPrintable(s) {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r >= 0x80 && r < 0xa0:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r) // geçersiz UTF-8 burada U+FFFD olarak gelir
		}
	}
	return b.String()
}

// isPrintable s'nin hiç değişmeyeceğini söyler (sıcak yol: çoğu ileti düz metindir).
func isPrintable(s string) bool {
	for _, r := range s {
		if (r < 0x20 && r != '\n' && r != '\t') || (r >= 0x7f && r < 0xa0) || r == utf8.RuneError {
			return false
		}
	}
	return true
}
