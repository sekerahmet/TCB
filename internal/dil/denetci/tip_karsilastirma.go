package denetci

import (
	"fmt"

	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/ipucu"
	"tr_dili/internal/dil/kod"
)

// Tip değeri metinle karşılaştırılmaz (docs/dil/03_tipler.md 3.11): tip(x) bir tip değeri
// verir, tip adları tırnaksız yazılır (tip(x) == Tam). tip(x) == "Tam" çalışma anında da
// hatadır; denetleyici, yerleşik tip() çağrısının bir metin sabitiyle karşılaştırıldığını
// çalıştırmadan önce bulur.

// tipMetinDenetle e, yerleşik tip()'in sonucunu bir metin sabitiyle (== ya da !=)
// karşılaştırıyorsa hata ekler.
func (c *Checker) tipMetinDenetle(e *agac.InfixExpression) {
	if e.Operator != "==" && e.Operator != "!=" {
		return
	}
	metin, ok := e.Right.(*agac.StringLiteral)
	cagri := e.Left
	if !ok {
		metin, ok = e.Left.(*agac.StringLiteral)
		cagri = e.Right
	}
	if !ok || !c.tipCagrisi(cagri) {
		return
	}
	c.errors = append(c.errors, fmt.Sprintf("Satır %d: %s", e.Line(),
		ipucu.TipMetinKarsilastirmasi(e.Operator, metin.Value, kod.Ad(kod.A200))))
}

// tipCagrisi ifade, kullanıcının gölgelemediği yerleşik tip()'in bir çağrısı mı?
func (c *Checker) tipCagrisi(ifade agac.Expression) bool {
	call, ok := ifade.(*agac.CallExpression)
	if !ok {
		return false
	}
	ad, ok := call.Function.(*agac.Identifier)
	if !ok || ad.Value != kod.Ad(kod.G201) {
		return false
	}
	_, golge := c.scope.Resolve(ad.Value)
	return !golge
}
