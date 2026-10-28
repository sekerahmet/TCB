package parser

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Deyim bağlamı: 'çık'/'devam' yalnızca bir döngünün, 'sonuç' (test gövdesinde, modülün ve
// programın üst düzeyinde) yalnızca bir fonksiyonun içinde geçerlidir. Bağlam ayrıştırırken izlenir;
// fonksiyon, ok fonksiyonu, sondaki blok, test, ertele ve eşzamanlı gövdeleri sınırdır:
// içlerinden dıştaki döngü görünmez.
type stmtContext struct {
	loops int  // içinde bulunulan döngü sayısı (en yakın sınırdan bu yana)
	funcs int  // içinde bulunulan fonksiyon sayısı
	test  bool // bir test gövdesindeyiz (fonksiyon dışı)
	block bool // bir sondaki bloğun gövdesindeyiz (iç içe fonksiyon dışı; 6.7)
}

// ParseModule parses the code as a module (a file brought in with 'aktar'): the same as
// ParseProgram, except that 'sonuç' at the module's top level is a compile error (a module
// is not a function; at run time it would silently end the importing program).
func (p *Parser) ParseModule() *ast.Program {
	p.module = true
	return p.ParseProgram()
}

// loopBody bir döngünün gövdesini ayrıştırır.
func (p *Parser) loopBody() *ast.BlockStatement {
	p.ctx.loops++
	defer func() { p.ctx.loops-- }()
	b := p.parseBlockStatement()
	p.headerBlockCheck()
	return b
}

// headerBlockCheck kontrol yapısının gövdesinden hemen sonra aynı satırda bir '{' daha geliyorsa başlıkta
// sondaki bloklu bir çağrı yazılmıştır (6.7): başlıkta '{' her zaman gövdeyi başlatır, ilk '{ ... }' gövde
// okundu. İleti çağrıyı paranteze almayı önerir.
func (p *Parser) headerBlockCheck() {
	if p.peekTokenIs(lexer.TokLBrace) && p.peekToken().Line == p.curToken().Line {
		p.syntaxError(p.peekToken(), messages.H0265)
	}
}

// withinLimit fn'i yeni bir sınırda (döngüsüz bağlamda) ayrıştırır. fonksiyon, sınırın bir
// fonksiyon gövdesi olduğunu söyler; test, bir test gövdesi olduğunu.
func (p *Parser) withinLimit(fnExpr, test bool, fn func()) {
	prev := p.ctx
	p.ctx.loops = 0
	if fnExpr {
		p.ctx.funcs++
		p.ctx.test = false
		p.ctx.block = false
	}
	if test {
		p.ctx = stmtContext{test: true}
	}
	defer func() { p.ctx = prev }()
	fn()
}

// functionBody bir fonksiyon (adlı fonksiyon, ok fonksiyonu) gövdesini ayrıştırır.
func (p *Parser) functionBody() (b *ast.BlockStatement) {
	p.withinLimit(true, false, func() { b = p.parseBlockStatement() })
	return b
}

// trailingBody sondaki bloğun gövdesini ayrıştırır: bir sınırdır (dıştaki döngü görünmez) ve içinde
// TR_return yoktur (6.7).
func (p *Parser) trailingBody() (b *ast.BlockStatement) {
	p.withinLimit(true, false, func() {
		p.ctx.block = true
		b = p.parseBlockStatement()
	})
	return b
}

// functionExpr tek ifadeli ok fonksiyonunun gövdesini fonksiyon bağlamında ayrıştırır.
func (p *Parser) functionExpr() (e ast.Expression) {
	p.withinLimit(true, false, func() { e = p.parseExpression(precLowest) })
	return e
}

// checkSignal 'çık'/'devam' deyiminin bir döngü içinde olduğunu denetler.
func (p *Parser) checkSignal(tok lexer.Token) {
	if p.ctx.loops > 0 {
		return
	}
	if p.ctx.block {
		// Sondaki blok bir döngü de fonksiyon da değildir: blok son ifadesiyle biter (6.7).
		p.syntaxError(tok, messages.H0264, p.tokenName(tok), p.word(codes.KwIf), p.word(codes.KwElse))
		return
	}
	var exit any = "" // boşsa iletide öneri yer almaz
	if p.ctx.funcs > 0 {
		exit = p.word(codes.KwReturn)
	}
	p.syntaxError(tok, messages.H0151, p.tokenName(tok), exit)
}

// checkReturn 'sonuç' deyiminin bulunduğu yerde geçerli olduğunu denetler.
func (p *Parser) checkReturn(tok lexer.Token) {
	if p.ctx.block {
		// Sondaki blokta TR_return yoktur (6.7): Python'un with içinden dönme alışkanlığı sessizce
		// yanlış yere dönmesin; blok son ifadesiyle biter.
		p.syntaxError(tok, messages.H0263, p.tokenName(tok), p.word(codes.KwIf), p.word(codes.KwElse))
		return
	}
	if p.ctx.funcs > 0 {
		return
	}
	switch {
	case p.ctx.test:
		p.syntaxError(tok, messages.H0153, p.tokenName(tok), p.word(codes.KwIf))
	case p.module:
		p.syntaxError(tok, messages.H0154, p.tokenName(tok))
	default:
		// Programın en dışında 'sonuç' yazılmaz: program TR_exit ile biter (1.10)
		p.syntaxError(tok, messages.H0171, p.tokenName(tok), codes.NameRef(codes.FnExit))
	}
}

// maxDepth ifade ve blokların en fazla iç içelik düzeyidir. Sınırsız iç içelik Go
// yığınını taşırıp süreci yakalanamaz biçimde çökertirdi; sınırda Türkçe bir sözdizimi
// hatası verilir ve dosyanın geri kalanı hata üretmeden geçilir.
const maxDepth = 1000

// enterDepth bir iç içelik düzeyine girer; sınır aşıldıysa hatayı kaydeder ve yanlış döner
// (çağıran hiçbir şey okumadan döner). Her doğru dönüşün ardından derinlikCik çağrılmalıdır.
func (p *Parser) enterDepth() bool {
	if p.depthExceeded {
		return false
	}
	if p.nesting >= maxDepth {
		p.errorMode = false
		p.syntaxError(p.curToken(), messages.H0155, maxDepth)
		p.depthExceeded = true
		return false
	}
	p.nesting++
	return true
}

func (p *Parser) leaveDepth() { p.nesting-- }
