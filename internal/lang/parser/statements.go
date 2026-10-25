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
	case lexer.TokWhile:
		return p.parseWhileStatement()
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
		// x := 5 kısa değişken tanımı kontrolü
		if p.peekTokenIs(lexer.TokWalrus) {
			return p.parseShortVarStatement()
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
	name := unescapeBraces(p.curToken().Literal)
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
		if !p.peekTokenIs(lexer.TokAssign) && !p.peekTokenIs(lexer.TokWalrus) {
			tok := p.peekToken()
			p.syntaxError(tok, messages.H0142, tokenDescription(tok))
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
		stmt.TypeName = p.curToken().Literal
		p.role(p.curToken(), RoleType)
	}

	if p.peekTokenIs(lexer.TokAssign) || p.peekTokenIs(lexer.TokWalrus) {
		p.nextToken() // '=' veya ':='
		p.nextToken()
		stmt.Value = p.parseExpression(precLowest)
	} else {
		stmt.Value = &ast.NullLiteral{Token: p.curToken()}
	}

	p.skipSemicolon()
	return stmt
}
func (p *Parser) parseShortVarStatement() *ast.VarStatement {
	identTok := p.curToken()
	p.nextToken() // ':=' üzerine gel
	walrusTok := p.curToken()
	p.nextToken() // değerin başına gel

	stmt := &ast.VarStatement{
		Token:   walrusTok,
		Name:    &ast.Identifier{Token: identTok, Value: identTok.Literal},
		Value:   p.parseExpression(precLowest),
		IsConst: false,
	}
	p.skipSemicolon()
	return stmt
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

	// Atama operatörü kontrolü (=, +=, -=, *=, /=)
	switch p.peekToken().Type {
	case lexer.TokAssign, lexer.TokPlusAssign, lexer.TokMinusAssign, lexer.TokMulAssign, lexer.TokDivAssign:
		p.nextToken()
		opTok := p.curToken()
		p.nextToken()
		val := p.parseExpression(precLowest)
		p.skipSemicolon()
		return &ast.AssignStatement{
			Token:    opTok,
			Target:   expr,
			Operator: opTok.Literal,
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
	// Modül yolu bir metin olarak yazılır: aktar "yol" (§11). Tırnaksız ad yol sayılmaz.
	if !p.peekTokenIs(lexer.TokString) {
		next := p.peekToken()
		example := "" // boşsa şablon bir örnek ad yazar
		if next.Type == lexer.TokIdent {
			example = next.Literal
		}
		imp := codes.NameRef(codes.KwImport)
		p.syntaxError(next, messages.H0143, imp, tokenDescription(next), example)
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
