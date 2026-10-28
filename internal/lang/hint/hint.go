// Package hint is the single source of the error hints about the language's names: names
// written without Turkish letters (ASCII) or familiar from other languages, and the visible
// name closest to an undefined one. The lexer, the parser, the checker (before running) and the runtime
// use the same texts. Every function runs only on the error path; names come from the code
// list (internal/lang/codes).
//
// Layer: language core (internal/lang); imports only the turkish and codes packages.
package hint

import (
	"slices"

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
// they may have been written out of habit. The suggestion is always the real name. lang is the
// language the name was written in (nil: not known, every writing language is asked).
func KeywordSpellingHint(name string, lang *codes.Language) *messages.Error {
	k, ok := suggest(lang, codes.NamespaceGlobal, name, func(k codes.Code) bool { return !k.IsBuiltin() && !k.IsLibrary() })
	if !ok {
		return nil
	}
	return messages.New(messages.H1103, name, suggested(k, lang))
}

// BuiltinSpellingHint returns the hint to append to an error message when ad is the ASCII
// form of a builtin (yazdir) or a foreign name of one (a "yabancı" name of a builtin in the
// dictionary); otherwise nil. These spellings are ordinary names (e.g.
// değişken boy = 180); only on an undefined-name error is the real name
// suggested, since they may have been written out of habit. lang is as in KeywordSpellingHint.
func BuiltinSpellingHint(name string, lang *codes.Language) *messages.Error {
	k, ok := suggest(lang, codes.NamespaceGlobal, name, codes.Code.IsBuiltin)
	if !ok {
		return nil
	}
	return NotInTRHint(name, suggested(k, lang))
}

// suggested is how a hint names the language's word k (codes.WordRef): in a file its spelling,
// TR_print, which can be copied into the file as it is.
func suggested(k codes.Code, lang *codes.Language) any { return codes.WordRef(k, lang) }

// suggest is lang's suggestion for a spelling (codes.Language.Suggest); when lang is nil the
// text's language is not known and every writing language is asked (codes.Suggest).
func suggest(lang *codes.Language, ns codes.Namespace, spelling string, accept func(codes.Code) bool) (codes.Code, bool) {
	if lang == nil {
		return codes.Suggest(ns, spelling, accept)
	}
	return lang.Suggest(ns, spelling, accept)
}

// NameSuggestion returns the suggestion to append to the error message for an undefined name:
// the real name for an ASCII-written keyword or builtin, otherwise the visible name
// closest to the spelling ("Bunu mu demek istediniz?"); nil when there is none. lang is the
// language the name was written in (the file's; nil: not known): see ClosestSuggestion.
func NameSuggestion(name string, visible []string, lang *codes.Language) *messages.Error {
	// Dilin sözcüğü TR_'siz ya da başka bir dilin adıyla yazılmış olabilir (2.3a): print → TR_print.
	if s, ok := codes.SuggestIdentity(name); ok {
		return messages.New(messages.H1021, s)
	}
	if i := KeywordSpellingHint(name, lang); i != nil {
		return i
	}
	if i := BuiltinSpellingHint(name, lang); i != nil {
		return i
	}
	return ClosestSuggestion(name, visible, lang)
}

// ClosestSuggestion returns the "Bunu mu demek istediniz?" suggestion (H1104) for the
// candidate closest to name; nil when none is close enough. A name of the language (an identity,
// TR_print) is matched by its spelling in lang, the language name was written in, and
// suggested as its code, written in the message's language (prnt → print on an English typed line);
// when lang is nil (a file) its name in every writing language is matched and the suggestion is its
// spelling (prnt → TR_print). A user's name is matched and suggested as written.
func ClosestSuggestion(name string, candidates []string, lang *codes.Language) *messages.Error {
	var spellings []string
	var names []any
	for _, c := range candidates {
		k, err := codes.Parse(c)
		if err != nil || !codes.IsReservedName(c) {
			spellings, names = append(spellings, c), append(names, c)
			continue
		}
		for _, s := range spellingsIn(lang, k) {
			spellings, names = append(spellings, s), append(names, suggested(k, lang))
		}
	}
	if o := turkish.Closest(name, spellings); o != "" {
		return messages.New(messages.H1104, names[slices.Index(spellings, o)])
	}
	return nil
}

// spellingsIn is k's name in lang; when lang is nil, its names in every writing language.
func spellingsIn(lang *codes.Language, k codes.Code) []string {
	if lang == nil {
		return codes.WritingNames(k)
	}
	if e, ok := lang.Entry(k); ok {
		return []string{e.Name}
	}
	return nil
}

// TypeTextComparisonError is the error reporting that a type value (tip(x)) is compared with a
// string (3.11, H1105): type names are written without quotes. If text is a type name in lang,
// the language the string was written in (or its old or ASCII spelling; nil: any writing
// language), that type's name is shown (suggested: in a file its spelling), otherwise example (the
// value's own type, a message part).
// The checker and the interpreter use the same text.
func TypeTextComparisonError(op, text string, example any, lang *codes.Language) *messages.Error {
	if k, ok := findType(lang, text); ok {
		example = suggested(k, lang)
	} else if k, ok := suggest(lang, codes.NamespaceType, text, nil); ok {
		example = suggested(k, lang)
	}
	g := suggested(codes.FnType, lang)
	return messages.New(messages.H1105, g, op, text, example)
}

// findType is the type whose name in lang is text; when lang is nil, in any writing language.
func findType(lang *codes.Language, text string) (codes.Code, bool) {
	if lang != nil {
		return lang.Find(codes.NamespaceType, text)
	}
	for _, lc := range codes.Languages() {
		if l, err := codes.LoadLanguage(lc); err == nil {
			if k, ok := l.Find(codes.NamespaceType, text); ok {
				return k, true
			}
		}
	}
	return codes.None, false
}

// UndefinedNameError is the same message for an undefined name at run time and in the checker
// (without the position prefix; H1106).
func UndefinedNameError(name string, visible []string, lang *codes.Language) *messages.Error {
	return messages.New(messages.H1106, name, NameSuggestion(name, visible, lang))
}
