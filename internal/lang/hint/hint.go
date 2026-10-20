// Package hint is the single source of the error hints about the language's names: removed
// or moved names, names written without Turkish letters (ASCII), and the visible name closest
// to an undefined one. The lexer, the parser, the checker (before running) and the runtime
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

// RemovedError is the error text for a name removed with no replacement.
func RemovedError(name string) string {
	return messages.Text(messages.H1100, name)
}

// NotInTRHint is the hint that suggests the real (Turkish) name in place of a name.
func NotInTRHint(name, canonical string) string {
	return messages.Text(messages.H1101, name, canonical)
}

// MovedGlobalNameError returns an error text showing the new form when ad is (among global
// names) the name or an old name of a moved code, or the ASCII form of either; otherwise
// the empty string. E.g. "'uzunluk(...)' TR'de bir fonksiyon değil; 'x.uzunluk' yazın".
func MovedGlobalNameError(name string) string {
	old, ok := codes.FindDead(codes.NamespaceGlobal, name)
	if !ok {
		return ""
	}
	if codes.Removed(old) {
		return RemovedError(name)
	}
	newCode, _ := codes.MovedTo(old)
	if newCode.Namespace() != codes.NamespaceMember {
		return ""
	}
	return messages.Text(messages.H1102, name, codes.MemberSpelling(newCode))
}

// KeywordSpellingHint returns the hint to append to an error message when ad is the ASCII
// form of a keyword (degisken) or a removed keyword (an "eski" name of an A code in the
// dictionary: işlev, islev); otherwise the empty string. These spellings are ordinary names
// (e.g. değişken sınıf = "9-A"); only when an error occurs is the real name suggested, since
// they may have been written out of habit. The suggestion is always the Turkish form.
func KeywordSpellingHint(name string) string {
	k, ok := codes.Suggest(codes.NamespaceGlobal, name, func(k codes.Code) bool { return k.Letter() == 'A' })
	if !ok {
		return ""
	}
	return messages.Text(messages.H1103, name, codes.Name(k))
}

// BuiltinSpellingHint returns the hint to append to an error message when ad is the ASCII
// form of a builtin (yazdir) or a removed builtin name (an "eski" name of a G/T/V code in
// the dictionary); otherwise the empty string. These spellings are ordinary names (e.g.
// değişken boy = 180); only on an undefined-name error is the real (Turkish) name
// suggested, since they may have been written out of habit.
func BuiltinSpellingHint(name string) string {
	k, ok := codes.Suggest(codes.NamespaceGlobal, name, codes.Code.IsBuiltin)
	if !ok {
		return ""
	}
	return NotInTRHint(name, codes.Name(k))
}

// NameSuggestion returns the suggestion to append to the error message for an undefined name:
// the real name for a removed or ASCII-written keyword or builtin, otherwise the visible name
// closest to the spelling ("Bunu mu demek istediniz?"); the empty string when there is none.
func NameSuggestion(name string, visible []string) string {
	if i := KeywordSpellingHint(name); i != "" {
		return i
	}
	if i := BuiltinSpellingHint(name); i != "" {
		return i
	}
	if o := turkish.Closest(name, visible); o != "" {
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
		example = codes.Name(k)
	} else if k, ok := codes.Suggest(codes.NamespaceType, text, nil); ok {
		example = codes.Name(k)
	}
	g := codes.Name(codes.G201)
	return messages.New(messages.H1105, g, op, text, example)
}

// UndefinedNameMessage returns the same message for an undefined name at run time and in the
// checker (without the position prefix).
func UndefinedNameMessage(name string, visible []string) string {
	if h := MovedGlobalNameError(name); h != "" {
		return h
	}
	return messages.Text(messages.H1106, name, NameSuggestion(name, visible))
}
