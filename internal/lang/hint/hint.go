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
// they may have been written out of habit. The suggestion is always the real name.
func KeywordSpellingHint(name string) *messages.Error {
	k, ok := codes.Suggest(codes.NamespaceGlobal, name, func(k codes.Code) bool { return !k.IsBuiltin() })
	if !ok {
		return nil
	}
	return messages.New(messages.H1103, name, codes.NameRef(k))
}

// BuiltinSpellingHint returns the hint to append to an error message when ad is the ASCII
// form of a builtin (yazdir) or a foreign name of one (a "yabancı" name of a builtin in the
// dictionary); otherwise nil. These spellings are ordinary names (e.g.
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
// closest to the spelling ("Bunu mu demek istediniz?"); nil when there is none. lang is the
// language the name was written in (the file's; nil: canonical): see closestVisible.
func NameSuggestion(name string, visible []string, lang *codes.Language) *messages.Error {
	if i := KeywordSpellingHint(name); i != nil {
		return i
	}
	if i := BuiltinSpellingHint(name); i != nil {
		return i
	}
	return closestVisible(name, visible, lang)
}

// closestVisible is ClosestSuggestion for visible names (canonical identities): a name of the
// language is matched by its spelling in lang and suggested as its code, written in the message's
// language (prnt → print in an English file); a user's name is matched and suggested as written.
func closestVisible(name string, visible []string, lang *codes.Language) *messages.Error {
	if lang == nil {
		lang = codes.Source() // DENEME (iç biçim kod): adlar kimlik, yazım Türkçe
	}
	spellings := make([]string, len(visible))
	suggested := make([]any, len(visible))
	for i, v := range visible {
		spellings[i], suggested[i] = v, v
		k, ok := codes.Find(codes.NamespaceGlobal, v)
		if !ok {
			k, ok = codes.Find(codes.NamespaceType, v)
		}
		if ok {
			suggested[i] = codes.NameRef(k)
			if lang != nil {
				spellings[i] = lang.Name(k)
			}
		}
	}
	o := turkish.Closest(name, spellings)
	if o == "" {
		return nil
	}
	return messages.New(messages.H1104, suggested[slices.Index(spellings, o)])
}

// ClosestSuggestion returns the "Bunu mu demek istediniz?" suggestion (H1104) for the
// candidate closest to name; nil when none is close enough.
func ClosestSuggestion(name string, candidates []string) *messages.Error {
	// DENEME (iç biçim kod): kimlik olan aday Türkçe yazımıyla karşılaştırılır, iletinin dilinde önerilir.
	spellings := make([]string, len(candidates))
	suggested := make([]any, len(candidates))
	for i, c := range candidates {
		spellings[i], suggested[i] = c, c
		if k, err := codes.Parse(c); err == nil && codes.IsReservedName(c) {
			spellings[i], suggested[i] = codes.Source().Name(k), codes.NameRef(k)
		}
	}
	if o := turkish.Closest(name, spellings); o != "" {
		return messages.New(messages.H1104, suggested[slices.Index(spellings, o)])
	}
	return nil
}

// TypeTextComparisonError is the error reporting that a type value (tip(x)) is compared with a
// string (3.11, H1105): type names are written without quotes. If text is a type name (or its
// old or ASCII spelling) that type's name is shown, otherwise example (the value's own type, a
// message part). The checker and the interpreter use the same text.
func TypeTextComparisonError(op, text string, example any) *messages.Error {
	if k, ok := codes.Source().Find(codes.NamespaceType, text); ok { // DENEME: metin Türkçe yazımdır
		example = codes.NameRef(k)
	} else if k, ok := codes.Suggest(codes.NamespaceType, text, nil); ok {
		example = codes.NameRef(k)
	}
	g := codes.NameRef(codes.FnType)
	return messages.New(messages.H1105, g, op, text, example)
}

// UndefinedNameError is the same message for an undefined name at run time and in the checker
// (without the position prefix; H1106).
func UndefinedNameError(name string, visible []string, lang *codes.Language) *messages.Error {
	return messages.New(messages.H1106, name, NameSuggestion(name, visible, lang))
}
