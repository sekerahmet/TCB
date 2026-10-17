package parser

import (
	"fmt"
	"strconv"
	"strings"
	"tr_dili/pkg/ast"
	"tr_dili/pkg/lexer"
)

const (
	_ int = iota
	LOWEST
	PIPE_PREC         // |>
	NULL_COALESC_PREC // ??
	OR_PREC           // || veya
	AND_PREC          // && ve
	EQUALS_PREC       // == !=
	LESSGREATER_PREC  // > < >= <=
	RANGE_PREC        // .. ..=
	SUM_PREC          // + -
	PRODUCT_PREC      // * / % @
	PREFIX_PREC       // -X !X  ('değil X' ise karşılaştırmalardan gevşek bağlanır, bkz. parsePrefixExpression)
	POWER_PREC        // **  (sağdan birleşir ve tekli eksiden sıkı bağlanır: -2 ** 2 = -4)
	CALL_PREC         // fn(X) dizi[i] nesne.alan nesne?.alan
)

var precedences = map[lexer.TokenType]int{
	lexer.TOKEN_PIPE:         PIPE_PREC,
	lexer.TOKEN_NULL_COALESC: NULL_COALESC_PREC,
	lexer.TOKEN_OR:           OR_PREC,
	lexer.TOKEN_VEYA:         OR_PREC,
	lexer.TOKEN_AND:          AND_PREC,
	lexer.TOKEN_VE:           AND_PREC,
	lexer.TOKEN_EQ:           EQUALS_PREC,
	lexer.TOKEN_NOT_EQ:       EQUALS_PREC,
	lexer.TOKEN_LT:           LESSGREATER_PREC,
	lexer.TOKEN_GT:           LESSGREATER_PREC,
	lexer.TOKEN_LTE:          LESSGREATER_PREC,
	lexer.TOKEN_GTE:          LESSGREATER_PREC,
	lexer.TOKEN_RANGE:        RANGE_PREC,
	lexer.TOKEN_RANGE_INCL:   RANGE_PREC,
	lexer.TOKEN_PLUS:         SUM_PREC,
	lexer.TOKEN_MINUS:        SUM_PREC,
	lexer.TOKEN_SLASH:        PRODUCT_PREC,
	lexer.TOKEN_ASTERISK:     PRODUCT_PREC,
	lexer.TOKEN_PERCENT:      PRODUCT_PREC,
	lexer.TOKEN_MATMUL:       PRODUCT_PREC,
	lexer.TOKEN_POWER:        POWER_PREC,
	lexer.TOKEN_LPAREN:       CALL_PREC,
	lexer.TOKEN_LBRACKET:     CALL_PREC,
	lexer.TOKEN_DOT:          CALL_PREC,
	lexer.TOKEN_SAFE_DOT:     CALL_PREC,
	lexer.TOKEN_ARROW:        PIPE_PREC,
	lexer.TOKEN_LBRACE:       CALL_PREC, // sondaki blok: liste.filtrele { o > 2 }
}

type Parser struct {
	tokens []lexer.Token
	pos    int
	errors []string

	// noBlock: 'eğer', 'iken', 'döngü' ve 'eşle' başlıklarında '{' gövdenin başlangıcıdır,
	// sondaki blok (o-lambda) olarak okunmamalıdır. Parantez/köşeli parantez içinde yeniden açılır.
	noBlock bool
}

// parseHeaderExpression bir kontrol yapısının başlık ifadesini (koşul, döngü kaynağı,
// eşle konusu) sondaki blokları kapatarak ayrıştırır.
func (p *Parser) parseHeaderExpression() ast.Expression {
	saved := p.noBlock
	p.noBlock = true
	defer func() { p.noBlock = saved }()
	return p.parseExpression(LOWEST)
}

// allowBlocks parantez, köşeli parantez ve blok içlerinde sondaki blokları yeniden etkinleştirir.
func (p *Parser) allowBlocks() func() {
	saved := p.noBlock
	p.noBlock = false
	return func() { p.noBlock = saved }
}

// isTrailingBlockTarget sondaki bloğun bağlanabileceği ifadeleri seçer:
// çağrı (f(x) { ... }), üye (liste.filtrele { ... }) ve ad (filtrele { ... }).
func isTrailingBlockTarget(e ast.Expression) bool {
	switch e.(type) {
	case *ast.CallExpression, *ast.MemberExpression, *ast.Identifier:
		return true
	}
	return false
}

// parseTrailingBlock '{ ... }' bloğunu tek parametresi 'o' olan bir fonksiyona çevirip
// çağrının son argümanı yapar: sayılar.filtrele { o > 2 } == sayılar.filtrele((o) => o > 2)
func (p *Parser) parseTrailingBlock(left ast.Expression) ast.Expression {
	tok := p.curToken() // '{'
	body := p.parseBlockStatement()
	fn := &ast.FunctionLiteral{
		Token:      tok,
		Parameters: []ast.Param{{Name: "o"}},
		Body:       body,
	}
	if call, ok := left.(*ast.CallExpression); ok {
		call.Arguments = append(call.Arguments, fn)
		return call
	}
	return &ast.CallExpression{Token: tok, Function: left, Arguments: []ast.Expression{fn}}
}

func New(l *lexer.Lexer) *Parser {
	var tokens []lexer.Token
	for {
		tok := l.NextToken()
		tokens = append(tokens, tok)
		if tok.Type == lexer.TOKEN_EOF {
			break
		}
	}
	// Sözcük çözümleme hataları (kapatılmamış metin vb.) önce gelir; ardından
	// gelen sözdizimi hataları çoğunlukla bunların sonucudur.
	errs := append([]string{}, l.Errors()...)
	return &Parser{
		tokens: tokens,
		pos:    0,
		errors: errs,
	}
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) curToken() lexer.Token {
	if p.pos >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[p.pos]
}

func (p *Parser) peekToken() lexer.Token {
	if p.pos+1 >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[p.pos+1]
}

func (p *Parser) peekTokenN(n int) lexer.Token {
	if p.pos+n >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[p.pos+n]
}

func (p *Parser) nextToken() {
	if p.pos < len(p.tokens)-1 {
		p.pos++
	}
}

func (p *Parser) curTokenIs(t lexer.TokenType) bool {
	return p.curToken().Type == t
}

func (p *Parser) peekTokenIs(t lexer.TokenType) bool {
	return p.peekToken().Type == t
}

func (p *Parser) expectPeek(t lexer.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.peekError(t)
	return false
}

func (p *Parser) peekError(t lexer.TokenType) {
	tok := p.peekToken()
	msg := fmt.Sprintf("Satır %d, Sütun %d: Sözdizimi hatası — %s bekleniyordu, ancak %s (%q) bulundu",
		tok.Line, tok.Column, t, tok.Type, tok.Literal)
	p.errors = append(p.errors, msg)
}

func (p *Parser) addError(format string, args ...interface{}) {
	tok := p.curToken()
	prefix := fmt.Sprintf("Satır %d, Sütun %d: ", tok.Line, tok.Column)
	p.errors = append(p.errors, prefix+fmt.Sprintf(format, args...))
}

func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken().Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken().Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	program.Statements = []ast.Statement{}

	for !p.curTokenIs(lexer.TOKEN_EOF) {
		if p.curTokenIs(lexer.TOKEN_SEMICOLON) {
			p.nextToken()
			continue
		}
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}

	return program
}

func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken().Type {
	case lexer.TOKEN_DEGISKEN:
		return p.parseVarStatement(false)
	case lexer.TOKEN_SABIT:
		return p.parseVarStatement(true)
	case lexer.TOKEN_SONUC:
		return p.parseReturnStatement()
	case lexer.TOKEN_CIK:
		stmt := &ast.BreakStatement{Token: p.curToken()}
		if p.peekTokenIs(lexer.TOKEN_SEMICOLON) {
			p.nextToken()
		}
		return stmt
	case lexer.TOKEN_DEVAM:
		stmt := &ast.ContinueStatement{Token: p.curToken()}
		if p.peekTokenIs(lexer.TOKEN_SEMICOLON) {
			p.nextToken()
		}
		return stmt
	case lexer.TOKEN_DONGU:
		return p.parseForStatement()
	case lexer.TOKEN_IKEN:
		return p.parseWhileStatement()
	case lexer.TOKEN_YAPI, lexer.TOKEN_SINIF:
		return p.parseStructStatement()
	case lexer.TOKEN_DENE:
		return p.parseTryCatchStatement()
	case lexer.TOKEN_FIRLAT:
		return p.parseThrowStatement()
	case lexer.TOKEN_AKTAR:
		return p.parseImportStatement()
	case lexer.TOKEN_ESZAMANLI:
		return p.parseAsyncStatement()
	case lexer.TOKEN_ERTELE:
		return p.parseDeferStatement()
	case lexer.TOKEN_KESME_NOKTASI:
		return p.parseBreakpointStatement()
	case lexer.TOKEN_IDENT:
		// test "ad" { ... } — 'test' yalnızca deyim başında ve ardından metin geldiğinde
		// anahtar kelimedir; başka her yerde sıradan bir addır (değişken test = 5).
		if p.curToken().Literal == "test" && p.peekTokenIs(lexer.TOKEN_STRING) {
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
	p.nextToken()       // test adı (metin)
	name := unescapeBraces(p.curToken().Literal)
	if !p.expectPeek(lexer.TOKEN_LBRACE) {
		return nil
	}
	body := p.parseBlockStatement()
	return &ast.TestStatement{Token: tok, Name: name, Body: body}
}

func (p *Parser) parseBreakpointStatement() *ast.BreakpointStatement {
	tok := p.curToken()
	stmt := &ast.BreakpointStatement{Token: tok}
	if p.peekTokenIs(lexer.TOKEN_EGER) {
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
		body := p.parseBlockStatement()
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
			p.addError("Çoklu değişken tanımında '=' bekleniyordu")
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

	p.nextToken()

	for !p.curTokenIs(lexer.TOKEN_RBRACE) && !p.curTokenIs(lexer.TOKEN_EOF) {
		if p.curTokenIs(lexer.TOKEN_SEMICOLON) {
			p.nextToken()
			continue
		}
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}
	p.requireClosingBrace(block.Token)

	return block
}

// requireClosingBrace, '{' ile açılan bir gövde dosya sonuna kadar kapatılmadıysa hata kaydeder.
// Aksi hâlde sonraki tüm kod sessizce bloğun içine girerdi (test raporu Y1).
func (p *Parser) requireClosingBrace(open lexer.Token) {
	if p.curTokenIs(lexer.TOKEN_EOF) {
		p.errors = append(p.errors, fmt.Sprintf(
			"Satır %d, Sütun %d: '{' kapatılmadı; dosya bitti ama bu bloğu kapatan '}' bulunamadı",
			open.Line, open.Column))
	}
}

func (p *Parser) parseForStatement() ast.Statement {
	tok := p.curToken() // döngü

	// Sonsuz döngü: döngü { ... }
	if p.peekTokenIs(lexer.TOKEN_LBRACE) {
		p.nextToken()
		body := p.parseBlockStatement()
		return &ast.WhileStatement{
			Token:     tok,
			Condition: &ast.BooleanLiteral{Token: tok, Value: true},
			Body:      body,
		}
	}

	// döngü i içinde liste { ... } veya döngü i, x içinde liste { ... }
	if p.peekTokenIs(lexer.TOKEN_IDENT) && (p.peekTokenN(2).Type == lexer.TOKEN_ICINDE || p.peekTokenN(2).Type == lexer.TOKEN_COMMA) {
		p.nextToken()
		firstVar := p.curToken().Literal
		keyVar := ""
		valVar := firstVar

		if p.peekTokenIs(lexer.TOKEN_COMMA) {
			p.nextToken() // ','
			if !p.expectPeek(lexer.TOKEN_IDENT) {
				return nil
			}
			keyVar = firstVar
			valVar = p.curToken().Literal
		}

		if !p.expectPeek(lexer.TOKEN_ICINDE) {
			return nil
		}
		p.nextToken() // iterable başına gel
		iterable := p.parseHeaderExpression()

		if !p.expectPeek(lexer.TOKEN_LBRACE) {
			return nil
		}
		body := p.parseBlockStatement()

		return &ast.ForInStatement{
			Token:    tok,
			KeyVar:   keyVar,
			ValueVar: valVar,
			Iterable: iterable,
			Body:     body,
		}
	}

	// Koşullu döngü: döngü x < 10 { ... }
	p.nextToken()
	cond := p.parseHeaderExpression()
	if !p.expectPeek(lexer.TOKEN_LBRACE) {
		return nil
	}
	body := p.parseBlockStatement()
	return &ast.WhileStatement{
		Token:     tok,
		Condition: cond,
		Body:      body,
	}
}

func (p *Parser) parseWhileStatement() *ast.WhileStatement {
	tok := p.curToken() // iken
	p.nextToken()
	cond := p.parseHeaderExpression()
	if !p.expectPeek(lexer.TOKEN_LBRACE) {
		return nil
	}
	body := p.parseBlockStatement()
	return &ast.WhileStatement{
		Token:     tok,
		Condition: cond,
		Body:      body,
	}
}

func (p *Parser) parseStructStatement() *ast.StructStatement {
	tok := p.curToken()
	if !p.expectPeek(lexer.TOKEN_IDENT) {
		return nil
	}
	stmt := &ast.StructStatement{
		Token:   tok,
		Name:    p.curToken().Literal,
		Fields:  []ast.StructField{},
		Methods: make(map[string]*ast.FunctionLiteral),
	}

	if !p.expectPeek(lexer.TOKEN_LBRACE) {
		return nil
	}
	structOpen := p.curToken()
	defer func() { p.requireClosingBrace(structOpen) }()
	p.nextToken()

	for !p.curTokenIs(lexer.TOKEN_RBRACE) && !p.curTokenIs(lexer.TOKEN_EOF) {
		if p.curTokenIs(lexer.TOKEN_SEMICOLON) || p.curTokenIs(lexer.TOKEN_COMMA) {
			p.nextToken()
			continue
		}
		if p.curTokenIs(lexer.TOKEN_FONKSIYON) {
			// Hatalı metod tanımında parseFunctionLiteral nil döner; tip dönüşümü panik yapmamalı
			fn, _ := p.parseFunctionLiteral().(*ast.FunctionLiteral)
			if fn == nil {
				// Hata zaten kaydedildi; iç içe süslü parantezleri sayarak yapının kapanışına kadar atla
				depth := 0
				for !p.curTokenIs(lexer.TOKEN_EOF) {
					if p.curTokenIs(lexer.TOKEN_LBRACE) {
						depth++
					} else if p.curTokenIs(lexer.TOKEN_RBRACE) {
						if depth == 0 {
							break
						}
						depth--
					}
					p.nextToken()
				}
				break
			}
			if fn.Name != "" {
				stmt.Methods[fn.Name] = fn
			}
			p.nextToken()
			continue
		}
		if p.curTokenIs(lexer.TOKEN_DEGISKEN) {
			p.nextToken()
		}
		if p.curTokenIs(lexer.TOKEN_IDENT) {
			field := ast.StructField{Name: p.curToken().Literal}
			if p.peekTokenIs(lexer.TOKEN_COLON) {
				p.nextToken() // ':'
				if p.expectPeek(lexer.TOKEN_IDENT) {
					field.TypeName = p.curToken().Literal
				}
			}
			if p.peekTokenIs(lexer.TOKEN_ASSIGN) {
				p.nextToken() // '='
				p.nextToken()
				field.DefaultValue = p.parseExpression(LOWEST)
			}
			stmt.Fields = append(stmt.Fields, field)
		}
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseTryCatchStatement() *ast.TryCatchStatement {
	tok := p.curToken() // dene
	if !p.expectPeek(lexer.TOKEN_LBRACE) {
		return nil
	}
	tryBody := p.parseBlockStatement()

	if !p.expectPeek(lexer.TOKEN_YAKALA) {
		return nil
	}

	errVar := "hata"
	if p.peekTokenIs(lexer.TOKEN_IDENT) {
		p.nextToken()
		errVar = p.curToken().Literal
	} else if p.peekTokenIs(lexer.TOKEN_LPAREN) {
		p.nextToken() // '('
		if p.expectPeek(lexer.TOKEN_IDENT) {
			errVar = p.curToken().Literal
		}
		p.expectPeek(lexer.TOKEN_RPAREN)
	}

	if !p.expectPeek(lexer.TOKEN_LBRACE) {
		return nil
	}
	catchBody := p.parseBlockStatement()

	return &ast.TryCatchStatement{
		Token:     tok,
		TryBody:   tryBody,
		ErrorVar:  errVar,
		CatchBody: catchBody,
	}
}

func (p *Parser) parseThrowStatement() *ast.ThrowStatement {
	tok := p.curToken()
	p.nextToken()
	val := p.parseExpression(LOWEST)
	if p.peekTokenIs(lexer.TOKEN_SEMICOLON) {
		p.nextToken()
	}
	return &ast.ThrowStatement{Token: tok, Value: val}
}

func (p *Parser) parseImportStatement() *ast.ImportStatement {
	tok := p.curToken()
	p.nextToken()
	path := p.curToken().Literal
	stmt := &ast.ImportStatement{Token: tok, Path: path}
	if p.peekTokenIs(lexer.TOKEN_IDENT) && (p.peekToken().Literal == "olarak" || p.peekToken().Literal == "as") {
		p.nextToken() // 'olarak'
		if p.expectPeek(lexer.TOKEN_IDENT) {
			stmt.Alias = p.curToken().Literal
		}
	}
	if p.peekTokenIs(lexer.TOKEN_SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseAsyncStatement() *ast.AsyncStatement {
	tok := p.curToken()
	if p.peekTokenIs(lexer.TOKEN_LBRACE) {
		p.nextToken()
		body := p.parseBlockStatement()
		return &ast.AsyncStatement{Token: tok, Body: body}
	}
	p.nextToken()
	call := p.parseExpression(LOWEST)
	if p.peekTokenIs(lexer.TOKEN_SEMICOLON) {
		p.nextToken()
	}
	return &ast.AsyncStatement{Token: tok, Call: call}
}

// --- İFADE ÇÖZÜMLEME (PRATT PARSER) ---

func (p *Parser) parseExpression(precedence int) ast.Expression {
	var leftExp ast.Expression

	switch p.curToken().Type {
	case lexer.TOKEN_IDENT:
		// Tek parametreli ok fonksiyonu: x => x * 2
		if p.peekTokenIs(lexer.TOKEN_ARROW) {
			leftExp = p.parseSingleParamLambda()
		} else {
			leftExp = &ast.Identifier{Token: p.curToken(), Value: p.curToken().Literal}
		}
	case lexer.TOKEN_KENDI:
		leftExp = &ast.Identifier{Token: p.curToken(), Value: "kendi"}
	case lexer.TOKEN_INT:
		leftExp = p.parseIntegerLiteral()
	case lexer.TOKEN_FLOAT:
		leftExp = p.parseFloatLiteral()
	case lexer.TOKEN_STRING:
		leftExp = p.parseStringOrInterpolation()
	case lexer.TOKEN_DOGRU:
		leftExp = &ast.BooleanLiteral{Token: p.curToken(), Value: true}
	case lexer.TOKEN_YANLIS:
		leftExp = &ast.BooleanLiteral{Token: p.curToken(), Value: false}
	case lexer.TOKEN_YOK:
		leftExp = &ast.NullLiteral{Token: p.curToken()}
	case lexer.TOKEN_BANG, lexer.TOKEN_MINUS, lexer.TOKEN_DEGIL:
		leftExp = p.parsePrefixExpression()
	case lexer.TOKEN_LPAREN:
		leftExp = p.parseGroupedOrLambdaExpression()
	case lexer.TOKEN_EGER:
		leftExp = p.parseIfExpression()
	case lexer.TOKEN_ESLE:
		leftExp = p.parseMatchExpression()
	case lexer.TOKEN_FONKSIYON:
		leftExp = p.parseFunctionLiteral()
	case lexer.TOKEN_LBRACKET:
		leftExp = p.parseListLiteral()
	case lexer.TOKEN_LBRACE:
		leftExp = p.parseDictLiteral()
	default:
		p.addError("beklenmeyen sözcük: %s (%q)", p.curToken().Type, p.curToken().Literal)
		return nil
	}

	for !p.peekTokenIs(lexer.TOKEN_SEMICOLON) && precedence < p.peekPrecedence() {
		switch p.peekToken().Type {
		case lexer.TOKEN_PLUS, lexer.TOKEN_MINUS, lexer.TOKEN_SLASH, lexer.TOKEN_ASTERISK,
			lexer.TOKEN_PERCENT, lexer.TOKEN_POWER, lexer.TOKEN_MATMUL,
			lexer.TOKEN_EQ, lexer.TOKEN_NOT_EQ, lexer.TOKEN_LT, lexer.TOKEN_GT,
			lexer.TOKEN_LTE, lexer.TOKEN_GTE, lexer.TOKEN_AND, lexer.TOKEN_OR,
			lexer.TOKEN_VE, lexer.TOKEN_VEYA, lexer.TOKEN_PIPE, lexer.TOKEN_NULL_COALESC:
			p.nextToken()
			leftExp = p.parseInfixExpression(leftExp)
		case lexer.TOKEN_RANGE, lexer.TOKEN_RANGE_INCL:
			p.nextToken()
			leftExp = p.parseRangeExpression(leftExp)
		case lexer.TOKEN_LPAREN, lexer.TOKEN_LBRACKET:
			// Yeni satırda başlayan '(' veya '[' önceki ifadeyi sürdürmez; yeni bir
			// deyim başlatır. Aksi hâlde 'değişken f = 5' ardından gelen
			// '(1 + 2) |> yazdır' satırı '5(1 + 2)' çağrısı olarak okunurdu.
			if p.peekToken().Line > p.curToken().Line {
				return leftExp
			}
			p.nextToken()
			if p.curTokenIs(lexer.TOKEN_LPAREN) {
				leftExp = p.parseCallExpression(leftExp)
			} else {
				leftExp = p.parseIndexExpression(leftExp)
			}
		case lexer.TOKEN_DOT, lexer.TOKEN_SAFE_DOT:
			p.nextToken()
			leftExp = p.parseMemberExpression(leftExp)
		case lexer.TOKEN_LBRACE:
			// Sondaki blok yalnızca aynı satırda ve kontrol yapısı başlığı dışında geçerlidir
			if p.noBlock || p.peekToken().Line != p.curToken().Line || !isTrailingBlockTarget(leftExp) {
				return leftExp
			}
			p.nextToken()
			leftExp = p.parseTrailingBlock(leftExp)
		default:
			return leftExp
		}
	}

	return leftExp
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	tok := p.curToken()
	val, err := strconv.ParseInt(tok.Literal, 10, 64)
	if err != nil {
		p.addError("%q tam sayıya dönüştürülemedi", tok.Literal)
		return nil
	}
	return &ast.IntegerLiteral{Token: tok, Value: val}
}

func (p *Parser) parseFloatLiteral() ast.Expression {
	tok := p.curToken()
	val, err := strconv.ParseFloat(tok.Literal, 64)
	if err != nil {
		p.addError("%q ondalık sayıya dönüştürülemedi", tok.Literal)
		return nil
	}
	return &ast.FloatLiteral{Token: tok, Value: val}
}

func (p *Parser) parseStringOrInterpolation() ast.Expression {
	tok := p.curToken()
	raw := tok.Literal

	if !strings.Contains(raw, "{") {
		return &ast.StringLiteral{Token: tok, Value: unescapeBraces(raw)}
	}

	// Metin içi {ifade} ayrıştırması
	runes := []rune(raw)
	var parts []ast.Expression
	var textBuf strings.Builder
	i := 0

	for i < len(runes) {
		if runes[i] == '{' {
			i++ // '{' geç
			exprStart := i
			depth := 1
			var exprBuf strings.Builder
			for i < len(runes) && depth > 0 {
				if runes[i] == '{' {
					depth++
				} else if runes[i] == '}' {
					depth--
					if depth == 0 {
						break
					}
				}
				exprBuf.WriteRune(runes[i])
				i++
			}
			if i < len(runes) && runes[i] == '}' {
				i++ // '}' geç
			}
			rawExpr := exprBuf.String()
			exprCode := strings.TrimSpace(rawExpr)
			interpolated := false
			if exprCode != "" {
				// İfadenin dosyadaki konumu: metnin açılış tırnağından sonraki karakter
				// sayısı kadar ilerisi (önündeki kaçış dizileri konumu bir-iki sütun kaydırabilir)
				line, col := interpolationPos(tok, runes[:exprStart])
				col += len([]rune(rawExpr)) - len([]rune(strings.TrimLeft(rawExpr, " \t")))
				subLex := lexer.NewAt(exprCode, line, col)
				subParser := New(subLex)
				subProg := subParser.ParseProgram()
				if len(subParser.Errors()) == 0 && len(subProg.Statements) == 1 {
					if es, ok := subProg.Statements[0].(*ast.ExpressionStatement); ok && es.Expression != nil {
						if textBuf.Len() > 0 {
							parts = append(parts, &ast.StringLiteral{Token: tok, Value: textBuf.String()})
							textBuf.Reset()
						}
						parts = append(parts, es.Expression)
						interpolated = true
					}
				}
			}
			if !interpolated {
				textBuf.WriteString("{" + unescapeBraces(exprBuf.String()) + "}")
			}
			continue
		}
		textBuf.WriteString(unescapeBraces(string(runes[i])))
		i++
	}

	if textBuf.Len() > 0 {
		parts = append(parts, &ast.StringLiteral{Token: tok, Value: textBuf.String()})
	}

	if len(parts) == 1 {
		if sl, ok := parts[0].(*ast.StringLiteral); ok {
			return sl
		}
	}

	return &ast.InterpolatedString{Token: tok, Parts: parts}
}

// interpolationPos metin sabitinde, 'onceki' karakterlerinden sonra başlayan yerleştirme
// ifadesinin dosyadaki satır ve sütununu hesaplar.
func interpolationPos(tok lexer.Token, onceki []rune) (int, int) {
	line, col := tok.Line, tok.Column+1 // açılış tırnağından sonraki ilk karakter
	for _, r := range onceki {
		if r == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}

// unescapeBraces sözcük çözümleyicinin '\{' ve '\}' için ürettiği işaretçi
// karakterleri düz süslü parantezlere çevirir.
func unescapeBraces(s string) string {
	s = strings.ReplaceAll(s, string(lexer.EscapedLBrace), "{")
	return strings.ReplaceAll(s, string(lexer.EscapedRBrace), "}")
}

func (p *Parser) parsePrefixExpression() ast.Expression {
	tok := p.curToken()
	op := tok.Literal
	if tok.Type == lexer.TOKEN_DEGIL {
		op = "!"
	}
	p.nextToken()
	prec := PREFIX_PREC
	if tok.Type == lexer.TOKEN_DEGIL {
		// Python'daki 'not' gibi: 'değil 1 == 2' -> değil (1 == 2), ancak
		// 'değil a ve b' -> (değil a) ve b
		prec = AND_PREC
	}
	right := p.parseExpression(prec)
	return &ast.PrefixExpression{Token: tok, Operator: op, Right: right}
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	tok := p.curToken()
	op := tok.Literal
	if tok.Type == lexer.TOKEN_VE {
		op = "&&"
	} else if tok.Type == lexer.TOKEN_VEYA {
		op = "||"
	}
	prec := p.curPrecedence()
	if tok.Type == lexer.TOKEN_POWER {
		// '**' sağdan birleşir: 2 ** 3 ** 2 = 2 ** (3 ** 2)
		prec--
	}
	p.nextToken()
	right := p.parseExpression(prec)
	return &ast.InfixExpression{Token: tok, Left: left, Operator: op, Right: right}
}

func (p *Parser) parseRangeExpression(left ast.Expression) ast.Expression {
	tok := p.curToken()
	inclusive := tok.Type == lexer.TOKEN_RANGE_INCL
	prec := p.curPrecedence()
	p.nextToken()
	right := p.parseExpression(prec)
	return &ast.RangeExpression{
		Token:     tok,
		Start:     left,
		End:       right,
		Inclusive: inclusive,
	}
}

func (p *Parser) parseGroupedOrLambdaExpression() ast.Expression {
	defer p.allowBlocks()()
	// Önce bunun '(a, b) => ...' bir ok fonksiyonu (lambda) olup olmadığını kontrol et
	if p.isLambdaAhead() {
		return p.parseLambdaExpression()
	}

	p.nextToken()
	exp := p.parseExpression(LOWEST)
	if !p.expectPeek(lexer.TOKEN_RPAREN) {
		return nil
	}
	return exp
}

func (p *Parser) isLambdaAhead() bool {
	// mevcut token '('
	idx := p.pos + 1
	depth := 1
	for idx < len(p.tokens) && depth > 0 {
		if p.tokens[idx].Type == lexer.TOKEN_LPAREN {
			depth++
		} else if p.tokens[idx].Type == lexer.TOKEN_RPAREN {
			depth--
		}
		idx++
	}
	if idx < len(p.tokens) && p.tokens[idx].Type == lexer.TOKEN_ARROW {
		return true
	}
	return false
}

func (p *Parser) parseSingleParamLambda() ast.Expression {
	tok := p.curToken()
	param := ast.Param{Name: tok.Literal}
	p.nextToken() // '=>'
	arrowTok := p.curToken()

	var body *ast.BlockStatement
	if p.peekTokenIs(lexer.TOKEN_LBRACE) {
		p.nextToken()
		body = p.parseBlockStatement()
	} else {
		p.nextToken()
		expr := p.parseExpression(LOWEST)
		body = &ast.BlockStatement{
			Token: arrowTok,
			Statements: []ast.Statement{
				&ast.ReturnStatement{Token: arrowTok, ReturnValue: expr},
			},
		}
	}

	return &ast.FunctionLiteral{
		Token:      tok,
		Parameters: []ast.Param{param},
		Body:       body,
	}
}

func (p *Parser) parseLambdaExpression() ast.Expression {
	tok := p.curToken() // '('
	params := p.parseFunctionParameters()
	if !p.expectPeek(lexer.TOKEN_ARROW) {
		return nil
	}
	arrowTok := p.curToken()

	var body *ast.BlockStatement
	if p.peekTokenIs(lexer.TOKEN_LBRACE) {
		p.nextToken()
		body = p.parseBlockStatement()
	} else {
		p.nextToken()
		expr := p.parseExpression(LOWEST)
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

func (p *Parser) parseIfExpression() ast.Expression {
	expr := &ast.IfExpression{Token: p.curToken()}

	p.nextToken()
	expr.Condition = p.parseHeaderExpression()

	if !p.expectPeek(lexer.TOKEN_LBRACE) {
		return nil
	}
	expr.Consequence = p.parseBlockStatement()

	for p.peekTokenIs(lexer.TOKEN_DEGILSE) {
		p.nextToken() // 'değilse'
		if p.peekTokenIs(lexer.TOKEN_EGER) {
			p.nextToken() // 'eğer'
			p.nextToken()
			cond := p.parseExpression(LOWEST)
			if !p.expectPeek(lexer.TOKEN_LBRACE) {
				return nil
			}
			block := p.parseBlockStatement()
			expr.ElseIfs = append(expr.ElseIfs, ast.ElseIfBranch{
				Condition:   cond,
				Consequence: block,
			})
		} else {
			if !p.expectPeek(lexer.TOKEN_LBRACE) {
				return nil
			}
			expr.Alternative = p.parseBlockStatement()
			break
		}
	}

	return expr
}

func (p *Parser) parseMatchExpression() ast.Expression {
	tok := p.curToken() // eşle
	p.nextToken()
	subject := p.parseHeaderExpression()

	if !p.expectPeek(lexer.TOKEN_LBRACE) {
		return nil
	}
	matchOpen := p.curToken()
	defer func() { p.requireClosingBrace(matchOpen) }()
	p.nextToken()

	var cases []ast.MatchCase
	for !p.curTokenIs(lexer.TOKEN_RBRACE) && !p.curTokenIs(lexer.TOKEN_EOF) {
		if p.curTokenIs(lexer.TOKEN_COMMA) || p.curTokenIs(lexer.TOKEN_SEMICOLON) {
			p.nextToken()
			continue
		}

		mc := ast.MatchCase{}
		if p.curTokenIs(lexer.TOKEN_VARSAYILAN) || (p.curTokenIs(lexer.TOKEN_IDENT) && p.curToken().Literal == "_") {
			mc.IsDefault = true
		} else {
			pat := p.parseExpression(LOWEST)
			mc.Patterns = append(mc.Patterns, pat)
			for p.peekTokenIs(lexer.TOKEN_COMMA) {
				p.nextToken() // ','
				p.nextToken()
				mc.Patterns = append(mc.Patterns, p.parseExpression(LOWEST))
			}
		}

		if !p.expectPeek(lexer.TOKEN_ARROW) {
			return nil
		}

		if p.peekTokenIs(lexer.TOKEN_LBRACE) {
			p.nextToken()
			mc.Body = p.parseBlockStatement()
		} else if p.peekTokenIs(lexer.TOKEN_SONUC) {
			p.nextToken()
			retStmt := p.parseReturnStatement()
			mc.Body = &ast.BlockStatement{
				Token:      retStmt.Token,
				Statements: []ast.Statement{retStmt},
			}
		} else {
			p.nextToken()
			mc.ValueExpr = p.parseExpression(LOWEST)
		}

		cases = append(cases, mc)
		p.nextToken()
	}

	return &ast.MatchExpression{
		Token:   tok,
		Subject: subject,
		Cases:   cases,
	}
}

func (p *Parser) parseFunctionLiteral() ast.Expression {
	lit := &ast.FunctionLiteral{Token: p.curToken()}

	// İsimli fonksiyon: fonksiyon topla(a, b) { ... }
	if p.peekTokenIs(lexer.TOKEN_IDENT) {
		p.nextToken()
		lit.Name = p.curToken().Literal
	}

	if !p.expectPeek(lexer.TOKEN_LPAREN) {
		return nil
	}

	lit.Parameters = p.parseFunctionParameters()

	// İsteğe bağlı dönüş tipi: fonksiyon topla(a: Tam, b: Tam): Tam { ... } veya -> Tam { ... }
	if p.peekTokenIs(lexer.TOKEN_COLON) || p.peekTokenIs(lexer.TOKEN_ARROW) {
		p.nextToken()
		if p.expectPeek(lexer.TOKEN_IDENT) {
			lit.ReturnType = p.curToken().Literal
		}
	} else if p.peekTokenIs(lexer.TOKEN_MINUS) {
		p.nextToken() // '-'
		if p.peekTokenIs(lexer.TOKEN_GT) {
			p.nextToken() // '>'
			if p.expectPeek(lexer.TOKEN_IDENT) {
				lit.ReturnType = p.curToken().Literal
			}
		}
	}

	if !p.expectPeek(lexer.TOKEN_LBRACE) {
		return nil
	}

	lit.Body = p.parseBlockStatement()
	return lit
}

func (p *Parser) parseFunctionParameters() []ast.Param {
	params := []ast.Param{}

	if p.peekTokenIs(lexer.TOKEN_RPAREN) {
		p.nextToken()
		return params
	}

	p.nextToken()

	for {
		param := ast.Param{Name: p.curToken().Literal}
		if p.peekTokenIs(lexer.TOKEN_COLON) {
			p.nextToken() // ':'
			if p.expectPeek(lexer.TOKEN_IDENT) {
				param.TypeName = p.curToken().Literal
			}
		}
		if p.peekTokenIs(lexer.TOKEN_ASSIGN) {
			p.nextToken() // '='
			p.nextToken()
			param.DefaultValue = p.parseExpression(LOWEST)
		}
		params = append(params, param)

		if !p.peekTokenIs(lexer.TOKEN_COMMA) {
			break
		}
		p.nextToken() // ','
		p.nextToken()
	}

	if !p.expectPeek(lexer.TOKEN_RPAREN) {
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
	args := []ast.Expression{}
	named := []ast.NamedArg{}

	if p.peekTokenIs(lexer.TOKEN_RPAREN) {
		p.nextToken()
		return args, named
	}

	p.nextToken()
	for {
		if p.curTokenIs(lexer.TOKEN_IDENT) && p.peekTokenIs(lexer.TOKEN_COLON) {
			name := p.curToken().Literal
			p.nextToken() // ':'
			p.nextToken()
			val := p.parseExpression(LOWEST)
			named = append(named, ast.NamedArg{Name: name, Value: val})
		} else {
			args = append(args, p.parseExpression(LOWEST))
		}

		if !p.peekTokenIs(lexer.TOKEN_COMMA) {
			break
		}
		p.nextToken() // ','
		if p.peekTokenIs(lexer.TOKEN_RPAREN) {
			break
		}
		p.nextToken()
	}

	if !p.expectPeek(lexer.TOKEN_RPAREN) {
		return nil, nil
	}

	return args, named
}

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
	if p.peekTokenIs(lexer.TOKEN_DONGU) {
		p.nextToken() // 'döngü'
		if !p.expectPeek(lexer.TOKEN_IDENT) {
			return nil
		}
		varName := p.curToken().Literal
		if !p.expectPeek(lexer.TOKEN_ICINDE) {
			return nil
		}
		p.nextToken()
		iterable := p.parseExpression(LOWEST)

		var cond ast.Expression
		if p.peekTokenIs(lexer.TOKEN_EGER) {
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
	p.nextToken()
	prop := p.curToken().Literal
	return &ast.MemberExpression{
		Token:    tok,
		Object:   left,
		Property: prop,
		Safe:     safe,
	}
}
