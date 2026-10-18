package ayristir

import (
	"fmt"
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/sozcuk"
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
func (p *Parser) ParseModule() *agac.Program {
	p.modul = true
	return p.ParseProgram()
}

// donguGovdesi bir döngünün gövdesini ayrıştırır.
func (p *Parser) donguGovdesi() *agac.BlockStatement {
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
func (p *Parser) fonksiyonGovdesi() (b *agac.BlockStatement) {
	p.sinirIcinde(true, false, func() { b = p.parseBlockStatement() })
	return b
}

// fonksiyonIfadesi tek ifadeli ok fonksiyonunun gövdesini fonksiyon bağlamında ayrıştırır.
func (p *Parser) fonksiyonIfadesi() (e agac.Expression) {
	p.sinirIcinde(true, false, func() { e = p.parseExpression(LOWEST) })
	return e
}

// sinyalDenetle 'çık'/'devam' deyiminin bir döngü içinde olduğunu denetler.
func (p *Parser) sinyalDenetle(tok sozcuk.Token) {
	if p.baglam.dongu > 0 {
		return
	}
	ek := ""
	if p.baglam.fonk > 0 {
		ek = "; fonksiyondan çıkmak için '" + kod.Ad(kod.A021) + "' yazın"
	}
	p.hataKaydet(tok, "Sözdizimi hatası — '"+tok.Literal+"' yalnızca bir döngünün içinde kullanılabilir"+ek)
}

// sonucDenetle 'sonuç' deyiminin bulunduğu yerde geçerli olduğunu denetler.
func (p *Parser) sonucDenetle(tok sozcuk.Token) {
	if p.baglam.fonk > 0 {
		return
	}
	switch {
	case p.baglam.test:
		p.hataKaydet(tok, "Sözdizimi hatası — '"+tok.Literal+"' test gövdesinde kullanılamaz; test kendi sonuna kadar çalışır (erken bitirmek istediğiniz kısmı bir '"+kod.Ad(kod.A001)+"' bloğuna alın)")
	case p.modul:
		p.hataKaydet(tok, "Sözdizimi hatası — '"+tok.Literal+"' modülün üst düzeyinde kullanılamaz; modül bir fonksiyon değildir, aktaran programı bitiremez")
	}
}

// enFazlaDerinlik ifade ve blokların en fazla iç içelik düzeyidir. Sınırsız iç içelik Go
// yığınını taşırıp süreci yakalanamaz biçimde çökertirdi; sınırda Türkçe bir sözdizimi
// hatası verilir ve dosyanın geri kalanı hata üretmeden geçilir.
const enFazlaDerinlik = 1000

// derinlikGir bir iç içelik düzeyine girer; sınır aşıldıysa hatayı kaydeder ve yanlış döner
// (çağıran hiçbir şey okumadan döner). Her doğru dönüşün ardından derinlikCik çağrılmalıdır.
func (p *Parser) derinlikGir() bool {
	if p.derinlikAsildi {
		return false
	}
	if p.derinlik >= enFazlaDerinlik {
		p.hataModu = false
		p.hataKaydet(p.curToken(), fmt.Sprintf("Sözdizimi hatası — ifade ya da blok çok derin iç içe (en fazla %d düzey); kodu ara değişkenlere ya da fonksiyonlara bölün", enFazlaDerinlik))
		p.derinlikAsildi = true
		return false
	}
	p.derinlik++
	return true
}

func (p *Parser) derinlikCik() { p.derinlik-- }
