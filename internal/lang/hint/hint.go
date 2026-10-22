// Package hint is the single source of the error hints about the language's names: names
// written without Turkish letters (ASCII) or familiar from other languages, and the visible
// name closest to an undefined one. The lexer, the parser, the checker (before running) and the runtime
// use the same texts. Every function runs only on the error path; names come from the code
// list (internal/lang/codes).
//
// Layer: language core (internal/lang); imports only the turkish and codes packages.
package hint

import (
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// NotInTRHint is the hint that suggests the real (Turkish) name in place of a name.
func NotInTRHint(name, canonical string) string {
	return messages.Text(messages.H1101, name, canonical)
}

// KeywordSpellingHint returns the hint to append to an error message when ad is the ASCII
// form of a keyword (degisken); otherwise the empty string. These spellings are ordinary names
// (e.g. değişken sınıf = "9-A"); only when an error occurs is the real name suggested, since
// they may have been written out of habit. The suggestion is always the Turkish form.
func KeywordSpellingHint(name string) string {
	k, ok := codes.Suggest(codes.NamespaceGlobal, name, func(k codes.Code) bool { return k.Letter() == 'A' })
	if !ok {
		return ""
	}
	return messages.Text(messages.H1103, name, codes.DisplayName(k))
}

// BuiltinSpellingHint returns the hint to append to an error message when ad is the ASCII
// form of a builtin (yazdir) or a foreign name of one (a "yabancı" name of a G/T/V code in
// the dictionary); otherwise the empty string. These spellings are ordinary names (e.g.
// değişken boy = 180); only on an undefined-name error is the real (Turkish) name
// suggested, since they may have been written out of habit.
func BuiltinSpellingHint(name string) string {
	k, ok := codes.Suggest(codes.NamespaceGlobal, name, codes.Code.IsBuiltin)
	if !ok {
		return ""
	}
	return NotInTRHint(name, codes.DisplayName(k))
}

// NameSuggestion returns the suggestion to append to the error message for an undefined name:
// the real name for an ASCII-written keyword or builtin, otherwise the visible name
// closest to the spelling ("Bunu mu demek istediniz?"); the empty string when there is none.
func NameSuggestion(name string, visible []string) string {
	if i := KeywordSpellingHint(name); i != "" {
		return i
	}
	if i := BuiltinSpellingHint(name); i != "" {
		return i
	}
	return ClosestSuggestion(name, visible)
}

// ClosestSuggestion returns the "Bunu mu demek istediniz?" suggestion (H1104) for the
// candidate closest to name; the empty string when none is close enough.
func ClosestSuggestion(name string, candidates []string) string {
	if o := turkish.Closest(name, candidates); o != "" {
		return messages.Text(messages.H1104, o)
	}
	return ""
}

// TypeTextComparison is the text reporting that a type value (tip(x)) is compared with a
// string (3.11): type names are written without quotes. If metin is a type name (or its old
// or ASCII spelling) that type's name is shown, otherwise ornek (the name of the value's own
// type). The checker and the interpreter use the same text.
func TypeTextComparison(op, text, example string) string {
	return TypeTextComparisonError(op, text, example).Error()
}

// TypeTextComparisonError is TypeTextComparison as a coded error (H1105).
func TypeTextComparisonError(op, text, example string) *messages.Error {
	if k, ok := codes.Find(codes.NamespaceType, text); ok {
		example = codes.DisplayName(k)
	} else if k, ok := codes.Suggest(codes.NamespaceType, text, nil); ok {
		example = codes.DisplayName(k)
	}
	g := codes.DisplayName(codes.G201)
	return messages.New(messages.H1105, g, op, text, example)
}

// UndefinedNameMessage returns the same message for an undefined name at run time and in the
// checker (without the position prefix).
func UndefinedNameMessage(name string, visible []string) string {
	return UndefinedNameError(name, visible).Error()
}

// UndefinedNameError is UndefinedNameMessage as a coded error (H1106).
func UndefinedNameError(name string, visible []string) *messages.Error {
	return messages.New(messages.H1106, name, NameSuggestion(name, visible))
}
