// Package ayristir TR'nin ayrıştırıcısıdır: sözcük birimlerinden (internal/dil/sozcuk)
// sözdizimi ağacını (internal/dil/agac) kurar. Deyimler özyinelemeli inişle, ifadeler
// öncelik tabanlı (Pratt) yöntemle çözümlenir. Hatalar Türkçe, satırlı ve yol
// göstericidir; kaldırılmış yazımlar için ipucu verilir.
//
// Katman: dil çekirdeği (internal/dil); yalnızca internal/dil paketlerini içe aktarır.
package ayristir

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/sozcuk"
)

// Bağlamsal sözcüklerin (test, olarak, yakala'nın varsayılan hata adı, sondaki bloğun
// parametresi) yazımları ve 'kendi'nin adı kod listesinden gelir (internal/dil/kod: A101, A100, A103,
// A104, A051).
var (
	testYazimlari   = kod.Yazimlar(kod.A101)
	olarakYazimlari = kod.Yazimlar(kod.A100)
	kendiAdi        = kod.Ad(kod.A051)
	hataAdi         = kod.Ad(kod.A103)
	oAdi            = kod.Ad(kod.A104)
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

var precedences = map[sozcuk.TokenType]int{
	sozcuk.TOKEN_PIPE:         PIPE_PREC,
	sozcuk.TOKEN_NULL_COALESC: NULL_COALESC_PREC,
	sozcuk.TOKEN_OR:           OR_PREC,
	sozcuk.TOKEN_VEYA:         OR_PREC,
	sozcuk.TOKEN_AND:          AND_PREC,
	sozcuk.TOKEN_VE:           AND_PREC,
	sozcuk.TOKEN_EQ:           EQUALS_PREC,
	sozcuk.TOKEN_NOT_EQ:       EQUALS_PREC,
	sozcuk.TOKEN_LT:           LESSGREATER_PREC,
	sozcuk.TOKEN_GT:           LESSGREATER_PREC,
	sozcuk.TOKEN_LTE:          LESSGREATER_PREC,
	sozcuk.TOKEN_GTE:          LESSGREATER_PREC,
	sozcuk.TOKEN_RANGE:        RANGE_PREC,
	sozcuk.TOKEN_RANGE_INCL:   RANGE_PREC,
	sozcuk.TOKEN_PLUS:         SUM_PREC,
	sozcuk.TOKEN_MINUS:        SUM_PREC,
	sozcuk.TOKEN_SLASH:        PRODUCT_PREC,
	sozcuk.TOKEN_ASTERISK:     PRODUCT_PREC,
	sozcuk.TOKEN_PERCENT:      PRODUCT_PREC,
	sozcuk.TOKEN_MATMUL:       PRODUCT_PREC,
	sozcuk.TOKEN_POWER:        POWER_PREC,
	sozcuk.TOKEN_LPAREN:       CALL_PREC,
	sozcuk.TOKEN_LBRACKET:     CALL_PREC,
	sozcuk.TOKEN_DOT:          CALL_PREC,
	sozcuk.TOKEN_SAFE_DOT:     CALL_PREC,
	sozcuk.TOKEN_ARROW:        PIPE_PREC,
	sozcuk.TOKEN_LBRACE:       CALL_PREC, // sondaki blok: liste.filtrele { o > 2 }
}

type Parser struct {
	tokens []sozcuk.Token
	pos    int
	errors []string

	// noBlock: 'eğer', 'iken', 'döngü' ve 'eşle' başlıklarında '{' gövdenin başlangıcıdır,
	// sondaki blok (o-lambda) olarak okunmamalıdır. Parantez/köşeli parantez içinde yeniden açılır.
	noBlock bool

	// hataModu bir deyimde sözdizimi hatası kaydedildikten sonra açıktır: deyim
	// senkronizeOl ile atlanana dek yeni hata kaydedilmez (art arda gelen hatalar
	// çoğunlukla ilkinin sonucudur). hataSatiri ilk hatanın satırıdır.
	hataModu   bool
	hataSatiri int

	// desenKipi bir eşle kolunun deseni okunurken açıktır: desenin hemen ardından gelen
	// '=>' kolun okudur, ok fonksiyonu değildir (durum => ..., (a + 1) => ...). Yalnızca
	// desenin en dıştaki ifadesini etkiler; parseExpression girişte kapatır.
	desenKipi bool

	// baglam 'çık'/'devam'/'sonuç' deyimlerinin geçerliliği için izlenen bağlamdır
	// (baglam.go); modul, kodun bir modül olarak ayrıştırıldığını söyler (ParseModule).
	baglam deyimBaglami
	modul  bool

	// derinlik içinde bulunulan ifade/blok iç içelik düzeyidir; derinlikAsildi sınır
	// aşıldıktan sonra açıktır (bkz. derinlikGir).
	derinlik       int
	derinlikAsildi bool
}

// parseMatchPattern bir eşle kolunun desenini okur.
func (p *Parser) parseMatchPattern() agac.Expression {
	p.desenKipi = true
	defer func() { p.desenKipi = false }()
	return p.parseExpression(LOWEST)
}

// parseHeaderExpression bir kontrol yapısının başlık ifadesini (koşul, döngü kaynağı,
// eşle konusu) sondaki blokları kapatarak ayrıştırır.
func (p *Parser) parseHeaderExpression() agac.Expression {
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
func isTrailingBlockTarget(e agac.Expression) bool {
	switch e.(type) {
	case *agac.CallExpression, *agac.MemberExpression, *agac.Identifier:
		return true
	}
	return false
}

// parseTrailingBlock '{ ... }' bloğunu tek parametresi 'o' olan bir fonksiyona çevirip
// çağrının son argümanı yapar: sayılar.filtrele { o > 2 } == sayılar.filtrele((o) => o > 2)
func (p *Parser) parseTrailingBlock(left agac.Expression) agac.Expression {
	tok := p.curToken() // '{'
	body := p.fonksiyonGovdesi()
	fn := &agac.FunctionLiteral{
		Token:      tok,
		Parameters: []agac.Param{{Name: oAdi}},
		Body:       body,
	}
	if call, ok := left.(*agac.CallExpression); ok {
		call.Arguments = append(call.Arguments, fn)
		return call
	}
	return &agac.CallExpression{Token: tok, Function: left, Arguments: []agac.Expression{fn}}
}

func New(l *sozcuk.Lexer) *Parser {
	var tokens []sozcuk.Token
	for {
		tok := l.NextToken()
		tokens = append(tokens, tok)
		if tok.Type == sozcuk.TOKEN_EOF {
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

func (p *Parser) curToken() sozcuk.Token {
	if p.pos >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[p.pos]
}

func (p *Parser) peekToken() sozcuk.Token {
	if p.pos+1 >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[p.pos+1]
}

func (p *Parser) peekTokenN(n int) sozcuk.Token {
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

func (p *Parser) curTokenIs(t sozcuk.TokenType) bool {
	return p.curToken().Type == t
}

func (p *Parser) peekTokenIs(t sozcuk.TokenType) bool {
	return p.peekToken().Type == t
}

func (p *Parser) expectPeek(t sozcuk.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.peekError(t)
	return false
}

func (p *Parser) peekError(t sozcuk.TokenType) {
	tok := p.peekToken()
	p.hataKaydet(tok, fmt.Sprintf("Sözdizimi hatası — %s bekleniyordu, ancak %s bulundu%s",
		turAciklamasi(t), tokenAciklamasi(tok), beklenmeyenIpucu(t, tok)))
}

// addError geçerli token'ın konumunda bir sözdizimi hatası kaydeder.
func (p *Parser) addError(format string, args ...interface{}) {
	p.hataKaydet(p.curToken(), fmt.Sprintf(format, args...))
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

func (p *Parser) ParseProgram() *agac.Program {
	program := &agac.Program{}
	program.Statements = []agac.Statement{}

	for !p.curTokenIs(sozcuk.TOKEN_EOF) {
		if p.curTokenIs(sozcuk.TOKEN_SEMICOLON) {
			p.nextToken()
			continue
		}
		deyimBas := p.pos
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.senkronizeOl(deyimBas, true)
		p.nextToken()
	}
	p.kaldirilanYazimIpuclari()
	return program
}

// kaldirilanYazimIpuclari sözdizimi hatası olan satırlarda kaldırılmış bir anahtar kelime
// (ör. 'döndür', 'kır') ad olarak geçiyorsa, eski alışkanlığa yol gösteren bir ipucu ekler.
func (p *Parser) kaldirilanYazimIpuclari() {
	if len(p.errors) == 0 {
		return
	}
	hataliSatir := map[int]bool{}
	for _, e := range p.errors {
		var satir int
		if _, err := fmt.Sscanf(e, "Satır %d", &satir); err == nil {
			hataliSatir[satir] = true
		}
	}
	for _, t := range p.tokens {
		if t.Type != sozcuk.TOKEN_IDENT || !hataliSatir[t.Line] {
			continue
		}
		if ipucu := sozcuk.KaldirilanYazimIpucu(t.Literal); ipucu != "" {
			p.errors = append(p.errors, fmt.Sprintf("Satır %d, Sütun %d: ipucu%s", t.Line, t.Column, ipucu))
		}
	}
}

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

func (p *Parser) parseForStatement() agac.Statement {
	tok := p.curToken() // döngü

	// Sonsuz döngü: döngü { ... }
	if p.peekTokenIs(sozcuk.TOKEN_LBRACE) {
		p.nextToken()
		body := p.donguGovdesi()
		return &agac.WhileStatement{
			Token:     tok,
			Condition: &agac.BooleanLiteral{Token: tok, Value: true},
			Body:      body,
		}
	}

	// döngü i içinde liste { ... } veya döngü i, x içinde liste { ... }
	if p.peekTokenIs(sozcuk.TOKEN_IDENT) && (p.peekTokenN(2).Type == sozcuk.TOKEN_ICINDE || p.peekTokenN(2).Type == sozcuk.TOKEN_COMMA) {
		p.nextToken()
		firstVar := p.curToken().Literal
		keyVar := ""
		valVar := firstVar

		if p.peekTokenIs(sozcuk.TOKEN_COMMA) {
			p.nextToken() // ','
			if !p.expectPeek(sozcuk.TOKEN_IDENT) {
				return nil
			}
			keyVar = firstVar
			valVar = p.curToken().Literal
		}

		if !p.expectPeek(sozcuk.TOKEN_ICINDE) {
			return nil
		}
		p.nextToken() // iterable başına gel
		iterable := p.parseHeaderExpression()

		if !p.expectPeek(sozcuk.TOKEN_LBRACE) {
			return nil
		}
		body := p.donguGovdesi()

		return &agac.ForInStatement{
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
	if !p.expectPeek(sozcuk.TOKEN_LBRACE) {
		return nil
	}
	body := p.donguGovdesi()
	return &agac.WhileStatement{
		Token:     tok,
		Condition: cond,
		Body:      body,
	}
}

func (p *Parser) parseWhileStatement() *agac.WhileStatement {
	tok := p.curToken() // iken
	p.nextToken()
	cond := p.parseHeaderExpression()
	if !p.expectPeek(sozcuk.TOKEN_LBRACE) {
		return nil
	}
	body := p.donguGovdesi()
	return &agac.WhileStatement{
		Token:     tok,
		Condition: cond,
		Body:      body,
	}
}

func (p *Parser) parseStructStatement() *agac.StructStatement {
	tok := p.curToken()
	if !p.expectPeek(sozcuk.TOKEN_IDENT) {
		return nil
	}
	stmt := &agac.StructStatement{
		Token:   tok,
		Name:    p.curToken().Literal,
		Fields:  []agac.StructField{},
		Methods: make(map[string]*agac.FunctionLiteral),
	}

	if !p.expectPeek(sozcuk.TOKEN_LBRACE) {
		return nil
	}
	structOpen := p.curToken()
	defer func() { p.requireClosingBrace(structOpen) }()
	p.nextToken()

	// Yapı gövdesinde yalnızca alanlar ve fonksiyon tanımları bulunur; her ad bir kez
	// tanımlanır. Başka içerik sessizce alan sayılmaz ya da atlanmaz: sözdizimi hatasıdır.
	tanimli := map[string]bool{}
	adDenetle := func(tok sozcuk.Token, ad, tur string) bool {
		if tanimli[ad] {
			p.hataKaydet(tok, fmt.Sprintf("Sözdizimi hatası — '%s' %s '%s' yapısında iki kez tanımlandı (her alan ve metot adı bir kez kullanılır)", ad, tur, stmt.Name))
			return false
		}
		tanimli[ad] = true
		return true
	}
	gecersiz := func(tok sozcuk.Token, ne string) *agac.StructStatement {
		p.hataKaydet(tok, "Sözdizimi hatası — yapı gövdesinde yalnızca alanlar (ad, ad: Tip, ad = değer) ve fonksiyon tanımları bulunabilir; "+ne)
		p.blokSonunaAtla()
		return stmt
	}
	for !p.curTokenIs(sozcuk.TOKEN_RBRACE) && !p.curTokenIs(sozcuk.TOKEN_EOF) {
		if p.curTokenIs(sozcuk.TOKEN_SEMICOLON) || p.curTokenIs(sozcuk.TOKEN_COMMA) {
			p.nextToken()
			continue
		}
		if p.curTokenIs(sozcuk.TOKEN_FONKSIYON) {
			fnTok := p.curToken()
			// Hatalı metod tanımında parseFunctionLiteral nil döner; tip dönüşümü panik yapmamalı
			fn, _ := p.parseFunctionLiteral().(*agac.FunctionLiteral)
			if fn == nil {
				// Hata zaten kaydedildi; yapının kapanışına kadar atla
				p.blokSonunaAtla()
				break
			}
			if fn.Name == "" {
				p.hataKaydet(fnTok, "Sözdizimi hatası — yapı metodunun bir adı olmalı: fonksiyon ad(...) { ... }")
				p.blokSonunaAtla()
				break
			}
			if !adDenetle(fnTok, fn.Name, "metodu") {
				p.blokSonunaAtla()
				break
			}
			stmt.Methods[fn.Name] = fn
			p.nextToken()
			continue
		}
		if p.curTokenIs(sozcuk.TOKEN_DEGISKEN) {
			p.nextToken()
		}
		if !p.curTokenIs(sozcuk.TOKEN_IDENT) {
			return gecersiz(p.curToken(), tokenAciklamasi(p.curToken())+" bir alan tanımı değil")
		}
		alanTok := p.curToken()
		field := agac.StructField{Name: alanTok.Literal}
		if p.peekTokenIs(sozcuk.TOKEN_COLON) {
			p.nextToken() // ':'
			if p.expectPeek(sozcuk.TOKEN_IDENT) {
				field.TypeName = p.curToken().Literal
			}
		}
		if p.peekTokenIs(sozcuk.TOKEN_ASSIGN) {
			p.nextToken() // '='
			p.nextToken()
			field.DefaultValue = p.parseExpression(LOWEST)
		}
		// Alan virgül, ';', satır sonu ya da '}' ile biter: aynı satırda başka bir şey
		// (ör. 'yazdır("...")' çağrısının '(' işareti) alan değildir.
		if sonraki := p.peekToken(); sonraki.Type != sozcuk.TOKEN_RBRACE && sonraki.Type != sozcuk.TOKEN_COMMA &&
			sonraki.Type != sozcuk.TOKEN_SEMICOLON && sonraki.Type != sozcuk.TOKEN_EOF && sonraki.Line == p.curToken().Line {
			return gecersiz(alanTok, fmt.Sprintf("'%s' ardından gelen %s bir alan tanımı değil (alanlar virgül ya da satır sonuyla ayrılır)", alanTok.Literal, tokenAciklamasi(sonraki)))
		}
		if !adDenetle(alanTok, field.Name, "alanı") {
			p.blokSonunaAtla()
			break
		}
		stmt.Fields = append(stmt.Fields, field)
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseTryCatchStatement() *agac.TryCatchStatement {
	tok := p.curToken() // dene
	if !p.expectPeek(sozcuk.TOKEN_LBRACE) {
		return nil
	}
	tryBody := p.parseBlockStatement()

	if !p.expectPeek(sozcuk.TOKEN_YAKALA) {
		return nil
	}

	errVar := hataAdi
	if p.peekTokenIs(sozcuk.TOKEN_IDENT) {
		p.nextToken()
		errVar = p.curToken().Literal
	} else if p.peekTokenIs(sozcuk.TOKEN_LPAREN) {
		p.nextToken() // '('
		if p.expectPeek(sozcuk.TOKEN_IDENT) {
			errVar = p.curToken().Literal
		}
		p.expectPeek(sozcuk.TOKEN_RPAREN)
	}

	if !p.expectPeek(sozcuk.TOKEN_LBRACE) {
		return nil
	}
	catchBody := p.parseBlockStatement()

	return &agac.TryCatchStatement{
		Token:     tok,
		TryBody:   tryBody,
		ErrorVar:  errVar,
		CatchBody: catchBody,
	}
}

func (p *Parser) parseThrowStatement() *agac.ThrowStatement {
	tok := p.curToken()
	p.nextToken()
	val := p.parseExpression(LOWEST)
	if p.peekTokenIs(sozcuk.TOKEN_SEMICOLON) {
		p.nextToken()
	}
	return &agac.ThrowStatement{Token: tok, Value: val}
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
		p.hataKaydet(sonraki, fmt.Sprintf("Sözdizimi hatası — 'aktar' modül yolunu metin olarak bekler, ancak %s bulundu: aktar \"%s\" yazın", tokenAciklamasi(sonraki), ornek))
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

func (p *Parser) parseAsyncStatement() *agac.AsyncStatement {
	tok := p.curToken()
	if p.peekTokenIs(sozcuk.TOKEN_LBRACE) {
		p.nextToken()
		var body *agac.BlockStatement
		p.sinirIcinde(false, false, func() { body = p.parseBlockStatement() })
		return &agac.AsyncStatement{Token: tok, Body: body}
	}
	p.nextToken()
	call := p.parseExpression(LOWEST)
	if p.peekTokenIs(sozcuk.TOKEN_SEMICOLON) {
		p.nextToken()
	}
	return &agac.AsyncStatement{Token: tok, Call: call}
}

// --- İFADE ÇÖZÜMLEME (PRATT PARSER) ---

func (p *Parser) parseExpression(precedence int) agac.Expression {
	if !p.derinlikGir() {
		return nil
	}
	defer p.derinlikCik()
	var leftExp agac.Expression
	desen := p.desenKipi
	p.desenKipi = false

	switch p.curToken().Type {
	case sozcuk.TOKEN_IDENT:
		// Tek parametreli ok fonksiyonu: x => x * 2 (eşle deseninde ad, değeriyle karşılaştırılır)
		if !desen && p.peekTokenIs(sozcuk.TOKEN_ARROW) {
			leftExp = p.parseSingleParamLambda()
		} else {
			leftExp = &agac.Identifier{Token: p.curToken(), Value: p.curToken().Literal}
		}
	case sozcuk.TOKEN_KENDI:
		leftExp = &agac.Identifier{Token: p.curToken(), Value: kendiAdi}
	case sozcuk.TOKEN_INT:
		leftExp = p.parseIntegerLiteral()
	case sozcuk.TOKEN_FLOAT:
		leftExp = p.parseFloatLiteral()
	case sozcuk.TOKEN_STRING:
		leftExp = p.parseStringOrInterpolation()
	case sozcuk.TOKEN_DOGRU:
		leftExp = &agac.BooleanLiteral{Token: p.curToken(), Value: true}
	case sozcuk.TOKEN_YANLIS:
		leftExp = &agac.BooleanLiteral{Token: p.curToken(), Value: false}
	case sozcuk.TOKEN_YOK:
		leftExp = &agac.NullLiteral{Token: p.curToken()}
	case sozcuk.TOKEN_BANG, sozcuk.TOKEN_MINUS, sozcuk.TOKEN_DEGIL:
		leftExp = p.parsePrefixExpression()
	case sozcuk.TOKEN_LPAREN:
		leftExp = p.parseGroupedOrLambdaExpression(desen)
	case sozcuk.TOKEN_EGER:
		leftExp = p.parseIfExpression()
	case sozcuk.TOKEN_ESLE:
		leftExp = p.parseMatchExpression()
	case sozcuk.TOKEN_FONKSIYON:
		leftExp = p.parseFunctionLiteral()
	case sozcuk.TOKEN_LBRACKET:
		leftExp = p.parseListLiteral()
	case sozcuk.TOKEN_LBRACE:
		leftExp = p.parseDictLiteral()
	default:
		p.addError("Sözdizimi hatası — burada bir değer ya da ifade bekleniyordu, ancak %s bulundu", tokenAciklamasi(p.curToken()))
		return nil
	}

	for !p.peekTokenIs(sozcuk.TOKEN_SEMICOLON) && precedence < p.peekPrecedence() {
		switch p.peekToken().Type {
		case sozcuk.TOKEN_PLUS, sozcuk.TOKEN_MINUS, sozcuk.TOKEN_SLASH, sozcuk.TOKEN_ASTERISK,
			sozcuk.TOKEN_PERCENT, sozcuk.TOKEN_POWER, sozcuk.TOKEN_MATMUL,
			sozcuk.TOKEN_EQ, sozcuk.TOKEN_NOT_EQ, sozcuk.TOKEN_LT, sozcuk.TOKEN_GT,
			sozcuk.TOKEN_LTE, sozcuk.TOKEN_GTE, sozcuk.TOKEN_AND, sozcuk.TOKEN_OR,
			sozcuk.TOKEN_VE, sozcuk.TOKEN_VEYA, sozcuk.TOKEN_PIPE, sozcuk.TOKEN_NULL_COALESC:
			p.nextToken()
			leftExp = p.parseInfixExpression(leftExp)
		case sozcuk.TOKEN_RANGE, sozcuk.TOKEN_RANGE_INCL:
			p.nextToken()
			leftExp = p.parseRangeExpression(leftExp)
		case sozcuk.TOKEN_LPAREN, sozcuk.TOKEN_LBRACKET:
			// Yeni satırda başlayan '(' veya '[' önceki ifadeyi sürdürmez; yeni bir
			// deyim başlatır. Aksi hâlde 'değişken f = 5' ardından gelen
			// '(1 + 2) |> yazdır' satırı '5(1 + 2)' çağrısı olarak okunurdu.
			if p.peekToken().Line > p.curToken().Line {
				return leftExp
			}
			p.nextToken()
			if p.curTokenIs(sozcuk.TOKEN_LPAREN) {
				leftExp = p.parseCallExpression(leftExp)
			} else {
				leftExp = p.parseIndexExpression(leftExp)
			}
		case sozcuk.TOKEN_DOT, sozcuk.TOKEN_SAFE_DOT:
			p.nextToken()
			leftExp = p.parseMemberExpression(leftExp)
		case sozcuk.TOKEN_LBRACE:
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

func (p *Parser) parseIntegerLiteral() agac.Expression {
	tok := p.curToken()
	val, err := strconv.ParseInt(tok.Literal, 10, 64)
	if err != nil {
		p.addError("%s tam sayı olarak çok büyük (en fazla %d olabilir; ondalık sayı için sonuna .0 ekleyin)", tok.Literal, int64(math.MaxInt64))
		return nil
	}
	return &agac.IntegerLiteral{Token: tok, Value: val}
}

func (p *Parser) parseFloatLiteral() agac.Expression {
	tok := p.curToken()
	val, err := strconv.ParseFloat(tok.Literal, 64)
	if err != nil {
		p.addError("%s ondalık sayı olarak çok büyük ya da geçersiz", tok.Literal)
		return nil
	}
	return &agac.FloatLiteral{Token: tok, Value: val}
}

func (p *Parser) parseStringOrInterpolation() agac.Expression {
	tok := p.curToken()
	raw := tok.Literal

	if !strings.Contains(raw, "{") {
		return &agac.StringLiteral{Token: tok, Value: unescapeBraces(raw)}
	}

	// Metin içi {ifade} ayrıştırması
	runes := []rune(raw)
	var parts []agac.Expression
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
				subLex := sozcuk.NewAt(exprCode, line, col)
				subParser := New(subLex)
				subProg := subParser.ParseProgram()
				if len(subParser.Errors()) == 0 && len(subProg.Statements) == 1 {
					if es, ok := subProg.Statements[0].(*agac.ExpressionStatement); ok && es.Expression != nil {
						if textBuf.Len() > 0 {
							parts = append(parts, &agac.StringLiteral{Token: tok, Value: textBuf.String()})
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
		parts = append(parts, &agac.StringLiteral{Token: tok, Value: textBuf.String()})
	}

	if len(parts) == 1 {
		if sl, ok := parts[0].(*agac.StringLiteral); ok {
			return sl
		}
	}

	return &agac.InterpolatedString{Token: tok, Parts: parts}
}

// interpolationPos metin sabitinde, 'onceki' karakterlerinden sonra başlayan yerleştirme
// ifadesinin dosyadaki satır ve sütununu hesaplar.
func interpolationPos(tok sozcuk.Token, onceki []rune) (int, int) {
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
	s = strings.ReplaceAll(s, string(sozcuk.EscapedLBrace), "{")
	return strings.ReplaceAll(s, string(sozcuk.EscapedRBrace), "}")
}

func (p *Parser) parsePrefixExpression() agac.Expression {
	tok := p.curToken()
	op := tok.Literal
	if tok.Type == sozcuk.TOKEN_DEGIL {
		op = "!"
	}
	p.nextToken()
	prec := PREFIX_PREC
	if tok.Type == sozcuk.TOKEN_DEGIL {
		// 'değil' karşılaştırmadan zayıf bağlar: 'değil 1 == 2' -> değil (1 == 2), ancak
		// 'değil a ve b' -> (değil a) ve b
		prec = AND_PREC
	}
	right := p.parseExpression(prec)
	return &agac.PrefixExpression{Token: tok, Operator: op, Right: right}
}

func (p *Parser) parseInfixExpression(left agac.Expression) agac.Expression {
	tok := p.curToken()
	op := tok.Literal
	if tok.Type == sozcuk.TOKEN_VE {
		op = "&&"
	} else if tok.Type == sozcuk.TOKEN_VEYA {
		op = "||"
	}
	prec := p.curPrecedence()
	if tok.Type == sozcuk.TOKEN_POWER {
		// '**' sağdan birleşir: 2 ** 3 ** 2 = 2 ** (3 ** 2)
		prec--
	}
	p.nextToken()
	right := p.parseExpression(prec)
	return &agac.InfixExpression{Token: tok, Left: left, Operator: op, Right: right}
}

func (p *Parser) parseRangeExpression(left agac.Expression) agac.Expression {
	tok := p.curToken()
	inclusive := tok.Type == sozcuk.TOKEN_RANGE_INCL
	prec := p.curPrecedence()
	p.nextToken()
	right := p.parseExpression(prec)
	return &agac.RangeExpression{
		Token:     tok,
		Start:     left,
		End:       right,
		Inclusive: inclusive,
	}
}

func (p *Parser) parseGroupedOrLambdaExpression(desen bool) agac.Expression {
	defer p.allowBlocks()()
	// Önce bunun '(a, b) => ...' bir ok fonksiyonu (lambda) olup olmadığını kontrol et; eşle
	// deseninde '(a + 1) => ...' parantezli bir desendir.
	if !desen && p.isLambdaAhead() {
		return p.parseLambdaExpression()
	}

	p.nextToken()
	exp := p.parseExpression(LOWEST)
	if !p.expectPeek(sozcuk.TOKEN_RPAREN) {
		return nil
	}
	return exp
}

func (p *Parser) isLambdaAhead() bool {
	// mevcut token '('
	idx := p.pos + 1
	depth := 1
	for idx < len(p.tokens) && depth > 0 {
		if p.tokens[idx].Type == sozcuk.TOKEN_LPAREN {
			depth++
		} else if p.tokens[idx].Type == sozcuk.TOKEN_RPAREN {
			depth--
		}
		idx++
	}
	if idx < len(p.tokens) && p.tokens[idx].Type == sozcuk.TOKEN_ARROW {
		return true
	}
	return false
}

func (p *Parser) parseSingleParamLambda() agac.Expression {
	tok := p.curToken()
	param := agac.Param{Name: tok.Literal}
	p.nextToken() // '=>'
	arrowTok := p.curToken()

	var body *agac.BlockStatement
	if p.peekTokenIs(sozcuk.TOKEN_LBRACE) {
		p.nextToken()
		body = p.fonksiyonGovdesi()
	} else {
		p.nextToken()
		expr := p.fonksiyonIfadesi()
		body = &agac.BlockStatement{
			Token: arrowTok,
			Statements: []agac.Statement{
				&agac.ReturnStatement{Token: arrowTok, ReturnValue: expr},
			},
		}
	}

	return &agac.FunctionLiteral{
		Token:      tok,
		Parameters: []agac.Param{param},
		Body:       body,
	}
}

func (p *Parser) parseLambdaExpression() agac.Expression {
	tok := p.curToken() // '('
	params := p.parseFunctionParameters()
	if !p.expectPeek(sozcuk.TOKEN_ARROW) {
		return nil
	}
	arrowTok := p.curToken()

	var body *agac.BlockStatement
	if p.peekTokenIs(sozcuk.TOKEN_LBRACE) {
		p.nextToken()
		body = p.fonksiyonGovdesi()
	} else {
		p.nextToken()
		expr := p.fonksiyonIfadesi()
		body = &agac.BlockStatement{
			Token: arrowTok,
			Statements: []agac.Statement{
				&agac.ReturnStatement{Token: arrowTok, ReturnValue: expr},
			},
		}
	}

	return &agac.FunctionLiteral{
		Token:      tok,
		Parameters: params,
		Body:       body,
	}
}

func (p *Parser) parseIfExpression() agac.Expression {
	expr := &agac.IfExpression{Token: p.curToken()}

	p.nextToken()
	expr.Condition = p.parseHeaderExpression()

	if !p.expectPeek(sozcuk.TOKEN_LBRACE) {
		return nil
	}
	expr.Consequence = p.parseBlockStatement()

	for p.peekTokenIs(sozcuk.TOKEN_DEGILSE) {
		p.nextToken() // 'değilse'
		if p.peekTokenIs(sozcuk.TOKEN_EGER) {
			p.nextToken() // 'eğer'
			p.nextToken()
			cond := p.parseExpression(LOWEST)
			if !p.expectPeek(sozcuk.TOKEN_LBRACE) {
				return nil
			}
			block := p.parseBlockStatement()
			expr.ElseIfs = append(expr.ElseIfs, agac.ElseIfBranch{
				Condition:   cond,
				Consequence: block,
			})
		} else {
			if !p.expectPeek(sozcuk.TOKEN_LBRACE) {
				return nil
			}
			expr.Alternative = p.parseBlockStatement()
			break
		}
	}

	return expr
}

func (p *Parser) parseMatchExpression() agac.Expression {
	tok := p.curToken() // eşle
	p.nextToken()
	subject := p.parseHeaderExpression()

	if !p.expectPeek(sozcuk.TOKEN_LBRACE) {
		return nil
	}
	matchOpen := p.curToken()
	defer func() { p.requireClosingBrace(matchOpen) }()
	p.nextToken()

	var cases []agac.MatchCase
	for !p.curTokenIs(sozcuk.TOKEN_RBRACE) && !p.curTokenIs(sozcuk.TOKEN_EOF) {
		if p.curTokenIs(sozcuk.TOKEN_COMMA) || p.curTokenIs(sozcuk.TOKEN_SEMICOLON) {
			p.nextToken()
			continue
		}

		mc := agac.MatchCase{}
		if p.curTokenIs(sozcuk.TOKEN_VARSAYILAN) || (p.curTokenIs(sozcuk.TOKEN_IDENT) && p.curToken().Literal == "_") {
			mc.IsDefault = true
		} else {
			pat := p.parseMatchPattern()
			mc.Patterns = append(mc.Patterns, pat)
			for p.peekTokenIs(sozcuk.TOKEN_COMMA) {
				p.nextToken() // ','
				p.nextToken()
				mc.Patterns = append(mc.Patterns, p.parseMatchPattern())
			}
		}

		if !p.expectPeek(sozcuk.TOKEN_ARROW) {
			// Kalan durumlar da hatalı okunup art arda hata vermesin diye eşle bloğu atlanır
			p.blokSonunaAtla()
			return nil
		}

		if p.peekTokenIs(sozcuk.TOKEN_LBRACE) {
			p.nextToken()
			mc.Body = p.parseBlockStatement()
		} else if p.peekTokenIs(sozcuk.TOKEN_SONUC) {
			p.nextToken()
			retStmt := p.parseReturnStatement()
			mc.Body = &agac.BlockStatement{
				Token:      retStmt.Token,
				Statements: []agac.Statement{retStmt},
			}
		} else {
			p.nextToken()
			mc.ValueExpr = p.parseExpression(LOWEST)
		}

		cases = append(cases, mc)
		p.nextToken()
	}

	return &agac.MatchExpression{
		Token:   tok,
		Subject: subject,
		Cases:   cases,
	}
}

func (p *Parser) parseFunctionLiteral() agac.Expression {
	lit := &agac.FunctionLiteral{Token: p.curToken()}

	// İsimli fonksiyon: fonksiyon topla(a, b) { ... }
	if p.peekTokenIs(sozcuk.TOKEN_IDENT) {
		p.nextToken()
		lit.Name = p.curToken().Literal
	}

	if !p.expectPeek(sozcuk.TOKEN_LPAREN) {
		return nil
	}

	lit.Parameters = p.parseFunctionParameters()

	// İsteğe bağlı dönüş tipi: fonksiyon topla(a: Tam, b: Tam): Tam { ... } veya -> Tam { ... }
	if p.peekTokenIs(sozcuk.TOKEN_COLON) || p.peekTokenIs(sozcuk.TOKEN_ARROW) {
		p.nextToken()
		if p.expectPeek(sozcuk.TOKEN_IDENT) {
			lit.ReturnType = p.curToken().Literal
		}
	} else if p.peekTokenIs(sozcuk.TOKEN_MINUS) {
		p.nextToken() // '-'
		if p.peekTokenIs(sozcuk.TOKEN_GT) {
			p.nextToken() // '>'
			if p.expectPeek(sozcuk.TOKEN_IDENT) {
				lit.ReturnType = p.curToken().Literal
			}
		}
	}

	if !p.expectPeek(sozcuk.TOKEN_LBRACE) {
		return nil
	}

	lit.Body = p.fonksiyonGovdesi()
	return lit
}

func (p *Parser) parseFunctionParameters() []agac.Param {
	params := []agac.Param{}

	if p.peekTokenIs(sozcuk.TOKEN_RPAREN) {
		p.nextToken()
		return params
	}

	p.nextToken()

	for {
		param := agac.Param{Name: p.curToken().Literal}
		if p.peekTokenIs(sozcuk.TOKEN_COLON) {
			p.nextToken() // ':'
			if p.expectPeek(sozcuk.TOKEN_IDENT) {
				param.TypeName = p.curToken().Literal
			}
		}
		if p.peekTokenIs(sozcuk.TOKEN_ASSIGN) {
			p.nextToken() // '='
			p.nextToken()
			param.DefaultValue = p.parseExpression(LOWEST)
		}
		params = append(params, param)

		if !p.peekTokenIs(sozcuk.TOKEN_COMMA) {
			break
		}
		p.nextToken() // ','
		p.nextToken()
	}

	if !p.expectPeek(sozcuk.TOKEN_RPAREN) {
		return nil
	}

	return params
}

func (p *Parser) parseCallExpression(function agac.Expression) agac.Expression {
	exp := &agac.CallExpression{Token: p.curToken(), Function: function}
	exp.Arguments, exp.NamedArgs = p.parseCallArguments()
	return exp
}

func (p *Parser) parseCallArguments() ([]agac.Expression, []agac.NamedArg) {
	defer p.allowBlocks()()
	args := []agac.Expression{}
	named := []agac.NamedArg{}

	if p.peekTokenIs(sozcuk.TOKEN_RPAREN) {
		p.nextToken()
		return args, named
	}

	p.nextToken()
	for {
		if p.curTokenIs(sozcuk.TOKEN_IDENT) && p.peekTokenIs(sozcuk.TOKEN_COLON) {
			name := p.curToken().Literal
			p.nextToken() // ':'
			p.nextToken()
			val := p.parseExpression(LOWEST)
			named = append(named, agac.NamedArg{Name: name, Value: val})
		} else {
			args = append(args, p.parseExpression(LOWEST))
		}

		if !p.peekTokenIs(sozcuk.TOKEN_COMMA) {
			break
		}
		p.nextToken() // ','
		if p.peekTokenIs(sozcuk.TOKEN_RPAREN) {
			break
		}
		p.nextToken()
	}

	if !p.expectPeek(sozcuk.TOKEN_RPAREN) {
		return nil, nil
	}

	return args, named
}

func (p *Parser) parseListLiteral() agac.Expression {
	defer p.allowBlocks()()
	tok := p.curToken()
	if p.peekTokenIs(sozcuk.TOKEN_RBRACKET) {
		p.nextToken()
		return &agac.ListLiteral{Token: tok, Elements: []agac.Expression{}}
	}

	p.nextToken()
	firstExpr := p.parseExpression(LOWEST)

	// Liste üreteci kontrolü: [x * x döngü x içinde 1..=10 eğer x % 2 == 0]
	if p.peekTokenIs(sozcuk.TOKEN_DONGU) {
		p.nextToken() // 'döngü'
		if !p.expectPeek(sozcuk.TOKEN_IDENT) {
			return nil
		}
		varName := p.curToken().Literal
		if !p.expectPeek(sozcuk.TOKEN_ICINDE) {
			return nil
		}
		p.nextToken()
		iterable := p.parseExpression(LOWEST)

		var cond agac.Expression
		if p.peekTokenIs(sozcuk.TOKEN_EGER) {
			p.nextToken() // 'eğer'
			p.nextToken()
			cond = p.parseExpression(LOWEST)
		}

		if !p.expectPeek(sozcuk.TOKEN_RBRACKET) {
			return nil
		}

		return &agac.ListComprehension{
			Token:     tok,
			Output:    firstExpr,
			VarName:   varName,
			Iterable:  iterable,
			Condition: cond,
		}
	}

	elements := []agac.Expression{firstExpr}
	for p.peekTokenIs(sozcuk.TOKEN_COMMA) {
		p.nextToken()
		if p.peekTokenIs(sozcuk.TOKEN_RBRACKET) {
			break
		}
		p.nextToken()
		elements = append(elements, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(sozcuk.TOKEN_RBRACKET) {
		return nil
	}

	return &agac.ListLiteral{Token: tok, Elements: elements}
}

func (p *Parser) parseDictLiteral() agac.Expression {
	defer p.allowBlocks()()
	dict := &agac.DictLiteral{Token: p.curToken(), Pairs: []agac.DictPair{}}

	for !p.peekTokenIs(sozcuk.TOKEN_RBRACE) && !p.peekTokenIs(sozcuk.TOKEN_EOF) {
		p.nextToken()
		var key agac.Expression
		// Sözlük anahtarı çıplak tanımlayıcı ise (örn: {ad: "Ali"}), metin olarak al
		if p.curTokenIs(sozcuk.TOKEN_IDENT) && p.peekTokenIs(sozcuk.TOKEN_COLON) {
			key = &agac.StringLiteral{Token: p.curToken(), Value: p.curToken().Literal}
		} else {
			key = p.parseExpression(LOWEST)
		}

		if !p.expectPeek(sozcuk.TOKEN_COLON) {
			return nil
		}
		p.nextToken()
		value := p.parseExpression(LOWEST)
		dict.Pairs = append(dict.Pairs, agac.DictPair{Key: key, Value: value})

		if p.peekTokenIs(sozcuk.TOKEN_COMMA) {
			p.nextToken()
		}
	}

	if !p.expectPeek(sozcuk.TOKEN_RBRACE) {
		return nil
	}

	return dict
}

func (p *Parser) parseIndexExpression(left agac.Expression) agac.Expression {
	defer p.allowBlocks()()
	exp := &agac.IndexExpression{Token: p.curToken(), Left: left}
	p.nextToken()
	exp.Index = p.parseExpression(LOWEST)
	if !p.expectPeek(sozcuk.TOKEN_RBRACKET) {
		return nil
	}
	return exp
}

func (p *Parser) parseMemberExpression(left agac.Expression) agac.Expression {
	tok := p.curToken()
	safe := tok.Type == sozcuk.TOKEN_SAFE_DOT
	p.nextToken()
	prop := p.curToken().Literal
	return &agac.MemberExpression{
		Token:    tok,
		Object:   left,
		Property: prop,
		Safe:     safe,
	}
}
