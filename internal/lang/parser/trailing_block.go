package parser

import (
	"slices"

	"github.com/sekerahmet/TCB/internal/lang/ast"
)

// isReduceTarget sondaki bloğun bağlandığı ifade bir liste.indirge çağrısı mı
// (l.indirge { ... }, l.indirge(0) { ... })? Bu bloklar iki parametrelidir (birikim, o).
func (p *Parser) isReduceTarget(e ast.Expression) bool {
	if c, ok := e.(*ast.CallExpression); ok {
		e = c.Function
	}
	m, ok := e.(*ast.MemberExpression)
	return ok && slices.Contains(p.words.reduce, m.Property)
}
