package parser

import "github.com/sekerahmet/TCB/internal/lang/lexer"

// NameRole is the syntactic role of a name token. It is kept for places where the name has
// no node of its own in the syntax tree (type annotations, member and struct field names,
// contextual words): the coded-form converter (internal/lang/source) works out from it, and
// from the checker's scope information (checker.NameBindings), which token is an element of
// the language (17.3).
type NameRole uint8

const (
	RoleNone         NameRole = iota
	RoleType                  // type annotation: değişken x: Tam, fonksiyon f(a: Metin): Tam
	RoleMember                // member name: x.ad, x?.ad
	RoleStructField           // struct field definition
	RoleStructMethod          // struct method definition (fonksiyon ad(...) in a struct body)
	RoleTest                  // the contextual word of a test statement (A101)
	RoleAs                    // aktar ... olarak (A100)
	RoleDictKey               // a bare-name key of a dict literal: {ad: değer}
)

// RecordRoles turns on recording of name-token roles during parsing (call it before
// ParseProgram). It is off during ordinary parsing.
func (p *Parser) RecordRoles() {
	if p.roller == nil {
		p.roller = map[[2]int]NameRole{}
	}
}

// Roles returns the recorded roles: token line and column → role. Tokens inside string
// interpolations are included (with their positions in the file).
func (p *Parser) Roles() map[[2]int]NameRole { return p.roller }

// rol kayıt açıksa bir ad sözcüğünün görevini kaydeder.
func (p *Parser) rol(tok lexer.Token, r NameRole) {
	if p.roller != nil && tok.Type == lexer.TOKEN_IDENT {
		p.roller[[2]int{tok.Line, tok.Column}] = r
	}
}
