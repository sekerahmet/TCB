package ayristir

import (
	"fmt"
	"slices"
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/sozcuk"
)

func (p *Parser) parseStatement() agac.Statement {
	switch p.curToken().Type {
	case sozcuk.TOKEN_DEGISKEN:
		return p.parseVarStatement(false)
	case sozcuk.TOKEN_SABIT:
		return p.parseVarStatement(true)
	case sozcuk.TOKEN_SONUC:
		return p.parseReturnStatement()
	case sozcuk.TOKEN_CIK:
		p.sinyalDenetle(p.curToken())
		stmt := &agac.BreakStatement{Token: p.curToken()}
		if p.peekTokenIs(sozcuk.TOKEN_SEMICOLON) {
			p.nextToken()
		}
		return stmt
	case sozcuk.TOKEN_DEVAM:
		p.sinyalDenetle(p.curToken())
		stmt := &agac.ContinueStatement{Token: p.curToken()}
		if p.peekTokenIs(sozcuk.TOKEN_SEMICOLON) {
			p.nextToken()
		}
		return stmt
	case sozcuk.TOKEN_DONGU:
		return p.parseForStatement()
	case sozcuk.TOKEN_IKEN:
		return p.parseWhileStatement()
	case sozcuk.TOKEN_YAPI:
		return p.parseStructStatement()
	case sozcuk.TOKEN_DENE:
		return p.parseTryCatchStatement()
	case sozcuk.TOKEN_FIRLAT:
		return p.parseThrowStatement()
	case sozcuk.TOKEN_AKTAR:
		return p.parseImportStatement()
	case sozcuk.TOKEN_ESZAMANLI:
		return p.parseAsyncStatement()
	case sozcuk.TOKEN_ERTELE:
		return p.parseDeferStatement()
	case sozcuk.TOKEN_KESME_NOKTASI:
		return p.parseBreakpointStatement()
	case sozcuk.TOKEN_IDENT:
		// test "ad" { ... } — 'test' yalnızca deyim başında ve ardından metin geldiğinde
		// anahtar kelimedir; başka her yerde sıradan bir addır (değişken test = 5).
		if slices.Contains(testYazimlari, p.curToken().Literal) && p.peekTokenIs(sozcuk.TOKEN_STRING) {
			return p.parseTestStatement()
		}
		// x := 5 kısa değişken tanımı kontrolü
		if p.peekTokenIs(sozcuk.TOKEN_WALRUS) {
			return p.parseShortVarStatement()
		}
		return p.parseExpressionOrAssignmentStatement()
	default:
		return p.parseExpressionOrAssignmentStatement()
	}
}
func (p *Parser) parseTestStatement() agac.Statement {
	tok := p.curToken() // test
	p.nextToken()       // test adı (metin)
	name := unescapeBraces(p.curToken().Literal)
	if !p.expectPeek(sozcuk.TOKEN_LBRACE) {
		return nil
	}
	var body *agac.BlockStatement
	p.sinirIcinde(false, true, func() { body = p.parseBlockStatement() })
	return &agac.TestStatement{Token: tok, Name: name, Body: body}
}
func (p *Parser) parseBreakpointStatement() *agac.BreakpointStatement {
	tok := p.curToken()
	stmt := &agac.BreakpointStatement{Token: tok}
	if p.peekTokenIs(sozcuk.TOKEN_EGER) {
		p.nextToken() // 'eğer'
		p.nextToken()
		stmt.Condition = p.parseExpression(LOWEST)
	} else if p.peekTokenIs(sozcuk.TOKEN_LPAREN) {
		p.nextToken() // '('
		if !p.peekTokenIs(sozcuk.TOKEN_RPAREN) {
			p.nextToken()
			stmt.Condition = p.parseExpression(LOWEST)
		}
		if p.peekTokenIs(sozcuk.TOKEN_RPAREN) {
			p.nextToken() // ')'
		}
	}
	if p.peekTokenIs(sozcuk.TOKEN_SEMICOLON) {
		p.nextToken()
	}
	return stmt
}
func (p *Parser) parseDeferStatement() *agac.DeferStatement {
	tok := p.curToken()
	if p.peekTokenIs(sozcuk.TOKEN_LBRACE) {
		p.nextToken()
		var body *agac.BlockStatement
		p.sinirIcinde(false, false, func() { body = p.parseBlockStatement() })
		return &agac.DeferStatement{Token: tok, Body: body}
	}
	p.nextToken()
	call := p.parseExpression(LOWEST)
	if p.peekTokenIs(sozcuk.TOKEN_SEMICOLON) {
		p.nextToken()
	}
	return &agac.DeferStatement{Token: tok, Call: call}
}
func (p *Parser) parseVarStatement(isConst bool) agac.Statement {
	tok := p.curToken()

	if !p.expectPeek(sozcuk.TOKEN_IDENT) {
		return nil
	}
	firstIdent := &agac.Identifier{Token: p.curToken(), Value: p.curToken().Literal}

	// Çoklu değişken tanımı: değişken a, b = [10, 20] veya değişken a, b = 10, 20
	if p.peekTokenIs(sozcuk.TOKEN_COMMA) {
		names := []*agac.Identifier{firstIdent}
		for p.peekTokenIs(sozcuk.TOKEN_COMMA) {
			p.nextToken() // ','
			if !p.expectPeek(sozcuk.TOKEN_IDENT) {
				return nil
			}
			names = append(names, &agac.Identifier{Token: p.curToken(), Value: p.curToken().Literal})
		}
		if !p.peekTokenIs(sozcuk.TOKEN_ASSIGN) && !p.peekTokenIs(sozcuk.TOKEN_WALRUS) {
			tok := p.peekToken()
			p.hataKaydet(tok, fmt.Sprintf("Sözdizimi hatası — çoklu değişken tanımında '=' bekleniyordu, ancak %s bulundu", tokenAciklamasi(tok)))
			return nil
		}
		p.nextToken() // '='
		p.nextToken()
		values := []agac.Expression{p.parseExpression(LOWEST)}
		for p.peekTokenIs(sozcuk.TOKEN_COMMA) {
			p.nextToken() // ','
			p.nextToken()
			values = append(values, p.parseExpression(LOWEST))
		}
		if p.peekTokenIs(sozcuk.TOKEN_SEMICOLON) {
			p.nextToken()
		}
		return &agac.MultiVarStatement{
			Token:   tok,
			Names:   names,
			Values:  values,
			IsConst: isConst,
		}
	}

	stmt := &agac.VarStatement{Token: tok, Name: firstIdent, IsConst: isConst}

	// İsteğe bağlı tip belirtimi: değişken yaş: Tam = 25
	if p.peekTokenIs(sozcuk.TOKEN_COLON) {
		p.nextToken() // ':'
		if !p.expectPeek(sozcuk.TOKEN_IDENT) {
			return nil
		}
		stmt.TypeName = p.curToken().Literal
	}

	if p.peekTokenIs(sozcuk.TOKEN_ASSIGN) || p.peekTokenIs(sozcuk.TOKEN_WALRUS) {
		p.nextToken() // '=' veya ':='
		p.nextToken()
		stmt.Value = p.parseExpression(LOWEST)
	} else {
		stmt.Value = &agac.NullLiteral{Token: p.curToken()}
	}

	if p.peekTokenIs(sozcuk.TOKEN_SEMICOLON) {
		p.nextToken()
	}
	return stmt
}
func (p *Parser) parseShortVarStatement() *agac.VarStatement {
	identTok := p.curToken()
	p.nextToken() // ':=' üzerine gel
	walrusTok := p.curToken()
	p.nextToken() // değerin başına gel

	stmt := &agac.VarStatement{
		Token:   walrusTok,
		Name:    &agac.Identifier{Token: identTok, Value: identTok.Literal},
		Value:   p.parseExpression(LOWEST),
		IsConst: false,
	}
	if p.peekTokenIs(sozcuk.TOKEN_SEMICOLON) {
		p.nextToken()
	}
	return stmt
}
func (p *Parser) parseReturnStatement() *agac.ReturnStatement {
	p.sonucDenetle(p.curToken())
	stmt := &agac.ReturnStatement{Token: p.curToken()}

	// Eğer hemen ardından '}' veya ';' veya EOF geliyorsa boş dönüş
	if p.peekTokenIs(sozcuk.TOKEN_RBRACE) || p.peekTokenIs(sozcuk.TOKEN_SEMICOLON) || p.peekTokenIs(sozcuk.TOKEN_EOF) {
		stmt.ReturnValue = &agac.NullLiteral{Token: p.curToken()}
		if p.peekTokenIs(sozcuk.TOKEN_SEMICOLON) {
			p.nextToken()
		}
		return stmt
	}

	p.nextToken()
	stmt.ReturnValue = p.parseExpression(LOWEST)

	if p.peekTokenIs(sozcuk.TOKEN_SEMICOLON) {
		p.nextToken()
	}
	return stmt
}
func (p *Parser) parseExpressionOrAssignmentStatement() agac.Statement {
	tok := p.curToken()
	expr := p.parseExpression(LOWEST)

	// Çoklu atama / değişken takası: a, b = b, a veya a, b = [10, 20]
	if p.peekTokenIs(sozcuk.TOKEN_COMMA) {
		targets := []agac.Expression{expr}
		for p.peekTokenIs(sozcuk.TOKEN_COMMA) {
			p.nextToken() // ','
			p.nextToken()
			targets = append(targets, p.parseExpression(LOWEST))
		}
		if !p.expectPeek(sozcuk.TOKEN_ASSIGN) {
			return nil
		}
		opTok := p.curToken()
		p.nextToken()
		values := []agac.Expression{p.parseExpression(LOWEST)}
		for p.peekTokenIs(sozcuk.TOKEN_COMMA) {
			p.nextToken() // ','
			p.nextToken()
			values = append(values, p.parseExpression(LOWEST))
		}
		if p.peekTokenIs(sozcuk.TOKEN_SEMICOLON) {
			p.nextToken()
		}
		return &agac.MultiAssignStatement{
			Token:   opTok,
			Targets: targets,
			Values:  values,
		}
	}

	// Atama operatörü kontrolü (=, +=, -=, *=, /=)
	switch p.peekToken().Type {
	case sozcuk.TOKEN_ASSIGN, sozcuk.TOKEN_PLUS_ASSIGN, sozcuk.TOKEN_MINUS_ASSIGN, sozcuk.TOKEN_MUL_ASSIGN, sozcuk.TOKEN_DIV_ASSIGN:
		p.nextToken()
		opTok := p.curToken()
		p.nextToken()
		val := p.parseExpression(LOWEST)
		if p.peekTokenIs(sozcuk.TOKEN_SEMICOLON) {
			p.nextToken()
		}
		return &agac.AssignStatement{
			Token:    opTok,
			Target:   expr,
			Operator: opTok.Literal,
			Value:    val,
		}
	}

	if p.peekTokenIs(sozcuk.TOKEN_SEMICOLON) {
		p.nextToken()
	}
	return &agac.ExpressionStatement{Token: tok, Expression: expr}
}
func (p *Parser) parseBlockStatement() *agac.BlockStatement {
	defer p.allowBlocks()()
	block := &agac.BlockStatement{Token: p.curToken()}
	block.Statements = []agac.Statement{}
	if !p.derinlikGir() {
		p.blokSonunaAtla()
		return block
	}
	defer p.derinlikCik()

	p.nextToken()

	for !p.curTokenIs(sozcuk.TOKEN_RBRACE) && !p.curTokenIs(sozcuk.TOKEN_EOF) {
		if p.curTokenIs(sozcuk.TOKEN_SEMICOLON) {
			p.nextToken()
			continue
		}
		deyimBas := p.pos
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.senkronizeOl(deyimBas, false)
		p.nextToken()
	}
	p.requireClosingBrace(block.Token)

	return block
}

// requireClosingBrace, '{' ile açılan bir gövde dosya sonuna kadar kapatılmadıysa hata kaydeder.
// Aksi hâlde sonraki tüm kod sessizce bloğun içine girerdi (test raporu Y1).
func (p *Parser) requireClosingBrace(open sozcuk.Token) {
	if p.curTokenIs(sozcuk.TOKEN_EOF) {
		p.hataKaydet(open, "'{' kapatılmadı; dosya bitti ama bu bloğu kapatan '}' bulunamadı")
	}
}
func (p *Parser) parseImportStatement() *agac.ImportStatement {
	tok := p.curToken()
	// Modül yolu bir metin olarak yazılır: aktar "yol" (§11). Tırnaksız ad yol sayılmaz.
	if !p.peekTokenIs(sozcuk.TOKEN_STRING) {
		sonraki := p.peekToken()
		ornek := "yardımcı"
		if sonraki.Type == sozcuk.TOKEN_IDENT {
			ornek = sonraki.Literal
		}
		aktar := kod.Ad(kod.A070)
		p.hataKaydet(sonraki, fmt.Sprintf("Sözdizimi hatası — '%s' modül yolunu metin olarak bekler, ancak %s bulundu: %s \"%s\" yazın", aktar, tokenAciklamasi(sonraki), aktar, ornek))
		return nil
	}
	p.nextToken()
	path := p.curToken().Literal
	stmt := &agac.ImportStatement{Token: tok, Path: path}
	if p.peekTokenIs(sozcuk.TOKEN_IDENT) && slices.Contains(olarakYazimlari, p.peekToken().Literal) {
		p.nextToken() // 'olarak'
		if p.expectPeek(sozcuk.TOKEN_IDENT) {
			stmt.Alias = p.curToken().Literal
		}
	}
	if p.peekTokenIs(sozcuk.TOKEN_SEMICOLON) {
		p.nextToken()
	}
	return stmt
}
