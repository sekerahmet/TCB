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
	case lexer.TOKEN_VAR:
		return p.parseVarStatement(false)
	case lexer.TOKEN_CONST:
		return p.parseVarStatement(true)
	case lexer.TOKEN_RETURN:
		return p.parseReturnStatement()
	case lexer.TOKEN_BREAK:
		p.checkSignal(p.curToken())
		stmt := &ast.BreakStatement{Token: p.curToken()}
		if p.peekTokenIs(lexer.TOKEN_SEMICOLON) {
			p.nextToken()
		}
		return stmt
	case lexer.TOKEN_CONTINUE:
		p.checkSignal(p.curToken())
		stmt := &ast.ContinueStatement{Token: p.curToken()}
		if p.peekTokenIs(lexer.TOKEN_SEMICOLON) {
			p.nextToken()
		}
		return stmt
	case lexer.TOKEN_FOR:
		return p.parseForStatement()
	case lexer.TOKEN_WHILE:
		return p.parseWhileStatement()
	case lexer.TOKEN_STRUCT:
		return p.parseStructStatement()
	case lexer.TOKEN_TRY:
		return p.parseTryCatchStatement()
	case lexer.TOKEN_THROW:
		return p.parseThrowStatement()
	case lexer.TOKEN_IMPORT:
		return p.parseImportStatement()
	case lexer.TOKEN_CONCURRENT:
		return p.parseAsyncStatement()
	case lexer.TOKEN_DEFER:
		return p.parseDeferStatement()
	case lexer.TOKEN_BREAKPOINT:
		return p.parseBreakpointStatement()
	case lexer.TOKEN_IDENT:
		// test "ad" { ... } — 'test' yalnızca deyim başında ve ardından metin geldiğinde
		// anahtar kelimedir; başka her yerde sıradan bir addır (değişken test = 5).
		if slices.Contains(testSpellings, p.curToken().Literal) && p.peekTokenIs(lexer.TOKEN_STRING) {
			return p.parseTestStatement()
		}
		// x := 5 kısa değişken tanımı kontrolü
		if p.peekTokenIs(lexer.TOKEN_WALRUS) {
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
	if !p.expectPeek(lexer.TOKEN_LBRACE) {
		return nil
	}
	var body *ast.BlockStatement
	p.withinLimit(false, true, func() { body = p.parseBlockStatement() })
	return &ast.TestStatement{Token: tok, Name: name, Body: body}
}
func (p *Parser) parseBreakpointStatement() *ast.BreakpointStatement {
	tok := p.curToken()
	stmt := &ast.BreakpointStatement{Token: tok}
	if p.peekTokenIs(lexer.TOKEN_IF) {
		p.nextToken() // 'eğer'
		p.nextToken()
		stmt.Condition = p.parseExpression(LOWEST)
	} else if p.peekTokenIs(lexer.TOKEN_LPAREN) {
		p.nextToken() // '('
		if !p.peekTokenIs(lexer.TOKEN_RPAREN) {
			p.nextToken()
			stmt.Condition = p.parseExpression(LOWEST)
		}
		if p.peekTokenIs(lexer.TOKEN_RPAREN) {
			p.nextToken() // ')'
		}
	}
	if p.peekTokenIs(lexer.TOKEN_SEMICOLON) {
		p.nextToken()
	}
	return stmt
}
func (p *Parser) parseDeferStatement() *ast.DeferStatement {
	tok := p.curToken()
	if p.peekTokenIs(lexer.TOKEN_LBRACE) {
		p.nextToken()
		var body *ast.BlockStatement
		p.withinLimit(false, false, func() { body = p.parseBlockStatement() })
		return &ast.DeferStatement{Token: tok, Body: body}
	}
	p.nextToken()
	call := p.parseExpression(LOWEST)
	if p.peekTokenIs(lexer.TOKEN_SEMICOLON) {
		p.nextToken()
	}
	return &ast.DeferStatement{Token: tok, Call: call}
}
func (p *Parser) parseVarStatement(isConst bool) ast.Statement {
	tok := p.curToken()

	if !p.expectPeek(lexer.TOKEN_IDENT) {
		return nil
	}
	firstIdent := &ast.Identifier{Token: p.curToken(), Value: p.curToken().Literal}

	// Çoklu değişken tanımı: değişken a, b = [10, 20] veya değişken a, b = 10, 20
	if p.peekTokenIs(lexer.TOKEN_COMMA) {
		names := []*ast.Identifier{firstIdent}
		for p.peekTokenIs(lexer.TOKEN_COMMA) {
			p.nextToken() // ','
			if !p.expectPeek(lexer.TOKEN_IDENT) {
				return nil
			}
			names = append(names, &ast.Identifier{Token: p.curToken(), Value: p.curToken().Literal})
		}
		if !p.peekTokenIs(lexer.TOKEN_ASSIGN) && !p.peekTokenIs(lexer.TOKEN_WALRUS) {
			tok := p.peekToken()
			p.syntaxError(tok, messages.H0142, tokenDescription(tok))
			return nil
		}
		p.nextToken() // '='
		p.nextToken()
		values := []ast.Expression{p.parseExpression(LOWEST)}
		for p.peekTokenIs(lexer.TOKEN_COMMA) {
			p.nextToken() // ','
			p.nextToken()
			values = append(values, p.parseExpression(LOWEST))
		}
		if p.peekTokenIs(lexer.TOKEN_SEMICOLON) {
			p.nextToken()
		}
		return &ast.MultiVarStatement{
			Token:   tok,
			Names:   names,
			Values:  values,
			IsConst: isConst,
		}
	}

	stmt := &ast.VarStatement{Token: tok, Name: firstIdent, IsConst: isConst}

	// İsteğe bağlı tip belirtimi: değişken yaş: Tam = 25
	if p.peekTokenIs(lexer.TOKEN_COLON) {
		p.nextToken() // ':'
		if !p.expectPeek(lexer.TOKEN_IDENT) {
			return nil
		}
		stmt.TypeName = p.curToken().Literal
		p.role(p.curToken(), RoleType)
	}

	if p.peekTokenIs(lexer.TOKEN_ASSIGN) || p.peekTokenIs(lexer.TOKEN_WALRUS) {
		p.nextToken() // '=' veya ':='
		p.nextToken()
		stmt.Value = p.parseExpression(LOWEST)
	} else {
		stmt.Value = &ast.NullLiteral{Token: p.curToken()}
	}

	if p.peekTokenIs(lexer.TOKEN_SEMICOLON) {
		p.nextToken()
	}
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
		Value:   p.parseExpression(LOWEST),
		IsConst: false,
	}
	if p.peekTokenIs(lexer.TOKEN_SEMICOLON) {
		p.nextToken()
	}
	return stmt
}
func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	p.checkReturn(p.curToken())
	stmt := &ast.ReturnStatement{Token: p.curToken()}

	// Eğer hemen ardından '}' veya ';' veya EOF geliyorsa boş dönüş
	if p.peekTokenIs(lexer.TOKEN_RBRACE) || p.peekTokenIs(lexer.TOKEN_SEMICOLON) || p.peekTokenIs(lexer.TOKEN_EOF) {
		stmt.ReturnValue = &ast.NullLiteral{Token: p.curToken()}
		if p.peekTokenIs(lexer.TOKEN_SEMICOLON) {
			p.nextToken()
		}
		return stmt
	}

	p.nextToken()
	stmt.ReturnValue = p.parseExpression(LOWEST)

	if p.peekTokenIs(lexer.TOKEN_SEMICOLON) {
		p.nextToken()
	}
	return stmt
}
func (p *Parser) parseExpressionOrAssignmentStatement() ast.Statement {
	tok := p.curToken()
	expr := p.parseExpression(LOWEST)

	// Çoklu atama / değişken takası: a, b = b, a veya a, b = [10, 20]
	if p.peekTokenIs(lexer.TOKEN_COMMA) {
		targets := []ast.Expression{expr}
		for p.peekTokenIs(lexer.TOKEN_COMMA) {
			p.nextToken() // ','
			p.nextToken()
			targets = append(targets, p.parseExpression(LOWEST))
		}
		if !p.expectPeek(lexer.TOKEN_ASSIGN) {
			return nil
		}
		opTok := p.curToken()
		p.nextToken()
		values := []ast.Expression{p.parseExpression(LOWEST)}
		for p.peekTokenIs(lexer.TOKEN_COMMA) {
			p.nextToken() // ','
			p.nextToken()
			values = append(values, p.parseExpression(LOWEST))
		}
		if p.peekTokenIs(lexer.TOKEN_SEMICOLON) {
			p.nextToken()
		}
		return &ast.MultiAssignStatement{
			Token:   opTok,
			Targets: targets,
			Values:  values,
		}
	}

	// Atama operatörü kontrolü (=, +=, -=, *=, /=)
	switch p.peekToken().Type {
	case lexer.TOKEN_ASSIGN, lexer.TOKEN_PLUS_ASSIGN, lexer.TOKEN_MINUS_ASSIGN, lexer.TOKEN_MUL_ASSIGN, lexer.TOKEN_DIV_ASSIGN:
		p.nextToken()
		opTok := p.curToken()
		p.nextToken()
		val := p.parseExpression(LOWEST)
		if p.peekTokenIs(lexer.TOKEN_SEMICOLON) {
			p.nextToken()
		}
		return &ast.AssignStatement{
			Token:    opTok,
			Target:   expr,
			Operator: opTok.Literal,
			Value:    val,
		}
	}

	if p.peekTokenIs(lexer.TOKEN_SEMICOLON) {
		p.nextToken()
	}
	return &ast.ExpressionStatement{Token: tok, Expression: expr}
}
func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	defer p.allowBlocks()()
	block := &ast.BlockStatement{Token: p.curToken()}
	block.Statements = []ast.Statement{}
	if !p.enterDepth() {
		p.skipToBlockEnd()
		return block
	}
	defer p.leaveDepth()

	p.nextToken()

	for !p.curTokenIs(lexer.TOKEN_RBRACE) && !p.curTokenIs(lexer.TOKEN_EOF) {
		if p.curTokenIs(lexer.TOKEN_SEMICOLON) {
			p.nextToken()
			continue
		}
		stmtStart := p.pos
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.synchronize(stmtStart, false)
		p.nextToken()
	}
	p.requireClosingBrace(block.Token)

	return block
}

// requireClosingBrace, '{' ile açılan bir gövde dosya sonuna kadar kapatılmadıysa hata kaydeder.
// Aksi hâlde sonraki tüm kod sessizce bloğun içine girerdi (test raporu Y1).
func (p *Parser) requireClosingBrace(open lexer.Token) {
	if p.curTokenIs(lexer.TOKEN_EOF) {
		p.recordError(open, messages.New(messages.H0156))
	}
}
func (p *Parser) parseImportStatement() *ast.ImportStatement {
	tok := p.curToken()
	// Modül yolu bir metin olarak yazılır: aktar "yol" (§11). Tırnaksız ad yol sayılmaz.
	if !p.peekTokenIs(lexer.TOKEN_STRING) {
		next := p.peekToken()
		example := "yardımcı"
		if next.Type == lexer.TOKEN_IDENT {
			example = next.Literal
		}
		imp := codes.Name(codes.A070)
		p.syntaxError(next, messages.H0143, imp, tokenDescription(next), example)
		return nil
	}
	p.nextToken()
	path := p.curToken().Literal
	stmt := &ast.ImportStatement{Token: tok, Path: path}
	if p.peekTokenIs(lexer.TOKEN_IDENT) && slices.Contains(asSpellings, p.peekToken().Literal) {
		p.nextToken() // 'olarak'
		p.role(p.curToken(), RoleAs)
		if p.expectPeek(lexer.TOKEN_IDENT) {
			stmt.Alias = p.curToken().Literal
		}
	}
	if p.peekTokenIs(lexer.TOKEN_SEMICOLON) {
		p.nextToken()
	}
	return stmt
}
