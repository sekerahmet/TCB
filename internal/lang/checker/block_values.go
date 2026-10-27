package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// checkUnusedValue değeri kullanılmayan, çağrı içermeyen ifade deyimini hata sayar (2.8): satır
// başındaki ikili işleç önceki satırı sürdürmediği için 'a = 1' ardından gelen '- b' ya da '+ c'
// bölünmüş bir formülün kopan parçasıdır; uyarı geçilebileceği için hatadır (Go da yalnız çağrıyı
// deyim olarak kabul eder). Değeri kullanılan bir bloğun son ifadesi (blockValues) bloğun
// değeridir; deyim olarak yazılan eğer ve eşle, fonksiyon tanımı ve kabuk satırı (interactive)
// sayılmaz.
func (c *Checker) checkUnusedValue(s *ast.ExpressionStatement) {
	if c.interactive || c.blockValues[s] || s.Expression == nil {
		return
	}
	switch s.Expression.(type) {
	case *ast.IfExpression, *ast.MatchExpression, *ast.FunctionLiteral:
		return
	}
	if containsCall(s.Expression) {
		return
	}
	c.addError(s.Line(), 0, messages.New(messages.H1017))
}

// containsCall ifadenin (metin yerleştirmeleri dahil) bir çağrı içerdiğidir.
func containsCall(e ast.Expression) bool {
	found := false
	ast.Walk(e, func(n any) bool {
		if _, ok := n.(*ast.CallExpression); ok {
			found = true
		}
		return !found
	})
	return found
}

// markBlockValue değeri kullanılan bloğun son deyimini, ifade deyimiyse, bloğun değeri olarak işaretler.
func (c *Checker) markBlockValue(b *ast.BlockStatement, used bool) {
	if !used || b == nil || len(b.Statements) == 0 {
		return
	}
	if s, ok := b.Statements[len(b.Statements)-1].(*ast.ExpressionStatement); ok {
		if c.blockValues == nil {
			c.blockValues = map[*ast.ExpressionStatement]bool{}
		}
		c.blockValues[s] = true
	}
}

// inferIf eğer ifadesinin koşulunu ve dallarını denetler; used, değerinin kullanıldığıdır (deyim olarak
// yazılmadı): o zaman dalların son ifadesi değerdir.
func (c *Checker) inferIf(e *ast.IfExpression, used bool) string {
	c.inferExpression(e.Condition)
	c.markBlockValue(e.Consequence, used)
	c.checkStatement(e.Consequence)
	for _, elif := range e.ElseIfs {
		c.inferExpression(elif.Condition)
		c.markBlockValue(elif.Consequence, used)
		c.checkStatement(elif.Consequence)
	}
	if e.Alternative != nil {
		c.markBlockValue(e.Alternative, used)
		c.checkStatement(e.Alternative)
	}
	return typeAny
}

// inferMatch eşle ifadesinin konusunu, desenlerini ve kollarını denetler; used inferIf'teki gibidir.
func (c *Checker) inferMatch(e *ast.MatchExpression, used bool) string {
	c.inferExpression(e.Subject)
	for _, mc := range e.Cases {
		for _, p := range mc.Patterns {
			c.inferExpression(p)
		}
		if mc.Body != nil {
			c.markBlockValue(mc.Body, used)
			c.checkStatement(mc.Body)
		}
		if mc.ValueExpr != nil {
			c.inferExpression(mc.ValueExpr)
		}
	}
	return typeAny
}
