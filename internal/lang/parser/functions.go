package parser

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

func (p *Parser) parseGroupedOrLambdaExpression(desen bool) ast.Expression {
	defer p.allowBlocks()()
	// Önce bunun '(a, b) => ...' bir ok fonksiyonu (lambda) olup olmadığını kontrol et; eşle
	// deseninde '(a + 1) => ...' parantezli bir desendir.
	if !desen && p.isLambdaAhead() {
		return p.parseLambdaExpression()
	}

	p.nextToken()
	exp := p.parseExpression(LOWEST)
	if !p.expectPeek(lexer.TOKEN_RPAREN) {
		return nil
	}
	return exp
}
func (p *Parser) isLambdaAhead() bool {
	// mevcut token '('
	idx := p.pos + 1
	depth := 1
	for idx < len(p.tokens) && depth > 0 {
		if p.tokens[idx].Type == lexer.TOKEN_LPAREN {
			depth++
		} else if p.tokens[idx].Type == lexer.TOKEN_RPAREN {
			depth--
		}
		idx++
	}
	if idx < len(p.tokens) && p.tokens[idx].Type == lexer.TOKEN_ARROW {
		return true
	}
	return false
}
func (p *Parser) parseSingleParamLambda() ast.Expression {
	tok := p.curToken()
	param := ast.Param{Name: tok.Literal}
	p.nextToken() // '=>'
	arrowTok := p.curToken()

	var body *ast.BlockStatement
	if p.peekTokenIs(lexer.TOKEN_LBRACE) {
		p.nextToken()
		body = p.fonksiyonGovdesi()
	} else {
		p.nextToken()
		expr := p.fonksiyonIfadesi()
		body = &ast.BlockStatement{
			Token: arrowTok,
			Statements: []ast.Statement{
				&ast.ReturnStatement{Token: arrowTok, ReturnValue: expr},
			},
		}
	}

	return &ast.FunctionLiteral{
		Token:      tok,
		Parameters: []ast.Param{param},
		Body:       body,
	}
}
func (p *Parser) parseLambdaExpression() ast.Expression {
	tok := p.curToken() // '('
	params := p.parseFunctionParameters()
	if !p.expectPeek(lexer.TOKEN_ARROW) {
		return nil
	}
	arrowTok := p.curToken()

	var body *ast.BlockStatement
	if p.peekTokenIs(lexer.TOKEN_LBRACE) {
		p.nextToken()
		body = p.fonksiyonGovdesi()
	} else {
		p.nextToken()
		expr := p.fonksiyonIfadesi()
		body = &ast.BlockStatement{
			Token: arrowTok,
			Statements: []ast.Statement{
				&ast.ReturnStatement{Token: arrowTok, ReturnValue: expr},
			},
		}
	}

	return &ast.FunctionLiteral{
		Token:      tok,
		Parameters: params,
		Body:       body,
	}
}
func (p *Parser) parseFunctionLiteral() ast.Expression {
	lit := &ast.FunctionLiteral{Token: p.curToken()}

	// İsimli fonksiyon: fonksiyon topla(a, b) { ... }
	if p.peekTokenIs(lexer.TOKEN_IDENT) {
		p.nextToken()
		lit.Name = p.curToken().Literal
	}

	if !p.expectPeek(lexer.TOKEN_LPAREN) {
		return nil
	}

	lit.Parameters = p.parseFunctionParameters()

	// İsteğe bağlı dönüş tipi: fonksiyon topla(a: Tam, b: Tam): Tam { ... } veya -> Tam { ... }
	if p.peekTokenIs(lexer.TOKEN_COLON) || p.peekTokenIs(lexer.TOKEN_ARROW) {
		p.nextToken()
		if p.expectPeek(lexer.TOKEN_IDENT) {
			lit.ReturnType = p.curToken().Literal
			p.rol(p.curToken(), RoleType)
		}
	} else if p.peekTokenIs(lexer.TOKEN_MINUS) {
		p.nextToken() // '-'
		if p.peekTokenIs(lexer.TOKEN_GT) {
			p.nextToken() // '>'
			if p.expectPeek(lexer.TOKEN_IDENT) {
				lit.ReturnType = p.curToken().Literal
				p.rol(p.curToken(), RoleType)
			}
		}
	}

	if !p.expectPeek(lexer.TOKEN_LBRACE) {
		return nil
	}

	lit.Body = p.fonksiyonGovdesi()
	return lit
}
func (p *Parser) parseFunctionParameters() []ast.Param {
	params := []ast.Param{}

	if p.peekTokenIs(lexer.TOKEN_RPAREN) {
		p.nextToken()
		return params
	}

	p.nextToken()

	for {
		param := ast.Param{Name: p.curToken().Literal}
		if p.peekTokenIs(lexer.TOKEN_COLON) {
			p.nextToken() // ':'
			if p.expectPeek(lexer.TOKEN_IDENT) {
				param.TypeName = p.curToken().Literal
				p.rol(p.curToken(), RoleType)
			}
		}
		if p.peekTokenIs(lexer.TOKEN_ASSIGN) {
			p.nextToken() // '='
			p.nextToken()
			param.DefaultValue = p.parseExpression(LOWEST)
		}
		params = append(params, param)

		if !p.peekTokenIs(lexer.TOKEN_COMMA) {
			break
		}
		p.nextToken() // ','
		p.nextToken()
	}

	if !p.expectPeek(lexer.TOKEN_RPAREN) {
		return nil
	}

	return params
}
func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {
	exp := &ast.CallExpression{Token: p.curToken(), Function: function}
	exp.Arguments, exp.NamedArgs = p.parseCallArguments()
	return exp
}
func (p *Parser) parseCallArguments() ([]ast.Expression, []ast.NamedArg) {
	defer p.allowBlocks()()
	args := []ast.Expression{}
	named := []ast.NamedArg{}

	if p.peekTokenIs(lexer.TOKEN_RPAREN) {
		p.nextToken()
		return args, named
	}

	p.nextToken()
	for {
		if p.curTokenIs(lexer.TOKEN_IDENT) && p.peekTokenIs(lexer.TOKEN_COLON) {
			adTok := p.curToken()
			p.nextToken() // ':'
			p.nextToken()
			val := p.parseExpression(LOWEST)
			named = append(named, ast.NamedArg{Token: adTok, Name: adTok.Literal, Value: val})
		} else {
			args = append(args, p.parseExpression(LOWEST))
		}

		if !p.peekTokenIs(lexer.TOKEN_COMMA) {
			break
		}
		p.nextToken() // ','
		if p.peekTokenIs(lexer.TOKEN_RPAREN) {
			break
		}
		p.nextToken()
	}

	if !p.expectPeek(lexer.TOKEN_RPAREN) {
		return nil, nil
	}

	return args, named
}
