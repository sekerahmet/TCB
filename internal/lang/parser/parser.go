// Package parser is TR's parser: it builds the syntax tree (internal/lang/ast) from tokens
// (internal/lang/lexer). Statements are parsed by recursive descent and expressions by
// precedence climbing (Pratt). Errors are in the user's language, carry a line and guide the
// user; removed spellings get a hint.
//
// Layer: language core (internal/lang); imports only internal/lang packages.
package parser

import (
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// contextWords bağlamsal sözcüklerin (test, olarak, yakala'nın varsayılan hata adı, sondaki
// bloğun parametresi, indirge bloğunun birikimi) yazımları ve 'kendi'nin adıdır; kod
// listesinden, sözcük çözümleyicinin okuduğu dilde gelir (internal/lang/codes: TR_ctx_test, TR_ctx_as,
// TR_ctx_error, TR_ctx_it, TR_ctx_acc, TR_kw_self, TR_mem_reduce). Programlar kimliklerle okunur (codes.Active); bir yazım
// dilindeki metin kendi dilinde okunur (lexer.NewBodyIn).
type contextWords struct {
	test, as, reduce   []string
	self, err, it, acc string
}

// contextWordsByLanguage dil kodu → o dilin bağlamsal sözcükleri (bir kez kurulur).
var contextWordsByLanguage sync.Map

func contextWordsOf(lang *codes.Language) *contextWords {
	if w, ok := contextWordsByLanguage.Load(lang.Code); ok {
		return w.(*contextWords)
	}
	w := &contextWords{
		test: lang.Spellings(codes.CtxTest), as: lang.Spellings(codes.CtxAs), reduce: lang.Spellings(codes.MemReduce),
		self: lang.Name(codes.KwSelf), err: lang.Name(codes.CtxError), it: lang.Name(codes.CtxIt), acc: lang.Name(codes.CtxAcc),
	}
	v, _ := contextWordsByLanguage.LoadOrStore(lang.Code, w)
	return v.(*contextWords)
}

// Öncelik, gevşekten sıkıya (4.1); düzeyler dil tanımının tablosundaki sırayla.
const (
	_ int = iota
	precLowest
	precPipe        // |>
	precOr          // TR_or (||)
	precAnd         // TR_and (&&)
	precNot         // TR_not X: karşılaştırmadan gevşek, TR_and'den sıkı (bkz. parsePrefixExpression)
	precCompare     // == != < > <= >= (tek düzey; zincir 4.4, bkz. parseComparison)
	precNullCoalesc // ?? (sağında parantezsiz ikili işleç yazılmaz, 4.7)
	precRange       // .. ..=
	precBitOr       // |
	precBitXor      // ^
	precBitAnd      // &
	precShift       // << >>
	precSum         // + -
	precProduct     // * / // % @
	precPrefix      // -X ~X !X
	precPower       // **  (sağdan birleşir ve tekli eksiden sıkı bağlanır: -2 ** 2 = -4)
	precCall        // fn(X) dizi[i] nesne.alan nesne?.alan
)

var precedences = map[lexer.TokenType]int{
	lexer.TokPipe:        precPipe,
	lexer.TokNullCoalesc: precNullCoalesc,
	lexer.TokOr:          precOr,
	lexer.TokKwOr:        precOr,
	lexer.TokAnd:         precAnd,
	lexer.TokKwAnd:       precAnd,
	lexer.TokEq:          precCompare,
	lexer.TokNotEq:       precCompare,
	lexer.TokLt:          precCompare,
	lexer.TokGt:          precCompare,
	lexer.TokLte:         precCompare,
	lexer.TokGte:         precCompare,
	lexer.TokRange:       precRange,
	lexer.TokRangeIncl:   precRange,
	lexer.TokBitOr:       precBitOr,
	lexer.TokBitXor:      precBitXor,
	lexer.TokBitAnd:      precBitAnd,
	lexer.TokShl:         precShift,
	lexer.TokShr:         precShift,
	lexer.TokPlus:        precSum,
	lexer.TokMinus:       precSum,
	lexer.TokSlash:       precProduct,
	lexer.TokSlashSlash:  precProduct,
	lexer.TokAsterisk:    precProduct,
	lexer.TokPercent:     precProduct,
	lexer.TokMatmul:      precProduct,
	lexer.TokPower:       precPower,
	lexer.TokLParen:      precCall,
	lexer.TokLBracket:    precCall,
	lexer.TokDot:         precCall,
	lexer.TokSafeDot:     precCall,
	lexer.TokArrow:       precPipe,
	lexer.TokLBrace:      precCall, // sondaki blok: liste.filtrele { o > 2 }
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
	// words çözümleyicinin okuduğu dilin bağlamsal sözcükleridir.
	words *contextWords

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
	// importsOpen, okunan deyim listesinde aktarma yazılabildiğini söyler: dosyanın en dışında ve
	// başka bir deyimden önce (parseStatementList).
	importsOpen bool

	// nesting içinde bulunulan ifade/blok iç içelik düzeyidir; derinlikAsildi sınır
	// aşıldıktan sonra açıktır (bkz. derinlikGir).
	nesting       int
	depthExceeded bool
}

// parseMatchPattern bir eşle kolunun desenini okur.
func (p *Parser) parseMatchPattern() ast.Expression {
	p.patternMode = true
	defer func() { p.patternMode = false }()
	return p.parseExpression(precLowest)
}

// parseHeaderExpression bir kontrol yapısının başlık ifadesini (koşul, döngü kaynağı,
// eşle konusu) sondaki blokları kapatarak ayrıştırır.
func (p *Parser) parseHeaderExpression() ast.Expression {
	saved := p.noBlock
	p.noBlock = true
	defer func() { p.noBlock = saved }()
	return p.parseExpression(precLowest)
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
	body := p.functionBody()
	params := []ast.Param{{Name: p.words.it}}
	if p.isReduceTarget(left) {
		params = []ast.Param{{Name: p.words.acc}, {Name: p.words.it}}
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
		if tok.Type == lexer.TokEOF {
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
		words:  contextWordsOf(l.Language()),
	}
}

// Errs returns the lexer and parser errors, each at its line and column (messages.Pos).
func (p *Parser) Errs() []error {
	return p.errors
}

// Errors returns the texts of Errs ("Satır N, Sütun M: ...").
func (p *Parser) Errors() []string {
	return messages.Texts(p.errors)
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
	p.syntaxError(tok, messages.H0140, p.typeDescription(t), p.tokenDescription(tok), unexpectedHint(t, tok))
}

func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken().Type]; ok {
		return p
	}
	return precLowest
}

func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken().Type]; ok {
		return p
	}
	return precLowest
}

// ParseProgram parses the whole input as a program.
func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{Statements: p.parseStatementList(true)}
	p.removedSpellingHints()
	return program
}

// parseStatementList deyimleri dosya sonuna (topLevel) ya da bloğun '}' ya da dosya sonuna kadar
// okur; her deyimden sonra hata kurtarma noktasına eşitlenir. Aktarmalar yalnız dosyanın en
// dışında, ilk başka deyimden önce açıktır (importsOpen, 1.3).
func (p *Parser) parseStatementList(topLevel bool) []ast.Statement {
	saved := p.importsOpen
	defer func() { p.importsOpen = saved }()
	p.importsOpen = topLevel
	stmts := []ast.Statement{}
	for !p.curTokenIs(lexer.TokEOF) && (topLevel || !p.curTokenIs(lexer.TokRBrace)) {
		if p.curTokenIs(lexer.TokSemicolon) {
			p.nextToken()
			continue
		}
		stmtStart := p.pos
		if !p.curTokenIs(lexer.TokImport) {
			p.importsOpen = false
		}
		stmt := p.parseStatement()
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
		p.synchronize(stmtStart, topLevel)
		p.nextToken()
	}
	return stmts
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
		if t.Type != lexer.TokIdent || !badLine[t.Line] {
			continue
		}
		if i := hint.KeywordSpellingHint(t.Literal, nil); i != nil {
			p.errors = append(p.errors, messages.At(t.Line, t.Column, messages.New(messages.H0164, i)))
		}
	}
}
