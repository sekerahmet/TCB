package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
)

// Tip değeri metinle karşılaştırılmaz (docs/spec/03_tipler.md 3.11): tip(x) bir tip değeri
// verir, tip adları tırnaksız yazılır (tip(x) == Tam). tip(x) == "Tam" çalışma anında da
// hatadır; denetleyici, yerleşik tip() çağrısının bir metin sabitiyle karşılaştırıldığını
// çalıştırmadan önce bulur.

// checkTypeTextComparison e, yerleşik tip()'in sonucunu bir metin sabitiyle (== ya da !=)
// karşılaştırıyorsa hata ekler.
func (c *Checker) checkTypeTextComparison(e *ast.InfixExpression) {
	if e.Operator != "==" && e.Operator != "!=" {
		return
	}
	text, ok := e.Right.(*ast.StringLiteral)
	call := e.Left
	if !ok {
		text, ok = e.Left.(*ast.StringLiteral)
		call = e.Right
	}
	if !ok || !c.isTypeCall(call) {
		return
	}
	c.addError(e.Line(), 0, hint.TypeTextComparisonError(e.Operator, text.Value, codes.Name(codes.A200)))
}

// isTypeCall ifade, kullanıcının gölgelemediği yerleşik tip()'in bir çağrısı mı?
func (c *Checker) isTypeCall(expression ast.Expression) bool {
	call, ok := expression.(*ast.CallExpression)
	if !ok {
		return false
	}
	name, ok := call.Function.(*ast.Identifier)
	if !ok || name.Value != codes.Name(codes.G201) {
		return false
	}
	_, shadowed := c.scope.Resolve(name.Value)
	return !shadowed
}
