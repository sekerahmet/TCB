package parser

import "github.com/sekerahmet/TCB/internal/lang/lexer"

// AdRolu bir ad sözcüğünün sözdizimindeki görevidir. Sözdizimi ağacında adı ayrı bir düğüm
// olmayan yerler (tip açıklamaları, üye ve yapı alanı adları, bağlamsal sözcükler) için
// tutulur: kodlu biçime çevirici (internal/lang/source) hangi sözcüğün dilin bir öğesi
// olduğunu buradan ve denetleyicinin kapsam bilgisinden (denetci.AdBaglari) çıkarır (17.3).
type AdRolu uint8

const (
	RolYok            AdRolu = iota
	RolTip                   // tip açıklaması: değişken x: Tam, fonksiyon f(a: Metin): Tam
	RolUye                   // üye adı: x.ad, x?.ad
	RolYapiAlani             // yapı alanının tanımı
	RolYapiMetodu            // yapı metodunun tanımı (fonksiyon ad(...) yapı gövdesinde)
	RolTest                  // test deyiminin bağlamsal sözcüğü (A101)
	RolOlarak                // aktar ... olarak (A100)
	RolSozlukAnahtari        // sözlük değişmezinin çıplak ad anahtarı: {ad: değer}
)

// RolleriKaydet ayrıştırma sırasında ad sözcüklerinin görevlerinin kaydedilmesini açar
// (ParseProgram'dan önce çağrılır). Sıradan ayrıştırmada kapalıdır.
func (p *Parser) RolleriKaydet() {
	if p.roller == nil {
		p.roller = map[[2]int]AdRolu{}
	}
}

// Roller kaydedilen görevlerdir: sözcüğün satır ve sütunu → görev. Metin içi
// yerleştirmelerdeki sözcükler de (dosyadaki konumlarıyla) buradadır.
func (p *Parser) Roller() map[[2]int]AdRolu { return p.roller }

// rol kayıt açıksa bir ad sözcüğünün görevini kaydeder.
func (p *Parser) rol(tok lexer.Token, r AdRolu) {
	if p.roller != nil && tok.Type == lexer.TOKEN_IDENT {
		p.roller[[2]int{tok.Line, tok.Column}] = r
	}
}
