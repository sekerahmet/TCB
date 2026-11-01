package parser

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
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
		var keyTok lexer.Token
		valTok := p.curToken()

		if p.peekTokenIs(lexer.TokComma) {
			p.nextToken() // ','
			if !p.expectPeek(lexer.TokIdent) {
				return nil
			}
			keyVar = firstVar
			keyTok = valTok
			valVar = p.curToken().Literal
			valTok = p.curToken()
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
			KeyToken: keyTok, ValueToken: valTok,
			Iterable: iterable,
			Body:     body,
		}
	}

	// Koşullu döngü: döngü x < 10 { ... }; imleç hâlâ 'döngü'dedir, 'iken' ile aynı gövde.
	// Boş *WhileStatement arayüze sarılıp nil olmaktan çıkmasın diye hata ayrıca döndürülür.
	if loop := p.parseWhileStatement(); loop != nil {
		return loop
	}
	return nil
}
func (p *Parser) parseWhileStatement() *ast.WhileStatement {
	tok := p.curToken() // iken ya da döngü (koşullu döngü)
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

	// Başka dillerin yazımları sözdizimi hatasıdır, ileti TR'deki yazımı söyler (8.1): Python'un
	// except'i TR_catch'tir; finally yoktur, iş TR_defer ile yazılır.
	switch {
	case p.peekWord("except"):
		p.nextToken()
		p.syntaxError(p.curToken(), messages.H2915)
	case p.peekWord("finally"):
		p.skipFinally()
		return nil
	case !p.expectPeek(lexer.TokCatch):
		return nil
	}

	var errVar string
	var errTok lexer.Token
	if p.peekTokenIs(lexer.TokLBrace) {
		// Ad her zaman yazılır (8.1): adsız yakalama yoktur, ileti adı önerir.
		p.syntaxError(p.curToken(), messages.H2910)
	} else if p.peekTokenIs(lexer.TokIdent) {
		p.nextToken()
		errVar, errTok = p.curToken().Literal, p.curToken()
	} else if p.peekTokenIs(lexer.TokLParen) {
		// Ad parantezsiz yazılır (8.1): TR_catch (e) bir yazım hatasıdır, ileti TR_catch e'yi önerir.
		open := p.peekToken()
		p.nextToken() // '('
		if p.expectPeek(lexer.TokIdent) {
			errVar, errTok = p.curToken().Literal, p.curToken()
			p.syntaxError(open, messages.H2917, errVar)
		}
		p.expectPeek(lexer.TokRParen)
	}

	if !p.expectPeek(lexer.TokLBrace) {
		return nil
	}
	catchBody := p.parseBlockStatement()
	if p.peekWord("finally") && p.peekTokenN(2).Type == lexer.TokLBrace {
		p.skipFinally()
	}

	return &ast.TryCatchStatement{
		Token:      tok,
		TryBody:    tryBody,
		ErrorVar:   errVar,
		ErrorToken: errTok,
		CatchBody:  catchBody,
	}
}
func (p *Parser) parseThrowStatement() *ast.ThrowStatement {
	tok := p.curToken()
	p.nextToken()
	val := p.parseExpression(precLowest)
	p.skipSemicolon()
	return &ast.ThrowStatement{Token: tok, Value: val}
}
func (p *Parser) parseAsyncStatement() *ast.AsyncStatement {
	tok := p.curToken()
	body, call := p.parseBlockOrCall(false)
	return &ast.AsyncStatement{Token: tok, Body: body, Call: call}
}

// peekWord sıradaki belirteç w yazımlı bir ad mı (başka bir dilin anahtar kelimesi: except, finally)?
func (p *Parser) peekWord(w string) bool {
	return p.peekTokenIs(lexer.TokIdent) && p.peekToken().Literal == w
}

// skipFinally Python'un finally bloğunu hatasıyla atlar (8.1): TR'de temizlik TR_defer'le yazılır.
func (p *Parser) skipFinally() {
	p.nextToken()
	p.syntaxError(p.curToken(), messages.H2916)
	if p.peekTokenIs(lexer.TokLBrace) {
		p.nextToken()
		p.parseBlockStatement()
	}
}
