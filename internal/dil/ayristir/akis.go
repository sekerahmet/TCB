package ayristir

import (
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/sozcuk"
)

func (p *Parser) parseForStatement() agac.Statement {
	tok := p.curToken() // döngü

	// Sonsuz döngü: döngü { ... }
	if p.peekTokenIs(sozcuk.TOKEN_LBRACE) {
		p.nextToken()
		body := p.donguGovdesi()
		return &agac.WhileStatement{
			Token:     tok,
			Condition: &agac.BooleanLiteral{Token: tok, Value: true},
			Body:      body,
		}
	}

	// döngü i içinde liste { ... } veya döngü i, x içinde liste { ... }
	if p.peekTokenIs(sozcuk.TOKEN_IDENT) && (p.peekTokenN(2).Type == sozcuk.TOKEN_ICINDE || p.peekTokenN(2).Type == sozcuk.TOKEN_COMMA) {
		p.nextToken()
		firstVar := p.curToken().Literal
		keyVar := ""
		valVar := firstVar

		if p.peekTokenIs(sozcuk.TOKEN_COMMA) {
			p.nextToken() // ','
			if !p.expectPeek(sozcuk.TOKEN_IDENT) {
				return nil
			}
			keyVar = firstVar
			valVar = p.curToken().Literal
		}

		if !p.expectPeek(sozcuk.TOKEN_ICINDE) {
			return nil
		}
		p.nextToken() // iterable başına gel
		iterable := p.parseHeaderExpression()

		if !p.expectPeek(sozcuk.TOKEN_LBRACE) {
			return nil
		}
		body := p.donguGovdesi()

		return &agac.ForInStatement{
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
	if !p.expectPeek(sozcuk.TOKEN_LBRACE) {
		return nil
	}
	body := p.donguGovdesi()
	return &agac.WhileStatement{
		Token:     tok,
		Condition: cond,
		Body:      body,
	}
}
func (p *Parser) parseWhileStatement() *agac.WhileStatement {
	tok := p.curToken() // iken
	p.nextToken()
	cond := p.parseHeaderExpression()
	if !p.expectPeek(sozcuk.TOKEN_LBRACE) {
		return nil
	}
	body := p.donguGovdesi()
	return &agac.WhileStatement{
		Token:     tok,
		Condition: cond,
		Body:      body,
	}
}
func (p *Parser) parseTryCatchStatement() *agac.TryCatchStatement {
	tok := p.curToken() // dene
	if !p.expectPeek(sozcuk.TOKEN_LBRACE) {
		return nil
	}
	tryBody := p.parseBlockStatement()

	if !p.expectPeek(sozcuk.TOKEN_YAKALA) {
		return nil
	}

	errVar := hataAdi
	if p.peekTokenIs(sozcuk.TOKEN_IDENT) {
		p.nextToken()
		errVar = p.curToken().Literal
	} else if p.peekTokenIs(sozcuk.TOKEN_LPAREN) {
		p.nextToken() // '('
		if p.expectPeek(sozcuk.TOKEN_IDENT) {
			errVar = p.curToken().Literal
		}
		p.expectPeek(sozcuk.TOKEN_RPAREN)
	}

	if !p.expectPeek(sozcuk.TOKEN_LBRACE) {
		return nil
	}
	catchBody := p.parseBlockStatement()

	return &agac.TryCatchStatement{
		Token:     tok,
		TryBody:   tryBody,
		ErrorVar:  errVar,
		CatchBody: catchBody,
	}
}
func (p *Parser) parseThrowStatement() *agac.ThrowStatement {
	tok := p.curToken()
	p.nextToken()
	val := p.parseExpression(LOWEST)
	if p.peekTokenIs(sozcuk.TOKEN_SEMICOLON) {
		p.nextToken()
	}
	return &agac.ThrowStatement{Token: tok, Value: val}
}
func (p *Parser) parseAsyncStatement() *agac.AsyncStatement {
	tok := p.curToken()
	if p.peekTokenIs(sozcuk.TOKEN_LBRACE) {
		p.nextToken()
		var body *agac.BlockStatement
		p.sinirIcinde(false, false, func() { body = p.parseBlockStatement() })
		return &agac.AsyncStatement{Token: tok, Body: body}
	}
	p.nextToken()
	call := p.parseExpression(LOWEST)
	if p.peekTokenIs(sozcuk.TOKEN_SEMICOLON) {
		p.nextToken()
	}
	return &agac.AsyncStatement{Token: tok, Call: call}
}
