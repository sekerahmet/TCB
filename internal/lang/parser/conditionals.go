package parser

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

func (p *Parser) parseIfExpression() ast.Expression {
	expr := &ast.IfExpression{Token: p.curToken()}

	p.nextToken()
	expr.Condition = p.parseHeaderExpression()

	if !p.expectPeek(lexer.TOKEN_LBRACE) {
		return nil
	}
	expr.Consequence = p.parseBlockStatement()

	for p.peekTokenIs(lexer.TOKEN_ELSE) {
		p.nextToken() // 'değilse'
		if p.peekTokenIs(lexer.TOKEN_IF) {
			p.nextToken() // 'eğer'
			p.nextToken()
			cond := p.parseExpression(LOWEST)
			if !p.expectPeek(lexer.TOKEN_LBRACE) {
				return nil
			}
			block := p.parseBlockStatement()
			expr.ElseIfs = append(expr.ElseIfs, ast.ElseIfBranch{
				Condition:   cond,
				Consequence: block,
			})
		} else {
			if !p.expectPeek(lexer.TOKEN_LBRACE) {
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

	if !p.expectPeek(lexer.TOKEN_LBRACE) {
		return nil
	}
	matchOpen := p.curToken()
	defer func() { p.requireClosingBrace(matchOpen) }()
	p.nextToken()

	var cases []ast.MatchCase
	for !p.curTokenIs(lexer.TOKEN_RBRACE) && !p.curTokenIs(lexer.TOKEN_EOF) {
		if p.curTokenIs(lexer.TOKEN_COMMA) || p.curTokenIs(lexer.TOKEN_SEMICOLON) {
			p.nextToken()
			continue
		}

		mc := ast.MatchCase{}
		if p.curTokenIs(lexer.TOKEN_DEFAULT) || (p.curTokenIs(lexer.TOKEN_IDENT) && p.curToken().Literal == "_") {
			mc.IsDefault = true
		} else {
			pat := p.parseMatchPattern()
			mc.Patterns = append(mc.Patterns, pat)
			for p.peekTokenIs(lexer.TOKEN_COMMA) {
				p.nextToken() // ','
				p.nextToken()
				mc.Patterns = append(mc.Patterns, p.parseMatchPattern())
			}
		}

		if !p.expectPeek(lexer.TOKEN_ARROW) {
			// Kalan durumlar da hatalı okunup art arda hata vermesin diye eşle bloğu atlanır
			p.blokSonunaAtla()
			return nil
		}

		if p.peekTokenIs(lexer.TOKEN_LBRACE) {
			p.nextToken()
			mc.Body = p.parseBlockStatement()
		} else if p.peekTokenIs(lexer.TOKEN_RETURN) {
			p.nextToken()
			retStmt := p.parseReturnStatement()
			mc.Body = &ast.BlockStatement{
				Token:      retStmt.Token,
				Statements: []ast.Statement{retStmt},
			}
		} else {
			p.nextToken()
			mc.ValueExpr = p.parseExpression(LOWEST)
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
