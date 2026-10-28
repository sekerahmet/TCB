package parser

import (
	"slices"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken().Type {
	case lexer.TokVar:
		return p.parseVarStatement(false)
	case lexer.TokConst:
		return p.parseVarStatement(true)
	case lexer.TokReturn:
		return p.parseReturnStatement()
	case lexer.TokBreak:
		p.checkSignal(p.curToken())
		stmt := &ast.BreakStatement{Token: p.curToken()}
		p.skipSemicolon()
		return stmt
	case lexer.TokContinue:
		p.checkSignal(p.curToken())
		stmt := &ast.ContinueStatement{Token: p.curToken()}
		p.skipSemicolon()
		return stmt
	case lexer.TokFor:
		return p.parseForStatement()
	case lexer.TokStruct:
		return p.parseStructStatement()
	case lexer.TokTry:
		return p.parseTryCatchStatement()
	case lexer.TokThrow:
		return p.parseThrowStatement()
	case lexer.TokImport:
		return p.parseImportStatement()
	case lexer.TokConcurrent:
		return p.parseAsyncStatement()
	case lexer.TokDefer:
		return p.parseDeferStatement()
	case lexer.TokBreakpoint:
		return p.parseBreakpointStatement()
	case lexer.TokIdent:
		// test "ad" { ... } — 'test' yalnızca deyim başında ve ardından metin geldiğinde
		// anahtar kelimedir; başka her yerde sıradan bir addır (değişken test = 5).
		if slices.Contains(p.words.test, p.curToken().Literal) && p.peekTokenIs(lexer.TokString) {
			return p.parseTestStatement()
		}
		// x := 5: kısa tanım yoktur (5.1), ileti TR_var'ı önerir
		if p.peekTokenIs(lexer.TokWalrus) {
			p.shortVarError()
			return nil
		}
		return p.parseExpressionOrAssignmentStatement()
	default:
		return p.parseExpressionOrAssignmentStatement()
	}
}
func (p *Parser) parseTestStatement() ast.Statement {
	tok := p.curToken() // test
	p.role(tok, RoleTest)
	p.nextToken() // test adı (metin)
	name := p.curToken().Literal
	if !p.expectPeek(lexer.TokLBrace) {
		return nil
	}
	var body *ast.BlockStatement
	p.withinLimit(false, true, func() { body = p.parseBlockStatement() })
	return &ast.TestStatement{Token: tok, Name: name, Body: body}
}
func (p *Parser) parseBreakpointStatement() *ast.BreakpointStatement {
	tok := p.curToken()
	stmt := &ast.BreakpointStatement{Token: tok}
	if p.peekTokenIs(lexer.TokIf) {
		p.nextToken() // 'eğer'
		p.nextToken()
		stmt.Condition = p.parseExpression(precLowest)
	} else if p.peekTokenIs(lexer.TokLParen) {
		p.nextToken() // '('
		if !p.peekTokenIs(lexer.TokRParen) {
			p.nextToken()
			stmt.Condition = p.parseExpression(precLowest)
		}
		if p.peekTokenIs(lexer.TokRParen) {
			p.nextToken() // ')'
		}
	}
	p.skipSemicolon()
	return stmt
}
func (p *Parser) parseDeferStatement() *ast.DeferStatement {
	tok := p.curToken()
	body, call := p.parseBlockOrCall()
	return &ast.DeferStatement{Token: tok, Body: body, Call: call}
}

// parseBlockOrCall ertele ve eşzamanlı'nın gövdesini okur: ardından '{' geliyorsa blok (call yok),
// değilse tek bir ifade (body yok).
func (p *Parser) parseBlockOrCall() (body *ast.BlockStatement, call ast.Expression) {
	if p.peekTokenIs(lexer.TokLBrace) {
		p.nextToken()
		p.withinLimit(false, false, func() { body = p.parseBlockStatement() })
		return body, nil
	}
	p.nextToken()
	call = p.parseExpression(precLowest)
	p.skipSemicolon()
	return nil, call
}

// skipSemicolon deyimin sonundaki isteğe bağlı ';' işaretini atlar.
func (p *Parser) skipSemicolon() {
	if p.peekTokenIs(lexer.TokSemicolon) {
		p.nextToken()
	}
}
func (p *Parser) parseVarStatement(isConst bool) ast.Statement {
	tok := p.curToken()

	if !p.expectPeek(lexer.TokIdent) {
		return nil
	}
	firstIdent := &ast.Identifier{Token: p.curToken(), Value: p.curToken().Literal}

	// Çoklu değişken tanımı: değişken a, b = [10, 20] veya değişken a, b = 10, 20
	if p.peekTokenIs(lexer.TokComma) {
		names := []*ast.Identifier{firstIdent}
		for p.peekTokenIs(lexer.TokComma) {
			p.nextToken() // ','
			if !p.expectPeek(lexer.TokIdent) {
				return nil
			}
			names = append(names, &ast.Identifier{Token: p.curToken(), Value: p.curToken().Literal})
		}
		if p.peekTokenIs(lexer.TokWalrus) {
			p.walrusInDeclError(tok)
			return nil
		}
		if !p.peekTokenIs(lexer.TokAssign) {
			tok := p.peekToken()
			p.syntaxError(tok, messages.H0142, p.tokenDescription(tok))
			return nil
		}
		p.nextToken() // '='
		p.nextToken()
		values := p.parseCommaExpressions(p.parseExpression(precLowest))
		p.skipSemicolon()
		return &ast.MultiVarStatement{
			Token:   tok,
			Names:   names,
			Values:  values,
			IsConst: isConst,
		}
	}

	stmt := &ast.VarStatement{Token: tok, Name: firstIdent, IsConst: isConst}

	// İsteğe bağlı tip belirtimi: değişken yaş: Tam = 25
	if p.peekTokenIs(lexer.TokColon) {
		p.nextToken() // ':'
		if !p.expectPeek(lexer.TokIdent) {
			return nil
		}
		stmt.TypeName, stmt.TypeToken = p.curToken().Literal, p.curToken()
		p.role(p.curToken(), RoleType)
	}

	switch {
	case p.peekTokenIs(lexer.TokAssign):
		p.nextToken() // '='
		p.nextToken()
		stmt.Value = p.parseExpression(precLowest)
	case p.peekTokenIs(lexer.TokWalrus):
		p.walrusInDeclError(tok)
		return nil
	default:
		// İlk değer her zaman yazılır (5.1); ağaç yine kurulur ki ardından gelen denetim boşluk görmesin.
		stmt.Value = &ast.NullLiteral{Token: p.curToken()}
		p.noFirstValueError(tok, firstIdent.Value, p.curToken())
	}

	p.skipSemicolon()
	return stmt
}

// underscoreReads '_'nin okunduğu her yeri sözdizimi hatası yapar (5.3): '_' bir ad değildir, gerekmeyen
// değerin yerini tutar. Yazılabildiği yerler tanımın adları, atamanın hedefleri ('=' ile), döngü ve
// üreteç değişkeni, fonksiyon parametresi ve eşlemenin her şeyle eşleşen kolu; son üçü ağaçta ad
// değil metindir, burada görünmez.
func (p *Parser) underscoreReads(prog *ast.Program) {
	if !p.underscore || p.interpolated {
		return // ağaç yalnız ifadede '_' yazılmışsa ve en dıştaki ayrıştırıcıda gezilir
	}
	writes := map[*ast.Identifier]bool{}
	ast.Walk(prog, func(n any) bool {
		switch s := n.(type) {
		case *ast.VarStatement:
			writes[s.Name] = true
		case *ast.MultiVarStatement:
			for _, x := range s.Names {
				writes[x] = true
			}
		case *ast.MultiAssignStatement:
			for _, x := range s.Targets {
				if id, ok := x.(*ast.Identifier); ok {
					writes[id] = true
				}
			}
		case *ast.AssignStatement:
			if id, ok := s.Target.(*ast.Identifier); ok && s.Operator == lexer.TokAssign {
				writes[id] = true
			}
		}
		return true
	})
	ast.Walk(prog, func(n any) bool {
		if id, ok := n.(*ast.Identifier); ok && id.Value == "_" && !writes[id] {
			p.syntaxError(id.Token, messages.H0253)
			p.errorMode = false // her okuma ayrı bir yerdir
		}
		return true
	})
}

// shortVarError 'x := 5' kısa tanımının hatasıdır (5.1): imleç adda, ardından ':=' gelir. Düzeltme
// 'x :=' yerine 'TR_var x =' yazar; olduğu gibi uygulanabilir (Go'da da ':=' yeni ad tanımlar).
func (p *Parser) shortVarError() {
	name, walrus := p.curToken(), p.peekToken()
	v := p.word(codes.KwVar)
	r := name.Range()
	r.EndLine, r.EndCol = walrus.EndLine, walrus.EndColumn
	fixed := p.wordText(codes.KwVar) + " " + name.Literal + " ="
	p.syntaxErrorFix(walrus, []messages.Fix{{Range: r, Replacement: fixed, Applicable: true}}, messages.H0250, v, name.Literal)
}

// walrusInDeclError 'TR_var x := 5' yazımının hatasıdır (5.1): imleç ':='den önceki belirteçte. Düzeltme
// ':='yi '=' yapar.
func (p *Parser) walrusInDeclError(declTok lexer.Token) {
	walrus := p.peekToken()
	p.syntaxErrorFix(walrus, []messages.Fix{{Range: walrus.Range(), Replacement: "=", Applicable: true}}, messages.H0251, p.word(declTok.Code))
}

// noFirstValueError ilk değeri yazılmamış tanımın hatasıdır (5.1); last tanımın son belirtecidir (ad
// ya da tip). Önerilen ' = TR_none' bugünkü anlamı korur ama asıl değer değildir: uygulanabilir değil.
func (p *Parser) noFirstValueError(declTok lexer.Token, name string, last lexer.Token) {
	r := last.Range()
	r.Line, r.Col = r.EndLine, r.EndCol
	none := p.wordText(codes.KwNone)
	p.syntaxErrorFix(last, []messages.Fix{{Range: r, Replacement: " = " + none}}, messages.H0252, p.word(declTok.Code), name, none)
}
func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	p.checkReturn(p.curToken())
	stmt := &ast.ReturnStatement{Token: p.curToken()}

	// Eğer hemen ardından '}' veya ';' veya EOF geliyorsa boş dönüş
	if p.peekTokenIs(lexer.TokRBrace) || p.peekTokenIs(lexer.TokSemicolon) || p.peekTokenIs(lexer.TokEOF) {
		stmt.ReturnValue = &ast.NullLiteral{Token: p.curToken()}
		p.skipSemicolon()
		return stmt
	}

	p.nextToken()
	stmt.ReturnValue = p.parseExpression(precLowest)

	p.skipSemicolon()
	return stmt
}
func (p *Parser) parseExpressionOrAssignmentStatement() ast.Statement {
	tok := p.curToken()
	expr := p.parseExpression(precLowest)

	// Çoklu atama / değişken takası: a, b = b, a veya a, b = [10, 20]
	if p.peekTokenIs(lexer.TokComma) {
		targets := p.parseCommaExpressions(expr)
		if !p.expectPeek(lexer.TokAssign) {
			return nil
		}
		opTok := p.curToken()
		p.nextToken()
		values := p.parseCommaExpressions(p.parseExpression(precLowest))
		p.skipSemicolon()
		return &ast.MultiAssignStatement{
			Token:   opTok,
			Targets: targets,
			Values:  values,
		}
	}

	// Atama ve on iki bileşik atama (5.4)
	switch p.peekToken().Type {
	case lexer.TokAssign, lexer.TokPlusAssign, lexer.TokMinusAssign, lexer.TokMulAssign, lexer.TokDivAssign,
		lexer.TokFloorDivAssign, lexer.TokModAssign, lexer.TokPowAssign, lexer.TokBitAndAssign, lexer.TokBitOrAssign,
		lexer.TokBitXorAssign, lexer.TokShlAssign, lexer.TokShrAssign:
		p.nextToken()
		opTok := p.curToken()
		p.nextToken()
		val := p.parseExpression(precLowest)
		p.skipSemicolon()
		return &ast.AssignStatement{
			Token:    opTok,
			Target:   expr,
			Operator: lexer.TokenType(opTok.Literal),
			Value:    val,
		}
	}

	p.skipSemicolon()
	return &ast.ExpressionStatement{Token: tok, Expression: expr}
}
func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	defer p.allowBlocks()()
	defer p.enterBlockBody()()
	block := &ast.BlockStatement{Token: p.curToken()}
	block.Statements = []ast.Statement{} // atlanırsa da boş dizi
	if !p.enterDepth() {
		p.skipToBlockEnd()
		return block
	}
	defer p.leaveDepth()

	p.nextToken()

	block.Statements = p.parseStatementList(false)
	p.requireClosingBrace(block.Token)

	return block
}

// requireClosingBrace, '{' ile açılan bir gövde dosya sonuna kadar kapatılmadıysa hata kaydeder.
// Aksi hâlde sonraki tüm kod sessizce bloğun içine girerdi (test raporu Y1).
func (p *Parser) requireClosingBrace(open lexer.Token) {
	if p.curTokenIs(lexer.TokEOF) {
		p.recordError(open, messages.New(messages.H0156))
	}
}
func (p *Parser) parseImportStatement() *ast.ImportStatement {
	tok := p.curToken()
	// Aktarma yalnız dosyanın başında, öbür tanım ve deyimlerden önce yazılır (1.3): bir bloğun,
	// fonksiyonun ya da testin içinde ve başka bir deyimden sonra sözdizimi hatasıdır.
	if !p.importsOpen {
		p.syntaxError(tok, messages.H0024, p.word(codes.KwImport))
	}
	// Modül yolu bir metin olarak yazılır: aktar "yol" (§11). Tırnaksız ad yol sayılmaz.
	if !p.peekTokenIs(lexer.TokString) {
		next := p.peekToken()
		example := "" // boşsa şablon bir örnek ad yazar
		if next.Type == lexer.TokIdent {
			example = next.Literal
		}
		imp := p.word(codes.KwImport)
		p.syntaxError(next, messages.H0143, imp, p.tokenDescription(next), example)
		return nil
	}
	p.nextToken()
	path := p.curToken().Literal
	stmt := &ast.ImportStatement{Token: tok, Path: path}
	if p.peekTokenIs(lexer.TokIdent) && slices.Contains(p.words.as, p.peekToken().Literal) {
		p.nextToken() // 'olarak'
		p.role(p.curToken(), RoleAs)
		if p.expectPeek(lexer.TokIdent) {
			stmt.Alias = p.curToken().Literal
		}
	}
	p.skipSemicolon()
	return stmt
}

// parseCommaExpressions ilk ifadeden sonra gelen ', ifade' dizisini okur ve hepsini döndürür.
func (p *Parser) parseCommaExpressions(first ast.Expression) []ast.Expression {
	list := []ast.Expression{first}
	for p.peekTokenIs(lexer.TokComma) {
		p.nextToken() // ','
		p.nextToken()
		list = append(list, p.parseExpression(precLowest))
	}
	return list
}
