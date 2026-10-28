package parser

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

func (p *Parser) parseGroupedOrLambdaExpression(pattern bool) ast.Expression {
	defer p.allowBlocks()()
	defer p.enterBrackets()()
	// Önce bunun '(a, b) => ...' bir ok fonksiyonu (lambda) olup olmadığını kontrol et; eşle
	// deseninde '(a + 1) => ...' parantezli bir desendir.
	if !pattern && p.isLambdaAhead() {
		return p.parseLambdaExpression()
	}

	p.nextToken()
	exp := p.parseExpression(precLowest)
	if !p.expectPeek(lexer.TokRParen) {
		return nil
	}
	return exp
}
func (p *Parser) isLambdaAhead() bool {
	// mevcut token '('
	idx := p.pos + 1
	depth := 1
	for idx < len(p.tokens) && depth > 0 {
		if p.tokens[idx].Type == lexer.TokLParen {
			depth++
		} else if p.tokens[idx].Type == lexer.TokRParen {
			depth--
		}
		idx++
	}
	if idx < len(p.tokens) && p.tokens[idx].Type == lexer.TokArrow {
		return true
	}
	return false
}
func (p *Parser) parseSingleParamLambda() ast.Expression {
	tok := p.curToken()
	param := ast.Param{Name: tok.Literal, NameToken: tok}
	p.nextToken() // '=>'
	return p.finishLambda(tok, []ast.Param{param})
}
func (p *Parser) parseLambdaExpression() ast.Expression {
	tok := p.curToken() // '('
	params := p.parseFunctionParameters()
	if !p.expectPeek(lexer.TokArrow) {
		return nil
	}
	return p.finishLambda(tok, params)
}

// finishLambda '=>' belirtecinde duran imleçten sonrasını (küme ayraçlı gövde ya da tek ifade)
// okuyup lambdayı kurar; tok lambdanın ilk belirtecidir.
func (p *Parser) finishLambda(tok lexer.Token, params []ast.Param) ast.Expression {
	arrowTok := p.curToken()

	var body *ast.BlockStatement
	if p.peekTokenIs(lexer.TokLBrace) {
		p.nextToken()
		body = p.functionBody()
	} else {
		p.nextToken()
		expr := p.functionExpr()
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
	if p.peekTokenIs(lexer.TokIdent) {
		p.nextToken()
		lit.Name, lit.NameToken = p.curToken().Literal, p.curToken()
	}

	if !p.expectPeek(lexer.TokLParen) {
		return nil
	}

	lit.Parameters = p.parseFunctionParameters()

	// İsteğe bağlı dönüş tipi: fonksiyon topla(a: Tam, b: Tam): Tam { ... } veya -> Tam { ... }
	if p.peekTokenIs(lexer.TokColon) || p.peekTokenIs(lexer.TokArrow) {
		p.nextToken()
		if p.expectPeek(lexer.TokIdent) {
			lit.ReturnType, lit.ReturnToken = p.curToken().Literal, p.curToken()
			p.role(p.curToken(), RoleType)
		}
	} else if p.peekTokenIs(lexer.TokMinus) {
		p.nextToken() // '-'
		if p.peekTokenIs(lexer.TokGt) {
			p.nextToken() // '>'
			if p.expectPeek(lexer.TokIdent) {
				lit.ReturnType, lit.ReturnToken = p.curToken().Literal, p.curToken()
				p.role(p.curToken(), RoleType)
			}
		}
	}

	if !p.expectPeek(lexer.TokLBrace) {
		return nil
	}

	lit.Body = p.functionBody()
	return lit
}
func (p *Parser) parseFunctionParameters() []ast.Param {
	params := []ast.Param{}

	if p.peekTokenIs(lexer.TokRParen) {
		p.nextToken()
		return params
	}

	p.nextToken()

	for {
		param := ast.Param{Name: p.curToken().Literal, NameToken: p.curToken()}
		param.TypeName, param.TypeToken, param.DefaultValue = p.parseTypeAndDefault()
		params = append(params, param)

		if !p.peekTokenIs(lexer.TokComma) {
			break
		}
		p.nextToken() // ','
		p.nextToken()
	}

	if !p.expectPeek(lexer.TokRParen) {
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
	defer p.enterBrackets()()
	args := []ast.Expression{}
	named := []ast.NamedArg{}

	if p.peekTokenIs(lexer.TokRParen) {
		p.nextToken()
		return args, named
	}

	p.nextToken()
	for {
		if p.curTokenIs(lexer.TokIdent) && p.peekTokenIs(lexer.TokColon) {
			nameTok := p.curToken()
			p.nextToken() // ':'
			p.nextToken()
			val := p.parseExpression(precLowest)
			named = append(named, ast.NamedArg{Token: nameTok, Name: nameTok.Literal, Value: val})
		} else {
			args = append(args, p.parseExpression(precLowest))
		}

		if !p.peekTokenIs(lexer.TokComma) {
			break
		}
		p.nextToken() // ','
		if p.peekTokenIs(lexer.TokRParen) {
			break
		}
		p.nextToken()
	}

	if !p.expectPeek(lexer.TokRParen) {
		return nil, nil
	}

	return args, named
}

// parseTypeAndDefault isteğe bağlı ': Tip' ve '= varsayılan' bölümlerini okur (parametre ve yapı
// alanı için); olmayan bölüm boş döner.
func (p *Parser) parseTypeAndDefault() (typeName string, typeTok lexer.Token, def ast.Expression) {
	if p.peekTokenIs(lexer.TokColon) {
		p.nextToken() // ':'
		if p.expectPeek(lexer.TokIdent) {
			typeName, typeTok = p.curToken().Literal, p.curToken()
			p.role(p.curToken(), RoleType)
		}
	}
	if p.peekTokenIs(lexer.TokAssign) {
		p.nextToken() // '='
		p.nextToken()
		def = p.parseExpression(precLowest)
	}
	return typeName, typeTok, def
}
