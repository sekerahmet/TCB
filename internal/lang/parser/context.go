package parser

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

// Deyim bağlamı: 'çık'/'devam' yalnızca bir döngünün, 'sonuç' (test gövdesinde ve modülün
// üst düzeyinde) yalnızca bir fonksiyonun içinde geçerlidir. Bağlam ayrıştırırken izlenir;
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

// donguGovdesi bir döngünün gövdesini ayrıştırır.
func (p *Parser) loopBody() *ast.BlockStatement {
	p.ctx.loops++
	defer func() { p.ctx.loops-- }()
	return p.parseBlockStatement()
}

// sinirIcinde fn'i yeni bir sınırda (döngüsüz bağlamda) ayrıştırır. fonksiyon, sınırın bir
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

// fonksiyonGovdesi bir fonksiyon (ok fonksiyonu, sondaki blok) gövdesini ayrıştırır.
func (p *Parser) functionBody() (b *ast.BlockStatement) {
	p.withinLimit(true, false, func() { b = p.parseBlockStatement() })
	return b
}

// fonksiyonIfadesi tek ifadeli ok fonksiyonunun gövdesini fonksiyon bağlamında ayrıştırır.
func (p *Parser) functionExpr() (e ast.Expression) {
	p.withinLimit(true, false, func() { e = p.parseExpression(LOWEST) })
	return e
}

// sinyalDenetle 'çık'/'devam' deyiminin bir döngü içinde olduğunu denetler.
func (p *Parser) checkSignal(tok lexer.Token) {
	if p.ctx.loops > 0 {
		return
	}
	extra := ""
	if p.ctx.funcs > 0 {
		extra = "; fonksiyondan çıkmak için '" + codes.Name(codes.A021) + "' yazın"
	}
	p.recordError(tok, "Sözdizimi hatası — '"+tok.Literal+"' yalnızca bir döngünün içinde kullanılabilir"+extra)
}

// sonucDenetle 'sonuç' deyiminin bulunduğu yerde geçerli olduğunu denetler.
func (p *Parser) checkReturn(tok lexer.Token) {
	if p.ctx.funcs > 0 {
		return
	}
	switch {
	case p.ctx.test:
		p.recordError(tok, "Sözdizimi hatası — '"+tok.Literal+"' test gövdesinde kullanılamaz; test kendi sonuna kadar çalışır (erken bitirmek istediğiniz kısmı bir '"+codes.Name(codes.A001)+"' bloğuna alın)")
	case p.module:
		p.recordError(tok, "Sözdizimi hatası — '"+tok.Literal+"' modülün üst düzeyinde kullanılamaz; modül bir fonksiyon değildir, aktaran programı bitiremez")
	}
}

// enFazlaDerinlik ifade ve blokların en fazla iç içelik düzeyidir. Sınırsız iç içelik Go
// yığınını taşırıp süreci yakalanamaz biçimde çökertirdi; sınırda Türkçe bir sözdizimi
// hatası verilir ve dosyanın geri kalanı hata üretmeden geçilir.
const maxDepth = 1000

// derinlikGir bir iç içelik düzeyine girer; sınır aşıldıysa hatayı kaydeder ve yanlış döner
// (çağıran hiçbir şey okumadan döner). Her doğru dönüşün ardından derinlikCik çağrılmalıdır.
func (p *Parser) enterDepth() bool {
	if p.depthExceeded {
		return false
	}
	if p.nesting >= maxDepth {
		p.errorMode = false
		p.recordError(p.curToken(), fmt.Sprintf("Sözdizimi hatası — ifade ya da blok çok derin iç içe (en fazla %d düzey); kodu ara değişkenlere ya da fonksiyonlara bölün", maxDepth))
		p.depthExceeded = true
		return false
	}
	p.nesting++
	return true
}

func (p *Parser) leaveDepth() { p.nesting-- }
