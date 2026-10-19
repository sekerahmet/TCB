// Package hint is the single source of the error hints about the language's names: removed
// or moved names, names written without Turkish letters (ASCII), and the visible name closest
// to an undefined one. The lexer, the parser, the checker (before running) and the runtime
// use the same texts. Every function runs only on the error path; names come from the code
// list (internal/lang/codes).
//
// Layer: language core (internal/lang); imports only the turkish and codes packages.
package hint

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// RemovedError is the error text for a name removed with no replacement.
func RemovedError(ad string) string {
	return fmt.Sprintf("'%s' TR'den kaldırıldı", ad)
}

// NotInTRHint is the hint that suggests the real (Turkish) name in place of a name.
func NotInTRHint(ad, asil string) string {
	return fmt.Sprintf(" ('%s' TR'de yok; '%s' yazmak mı istediniz?)", ad, asil)
}

// MovedGlobalNameError returns an error text showing the new form when ad is (among global
// names) the name or an old name of a moved code, or the ASCII form of either; otherwise
// the empty string. E.g. "'uzunluk(...)' TR'de bir fonksiyon değil; 'x.uzunluk' yazın".
func MovedGlobalNameError(ad string) string {
	eski, ok := codes.FindDead(codes.NamespaceGlobal, ad)
	if !ok {
		return ""
	}
	if codes.Removed(eski) {
		return RemovedError(ad)
	}
	yeni, _ := codes.MovedTo(eski)
	if yeni.Namespace() != codes.NamespaceMember {
		return ""
	}
	return fmt.Sprintf("'%s(...)' TR'de bir fonksiyon değil; '%s' yazın", ad, codes.MemberSpelling(yeni))
}

// KeywordSpellingHint returns the hint to append to an error message when ad is the ASCII
// form of a keyword (degisken) or a removed keyword (an "eski" name of an A code in the
// dictionary: işlev, islev); otherwise the empty string. These spellings are ordinary names
// (e.g. değişken sınıf = "9-A"); only when an error occurs is the real name suggested, since
// they may have been written out of habit. The suggestion is always the Turkish form.
func KeywordSpellingHint(ad string) string {
	k, ok := codes.Suggest(codes.NamespaceGlobal, ad, func(k codes.Code) bool { return k.Letter() == 'A' })
	if !ok {
		return ""
	}
	return fmt.Sprintf(" ('%s' TR'de anahtar kelime değil; '%s' yazmak mı istediniz?)", ad, codes.Name(k))
}

// BuiltinSpellingHint returns the hint to append to an error message when ad is the ASCII
// form of a builtin (yazdir) or a removed builtin name (an "eski" name of a G/T/V code in
// the dictionary); otherwise the empty string. These spellings are ordinary names (e.g.
// değişken boy = 180); only on an undefined-name error is the real (Turkish) name
// suggested, since they may have been written out of habit.
func BuiltinSpellingHint(ad string) string {
	k, ok := codes.Suggest(codes.NamespaceGlobal, ad, codes.Code.IsBuiltin)
	if !ok {
		return ""
	}
	return NotInTRHint(ad, codes.Name(k))
}

// NameSuggestion returns the suggestion to append to the error message for an undefined name:
// the real name for a removed or ASCII-written keyword or builtin, otherwise the visible name
// closest to the spelling ("Bunu mu demek istediniz?"); the empty string when there is none.
func NameSuggestion(ad string, gorunenler []string) string {
	if i := KeywordSpellingHint(ad); i != "" {
		return i
	}
	if i := BuiltinSpellingHint(ad); i != "" {
		return i
	}
	if o := turkish.Closest(ad, gorunenler); o != "" {
		return fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
	}
	return ""
}

// TypeTextComparison is the text reporting that a type value (tip(x)) is compared with a
// string (3.11): type names are written without quotes. If metin is a type name (or its old
// or ASCII spelling) that type's name is shown, otherwise ornek (the name of the value's own
// type). The checker and the interpreter use the same text.
func TypeTextComparison(op, metin, ornek string) string {
	if k, ok := codes.Find(codes.NamespaceType, metin); ok {
		ornek = codes.Name(k)
	} else if k, ok := codes.Suggest(codes.NamespaceType, metin, nil); ok {
		ornek = codes.Name(k)
	}
	g := codes.Name(codes.G201)
	return fmt.Sprintf("tip değeri metinle karşılaştırılamaz: %s(x) %s %q yerine %s(x) %s %s yazın (tip adları tırnaksız yazılır)",
		g, op, metin, g, op, ornek)
}

// UndefinedNameMessage returns the same message for an undefined name at run time and in the
// checker (without the position prefix).
func UndefinedNameMessage(ad string, gorunenler []string) string {
	if h := MovedGlobalNameError(ad); h != "" {
		return h
	}
	return fmt.Sprintf("'%s' adında bir değişken ya da fonksiyon yok%s", ad, NameSuggestion(ad, gorunenler))
}
