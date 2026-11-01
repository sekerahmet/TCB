package ast

import "github.com/sekerahmet/TCB/internal/lang/messages"

// typeTextParser reads a type written as text with identities (parser.ParseTypeText); the parser
// package registers it (RegisterTypeTextParser) so that the checker, which does not import the parser
// (it only reads the tree), reads the definitions' types with the language's own type parser.
var typeTextParser func(string) (*TypeRef, error)

// RegisterTypeTextParser sets the reader of type texts; the parser package calls it from init.
func RegisterTypeTextParser(f func(string) (*TypeRef, error)) { typeTextParser = f }

// ParseTypeText reads a type text with the registered parser (13.9: TR_List[T], T?, (T) -> U); without
// one (the parser package is not linked) it is an error.
func ParseTypeText(text string) (*TypeRef, error) {
	if typeTextParser == nil {
		return nil, messages.Internalf("no type text parser is registered (import internal/lang/parser)")
	}
	return typeTextParser(text)
}
