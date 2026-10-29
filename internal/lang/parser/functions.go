package parser

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func (p *Parser) parseGroupedOrLambdaExpression(pattern bool) ast.Expression {
	defer p.allowBlocks()()
	defer p.enterBrackets()()
	// Önce bunun '(a, b) => ...' bir ok fonksiyonu (lambda) olup olmadığını kontrol et; eşle
	// deseninde '(a + 1) => ...' parantezli bir desendir.
	if !pattern && p.isLambdaAhead() {
		return p.parseLambdaExpression()
	}

	p.nextToken()
	exp := p.parseExpression(precLowest)
	if !p.expectPeek(lexer.TokRParen) {
		return nil
	}
	return exp
}
func (p *Parser) isLambdaAhead() bool {
	// mevcut token '('
	idx := p.pos + 1
	depth := 1
	for idx < len(p.tokens) && depth > 0 {
		if p.tokens[idx].Type == lexer.TokLParen {
			depth++
		} else if p.tokens[idx].Type == lexer.TokRParen {
			depth--
		}
		idx++
	}
	if idx < len(p.tokens) && p.tokens[idx].Type == lexer.TokArrow {
		return true
	}
	return false
}
func (p *Parser) parseSingleParamLambda() ast.Expression {
	tok := p.curToken()
	// Parametre her zaman parantezle yazılır (6.6): x => e yerine (x) => e; düzeltme uygulanabilir.
	p.syntaxErrorFix(tok, []messages.Fix{{Range: tok.Range(), Replacement: "(" + tok.Literal + ")", Applicable: true}}, messages.H0267, tok.Literal)
	param := ast.Param{Name: tok.Literal, NameToken: tok}
	p.nextToken() // '=>'
	return p.finishLambda(tok, []ast.Param{param})
}
func (p *Parser) parseLambdaExpression() ast.Expression {
	tok := p.curToken() // '('
	params := p.parseFunctionParameters()
	if !p.expectPeek(lexer.TokArrow) {
		return nil
	}
	return p.finishLambda(tok, params)
}

// finishLambda '=>' belirtecinde duran imleçten sonrasını (küme ayraçlı gövde ya da tek ifade)
// okuyup lambdayı kurar; tok lambdanın ilk belirtecidir.
func (p *Parser) finishLambda(tok lexer.Token, params []ast.Param) ast.Expression {
	arrowTok := p.curToken()

	var body *ast.BlockStatement
	if p.peekTokenIs(lexer.TokLBrace) {
		p.nextToken()
		body = p.functionBody()
	} else {
		p.nextToken()
		expr := p.functionExpr()
		body = &ast.BlockStatement{
			Token: arrowTok,
			Statements: []ast.Statement{
				&ast.ReturnStatement{Token: arrowTok, ReturnValue: expr},
			},
		}
	}

	return &ast.FunctionLiteral{
		Token:      tok,
		Parameters: params,
		Body:       body,
	}
}
func (p *Parser) parseFunctionLiteral() ast.Expression {
	lit := &ast.FunctionLiteral{Token: p.curToken()}
	p.funcKeywords = true

	// İsimli fonksiyon: fonksiyon topla(a, b) { ... }
	if p.peekTokenIs(lexer.TokIdent) {
		p.nextToken()
		lit.Name, lit.NameToken = p.curToken().Literal, p.curToken()
		p.namedFunctions = true
	}

	if !p.expectPeek(lexer.TokLParen) {
		return nil
	}

	lit.Parameters = p.parseFunctionParameters()
	rparen := p.curToken()

	// Dönüş tipi '->' ile yazılır (6.1): TR_func topla(a: TR_Int, b: TR_Int) -> TR_Int { ... }
	switch {
	case p.peekTokenIs(lexer.TokColon):
		p.returnColonError(rparen)
		return nil
	case p.peekTokenIs(lexer.TokMinus):
		p.nextToken() // '-'
		if p.peekTokenIs(lexer.TokGt) {
			p.nextToken() // '>'
			// -> (A, B): birden çok değer, bir demettir (6.3); -> (T) -> R: fonksiyon döndürür (6.9)
			ret, ok := p.typeAfter()
			if !ok {
				return nil
			}
			if ret.Token.Code == codes.TypeNoneType && ret.Args == nil && !ret.Optional {
				p.returnNoneError(rparen, ret.Token)
				return nil
			}
			lit.ReturnType = ret
		}
	}

	if !p.expectPeek(lexer.TokLBrace) {
		return nil
	}

	lit.Body = p.functionBody()
	return lit
}
func (p *Parser) parseFunctionParameters() []ast.Param {
	params := []ast.Param{}
	withDefault := false

	if p.peekTokenIs(lexer.TokRParen) {
		p.nextToken()
		return params
	}

	p.nextToken()

	for {
		if p.curTokenIs(lexer.TokSelf) { // 7.3: struct.go
			if p.selfParameterError() {
				continue
			}
			break
		}
		param := ast.Param{Name: p.curToken().Literal, NameToken: p.curToken()}
		param.Type, param.DefaultValue = p.parseTypeAndDefault()
		params = append(params, param)
		if param.DefaultValue == nil && withDefault {
			// Varsayılanlı parametreden sonrakilerin de varsayılanı vardır (6.4), Python'daki gibi.
			p.syntaxError(param.NameToken, messages.H0261, param.Name)
		}
		withDefault = withDefault || param.DefaultValue != nil

		if !p.peekTokenIs(lexer.TokComma) {
			break
		}
		p.nextToken() // ','
		p.nextToken()
	}

	if !p.expectPeek(lexer.TokRParen) {
		return nil
	}

	return params
}
func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {
	exp := &ast.CallExpression{Token: p.curToken(), Function: function}
	exp.Arguments, exp.NamedArgs = p.parseCallArguments()
	if n := len(exp.Arguments); n > 0 {
		// Son argümandaki parametresiz fonksiyon yalnız sondaki blokla yazılır (6.7).
		if f, ok := exp.Arguments[n-1].(*ast.FunctionLiteral); ok && f.Name == "" && len(f.Parameters) == 0 && f.Token.Type == lexer.TokLParen {
			p.syntaxError(f.Token, messages.H0262)
		}
	}
	return exp
}
func (p *Parser) parseCallArguments() ([]ast.Expression, []ast.NamedArg) {
	defer p.allowBlocks()()
	defer p.enterBrackets()()
	args := []ast.Expression{}
	named := []ast.NamedArg{}

	if p.peekTokenIs(lexer.TokRParen) {
		p.nextToken()
		return args, named
	}

	p.nextToken()
	for {
		if p.curTokenIs(lexer.TokIdent) && p.peekTokenIs(lexer.TokAssign) {
			// Python'un k=2'si: adlı argüman iki noktayla yazılır (6.5); '=' atamadır. Hata '='de,
			// düzeltme onu ':' yapar ve olduğu gibi uygulanabilir; okuma adlı argüman olarak sürer.
			eq := p.peekToken()
			colon := ":"
			if v := p.peekTokenN(2); v.Line == eq.EndLine && v.Column == eq.EndColumn {
				colon = ": " // k=2 → k: 2
			}
			p.syntaxErrorFix(eq, []messages.Fix{{Range: eq.Range(), Replacement: colon, Applicable: true}}, messages.H0266, p.curToken().Literal)
		}
		if p.curTokenIs(lexer.TokIdent) && (p.peekTokenIs(lexer.TokColon) || p.peekTokenIs(lexer.TokAssign)) {
			nameTok := p.curToken()
			p.nextToken() // ':'
			p.nextToken()
			val := p.parseExpression(precLowest)
			named = append(named, ast.NamedArg{Token: nameTok, Name: nameTok.Literal, Value: val})
		} else {
			args = append(args, p.parseExpression(precLowest))
		}

		if !p.peekTokenIs(lexer.TokComma) {
			break
		}
		p.nextToken() // ','
		if p.peekTokenIs(lexer.TokRParen) {
			break
		}
		p.nextToken()
	}

	if !p.expectPeek(lexer.TokRParen) {
		return nil, nil
	}

	return args, named
}

// functionExpressions TR_func'ın yalnız deyim olarak yazılmasını denetler (6.6): fonksiyon ifadesinin tek
// yazımı oktur. Adsız TR_func(x) { ... } H0268, ifade yerindeki adlı TR_func H0269; deyim olarak yazılan
// adlı fonksiyon ve yapı metodu tanımdır.
func (p *Parser) functionExpressions(prog *ast.Program) {
	if !p.funcKeywords || p.interpolated {
		return
	}
	definitions := map[*ast.FunctionLiteral]bool{}
	ast.Walk(prog, func(n any) bool {
		switch x := n.(type) {
		case *ast.ExpressionStatement:
			if f, ok := x.Expression.(*ast.FunctionLiteral); ok && f.Name != "" {
				definitions[f] = true
			}
		case *ast.StructStatement:
			for _, m := range x.Methods {
				definitions[m] = true
			}
		}
		return true
	})
	ast.Walk(prog, func(n any) bool {
		f, ok := n.(*ast.FunctionLiteral)
		if !ok || f.Token.Type != lexer.TokFunction || definitions[f] {
			return true
		}
		if f.Name == "" {
			p.syntaxError(f.Token, messages.H0268, p.word(codes.KwFunc))
		} else {
			p.syntaxError(f.Token, messages.H0269, p.word(codes.KwFunc), f.Name)
		}
		p.errorMode = false // her fonksiyon ifadesi ayrı bir yerdir
		return true
	})
}

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
func (p *Parser) typeAfter() (*ast.TypeRef, bool) {
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

// returnColonError dönüş tipinin ': R' ile yazımının hatasıdır (6.1): imleç parametrelerin ')'sinde,
// ardından ':' gelir. Düzeltme ':'yi '->' yapar; olduğu gibi uygulanabilir.
func (p *Parser) returnColonError(rparen lexer.Token) {
	colon := p.peekToken()
	arrow := "->"
	if colon.Line == rparen.EndLine && colon.Column == rparen.EndColumn {
		arrow = " ->" // f(): T → f() -> T
	}
	p.syntaxErrorFix(colon, []messages.Fix{{Range: colon.Range(), Replacement: arrow, Applicable: true}}, messages.H0256)
}

// returnNoneError '-> TR_NoneType' yazımının hatasıdır (6.1): değer döndürmeyen fonksiyonda '->' yazılmaz.
// Düzeltme ')'den tipin sonuna kadarını siler; olduğu gibi uygulanabilir.
func (p *Parser) returnNoneError(rparen, typ lexer.Token) {
	r := typ.Range()
	r.Line, r.Col = rparen.EndLine, rparen.EndColumn
	p.syntaxErrorFix(typ, []messages.Fix{{Range: r, Replacement: "", Applicable: true}}, messages.H0257, p.word(codes.TypeNoneType))
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
