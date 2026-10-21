package parser

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
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

	return expr
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
		if p.curTokenIs(lexer.TokDefault) || (p.curTokenIs(lexer.TokIdent) && p.curToken().Literal == "_") {
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
