package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// checkWrittenAny program metninde yazılan TR_Any'yi bildirir: her değeri tutan genel bir tip yoktur
// (3.7a). TR_Any denetleyicinin tipi bilinmeyen değer için içeride kullandığı addır; katalogdaki
// dönüşler ve çıkarılamayan tip kalır. Tek geçiş bütün ağaca bakar: tip yazımının her yeri ve değer.
func (c *Checker) checkWrittenAny(prog *ast.Program) {
	ast.Walk(prog, func(n any) bool {
		switch n := n.(type) {
		case *ast.TypeRef:
			c.checkTypeRefAny(n)
			return false // Walk iç içe tip yazımlarının yalnız bir kısmını gösterir: tamamı burada gezildi
		case *ast.Identifier:
			c.reportAny(n.Token)
		}
		return true
	})
}

// checkTypeRefAny t'nin ve içindeki bütün tip yazımlarının (öğe, demet, fonksiyon tipi) TR_Any'sini bildirir.
func (c *Checker) checkTypeRefAny(t *ast.TypeRef) {
	c.reportAny(t.Token)
	for _, list := range [][]ast.TypeRef{t.Args, t.Elems, t.Params} {
		for i := range list {
			c.checkTypeRefAny(&list[i])
		}
	}
	if t.Return != nil {
		c.checkTypeRefAny(t.Return)
	}
}

// reportAny yazılan ad TR_Any ise hatayı adın yerine koyar.
func (c *Checker) reportAny(tok lexer.Token) {
	if tok.Code == codes.TypeAny {
		c.addErrorAtToken(tok, messages.New(messages.H1147))
	}
}
