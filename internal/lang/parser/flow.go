package parser

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

func (p *Parser) parseForStatement() ast.Statement {
	tok := p.curToken() // döngü

	// Sonsuz döngü: döngü { ... }
	if p.peekTokenIs(lexer.TokLBrace) {
		p.nextToken()
		body := p.loopBody()
		return &ast.WhileStatement{
			Token:     tok,
			Condition: &ast.BooleanLiteral{Token: tok, Value: true},
			Body:      body,
		}
	}

	// döngü i içinde liste { ... } veya döngü i, x içinde liste { ... }
	if p.peekTokenIs(lexer.TokIdent) && (p.peekTokenN(2).Type == lexer.TokIn || p.peekTokenN(2).Type == lexer.TokComma) {
		p.nextToken()
		firstVar := p.curToken().Literal
		keyVar := ""
		valVar := firstVar

		if p.peekTokenIs(lexer.TokComma) {
			p.nextToken() // ','
			if !p.expectPeek(lexer.TokIdent) {
				return nil
			}
			keyVar = firstVar
			valVar = p.curToken().Literal
		}

		if !p.expectPeek(lexer.TokIn) {
			return nil
		}
		p.nextToken() // iterable başına gel
		iterable := p.parseHeaderExpression()

		if !p.expectPeek(lexer.TokLBrace) {
			return nil
		}
		body := p.loopBody()

		return &ast.ForInStatement{
			Token:    tok,
			KeyVar:   keyVar,
			ValueVar: valVar,
			Iterable: iterable,
			Body:     body,
		}
	}

	// Koşullu döngü: döngü x < 10 { ... }
	p.nextToken()
	cond := p.parseHeaderExpression()
	if !p.expectPeek(lexer.TokLBrace) {
		return nil
	}
	body := p.loopBody()
	return &ast.WhileStatement{
		Token:     tok,
		Condition: cond,
		Body:      body,
	}
}
func (p *Parser) parseWhileStatement() *ast.WhileStatement {
	tok := p.curToken() // iken
	p.nextToken()
	cond := p.parseHeaderExpression()
	if !p.expectPeek(lexer.TokLBrace) {
		return nil
	}
	body := p.loopBody()
	return &ast.WhileStatement{
		Token:     tok,
		Condition: cond,
		Body:      body,
	}
}
func (p *Parser) parseTryCatchStatement() *ast.TryCatchStatement {
	tok := p.curToken() // dene
	if !p.expectPeek(lexer.TokLBrace) {
		return nil
	}
	tryBody := p.parseBlockStatement()

	if !p.expectPeek(lexer.TokCatch) {
		return nil
	}

	errVar := errName
	implicit := !p.peekTokenIs(lexer.TokIdent) && !p.peekTokenIs(lexer.TokLParen)
	if p.peekTokenIs(lexer.TokIdent) {
		p.nextToken()
		errVar = p.curToken().Literal
	} else if p.peekTokenIs(lexer.TokLParen) {
		p.nextToken() // '('
		if p.expectPeek(lexer.TokIdent) {
			errVar = p.curToken().Literal
		}
		p.expectPeek(lexer.TokRParen)
	}

	if !p.expectPeek(lexer.TokLBrace) {
		return nil
	}
	catchBody := p.parseBlockStatement()

	return &ast.TryCatchStatement{
		Token:            tok,
		TryBody:          tryBody,
		ErrorVar:         errVar,
		ImplicitErrorVar: implicit,
		CatchBody:        catchBody,
	}
}
func (p *Parser) parseThrowStatement() *ast.ThrowStatement {
	tok := p.curToken()
	p.nextToken()
	val := p.parseExpression(precLowest)
	if p.peekTokenIs(lexer.TokSemicolon) {
		p.nextToken()
	}
	return &ast.ThrowStatement{Token: tok, Value: val}
}
func (p *Parser) parseAsyncStatement() *ast.AsyncStatement {
	tok := p.curToken()
	if p.peekTokenIs(lexer.TokLBrace) {
		p.nextToken()
		var body *ast.BlockStatement
		p.withinLimit(false, false, func() { body = p.parseBlockStatement() })
		return &ast.AsyncStatement{Token: tok, Body: body}
	}
	p.nextToken()
	call := p.parseExpression(precLowest)
	if p.peekTokenIs(lexer.TokSemicolon) {
		p.nextToken()
	}
	return &ast.AsyncStatement{Token: tok, Call: call}
}
