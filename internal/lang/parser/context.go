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
	return p.parseBlockStatement()
}

// withinLimit fn'i yeni bir sınırda (döngüsüz bağlamda) ayrıştırır. fonksiyon, sınırın bir
// fonksiyon gövdesi olduğunu söyler; test, bir test gövdesi olduğunu.
func (p *Parser) withinLimit(fnExpr, test bool, fn func()) {
	prev := p.ctx
	p.ctx.loops = 0
	if fnExpr {
		p.ctx.funcs++
		p.ctx.test = false
	}
	if test {
		p.ctx = stmtContext{test: true}
	}
	defer func() { p.ctx = prev }()
	fn()
}

// functionBody bir fonksiyon (ok fonksiyonu, sondaki blok) gövdesini ayrıştırır.
func (p *Parser) functionBody() (b *ast.BlockStatement) {
	p.withinLimit(true, false, func() { b = p.parseBlockStatement() })
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
	var exit any = "" // boşsa iletide öneri yer almaz
	if p.ctx.funcs > 0 {
		exit = codes.NameRef(codes.KwReturn)
	}
	p.syntaxError(tok, messages.H0151, tokenName(tok), exit)
}

// checkReturn 'sonuç' deyiminin bulunduğu yerde geçerli olduğunu denetler.
func (p *Parser) checkReturn(tok lexer.Token) {
	if p.ctx.funcs > 0 {
		return
	}
	switch {
	case p.ctx.test:
		p.syntaxError(tok, messages.H0153, tokenName(tok), codes.NameRef(codes.KwIf))
	case p.module:
		p.syntaxError(tok, messages.H0154, tokenName(tok))
	default:
		// Programın en dışında 'sonuç' yazılmaz: program TR_exit ile biter (1.10)
		p.syntaxError(tok, messages.H0171, tokenName(tok), codes.NameRef(codes.FnExit))
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
