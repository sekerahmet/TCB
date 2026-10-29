package parser

import (
	"strconv"

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
		saved := p.comprehensionSource
		p.comprehensionSource = true // ardındaki TR_if üretecin koşuludur, Python'un koşullu ifadesi değil
		iterable := p.parseExpression(precLowest)
		p.comprehensionSource = saved

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
		// Anahtar bir ifadedir (3.7): {k: 1}'de k değişkendir, metin anahtarı {"k": 1} yazılır.
		key := p.parseExpression(precLowest)

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
func (p *Parser) parseIndexExpression(left ast.Expression, leftTok lexer.Token) ast.Expression {
	defer p.allowBlocks()()
	defer p.enterBrackets()()
	exp := &ast.IndexExpression{Token: p.curToken(), Left: left, LeftToken: leftTok}
	p.nextToken()
	idxStart := p.curToken()
	exp.Index = p.parseExpression(precLowest)
	exp.IndexToken = p.spanToken(idxStart, p.curToken())
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
func (p *Parser) parseMemberExpression(left ast.Expression, leftTok lexer.Token) ast.Expression {
	tok := p.curToken()
	safe := tok.Type == lexer.TokSafeDot
	if !safe && p.peekTokenIs(lexer.TokInt) {
		// t.0: demetin öğesi (3.6a); sözcük çözümleyici noktadan sonraki rakamları indeks okur
		p.nextToken()
		idx := p.curToken()
		n, err := strconv.Atoi(idx.Literal)
		if err != nil {
			p.syntaxError(idx, messages.H0036, idx.Literal)
			return nil
		}
		return &ast.ElementExpression{Token: tok, Object: left, ObjectToken: leftTok, Index: n, IndexToken: idx}
	}
	// Üye adı bir ad ya da demetin indeksidir: başka bir sözcük (x.$, x.eğer) sessizce üye adı sayılmaz.
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
