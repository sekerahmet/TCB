// Package ayristir TR'nin ayrıştırıcısıdır: sözcük birimlerinden (internal/dil/sozcuk)
// sözdizimi ağacını (internal/dil/agac) kurar. Deyimler özyinelemeli inişle, ifadeler
// öncelik tabanlı (Pratt) yöntemle çözümlenir. Hatalar Türkçe, satırlı ve yol
// göstericidir; kaldırılmış yazımlar için ipucu verilir.
//
// Katman: dil çekirdeği (internal/dil); yalnızca internal/dil paketlerini içe aktarır.
package ayristir

import (
	"fmt"
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/ipucu"
	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/sozcuk"
)

// Bağlamsal sözcüklerin (test, olarak, yakala'nın varsayılan hata adı, sondaki bloğun
// parametresi, indirge bloğunun birikimi) yazımları ve 'kendi'nin adı kod listesinden gelir
// (internal/dil/kod: A101, A100, A103, A104, A105, A051).
var (
	testYazimlari    = kod.Yazimlar(kod.A101)
	olarakYazimlari  = kod.Yazimlar(kod.A100)
	kendiAdi         = kod.Ad(kod.A051)
	hataAdi          = kod.Ad(kod.A103)
	oAdi             = kod.Ad(kod.A104)
	birikimAdi       = kod.Ad(kod.A105)
	indirgeYazimlari = kod.Yazimlar(kod.Y207)
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
	// l sözcük birimlerini üreten çözümleyicidir; metin içi yerleştirmeler onun alt
	// çözümleyicisiyle okunur (konumlar, kodlu kip ve kayıt paylaşılır).
	l *sozcuk.Lexer
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
// çağrının son argümanı yapar: sayılar.filtrele { o > 2 } == sayılar.filtrele((o) => o > 2).
// liste.indirge'nin bloğu iki parametrelidir: 'birikim' önceki sonuç, 'o' eleman
// (l.indirge(0) { birikim + o } == l.indirge(0, (birikim, o) => birikim + o)).
func (p *Parser) parseTrailingBlock(left agac.Expression) agac.Expression {
	tok := p.curToken() // '{'
	bas := p.pos
	body := p.fonksiyonGovdesi()
	params := []agac.Param{{Name: oAdi}}
	if indirgeHedefi(left) {
		params = []agac.Param{{Name: birikimAdi}, {Name: oAdi}}
		p.eskiIndirgeBlogu(tok, bas)
	}
	fn := &agac.FunctionLiteral{
		Token:      tok,
		Parameters: params,
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
		l:      l,
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
		if i := ipucu.KaldirilanYazimIpucu(t.Literal); i != "" {
			p.errors = append(p.errors, fmt.Sprintf("Satır %d, Sütun %d: ipucu%s", t.Line, t.Column, i))
		}
	}
}
