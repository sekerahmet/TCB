package parser

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ParseTypeText reads a type written as text with identities, as a program writes it after ':' (13.9:
// TR_Int, TR_List[T], TR_Dict[TR_String, T], T?, (T) -> U). It is the only reader of a type text: the
// definitions' types (internal/lang/codes/definitions.json) are read with it, with the grammar of
// typeAfter. Text left after the type is an error.
func ParseTypeText(text string) (*ast.TypeRef, error) {
	p := New(lexer.NewBody(":"+text, true))
	t, ok := p.typeAfter()
	if errs := p.Errs(); len(errs) > 0 {
		return nil, errs[0]
	}
	if !ok {
		return nil, messages.Internalf("type text %q is not a type", text)
	}
	if !p.peekTokenIs(lexer.TokEOF) {
		return nil, messages.Internalf("type text %q has text after the type: %q", text, p.peekToken().Literal)
	}
	return t, nil
}
