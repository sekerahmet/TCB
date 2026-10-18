package ayristir

import (
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/sozcuk"
)

func (p *Parser) parseIfExpression() agac.Expression {
	expr := &agac.IfExpression{Token: p.curToken()}

	p.nextToken()
	expr.Condition = p.parseHeaderExpression()

	if !p.expectPeek(sozcuk.TOKEN_LBRACE) {
		return nil
	}
	expr.Consequence = p.parseBlockStatement()

	for p.peekTokenIs(sozcuk.TOKEN_DEGILSE) {
		p.nextToken() // 'değilse'
		if p.peekTokenIs(sozcuk.TOKEN_EGER) {
			p.nextToken() // 'eğer'
			p.nextToken()
			cond := p.parseExpression(LOWEST)
			if !p.expectPeek(sozcuk.TOKEN_LBRACE) {
				return nil
			}
			block := p.parseBlockStatement()
			expr.ElseIfs = append(expr.ElseIfs, agac.ElseIfBranch{
				Condition:   cond,
				Consequence: block,
			})
		} else {
			if !p.expectPeek(sozcuk.TOKEN_LBRACE) {
				return nil
			}
			expr.Alternative = p.parseBlockStatement()
			break
		}
	}

	return expr
}
func (p *Parser) parseMatchExpression() agac.Expression {
	tok := p.curToken() // eşle
	p.nextToken()
	subject := p.parseHeaderExpression()

	if !p.expectPeek(sozcuk.TOKEN_LBRACE) {
		return nil
	}
	matchOpen := p.curToken()
	defer func() { p.requireClosingBrace(matchOpen) }()
	p.nextToken()

	var cases []agac.MatchCase
	for !p.curTokenIs(sozcuk.TOKEN_RBRACE) && !p.curTokenIs(sozcuk.TOKEN_EOF) {
		if p.curTokenIs(sozcuk.TOKEN_COMMA) || p.curTokenIs(sozcuk.TOKEN_SEMICOLON) {
			p.nextToken()
			continue
		}

		mc := agac.MatchCase{}
		if p.curTokenIs(sozcuk.TOKEN_VARSAYILAN) || (p.curTokenIs(sozcuk.TOKEN_IDENT) && p.curToken().Literal == "_") {
			mc.IsDefault = true
		} else {
			pat := p.parseMatchPattern()
			mc.Patterns = append(mc.Patterns, pat)
			for p.peekTokenIs(sozcuk.TOKEN_COMMA) {
				p.nextToken() // ','
				p.nextToken()
				mc.Patterns = append(mc.Patterns, p.parseMatchPattern())
			}
		}

		if !p.expectPeek(sozcuk.TOKEN_ARROW) {
			// Kalan durumlar da hatalı okunup art arda hata vermesin diye eşle bloğu atlanır
			p.blokSonunaAtla()
			return nil
		}

		if p.peekTokenIs(sozcuk.TOKEN_LBRACE) {
			p.nextToken()
			mc.Body = p.parseBlockStatement()
		} else if p.peekTokenIs(sozcuk.TOKEN_SONUC) {
			p.nextToken()
			retStmt := p.parseReturnStatement()
			mc.Body = &agac.BlockStatement{
				Token:      retStmt.Token,
				Statements: []agac.Statement{retStmt},
			}
		} else {
			p.nextToken()
			mc.ValueExpr = p.parseExpression(LOWEST)
		}

		cases = append(cases, mc)
		p.nextToken()
	}

	return &agac.MatchExpression{
		Token:   tok,
		Subject: subject,
		Cases:   cases,
	}
}
