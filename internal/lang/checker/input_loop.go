package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Girdi döngüsü tuzağı (13.1): TR_input giriş bitince TR_none verir; 'TR_while TR_input() != "son"'
// koşulu TR_none'da hep doğrudur, girdi bitince döngü durmaz, boş dönmeye sürer (Python'un
// 'while line := input()' alışkanlığı; Python'da girdi sonu EOFError'dur). Koşul doğrudan TR_input
// çağrısını TR_none olmayan bir değerle '!=' ile karşılaştırıyorsa denetim hatasıdır (H1162); ileti
// girdi sonunu da bitiş sayan yazımı verir: '(TR_input() ?? "") != "son"'. Satırı önce değişkene alan
// biçim yakalanmaz ('TR_var s = TR_input()' ... 'TR_while s != ""'), orada değişkenin tipi TR_String?'dir.

// checkInputLoop döngü koşulundaki girdi tuzağını denetler.
func (c *Checker) checkInputLoop(cond ast.Expression) {
	e, ok := cond.(*ast.InfixExpression)
	if !ok || e.Operator != lexer.TokNotEq {
		return
	}
	for _, side := range [][2]ast.Expression{{e.Left, e.Right}, {e.Right, e.Left}} {
		call, ok := side[0].(*ast.CallExpression)
		if !ok || isNoneLiteral(side[1]) {
			continue
		}
		if k, _ := c.calledBuiltin(call); k != codes.FnInput {
			continue
		}
		callText, ok1 := c.exprText(call)
		other, ok2 := c.exprText(side[1])
		if !ok1 || !ok2 {
			return
		}
		c.addErrorAt(call, messages.New(messages.H1162, callText, other))
		return
	}
}

// isNoneLiteral ifade TR_none sözcüğü mü.
func isNoneLiteral(e ast.Expression) bool {
	_, ok := e.(*ast.NullLiteral)
	return ok
}
