package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Değer yerleri (3.5): değer vermeyen çağrı (no_value.go) değer yerinde denetim hatasıdır, H1143.
// Denetim tip gezintisinden sonra bütün ağaç üzerinde yapılır ve varsayılanı kapalıdır: her çağrının
// yeri değer yeridir, yalnız muaf yerler sayılır. Muaflar: deyim; TR_async f(); `=>`'nin ifade gövdesi; sondaki bloğun son ifadesi; değeri kullanılmayan TR_if, TR_match ve TR_try'ın
// dallarının son ifadesi.

// checkValuePlaces değer yerindeki değersiz çağrıları bildirir.
func (c *Checker) checkValuePlaces(prog *ast.Program) {
	if len(c.noValue) == 0 {
		return
	}
	free := exemptPlaces(prog)
	ast.Walk(prog, func(n any) bool {
		if e, ok := n.(*ast.CallExpression); ok && c.noValue[e] && !free[e] {
			text, ok := c.exprText(e)
			if !ok {
				text = c.nodeText(e)
			}
			c.addErrorAt(e, messages.New(messages.H1143, text))
		}
		return true
	})
}

// exemptPlaces değeri kullanılmayan ifadelerdir.
func exemptPlaces(prog *ast.Program) map[ast.Expression]bool {
	tails := branchTails(prog)
	free := map[ast.Expression]bool{}
	ast.Walk(prog, func(n any) bool {
		switch s := n.(type) {
		case *ast.ExpressionStatement:
			if !tails[s] {
				release(s.Expression, free)
			}
		case *ast.ReturnStatement:
			if s.Token.Type == lexer.TokArrow {
				release(s.ReturnValue, free)
			}
		case *ast.AsyncStatement:
			free[s.Call] = true
		}
		return true
	})
	return free
}

// release ifadeyi ve, ifade dallıysa, dallarının son ifadesini muaf sayar.
func release(expr ast.Expression, free map[ast.Expression]bool) {
	free[expr] = true
	for _, b := range branches(expr) {
		if s := tailOf(b); s != nil {
			release(s.Expression, free)
		}
	}
	if m, ok := expr.(*ast.MatchExpression); ok {
		for _, mc := range m.Cases {
			if mc.ValueExpr != nil {
				release(mc.ValueExpr, free)
			}
		}
	}
}

// branches TR_if, TR_match ve TR_try ifadesinin dal bloklarıdır; öbür ifadelerde nil.
func branches(expr ast.Expression) []*ast.BlockStatement {
	var out []*ast.BlockStatement
	switch e := expr.(type) {
	case *ast.IfExpression:
		out = append(out, e.Consequence)
		for _, elif := range e.ElseIfs {
			out = append(out, elif.Consequence)
		}
		out = append(out, e.Alternative)
	case *ast.MatchExpression:
		for _, mc := range e.Cases {
			out = append(out, mc.Body)
		}
	case *ast.TryExpression:
		out = append(out, e.Statement.TryBody, e.Statement.CatchBody)
	}
	return out
}

// tailOf bloğun son deyimi ifade deyimiyse onu verir.
func tailOf(b *ast.BlockStatement) *ast.ExpressionStatement {
	if b == nil || len(b.Statements) == 0 {
		return nil
	}
	s, _ := b.Statements[len(b.Statements)-1].(*ast.ExpressionStatement)
	return s
}

// branchTails TR_if, TR_match ve TR_try ifadelerinin dallarının son ifade deyimleridir: değeri
// kullanılan ifadede dalın değeridirler, deyim sayılmazlar.
func branchTails(prog *ast.Program) map[*ast.ExpressionStatement]bool {
	tails := map[*ast.ExpressionStatement]bool{}
	ast.Walk(prog, func(n any) bool {
		if e, ok := n.(ast.Expression); ok {
			for _, b := range branches(e) {
				if s := tailOf(b); s != nil {
					tails[s] = true
				}
			}
		}
		return true
	})
	return tails
}
