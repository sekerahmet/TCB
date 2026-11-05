package source

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// reservedNames is the gate of 2.3a and 17: a name the user defines does not start with TR_, even
// when it is a valid identity. TR_func TR_print in a module would otherwise replace the builtin in
// the importing file without a word. The lexer stops a TR_ name it cannot resolve; a name it can
// resolve has the code of its place, and only the tree says the place defines it. The library
// defines its own names (17.1), so Prepare does not ask for its files.
func reservedNames(prog *ast.Program) []error {
	g := reservedGate{seen: map[[2]int]bool{}, methods: map[*ast.FunctionLiteral]bool{}}
	ast.Walk(prog, func(n any) bool {
		switch x := n.(type) {
		case *ast.VarStatement:
			g.name(x.Name.Token)
		case *ast.MultiVarStatement:
			for _, id := range x.Names {
				g.name(id.Token)
			}
		case *ast.ImportStatement:
			g.name(x.AliasToken)
		case *ast.StructStatement:
			g.definition(x)
		case *ast.FunctionLiteral:
			if !g.methods[x] {
				g.name(x.NameToken)
			}
			for _, p := range x.Parameters {
				g.name(p.NameToken)
			}
		}
		return true
	})
	return g.errs
}

// ParseTyped reads text typed while a program runs (the debugger's expressions and console): it
// runs in the program's scope, so the gate of Prepare stands here too.
func ParseTyped(text string) (*ast.Program, []error) {
	p := parser.New(TypedLexer(text))
	prog := p.ParseProgram()
	if errs := p.Errs(); len(errs) > 0 {
		return prog, errs
	}
	return prog, reservedNames(prog)
}

// reservedGate collects the errors of one program. A name that two nodes carry is reported once.
type reservedGate struct {
	errs    []error
	seen    map[[2]int]bool
	methods map[*ast.FunctionLiteral]bool
}

// definition checks the name of a struct, enum or interface and of an enum's states. Its methods
// are visited as functions: they are remembered here, so that they are not read as functions of
// the file.
func (g *reservedGate) definition(s *ast.StructStatement) {
	g.name(s.NameToken)
	for _, v := range s.Variants {
		g.name(v.NameToken)
	}
	for _, m := range s.Methods {
		g.methods[m] = true
	}
}

// name reports tok when it is written with the identity prefix. A zero token has no literal: the
// parser made the name, or the optional name is not written.
func (g *reservedGate) name(tok lexer.Token) {
	if !strings.HasPrefix(tok.Literal, codes.IdentityPrefix) {
		return
	}
	at := [2]int{tok.Line, tok.Column}
	if g.seen[at] {
		return
	}
	g.seen[at] = true
	written := tok.Literal
	if tok.Code != codes.None {
		written = codes.Spelling(tok.Code) // the token keeps the classed form (TR_arg_lr)
	}
	e := messages.AtRange(tok.Range(), lexer.ReservedName(written))
	g.errs = append(g.errs, e.WithFix(messages.Fix{Range: e.Range(), Replacement: lexer.UserName(written)}))
}
