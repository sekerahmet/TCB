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

// scripts are the writing systems whose letters look alike; a script's name is a message part
// (H0033–H0035), written in the outer message's language.
var scripts = []struct {
	name  messages.Code
	table *unicode.RangeTable
}{
	{messages.H0033, unicode.Latin},
	{messages.H0034, unicode.Cyrillic},
	{messages.H0035, unicode.Greek},
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
// (Latin, Cyrillic, Greek) and names the first two as message parts (2.1a: a warning).
func MixedScripts(name string) (first, second *messages.Error, mixed bool) {
	var seen messages.Code
	for _, r := range name {
		for _, s := range scripts {
			if !unicode.Is(s.table, r) {
				continue
			}
			if seen != 0 && seen != s.name {
				return messages.New(seen), messages.New(s.name), true
			}
			seen = s.name
		}
	}
	return nil, nil, false
}
