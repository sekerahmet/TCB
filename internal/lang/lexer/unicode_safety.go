package lexer

import (
	"fmt"
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Source text that reads differently from what it shows is rejected (2.1a):
//
//   - bidirectional control characters anywhere (comments and strings included): they reorder
//     how a line is displayed, so code can look like something it is not ("Trojan Source");
//     in a string they are written with an escape (\u{202E});
//   - invisible filler letters in names (Unicode counts them as letters).
//
// A name that mixes writing systems (θ_eski) is a warning, not an error: the checker reports it
// (MixedScripts), since the letters of machine learning formulas (θ, α) are confusable too.

// isBidiControl reports whether r changes the display order of text.
func isBidiControl(r rune) bool {
	switch {
	case r >= 0x202A && r <= 0x202E, r >= 0x2066 && r <= 0x2069:
		return true
	}
	return r == 0x200E || r == 0x200F || r == 0x061C
}

// isInvisibleLetter reports whether r is a letter that shows nothing (the Hangul fillers).
func isInvisibleLetter(r rune) bool {
	return r == 0x115F || r == 0x1160 || r == 0x3164 || r == 0xFFA0
}

// checkBidi reports every bidirectional control character of the root lexer's input with its
// line and column. Interpolation sub-lexers read the same text, so only roots call it.
func (l *Lexer) checkBidi() {
	line, col := 1, 0
	for _, r := range l.input {
		col++
		if r == '\n' {
			line, col = line+1, 0
			continue
		}
		if isBidiControl(r) {
			l.addError(line, col, messages.New(messages.H0011, fmt.Sprintf("%04X", r)))
		}
	}
}

// scripts are the writing systems whose letters look alike.
var scripts = []struct {
	name  string
	table *unicode.RangeTable
}{
	{"Latin", unicode.Latin},
	{"Kiril", unicode.Cyrillic},
	{"Yunan", unicode.Greek},
}

// checkName reports a name with an invisible letter.
func (l *Lexer) checkName(name string, line, col int) {
	for _, r := range name {
		if isInvisibleLetter(r) {
			l.addError(line, col, messages.New(messages.H0012, name, fmt.Sprintf("%04X", r)))
			return
		}
	}
}

// MixedScripts reports whether name has letters of more than one of the look-alike scripts
// (Latin, Cyrillic, Greek) and names the first two (2.1a: a warning).
func MixedScripts(name string) (first, second string, mixed bool) {
	for _, r := range name {
		for _, s := range scripts {
			if !unicode.Is(s.table, r) {
				continue
			}
			if first != "" && first != s.name {
				return first, s.name, true
			}
			first = s.name
		}
	}
	return "", "", false
}
