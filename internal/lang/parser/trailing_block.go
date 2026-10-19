package parser

import (
	"slices"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

// indirgeHedefi sondaki bloğun bağlandığı ifade bir liste.indirge çağrısı mı
// (l.indirge { ... }, l.indirge(0) { ... })? Bu bloklar iki parametrelidir (birikim, o).
func isReduceTarget(e ast.Expression) bool {
	if c, ok := e.(*ast.CallExpression); ok {
		e = c.Function
	}
	m, ok := e.(*ast.MemberExpression)
	return ok && slices.Contains(reduceSpellings, m.Property)
}

// eskiIndirgeBlogu indirge bloğu eski biçimde (o, [birikim, eleman] listesiyken: o[0] ve
// o[1]) yazılmışsa ve 'birikim' hiç geçmiyorsa yol gösteren bir sözdizimi hatası kaydeder.
// bas bloğun '{' sözcük biriminin indeksidir; blok p.pos'ta bitmiştir.
func (p *Parser) oldReduceBlock(tok lexer.Token, begin int) {
	end := min(p.pos+1, len(p.tokens))
	var o0, o1 bool
	for i := begin; i < end; i++ {
		t := p.tokens[i]
		if t.Type == lexer.TOKEN_IDENT && t.Literal == accName {
			return
		}
		if t.Type == lexer.TOKEN_IDENT && t.Literal == itName && i+3 < end &&
			p.tokens[i+1].Type == lexer.TOKEN_LBRACKET && p.tokens[i+2].Type == lexer.TOKEN_INT &&
			p.tokens[i+3].Type == lexer.TOKEN_RBRACKET {
			o0 = o0 || p.tokens[i+2].Literal == "0"
			o1 = o1 || p.tokens[i+2].Literal == "1"
		}
	}
	if o0 && o1 {
		reduce := codes.MemberName(codes.Y207)
		p.recordError(tok, reduce+" metodunun sondaki bloğunda "+accName+" önceki sonuç, "+itName+" sıradaki elemandır; "+
			itName+"[0] ve "+itName+"[1] yerine "+accName+" ve "+itName+" yazın: "+reduce+"(0) { "+accName+" + "+itName+" }")
	}
}
