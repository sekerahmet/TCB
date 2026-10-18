package parser

import (
	"tr_dili/pkg/ast"
	"tr_dili/pkg/lexer"
)

// Deyim bağlamı: 'çık'/'devam' yalnızca bir döngünün, 'sonuç' (test gövdesinde ve modülün
// üst düzeyinde) yalnızca bir fonksiyonun içinde geçerlidir. Bağlam ayrıştırırken izlenir;
// fonksiyon, ok fonksiyonu, sondaki blok, test, ertele ve eşzamanlı gövdeleri sınırdır:
// içlerinden dıştaki döngü görünmez.
type deyimBaglami struct {
	dongu int  // içinde bulunulan döngü sayısı (en yakın sınırdan bu yana)
	fonk  int  // içinde bulunulan fonksiyon sayısı
	test  bool // bir test gövdesindeyiz (fonksiyon dışı)
}

// ParseModule kodu bir modül olarak ayrıştırır ('aktar' ile alınan dosya): ParseProgram ile
// aynıdır, ek olarak modülün üst düzeyindeki 'sonuç' derleme hatasıdır (modül bir fonksiyon
// değildir; çalışırken aktaran programı sessizce bitirirdi).
func (p *Parser) ParseModule() *ast.Program {
	p.modul = true
	return p.ParseProgram()
}

// donguGovdesi bir döngünün gövdesini ayrıştırır.
func (p *Parser) donguGovdesi() *ast.BlockStatement {
	p.baglam.dongu++
	defer func() { p.baglam.dongu-- }()
	return p.parseBlockStatement()
}

// sinirIcinde fn'i yeni bir sınırda (döngüsüz bağlamda) ayrıştırır. fonksiyon, sınırın bir
// fonksiyon gövdesi olduğunu söyler; test, bir test gövdesi olduğunu.
func (p *Parser) sinirIcinde(fonksiyon, test bool, fn func()) {
	onceki := p.baglam
	p.baglam.dongu = 0
	if fonksiyon {
		p.baglam.fonk++
		p.baglam.test = false
	}
	if test {
		p.baglam = deyimBaglami{test: true}
	}
	defer func() { p.baglam = onceki }()
	fn()
}

// fonksiyonGovdesi bir fonksiyon (ok fonksiyonu, sondaki blok) gövdesini ayrıştırır.
func (p *Parser) fonksiyonGovdesi() (b *ast.BlockStatement) {
	p.sinirIcinde(true, false, func() { b = p.parseBlockStatement() })
	return b
}

// fonksiyonIfadesi tek ifadeli ok fonksiyonunun gövdesini fonksiyon bağlamında ayrıştırır.
func (p *Parser) fonksiyonIfadesi() (e ast.Expression) {
	p.sinirIcinde(true, false, func() { e = p.parseExpression(LOWEST) })
	return e
}

// sinyalDenetle 'çık'/'devam' deyiminin bir döngü içinde olduğunu denetler.
func (p *Parser) sinyalDenetle(tok lexer.Token) {
	if p.baglam.dongu > 0 {
		return
	}
	ek := ""
	if p.baglam.fonk > 0 {
		ek = "; fonksiyondan çıkmak için 'sonuç' yazın"
	}
	p.hataKaydet(tok, "Sözdizimi hatası — '"+tok.Literal+"' yalnızca bir döngünün içinde kullanılabilir"+ek)
}

// sonucDenetle 'sonuç' deyiminin bulunduğu yerde geçerli olduğunu denetler.
func (p *Parser) sonucDenetle(tok lexer.Token) {
	if p.baglam.fonk > 0 {
		return
	}
	switch {
	case p.baglam.test:
		p.hataKaydet(tok, "Sözdizimi hatası — '"+tok.Literal+"' test gövdesinde kullanılamaz; test kendi sonuna kadar çalışır (erken bitirmek istediğiniz kısmı bir 'eğer' bloğuna alın)")
	case p.modul:
		p.hataKaydet(tok, "Sözdizimi hatası — '"+tok.Literal+"' modülün üst düzeyinde kullanılamaz; modül bir fonksiyon değildir, aktaran programı bitiremez")
	}
}
