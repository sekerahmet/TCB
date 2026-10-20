// Package parser is TR's parser: it builds the syntax tree (internal/lang/ast) from tokens
// (internal/lang/lexer). Statements are parsed by recursive descent and expressions by
// precedence climbing (Pratt). Errors are in the user's language, carry a line and guide the
// user; removed spellings get a hint.
//
// Layer: language core (internal/lang); imports only internal/lang packages.
package parser

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Bağlamsal sözcüklerin (test, olarak, yakala'nın varsayılan hata adı, sondaki bloğun
// parametresi, indirge bloğunun birikimi) yazımları ve 'kendi'nin adı kod listesinden gelir
// (internal/lang/codes: A101, A100, A103, A104, A105, A051).
var (
	testSpellings   = codes.Spellings(codes.A101)
	asSpellings     = codes.Spellings(codes.A100)
	selfName        = codes.Name(codes.A051)
	errName         = codes.Name(codes.A103)
	itName          = codes.Name(codes.A104)
	accName         = codes.Name(codes.A105)
	reduceSpellings = codes.Spellings(codes.Y207)
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
	lexer.TOKEN_KW_OR:        OR_PREC,
	lexer.TOKEN_AND:          AND_PREC,
	lexer.TOKEN_KW_AND:       AND_PREC,
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
	errors []error
	// l sözcük birimlerini üreten çözümleyicidir; metin içi yerleştirmeler onun alt
	// çözümleyicisiyle okunur (konumlar, kodlu kip ve kayıt paylaşılır).
	l *lexer.Lexer
	// roles açıksa (RecordRoles) ad sözcüklerinin görevleri kaydedilir (roles.go).
	roles map[[2]int]NameRole

	// noBlock: 'eğer', 'iken', 'döngü' ve 'eşle' başlıklarında '{' gövdenin başlangıcıdır,
	// sondaki blok (o-lambda) olarak okunmamalıdır. Parantez/köşeli parantez içinde yeniden açılır.
	noBlock bool

	// brackets açık '(' '[' ya da sözlük '{' sayısıdır: içlerinde satır sonu ifadeyi bitirmez
	// (2.8; Python, Go gibi). Bir blok gövdesi sayımı sıfırlar: gövdede satır sonu yine deyim
	// ayırır.
	brackets int

	// errorMode bir deyimde sözdizimi hatası kaydedildikten sonra açıktır: deyim
	// senkronizeOl ile atlanana dek yeni hata kaydedilmez (art arda gelen hatalar
	// çoğunlukla ilkinin sonucudur). hataSatiri ilk hatanın satırıdır.
	errorMode bool
	errorLine int

	// patternMode bir eşle kolunun deseni okunurken açıktır: desenin hemen ardından gelen
	// '=>' kolun okudur, ok fonksiyonu değildir (durum => ..., (a + 1) => ...). Yalnızca
	// desenin en dıştaki ifadesini etkiler; parseExpression girişte kapatır.
	patternMode bool

	// ctx 'çık'/'devam'/'sonuç' deyimlerinin geçerliliği için izlenen bağlamdır
	// (context.go); modul, kodun bir modül olarak ayrıştırıldığını söyler (ParseModule).
	ctx    stmtContext
	module bool

	// nesting içinde bulunulan ifade/blok iç içelik düzeyidir; derinlikAsildi sınır
	// aşıldıktan sonra açıktır (bkz. derinlikGir).
	nesting       int
	depthExceeded bool
}

// parseMatchPattern bir eşle kolunun desenini okur.
func (p *Parser) parseMatchPattern() ast.Expression {
	p.patternMode = true
	defer func() { p.patternMode = false }()
	return p.parseExpression(LOWEST)
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

// enterBrackets parantez içine girer; dönen fonksiyon çıkar.
func (p *Parser) enterBrackets() func() {
	p.brackets++
	return func() { p.brackets-- }
}

// enterBlockBody blok gövdesinde parantez sayımını sıfırlar; dönen fonksiyon geri yükler.
func (p *Parser) enterBlockBody() func() {
	saved := p.brackets
	p.brackets = 0
	return func() { p.brackets = saved }
}

// lineBreakEnds sıradaki belirteç yeni satırdaysa ve parantez içinde değilsek ifadenin orada
// bittiğini söyler.
func (p *Parser) lineBreakEnds() bool {
	return p.brackets == 0 && p.peekToken().Line > p.curToken().Line
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
// çağrının son argümanı yapar: sayılar.filtrele { o > 2 } == sayılar.filtrele((o) => o > 2).
// liste.indirge'nin bloğu iki parametrelidir: 'birikim' önceki sonuç, 'o' eleman
// (l.indirge(0) { birikim + o } == l.indirge(0, (birikim, o) => birikim + o)).
func (p *Parser) parseTrailingBlock(left ast.Expression) ast.Expression {
	tok := p.curToken() // '{'
	begin := p.pos
	body := p.functionBody()
	params := []ast.Param{{Name: itName}}
	if isReduceTarget(left) {
		params = []ast.Param{{Name: accName}, {Name: itName}}
		p.oldReduceBlock(tok, begin)
	}
	fn := &ast.FunctionLiteral{
		Token:      tok,
		Parameters: params,
		Body:       body,
	}
	if call, ok := left.(*ast.CallExpression); ok {
		call.Arguments = append(call.Arguments, fn)
		return call
	}
	return &ast.CallExpression{Token: tok, Function: left, Arguments: []ast.Expression{fn}}
}

// New reads every token from l and returns a parser for them; lexer errors come first in
// Errors.
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
	errs := append([]error{}, l.Errs()...)
	return &Parser{
		tokens: tokens,
		pos:    0,
		errors: errs,
		l:      l,
	}
}

// Errs returns the lexer and parser errors, each at its line and column (messages.Pos).
func (p *Parser) Errs() []error {
	return p.errors
}

// Errors returns the texts of Errs ("Satır N, Sütun M: ...").
func (p *Parser) Errors() []string {
	out := make([]string, len(p.errors))
	for i, e := range p.errors {
		out[i] = e.Error()
	}
	return out
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
	p.syntaxError(tok, messages.H0140, typeDescription(t), tokenDescription(tok), unexpectedHint(t, tok))
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

// ParseProgram parses the whole input as a program.
func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	program.Statements = []ast.Statement{}

	for !p.curTokenIs(lexer.TOKEN_EOF) {
		if p.curTokenIs(lexer.TOKEN_SEMICOLON) {
			p.nextToken()
			continue
		}
		stmtStart := p.pos
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.synchronize(stmtStart, true)
		p.nextToken()
	}
	p.removedSpellingHints()
	return program
}

// removedSpellingHints sözdizimi hatası olan satırlarda kaldırılmış bir anahtar kelime
// (ör. 'döndür', 'kır') ad olarak geçiyorsa, eski alışkanlığa yol gösteren bir ipucu ekler.
func (p *Parser) removedSpellingHints() {
	if len(p.errors) == 0 {
		return
	}
	badLine := map[int]bool{}
	for _, e := range p.errors {
		if line, _, ok := messages.Pos(e); ok {
			badLine[line] = true
		}
	}
	for _, t := range p.tokens {
		if t.Type != lexer.TOKEN_IDENT || !badLine[t.Line] {
			continue
		}
		if i := hint.KeywordSpellingHint(t.Literal); i != "" {
			p.errors = append(p.errors, messages.At(t.Line, t.Column, messages.New(messages.H0164, i)))
		}
	}
}
