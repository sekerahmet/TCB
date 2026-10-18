package ayristir

import (
	"slices"

	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/sozcuk"
)

// indirgeHedefi sondaki bloğun bağlandığı ifade bir liste.indirge çağrısı mı
// (l.indirge { ... }, l.indirge(0) { ... })? Bu bloklar iki parametrelidir (birikim, o).
func indirgeHedefi(e agac.Expression) bool {
	if c, ok := e.(*agac.CallExpression); ok {
		e = c.Function
	}
	m, ok := e.(*agac.MemberExpression)
	return ok && slices.Contains(indirgeYazimlari, m.Property)
}

// eskiIndirgeBlogu indirge bloğu eski biçimde (o, [birikim, eleman] listesiyken: o[0] ve
// o[1]) yazılmışsa ve 'birikim' hiç geçmiyorsa yol gösteren bir sözdizimi hatası kaydeder.
// bas bloğun '{' sözcük biriminin indeksidir; blok p.pos'ta bitmiştir.
func (p *Parser) eskiIndirgeBlogu(tok sozcuk.Token, bas int) {
	son := min(p.pos+1, len(p.tokens))
	var o0, o1 bool
	for i := bas; i < son; i++ {
		t := p.tokens[i]
		if t.Type == sozcuk.TOKEN_IDENT && t.Literal == birikimAdi {
			return
		}
		if t.Type == sozcuk.TOKEN_IDENT && t.Literal == oAdi && i+3 < son &&
			p.tokens[i+1].Type == sozcuk.TOKEN_LBRACKET && p.tokens[i+2].Type == sozcuk.TOKEN_INT &&
			p.tokens[i+3].Type == sozcuk.TOKEN_RBRACKET {
			o0 = o0 || p.tokens[i+2].Literal == "0"
			o1 = o1 || p.tokens[i+2].Literal == "1"
		}
	}
	if o0 && o1 {
		p.hataKaydet(tok, "indirge'nin sondaki bloğunda "+birikimAdi+" önceki sonuç, "+oAdi+" sıradaki elemandır; "+
			oAdi+"[0] ve "+oAdi+"[1] yerine "+birikimAdi+" ve "+oAdi+" yazın: liste.indirge(0) { "+birikimAdi+" + "+oAdi+" }")
	}
}
