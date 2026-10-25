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

// The hints are message parts (*messages.Error, nil when there is none): the outer message writes
// them in its own language, so a hint and the names in it follow the message (TR_DIL on screen,
// the catching file's language in a caught error).

// NotInTRHint is the hint that suggests the real name (canonical, a codes.Ref) in place of a name.
func NotInTRHint(name string, canonical any) *messages.Error {
	return messages.New(messages.H1101, name, canonical)
}

// KeywordSpellingHint returns the hint to append to an error message when ad is the ASCII
// form of a keyword (degisken); otherwise nil. These spellings are ordinary names
// (e.g. değişken sınıf = "9-A"); only when an error occurs is the real name suggested, since
// they may have been written out of habit. The suggestion is always the real name.
func KeywordSpellingHint(name string) *messages.Error {
	k, ok := codes.Suggest(codes.NamespaceGlobal, name, func(k codes.Code) bool { return k.Letter() == 'A' })
	if !ok {
		return nil
	}
	return messages.New(messages.H1103, name, codes.NameRef(k))
}

// BuiltinSpellingHint returns the hint to append to an error message when ad is the ASCII
// form of a builtin (yazdir) or a foreign name of one (a "yabancı" name of a G/T/V code in
// the dictionary); otherwise nil. These spellings are ordinary names (e.g.
// değişken boy = 180); only on an undefined-name error is the real name
// suggested, since they may have been written out of habit.
func BuiltinSpellingHint(name string) *messages.Error {
	k, ok := codes.Suggest(codes.NamespaceGlobal, name, codes.Code.IsBuiltin)
	if !ok {
		return nil
	}
	return NotInTRHint(name, codes.NameRef(k))
}

// NameSuggestion returns the suggestion to append to the error message for an undefined name:
// the real name for an ASCII-written keyword or builtin, otherwise the visible name
// closest to the spelling ("Bunu mu demek istediniz?"); nil when there is none.
func NameSuggestion(name string, visible []string) *messages.Error {
	if i := KeywordSpellingHint(name); i != nil {
		return i
	}
	if i := BuiltinSpellingHint(name); i != nil {
		return i
	}
	return ClosestSuggestion(name, visible)
}

// ClosestSuggestion returns the "Bunu mu demek istediniz?" suggestion (H1104) for the
// candidate closest to name; nil when none is close enough.
func ClosestSuggestion(name string, candidates []string) *messages.Error {
	if o := turkish.Closest(name, candidates); o != "" {
		return messages.New(messages.H1104, o)
	}
	return nil
}

// TypeTextComparisonError is the error reporting that a type value (tip(x)) is compared with a
// string (3.11, H1105): type names are written without quotes. If text is a type name (or its
// old or ASCII spelling) that type's name is shown, otherwise example (the value's own type, a
// message part). The checker and the interpreter use the same text.
func TypeTextComparisonError(op, text string, example any) *messages.Error {
	if k, ok := codes.Find(codes.NamespaceType, text); ok {
		example = codes.NameRef(k)
	} else if k, ok := codes.Suggest(codes.NamespaceType, text, nil); ok {
		example = codes.NameRef(k)
	}
	g := codes.NameRef(codes.G201)
	return messages.New(messages.H1105, g, op, text, example)
}

// UndefinedNameError is the same message for an undefined name at run time and in the checker
// (without the position prefix; H1106).
func UndefinedNameError(name string, visible []string) *messages.Error {
	return messages.New(messages.H1106, name, NameSuggestion(name, visible))
}
