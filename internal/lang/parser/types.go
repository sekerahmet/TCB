package parser

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

// arrowType dönüş okunun '->' yazımıdır: sözcük çözümleyici onu '-' ve '>' olarak verir, iletide tek
// işarettir.
const arrowType lexer.TokenType = "->"

// typeAfter imlecin ardındaki tipi okur; imleç tipin son belirtecinde kalır. Yazımlar:
//   - ad ve tip argümanları: TR_Int, TR_List[TR_Int], TR_Dict[K, V], T[...] (3.6, 3.7, 6.10, 15.1);
//   - demet tipi (A, B): en az iki öğe (3.6a);
//   - fonksiyon tipi (T1, T2) -> R, () -> R; '->' sağdan birleşir (6.9);
//   - isteğe bağlı T? (3.5): ad ve demet tipinden sonra; fonksiyon tipinin '?'si dönüş tipinindir.
//
// Parantezden sonra '->' geliyorsa fonksiyon tipi, gelmiyorsa demet tipidir. Tek öğeli '(T)' ve
// '->'süz '()' tip değildir: tek öğeli parantezin tip mi gruplama mı olduğunu dil tanımı söylemez.
// İç içelik ifadelerinkiyle aynı sınırdadır (enterDepth).
func (p *Parser) typeAfter() (*ast.TypeRef, bool) {
	if !p.enterDepth() {
		return nil, false
	}
	defer p.leaveDepth()
	if p.peekTokenIs(lexer.TokLParen) {
		p.nextToken()
		t := &ast.TypeRef{Token: p.curToken()}
		list, ok := p.typeList(lexer.TokRParen)
		if !ok {
			return nil, false
		}
		if p.arrowAhead() {
			p.nextToken() // '-'
			p.nextToken() // '>'
			ret, ok := p.typeAfter()
			if !ok {
				return nil, false
			}
			t.Params, t.Return = list, ret
			return t, true
		}
		if len(list) < 2 {
			p.peekError(arrowType)
			return nil, false
		}
		t.Elems = list
		return p.optionalMark(t), true
	}
	if !p.expectPeek(lexer.TokIdent) {
		return nil, false
	}
	p.role(p.curToken(), RoleType)
	t := &ast.TypeRef{Name: p.curToken().Literal, Token: p.curToken()}
	if p.peekTokenIs(lexer.TokLBracket) {
		p.nextToken()
		if p.peekTokenIs(lexer.TokRBracket) {
			p.peekError(lexer.TokIdent) // tip argümanı en az birdir
			return nil, false
		}
		args, ok := p.typeList(lexer.TokRBracket)
		if !ok {
			return nil, false
		}
		t.Args = args
	}
	return p.optionalMark(t), true
}

// optionalMark tipin ardındaki '?'yi okur (3.5).
func (p *Parser) optionalMark(t *ast.TypeRef) *ast.TypeRef {
	if p.peekTokenIs(lexer.TokQuestion) {
		p.nextToken()
		t.Optional = true
	}
	return t
}

// typeList açılış ('(' ya da '[') üzerindeki imleçten virgülle ayrılmış tipleri close'a kadar
// okur; imleç close'da kalır. Liste boş olabilir.
func (p *Parser) typeList(close lexer.TokenType) ([]ast.TypeRef, bool) {
	types := []ast.TypeRef{}
	if p.peekTokenIs(close) {
		p.nextToken()
		return types, true
	}
	for {
		t, ok := p.typeAfter()
		if !ok {
			return nil, false
		}
		types = append(types, *t)
		if !p.peekTokenIs(lexer.TokComma) {
			break
		}
		p.nextToken() // ','
	}
	if !p.expectPeek(close) {
		return nil, false
	}
	return types, true
}

// arrowAhead ardından dönüş oku '->' ('-' ve '>') mı geliyor.
func (p *Parser) arrowAhead() bool {
	return p.peekTokenIs(lexer.TokMinus) && p.peekTokenN(2).Type == lexer.TokGt
}

// parseTypeAndDefault isteğe bağlı ': Tip' ve '= varsayılan' bölümlerini okur (parametre ve yapı
// alanı için); olmayan bölüm boş döner.
func (p *Parser) parseTypeAndDefault() (typ *ast.TypeRef, def ast.Expression) {
	if p.peekTokenIs(lexer.TokColon) {
		p.nextToken() // ':'
		typ, _ = p.typeAfter()
	}
	if p.peekTokenIs(lexer.TokAssign) {
		p.nextToken() // '='
		p.nextToken()
		def = p.parseExpression(precLowest)
	}
	return typ, def
}
