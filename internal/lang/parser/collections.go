package parser

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

func (p *Parser) parseListLiteral() ast.Expression {
	defer p.allowBlocks()()
	tok := p.curToken()
	if p.peekTokenIs(lexer.TOKEN_RBRACKET) {
		p.nextToken()
		return &ast.ListLiteral{Token: tok, Elements: []ast.Expression{}}
	}

	p.nextToken()
	firstExpr := p.parseExpression(LOWEST)

	// Liste üreteci kontrolü: [x * x döngü x içinde 1..=10 eğer x % 2 == 0]
	if p.peekTokenIs(lexer.TOKEN_FOR) {
		p.nextToken() // 'döngü'
		if !p.expectPeek(lexer.TOKEN_IDENT) {
			return nil
		}
		varName := p.curToken().Literal
		if !p.expectPeek(lexer.TOKEN_IN) {
			return nil
		}
		p.nextToken()
		iterable := p.parseExpression(LOWEST)

		var cond ast.Expression
		if p.peekTokenIs(lexer.TOKEN_IF) {
			p.nextToken() // 'eğer'
			p.nextToken()
			cond = p.parseExpression(LOWEST)
		}

		if !p.expectPeek(lexer.TOKEN_RBRACKET) {
			return nil
		}

		return &ast.ListComprehension{
			Token:     tok,
			Output:    firstExpr,
			VarName:   varName,
			Iterable:  iterable,
			Condition: cond,
		}
	}

	elements := []ast.Expression{firstExpr}
	for p.peekTokenIs(lexer.TOKEN_COMMA) {
		p.nextToken()
		if p.peekTokenIs(lexer.TOKEN_RBRACKET) {
			break
		}
		p.nextToken()
		elements = append(elements, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(lexer.TOKEN_RBRACKET) {
		return nil
	}

	return &ast.ListLiteral{Token: tok, Elements: elements}
}
func (p *Parser) parseDictLiteral() ast.Expression {
	defer p.allowBlocks()()
	dict := &ast.DictLiteral{Token: p.curToken(), Pairs: []ast.DictPair{}}

	for !p.peekTokenIs(lexer.TOKEN_RBRACE) && !p.peekTokenIs(lexer.TOKEN_EOF) {
		p.nextToken()
		var key ast.Expression
		// Sözlük anahtarı çıplak tanımlayıcı ise (örn: {ad: "Ali"}), metin olarak al
		if p.curTokenIs(lexer.TOKEN_IDENT) && p.peekTokenIs(lexer.TOKEN_COLON) {
			key = &ast.StringLiteral{Token: p.curToken(), Value: p.curToken().Literal}
			p.role(p.curToken(), RoleDictKey)
		} else {
			key = p.parseExpression(LOWEST)
		}

		if !p.expectPeek(lexer.TOKEN_COLON) {
			return nil
		}
		p.nextToken()
		value := p.parseExpression(LOWEST)
		dict.Pairs = append(dict.Pairs, ast.DictPair{Key: key, Value: value})

		if p.peekTokenIs(lexer.TOKEN_COMMA) {
			p.nextToken()
		}
	}

	if !p.expectPeek(lexer.TOKEN_RBRACE) {
		return nil
	}

	return dict
}
func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	defer p.allowBlocks()()
	exp := &ast.IndexExpression{Token: p.curToken(), Left: left}
	p.nextToken()
	exp.Index = p.parseExpression(LOWEST)
	if !p.expectPeek(lexer.TOKEN_RBRACKET) {
		return nil
	}
	return exp
}
func (p *Parser) parseMemberExpression(left ast.Expression) ast.Expression {
	tok := p.curToken()
	safe := tok.Type == lexer.TOKEN_SAFE_DOT
	// Üye adı bir ad olmalıdır: başka bir sözcük (x.1, x.$, x.eğer) sessizce üye adı sayılmaz.
	if !p.expectPeek(lexer.TOKEN_IDENT) {
		return nil
	}
	name := p.curToken()
	p.role(name, RoleMember)
	return &ast.MemberExpression{
		Token:     tok,
		Object:    left,
		Property:  name.Literal,
		NameToken: name,
		Safe:      safe,
	}
}
