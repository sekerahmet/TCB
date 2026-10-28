package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// checkSkeletons dosyanın adları arasında Unicode benzerlik iskeleti (UTS #39) aynı olan iki ayrı adı
// hata sayar (2.1a): yetki ile son harfi Kiril і olan yetkі, rn ile m. Okunan ile çalışan aynı
// olmalıdır; görünüşü aynı iki ad okuyanın birini öbürü sanmasına yol açar. Her ad ilk geçtiği
// satırla tutulur; aynı iskeletli ikinci ad o satırı anan bir hata alır. Kimlikler (TR_print)
// karşılaştırılmaz. Aktarılan adlarla karşılaştırma tip denetimiyle gelir (4b).
func (c *Checker) checkSkeletons(prog *ast.Program) {
	type seen struct {
		name string
		line int
		tok  lexer.Token // where the name is written (no column when the tree kept none)
	}
	first := map[string]seen{}
	skeletons := map[string]string{}
	reported := map[string]bool{}
	visit := func(name string, tok lexer.Token, line int) {
		tok = siteOf(tok, line)
		if name == "" || name == "_" || codes.IsReservedName(name) {
			return
		}
		sk, ok := skeletons[name]
		if !ok {
			sk = lexer.Skeleton(name)
			skeletons[name] = sk
		}
		f, ok := first[sk]
		switch {
		case !ok:
			first[sk] = seen{name, line, tok}
		case f.name != name && !reported[name]:
			reported[name] = true
			e := c.addErrorAtToken(tok, messages.New(messages.H1020, name, f.name, f.line))
			if f.tok.Column > 0 {
				e.WithRelated(messages.Related{Range: f.tok.Range(), Err: messages.New(messages.H8300, f.name)})
			}
		}
	}
	ast.Walk(prog, func(n any) bool {
		switch x := n.(type) {
		case *ast.Identifier:
			visit(x.Value, x.Token, x.Line())
		case *ast.FunctionLiteral:
			visit(x.Name, x.NameToken, x.Line())
			for _, p := range x.Parameters {
				visit(p.Name, p.NameToken, x.Line())
			}
		case *ast.StructStatement:
			visit(x.Name, x.NameToken, x.Line())
			for _, f := range x.Fields {
				visit(f.Name, f.NameToken, x.Line())
			}
		case *ast.ForInStatement:
			visit(x.KeyVar, x.KeyToken, x.Line())
			visit(x.ValueVar, x.ValueToken, x.Line())
		}
		return true
	})
}
