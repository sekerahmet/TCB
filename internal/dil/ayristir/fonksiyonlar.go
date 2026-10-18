package ayristir

import (
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/sozcuk"
)

func (p *Parser) parseGroupedOrLambdaExpression(desen bool) agac.Expression {
	defer p.allowBlocks()()
	// Önce bunun '(a, b) => ...' bir ok fonksiyonu (lambda) olup olmadığını kontrol et; eşle
	// deseninde '(a + 1) => ...' parantezli bir desendir.
	if !desen && p.isLambdaAhead() {
		return p.parseLambdaExpression()
	}

	p.nextToken()
	exp := p.parseExpression(LOWEST)
	if !p.expectPeek(sozcuk.TOKEN_RPAREN) {
		return nil
	}
	return exp
}
func (p *Parser) isLambdaAhead() bool {
	// mevcut token '('
	idx := p.pos + 1
	depth := 1
	for idx < len(p.tokens) && depth > 0 {
		if p.tokens[idx].Type == sozcuk.TOKEN_LPAREN {
			depth++
		} else if p.tokens[idx].Type == sozcuk.TOKEN_RPAREN {
			depth--
		}
		idx++
	}
	if idx < len(p.tokens) && p.tokens[idx].Type == sozcuk.TOKEN_ARROW {
		return true
	}
	return false
}
func (p *Parser) parseSingleParamLambda() agac.Expression {
	tok := p.curToken()
	param := agac.Param{Name: tok.Literal}
	p.nextToken() // '=>'
	arrowTok := p.curToken()

	var body *agac.BlockStatement
	if p.peekTokenIs(sozcuk.TOKEN_LBRACE) {
		p.nextToken()
		body = p.fonksiyonGovdesi()
	} else {
		p.nextToken()
		expr := p.fonksiyonIfadesi()
		body = &agac.BlockStatement{
			Token: arrowTok,
			Statements: []agac.Statement{
				&agac.ReturnStatement{Token: arrowTok, ReturnValue: expr},
			},
		}
	}

	return &agac.FunctionLiteral{
		Token:      tok,
		Parameters: []agac.Param{param},
		Body:       body,
	}
}
func (p *Parser) parseLambdaExpression() agac.Expression {
	tok := p.curToken() // '('
	params := p.parseFunctionParameters()
	if !p.expectPeek(sozcuk.TOKEN_ARROW) {
		return nil
	}
	arrowTok := p.curToken()

	var body *agac.BlockStatement
	if p.peekTokenIs(sozcuk.TOKEN_LBRACE) {
		p.nextToken()
		body = p.fonksiyonGovdesi()
	} else {
		p.nextToken()
		expr := p.fonksiyonIfadesi()
		body = &agac.BlockStatement{
			Token: arrowTok,
			Statements: []agac.Statement{
				&agac.ReturnStatement{Token: arrowTok, ReturnValue: expr},
			},
		}
	}

	return &agac.FunctionLiteral{
		Token:      tok,
		Parameters: params,
		Body:       body,
	}
}
func (p *Parser) parseFunctionLiteral() agac.Expression {
	lit := &agac.FunctionLiteral{Token: p.curToken()}

	// İsimli fonksiyon: fonksiyon topla(a, b) { ... }
	if p.peekTokenIs(sozcuk.TOKEN_IDENT) {
		p.nextToken()
		lit.Name = p.curToken().Literal
	}

	if !p.expectPeek(sozcuk.TOKEN_LPAREN) {
		return nil
	}

	lit.Parameters = p.parseFunctionParameters()

	// İsteğe bağlı dönüş tipi: fonksiyon topla(a: Tam, b: Tam): Tam { ... } veya -> Tam { ... }
	if p.peekTokenIs(sozcuk.TOKEN_COLON) || p.peekTokenIs(sozcuk.TOKEN_ARROW) {
		p.nextToken()
		if p.expectPeek(sozcuk.TOKEN_IDENT) {
			lit.ReturnType = p.curToken().Literal
			p.rol(p.curToken(), RolTip)
		}
	} else if p.peekTokenIs(sozcuk.TOKEN_MINUS) {
		p.nextToken() // '-'
		if p.peekTokenIs(sozcuk.TOKEN_GT) {
			p.nextToken() // '>'
			if p.expectPeek(sozcuk.TOKEN_IDENT) {
				lit.ReturnType = p.curToken().Literal
				p.rol(p.curToken(), RolTip)
			}
		}
	}

	if !p.expectPeek(sozcuk.TOKEN_LBRACE) {
		return nil
	}

	lit.Body = p.fonksiyonGovdesi()
	return lit
}
func (p *Parser) parseFunctionParameters() []agac.Param {
	params := []agac.Param{}

	if p.peekTokenIs(sozcuk.TOKEN_RPAREN) {
		p.nextToken()
		return params
	}

	p.nextToken()

	for {
		param := agac.Param{Name: p.curToken().Literal}
		if p.peekTokenIs(sozcuk.TOKEN_COLON) {
			p.nextToken() // ':'
			if p.expectPeek(sozcuk.TOKEN_IDENT) {
				param.TypeName = p.curToken().Literal
				p.rol(p.curToken(), RolTip)
			}
		}
		if p.peekTokenIs(sozcuk.TOKEN_ASSIGN) {
			p.nextToken() // '='
			p.nextToken()
			param.DefaultValue = p.parseExpression(LOWEST)
		}
		params = append(params, param)

		if !p.peekTokenIs(sozcuk.TOKEN_COMMA) {
			break
		}
		p.nextToken() // ','
		p.nextToken()
	}

	if !p.expectPeek(sozcuk.TOKEN_RPAREN) {
		return nil
	}

	return params
}
func (p *Parser) parseCallExpression(function agac.Expression) agac.Expression {
	exp := &agac.CallExpression{Token: p.curToken(), Function: function}
	exp.Arguments, exp.NamedArgs = p.parseCallArguments()
	return exp
}
func (p *Parser) parseCallArguments() ([]agac.Expression, []agac.NamedArg) {
	defer p.allowBlocks()()
	args := []agac.Expression{}
	named := []agac.NamedArg{}

	if p.peekTokenIs(sozcuk.TOKEN_RPAREN) {
		p.nextToken()
		return args, named
	}

	p.nextToken()
	for {
		if p.curTokenIs(sozcuk.TOKEN_IDENT) && p.peekTokenIs(sozcuk.TOKEN_COLON) {
			adTok := p.curToken()
			p.nextToken() // ':'
			p.nextToken()
			val := p.parseExpression(LOWEST)
			named = append(named, agac.NamedArg{Token: adTok, Name: adTok.Literal, Value: val})
		} else {
			args = append(args, p.parseExpression(LOWEST))
		}

		if !p.peekTokenIs(sozcuk.TOKEN_COMMA) {
			break
		}
		p.nextToken() // ','
		if p.peekTokenIs(sozcuk.TOKEN_RPAREN) {
			break
		}
		p.nextToken()
	}

	if !p.expectPeek(sozcuk.TOKEN_RPAREN) {
		return nil, nil
	}

	return args, named
}
