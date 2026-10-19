package turkish

import (
	"strings"
	"unicode"
)

// Lower lowercases text with the Turkish rule: I → ı, İ → i (IŞIK → ışık, İşler → işler).
// Every case-insensitive comparison of language names and user names (table name, color
// name) uses this single rule.
func Lower(s string) string { return strings.ToLowerSpecial(unicode.TurkishCase, s) }

// Upper uppercases text with the Turkish rule: i → İ, ı → I (seç → SEÇ, limit → LİMİT).
func Upper(s string) string { return strings.ToUpperSpecial(unicode.TurkishCase, s) }
