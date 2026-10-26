package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// warnUnusedNegation değeri kullanılmayan '-x' deyimini bildirir (2.8): satır başındaki '-'
// önceki satırı sürdürmediği için 'a = 1' ardından gelen '- b' sessizce etkisiz kalırdı.
func (c *Checker) warnUnusedNegation(s *ast.ExpressionStatement) {
	if c.interactive {
		return
	}
	if p, ok := s.Expression.(*ast.PrefixExpression); ok && p.Operator == "-" {
		c.warnings = append(c.warnings, messages.At(p.Token.Line, 0, messages.New(messages.H1011)))
	}
}

// inferIf eğer ifadesinin koşulunu ve dallarını denetler.
func (c *Checker) inferIf(e *ast.IfExpression) string {
	c.inferExpression(e.Condition)
	c.checkStatement(e.Consequence)
	for _, elif := range e.ElseIfs {
		c.inferExpression(elif.Condition)
		c.checkStatement(elif.Consequence)
	}
	if e.Alternative != nil {
		c.checkStatement(e.Alternative)
	}
	return typeAny
}

// inferMatch eşle ifadesinin konusunu, desenlerini ve kollarını denetler.
func (c *Checker) inferMatch(e *ast.MatchExpression) string {
	c.inferExpression(e.Subject)
	for _, mc := range e.Cases {
		for _, p := range mc.Patterns {
			c.inferExpression(p)
		}
		if mc.Body != nil {
			c.checkStatement(mc.Body)
		}
		if mc.ValueExpr != nil {
			c.inferExpression(mc.ValueExpr)
		}
	}
	return typeAny
}
