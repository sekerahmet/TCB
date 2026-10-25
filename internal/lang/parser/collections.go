package parser

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func (p *Parser) parseListLiteral() ast.Expression {
	defer p.allowBlocks()()
	defer p.enterBrackets()()
	tok := p.curToken()
	if p.peekTokenIs(lexer.TokRBracket) {
		p.nextToken()
		return &ast.ListLiteral{Token: tok, Elements: []ast.Expression{}}
	}

	p.nextToken()
	firstExpr := p.parseExpression(precLowest)

	// Liste üreteci kontrolü: [x * x döngü x içinde 1..=10 eğer x % 2 == 0]
	if p.peekTokenIs(lexer.TokFor) {
		p.nextToken() // 'döngü'
		if !p.expectPeek(lexer.TokIdent) {
			return nil
		}
		varName := p.curToken().Literal
		if !p.expectPeek(lexer.TokIn) {
			return nil
		}
		p.nextToken()
		iterable := p.parseExpression(precLowest)

		var cond ast.Expression
		if p.peekTokenIs(lexer.TokIf) {
			p.nextToken() // 'eğer'
			p.nextToken()
			cond = p.parseExpression(precLowest)
		}

		if !p.expectPeek(lexer.TokRBracket) {
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
	for p.peekTokenIs(lexer.TokComma) {
		p.nextToken()
		if p.peekTokenIs(lexer.TokRBracket) {
			break
		}
		p.nextToken()
		elements = append(elements, p.parseExpression(precLowest))
	}

	if !p.expectPeek(lexer.TokRBracket) {
		return nil
	}

	return &ast.ListLiteral{Token: tok, Elements: elements}
}
func (p *Parser) parseDictLiteral() ast.Expression {
	defer p.allowBlocks()()
	defer p.enterBrackets()()
	dict := &ast.DictLiteral{Token: p.curToken(), Pairs: []ast.DictPair{}}

	for !p.peekTokenIs(lexer.TokRBrace) && !p.peekTokenIs(lexer.TokEOF) {
		p.nextToken()
		var key ast.Expression
		// Sözlük anahtarı çıplak tanımlayıcı ise (örn: {ad: "Ali"}), metin olarak al
		if p.curTokenIs(lexer.TokIdent) && p.peekTokenIs(lexer.TokColon) {
			key = &ast.StringLiteral{Token: p.curToken(), Value: p.curToken().Literal}
			p.role(p.curToken(), RoleDictKey)
		} else {
			key = p.parseExpression(precLowest)
		}

		if !p.expectPeek(lexer.TokColon) {
			return nil
		}
		p.nextToken()
		value := p.parseExpression(precLowest)
		dict.Pairs = append(dict.Pairs, ast.DictPair{Key: key, Value: value})

		if p.peekTokenIs(lexer.TokComma) {
			p.nextToken()
		}
	}

	if !p.expectPeek(lexer.TokRBrace) {
		return nil
	}

	return dict
}
func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	defer p.allowBlocks()()
	defer p.enterBrackets()()
	exp := &ast.IndexExpression{Token: p.curToken(), Left: left}
	p.nextToken()
	exp.Index = p.parseExpression(precLowest)
	if p.peekTokenIs(lexer.TokColon) {
		// Python'un l[a:b]'si: dilim '..' ile yazılır (3.6); genel "']' bekleniyordu" yerine doğru biçim.
		p.syntaxError(p.peekToken(), messages.H0170)
		return nil
	}
	if !p.expectPeek(lexer.TokRBracket) {
		return nil
	}
	return exp
}
func (p *Parser) parseMemberExpression(left ast.Expression) ast.Expression {
	tok := p.curToken()
	safe := tok.Type == lexer.TokSafeDot
	// Üye adı bir ad olmalıdır: başka bir sözcük (x.1, x.$, x.eğer) sessizce üye adı sayılmaz.
	if !p.expectPeek(lexer.TokIdent) {
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
