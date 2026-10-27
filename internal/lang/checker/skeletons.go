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
	}
	first := map[string]seen{}
	skeletons := map[string]string{}
	reported := map[string]bool{}
	visit := func(name string, line int) {
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
			first[sk] = seen{name, line}
		case f.name != name && !reported[name]:
			reported[name] = true
			c.addError(line, 0, messages.New(messages.H1020, name, f.name, f.line))
		}
	}
	ast.Walk(prog, func(n any) bool {
		switch x := n.(type) {
		case *ast.Identifier:
			visit(x.Value, x.Line())
		case *ast.FunctionLiteral:
			visit(x.Name, x.Line())
			for _, p := range x.Parameters {
				visit(p.Name, x.Line())
			}
		case *ast.StructStatement:
			visit(x.Name, x.Line())
			for _, f := range x.Fields {
				visit(f.Name, x.Line())
			}
		case *ast.ForInStatement:
			visit(x.KeyVar, x.Line())
			visit(x.ValueVar, x.Line())
		}
		return true
	})
}
