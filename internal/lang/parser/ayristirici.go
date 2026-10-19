// Package parser TR'nin ayrıştırıcısıdır: sözcük birimlerinden (internal/lang/lexer)
// sözdizimi ağacını (internal/lang/ast) kurar. Deyimler özyinelemeli inişle, ifadeler
// öncelik tabanlı (Pratt) yöntemle çözümlenir. Hatalar Türkçe, satırlı ve yol
// göstericidir; kaldırılmış yazımlar için ipucu verilir.
//
// Katman: dil çekirdeği (internal/lang); yalnızca internal/lang paketlerini içe aktarır.
package parser

import (
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

// Bağlamsal sözcüklerin (test, olarak, yakala'nın varsayılan hata adı, sondaki bloğun
// parametresi, indirge bloğunun birikimi) yazımları ve 'kendi'nin adı kod listesinden gelir
// (internal/lang/codes: A101, A100, A103, A104, A105, A051).
var (
	testYazimlari    = codes.Yazimlar(codes.A101)
	olarakYazimlari  = codes.Yazimlar(codes.A100)
	kendiAdi         = codes.Ad(codes.A051)
	hataAdi          = codes.Ad(codes.A103)
	oAdi             = codes.Ad(codes.A104)
	birikimAdi       = codes.Ad(codes.A105)
	indirgeYazimlari = codes.Yazimlar(codes.Y207)
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
	// l sözcük birimlerini üreten çözümleyicidir; metin içi yerleştirmeler onun alt
	// çözümleyicisiyle okunur (konumlar, kodlu kip ve kayıt paylaşılır).
	l *lexer.Lexer
	// roller açıksa (RolleriKaydet) ad sözcüklerinin görevleri kaydedilir (roller.go).
	roller map[[2]int]AdRolu

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
func (p *Parser) parseMatchPattern() ast.Expression {
	p.desenKipi = true
	defer func() { p.desenKipi = false }()
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
	bas := p.pos
	body := p.fonksiyonGovdesi()
	params := []ast.Param{{Name: oAdi}}
	if indirgeHedefi(left) {
		params = []ast.Param{{Name: birikimAdi}, {Name: oAdi}}
		p.eskiIndirgeBlogu(tok, bas)
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
		l:      l,
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

func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	program.Statements = []ast.Statement{}

	for !p.curTokenIs(lexer.TOKEN_EOF) {
		if p.curTokenIs(lexer.TOKEN_SEMICOLON) {
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
		if t.Type != lexer.TOKEN_IDENT || !hataliSatir[t.Line] {
			continue
		}
		if i := hint.KaldirilanYazimIpucu(t.Literal); i != "" {
			p.errors = append(p.errors, fmt.Sprintf("Satır %d, Sütun %d: ipucu%s", t.Line, t.Column, i))
		}
	}
}
