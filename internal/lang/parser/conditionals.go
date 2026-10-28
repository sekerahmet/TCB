package parser

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func (p *Parser) parseIfExpression() ast.Expression {
	expr := &ast.IfExpression{Token: p.curToken()}

	p.nextToken()
	expr.Condition = p.parseHeaderExpression()

	if !p.expectPeek(lexer.TokLBrace) {
		return nil
	}
	expr.Consequence = p.parseBlockStatement()

	for p.peekTokenIs(lexer.TokElse) {
		p.nextToken() // 'değilse'
		if p.peekTokenIs(lexer.TokIf) {
			p.nextToken() // 'eğer'
			p.nextToken()
			cond := p.parseHeaderExpression() // 6.7: değilse eğer da bir başlıktır
			if !p.expectPeek(lexer.TokLBrace) {
				return nil
			}
			block := p.parseBlockStatement()
			expr.ElseIfs = append(expr.ElseIfs, ast.ElseIfBranch{
				Condition:   cond,
				Consequence: block,
			})
		} else {
			if !p.expectPeek(lexer.TokLBrace) {
				return nil
			}
			expr.Alternative = p.parseBlockStatement()
			break
		}
	}
	p.ifWithoutElse = p.ifWithoutElse || expr.Alternative == nil

	return expr
}

// valueIfs değer yerinde TR_else'siz yazılmış TR_if'i sözdizimi hatası yapar (5.5): değer veren TR_if her
// durumda bir değer verir. Değer yeri bir ifadenin parçasıdır (atamanın sağı, argüman, TR_return'ün
// ardı, işlenen, liste ve sözlük öğesi, yerleştirme, indeks) ya da değeri kullanılan bir bloğun
// (değer yerindeki TR_if dalı ya da TR_match kolu) son ifadesi. Deyim olarak yazılan TR_if, fonksiyonun
// ve sondaki bloğun gövdesi değer yeri değildir (sondaki bloğun son ifadesi ayrıştırmada bilinemez).
func (p *Parser) valueIfs(prog *ast.Program) {
	if !p.ifWithoutElse || p.interpolated {
		return
	}
	p.valueBlock(prog.Statements, false)
}

// valueBlock bir bloğun deyimlerini gezer; lastIsValue, son deyimin değerinin kullanıldığıdır.
func (p *Parser) valueBlock(stmts []ast.Statement, lastIsValue bool) {
	for i, s := range stmts {
		if es, ok := s.(*ast.ExpressionStatement); ok {
			p.valueExpr(es.Expression, lastIsValue && i == len(stmts)-1)
			continue
		}
		p.valueChildren(s)
	}
}

// valueExpr bir ifadeyi gezer; value, ifadenin değerinin kullanıldığıdır.
func (p *Parser) valueExpr(e ast.Expression, value bool) {
	switch x := e.(type) {
	case *ast.IfExpression:
		if value && x.Alternative == nil {
			p.syntaxError(x.Token, messages.H0254, p.word(codes.KwIf), p.word(codes.KwElse))
			p.errorMode = false // her TR_if ayrı bir yerdir
		}
		p.valueExpr(x.Condition, true)
		p.valueBlockOf(x.Consequence, value)
		for _, ei := range x.ElseIfs {
			p.valueExpr(ei.Condition, true)
			p.valueBlockOf(ei.Consequence, value)
		}
		p.valueBlockOf(x.Alternative, value)
	case *ast.MatchExpression:
		p.valueExpr(x.Subject, true)
		for _, mc := range x.Cases {
			for _, pat := range mc.Patterns {
				p.valueExpr(pat, true)
			}
			p.valueBlockOf(mc.Body, value)
			if mc.ValueExpr != nil {
				p.valueExpr(mc.ValueExpr, value)
			}
		}
	case nil:
	default:
		p.valueChildren(e)
	}
}

func (p *Parser) valueBlockOf(b *ast.BlockStatement, lastIsValue bool) {
	if b != nil {
		p.valueBlock(b.Statements, lastIsValue)
	}
}

// valueChildren düğümün çocuklarını gezer: her ifade değer yerindedir, her blok (döngü, fonksiyon,
// sondaki blok, test gövdesi) değeri kullanılmayan bir bloktur.
func (p *Parser) valueChildren(n any) {
	ast.Walk(n, func(c any) bool {
		if c == n {
			return true
		}
		switch x := c.(type) {
		case *ast.BlockStatement:
			p.valueBlock(x.Statements, false)
		case ast.Statement:
			p.valueBlock([]ast.Statement{x}, false)
		case ast.Expression:
			p.valueExpr(x, true)
		default:
			return true
		}
		return false
	})
}

func (p *Parser) parseMatchExpression() ast.Expression {
	tok := p.curToken() // eşle
	p.nextToken()
	subject := p.parseHeaderExpression()

	if !p.expectPeek(lexer.TokLBrace) {
		return nil
	}
	matchOpen := p.curToken()
	defer func() { p.requireClosingBrace(matchOpen) }()
	p.nextToken()

	var cases []ast.MatchCase
	for !p.curTokenIs(lexer.TokRBrace) && !p.curTokenIs(lexer.TokEOF) {
		if p.curTokenIs(lexer.TokComma) || p.curTokenIs(lexer.TokSemicolon) {
			p.nextToken()
			continue
		}

		mc := ast.MatchCase{}
		if p.curTokenIs(lexer.TokIdent) && p.curToken().Literal == "_" {
			mc.IsDefault = true
		} else {
			pat := p.parseMatchPattern()
			mc.Patterns = append(mc.Patterns, pat)
			for p.peekTokenIs(lexer.TokComma) {
				p.nextToken() // ','
				p.nextToken()
				mc.Patterns = append(mc.Patterns, p.parseMatchPattern())
			}
		}

		if !p.expectPeek(lexer.TokArrow) {
			// Kalan durumlar da hatalı okunup art arda hata vermesin diye eşle bloğu atlanır
			p.skipToBlockEnd()
			return nil
		}

		if p.peekTokenIs(lexer.TokLBrace) {
			p.nextToken()
			mc.Body = p.parseBlockStatement()
		} else if p.peekTokenIs(lexer.TokReturn) {
			p.nextToken()
			retStmt := p.parseReturnStatement()
			mc.Body = &ast.BlockStatement{
				Token:      retStmt.Token,
				Statements: []ast.Statement{retStmt},
			}
		} else {
			p.nextToken()
			mc.ValueExpr = p.parseExpression(precLowest)
		}

		cases = append(cases, mc)
		p.nextToken()
	}

	return &ast.MatchExpression{
		Token:   tok,
		Subject: subject,
		Cases:   cases,
	}
}
