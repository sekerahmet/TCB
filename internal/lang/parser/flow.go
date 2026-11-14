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

	// döngü x içinde l { ... }, döngü i, x içinde l { ... } ya da desenle döngü (a, b) içinde l,
	// döngü i, (a, b) içinde l (5.7)
	if p.peekTokenIs(lexer.TokIdent) && (p.peekTokenN(2).Type == lexer.TokIn || p.peekTokenN(2).Type == lexer.TokComma) ||
		p.patternAhead(1) > 0 {
		if s := p.parseForIn(tok); s != nil {
			return s
		}
		return nil
	}

	// Koşullu döngü: döngü x < 10 { ... }; imleç hâlâ 'döngü'dedir, 'iken' ile aynı gövde.
	// Boş *WhileStatement arayüze sarılıp nil olmaktan çıkmasın diye hata ayrıca döndürülür.
	if loop := p.parseWhileStatement(); loop != nil {
		return loop
	}
	return nil
}

// parseForIn döngünün 'döngü <adlar> içinde <kaynak> { ... }' biçimini okur; imleç 'döngü'dedir. Adlar
// tek ad (x), indeksli iki ad (i, x) ya da öğenin yerinde bir demet desenidir ((a, b), i, (a, b); 5.7).
// Parantezli tek ad (a) desen değildir: H0040, döngü yine o adla okunur. Ayrıştırıcı öğenin tipini
// bilmez; ileti iki yolu da önerir: demet olmayan öğede parantezsiz ad, demette (a, _) deseni.
func (p *Parser) parseForIn(tok lexer.Token) *ast.ForInStatement {
	s := &ast.ForInStatement{Token: tok}
	if p.peekTokenIs(lexer.TokIdent) {
		p.nextToken()
		s.ValueVar, s.ValueToken = p.curToken().Literal, p.curToken()
		if p.peekTokenIs(lexer.TokComma) {
			p.nextToken() // ','
			s.KeyVar, s.KeyToken = s.ValueVar, s.ValueToken
			s.ValueVar, s.ValueToken = "", lexer.Token{}
			if p.patternAhead(1) == 0 {
				if !p.expectPeek(lexer.TokIdent) {
					return nil
				}
				s.ValueVar, s.ValueToken = p.curToken().Literal, p.curToken()
			}
		}
	}
	paren := p.peekToken()
	if p.patternAhead(1) > 0 {
		s.ValueVar, s.ValueToken = "", lexer.Token{}
		s.Pattern = p.parseLoopPattern()
	}
	if !p.expectPeek(lexer.TokIn) {
		return nil
	}
	p.nextToken() // iterable başına gel
	start := p.curToken()
	s.Iterable = p.parseHeaderExpression()
	s.IterableToken = p.spanToken(start, p.curToken())
	if len(s.Pattern) == 1 {
		head, pattern := s.Pattern[0].Name, ast.PatternText([]string{s.Pattern[0].Name, "_"})
		if s.KeyVar != "" {
			head, pattern = s.KeyVar+", "+head, s.KeyVar+", "+pattern
		}
		p.syntaxError(paren, messages.H0040, head, pattern, s.IterableToken.Literal)
		s.ValueVar, s.ValueToken, s.Pattern = s.Pattern[0].Name, s.Pattern[0].Token, nil
	}
	if !p.expectPeek(lexer.TokLBrace) {
		return nil
	}
	s.Body = p.loopBody()
	return s
}

// patternAhead imlecin off sözcük birimi ilerisinde 'içinde'yle biten bir döngü deseni
// ('(' ad (',' ad)* ')' 'içinde') varsa ad sayısını, yoksa 0 verir. Koşullu döngünün parantezli
// koşulu ('döngü (x < 3) { ... }') desen değildir.
func (p *Parser) patternAhead(off int) int {
	if p.peekTokenN(off).Type != lexer.TokLParen {
		return 0
	}
	n := 0
	for i := off + 1; ; i += 2 {
		if p.peekTokenN(i).Type != lexer.TokIdent {
			return 0
		}
		n++
		switch p.peekTokenN(i + 1).Type {
		case lexer.TokComma:
			continue
		case lexer.TokRParen:
			if p.peekTokenN(i+2).Type == lexer.TokIn {
				return n
			}
		}
		return 0
	}
}

// parseLoopPattern patternAhead'in gördüğü deseni okur; imleç desenin ')'sinde kalır.
func (p *Parser) parseLoopPattern() []ast.LoopName {
	p.nextToken() // '('
	var names []ast.LoopName
	for {
		p.nextToken() // ad
		names = append(names, ast.LoopName{Name: p.curToken().Literal, Token: p.curToken()})
		p.nextToken() // ',' ya da ')'
		if p.curTokenIs(lexer.TokRParen) {
			return names
		}
	}
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
