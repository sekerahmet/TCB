package codes

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// Ref is a language element as a message parameter (messages.Localized): the catalog writes its
// name in the language of the message, so a message and the names inside it are in one language
// (TR_DIL on screen; a caught error in the language of the file that catches it). The parameter
// carries the code, not a name built when the message is made. String is the display name.
type Ref struct {
	Code Code
	Form RefForm
	// Receiver is a member's receiver type in the call forms (liste.içerir, metin.içerir); None:
	// the member's single type from its definition, x when it has several.
	Receiver Code
}

// RefForm is how a Ref is written.
type RefForm uint8

const (
	// RefName is the name itself: aralık, Tam, öğrenme_oranı.
	RefName RefForm = iota
	// RefCallName is a code as it is called: a builtin's name (yuvarla), a member with its
	// receiver (liste.filtrele): on a single type its name, otherwise x.
	RefCallName
	// RefCall is RefCallName with "()": yuvarla(), liste.filtrele().
	RefCall
	// RefReceiver is a type's name as a member's receiver, in lower case (liste, metin).
	RefReceiver
	// RefNameCall is RefName with "()": devrik() (a member without its receiver).
	RefNameCall
)

// NameRef, CallNameRef and CallRef are k as a message parameter in the three forms.
func NameRef(k Code) Ref     { return Ref{Code: k, Form: RefName} }
func CallNameRef(k Code) Ref { return Ref{Code: k, Form: RefCallName} }
func CallRef(k Code) Ref     { return Ref{Code: k, Form: RefCall} }

// WordRef is the language's word k as a message parameter about a text: in a file (lang nil: the
// file has no language, 2.1) its spelling, TR_break, as it is written there; in text written in a
// writing language (a typed line) the word in the message's language (NameRef).
func WordRef(k Code, lang *Language) any {
	if lang == nil {
		return Spelling(k)
	}
	return NameRef(k)
}

// ReceiverRef is type typ as a member's receiver (liste).
func ReceiverRef(typ Code) Ref { return Ref{Code: typ, Form: RefReceiver} }

// MemberRef is member y on receiver type typ in its call-name form (liste.içerir).
func MemberRef(typ, y Code) Ref { return Ref{Code: y, Form: RefCallName, Receiver: typ} }

// Called is r with "()": a name as devrik(), a call name as liste.filtrele().
func (r Ref) Called() Ref {
	switch r.Form {
	case RefName:
		r.Form = RefNameCall
	case RefCallName:
		r.Form = RefCall
	}
	return r
}

// MessageText writes the element in lang (messages.Localized); a language that does not name
// the code writes the canonical name.
func (r Ref) MessageText(lang string) string {
	l, err := LoadLanguage(lang)
	if err != nil {
		l = active
	}
	switch r.Form {
	case RefCallName:
		return callNameIn(l, r.Code, r.Receiver)
	case RefCall:
		return callNameIn(l, r.Code, r.Receiver) + "()"
	case RefReceiver:
		return lowerIn(l.Code, nameIn(l, r.Code))
	case RefNameCall:
		return nameIn(l, r.Code) + "()"
	}
	return nameIn(l, r.Code)
}

// String is the display name (TR_DIL), for text that is not a message.
func (r Ref) String() string { return r.MessageText(display.Code) }

// nameIn kodun l'deki adıdır; l kodu adlandırmıyorsa kanonik ad.
func nameIn(l *Language, k Code) string {
	if e, ok := l.Entry(k); ok {
		return e.Name
	}
	return active.Name(k)
}

// callNameIn kodun çağrı adıdır: üyede tek tipin alıcı adıyla (birden çok tipte x).
func callNameIn(l *Language, k, receiverType Code) string {
	if k.Namespace() != NamespaceMember {
		return nameIn(l, k)
	}
	if receiverType == None {
		if t, _ := DefinitionOf(k); len(t.Members) == 1 {
			receiverType = t.Members[0].Type
		}
	}
	receiver := "x"
	if receiverType != None {
		receiver = lowerIn(l.Code, nameIn(l, receiverType))
	}
	return receiver + "." + nameIn(l, k)
}

// lowerIn metni dilin büyük-küçük harf kuralıyla küçültür (Türkçede I → ı, İ → i).
func lowerIn(lang, s string) string {
	if lang == SourceLanguage {
		return turkish.Lower(s)
	}
	return strings.ToLower(s)
}
