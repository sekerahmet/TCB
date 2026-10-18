package ayristir

import (
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/sozcuk"
)

func (p *Parser) parseListLiteral() agac.Expression {
	defer p.allowBlocks()()
	tok := p.curToken()
	if p.peekTokenIs(sozcuk.TOKEN_RBRACKET) {
		p.nextToken()
		return &agac.ListLiteral{Token: tok, Elements: []agac.Expression{}}
	}

	p.nextToken()
	firstExpr := p.parseExpression(LOWEST)

	// Liste üreteci kontrolü: [x * x döngü x içinde 1..=10 eğer x % 2 == 0]
	if p.peekTokenIs(sozcuk.TOKEN_DONGU) {
		p.nextToken() // 'döngü'
		if !p.expectPeek(sozcuk.TOKEN_IDENT) {
			return nil
		}
		varName := p.curToken().Literal
		if !p.expectPeek(sozcuk.TOKEN_ICINDE) {
			return nil
		}
		p.nextToken()
		iterable := p.parseExpression(LOWEST)

		var cond agac.Expression
		if p.peekTokenIs(sozcuk.TOKEN_EGER) {
			p.nextToken() // 'eğer'
			p.nextToken()
			cond = p.parseExpression(LOWEST)
		}

		if !p.expectPeek(sozcuk.TOKEN_RBRACKET) {
			return nil
		}

		return &agac.ListComprehension{
			Token:     tok,
			Output:    firstExpr,
			VarName:   varName,
			Iterable:  iterable,
			Condition: cond,
		}
	}

	elements := []agac.Expression{firstExpr}
	for p.peekTokenIs(sozcuk.TOKEN_COMMA) {
		p.nextToken()
		if p.peekTokenIs(sozcuk.TOKEN_RBRACKET) {
			break
		}
		p.nextToken()
		elements = append(elements, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(sozcuk.TOKEN_RBRACKET) {
		return nil
	}

	return &agac.ListLiteral{Token: tok, Elements: elements}
}
func (p *Parser) parseDictLiteral() agac.Expression {
	defer p.allowBlocks()()
	dict := &agac.DictLiteral{Token: p.curToken(), Pairs: []agac.DictPair{}}

	for !p.peekTokenIs(sozcuk.TOKEN_RBRACE) && !p.peekTokenIs(sozcuk.TOKEN_EOF) {
		p.nextToken()
		var key agac.Expression
		// Sözlük anahtarı çıplak tanımlayıcı ise (örn: {ad: "Ali"}), metin olarak al
		if p.curTokenIs(sozcuk.TOKEN_IDENT) && p.peekTokenIs(sozcuk.TOKEN_COLON) {
			key = &agac.StringLiteral{Token: p.curToken(), Value: p.curToken().Literal}
		} else {
			key = p.parseExpression(LOWEST)
		}

		if !p.expectPeek(sozcuk.TOKEN_COLON) {
			return nil
		}
		p.nextToken()
		value := p.parseExpression(LOWEST)
		dict.Pairs = append(dict.Pairs, agac.DictPair{Key: key, Value: value})

		if p.peekTokenIs(sozcuk.TOKEN_COMMA) {
			p.nextToken()
		}
	}

	if !p.expectPeek(sozcuk.TOKEN_RBRACE) {
		return nil
	}

	return dict
}
func (p *Parser) parseIndexExpression(left agac.Expression) agac.Expression {
	defer p.allowBlocks()()
	exp := &agac.IndexExpression{Token: p.curToken(), Left: left}
	p.nextToken()
	exp.Index = p.parseExpression(LOWEST)
	if !p.expectPeek(sozcuk.TOKEN_RBRACKET) {
		return nil
	}
	return exp
}
func (p *Parser) parseMemberExpression(left agac.Expression) agac.Expression {
	tok := p.curToken()
	safe := tok.Type == sozcuk.TOKEN_SAFE_DOT
	p.nextToken()
	prop := p.curToken().Literal
	return &agac.MemberExpression{
		Token:    tok,
		Object:   left,
		Property: prop,
		Safe:     safe,
	}
}
