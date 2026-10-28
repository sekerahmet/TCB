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
			if p.peekTokenIs(lexer.TokLParen) {
				// -> (A, B): birden çok değer (6.3); -> (T) -> R: fonksiyon döndürür (6.9)
				p.nextToken()
				open := p.curToken()
				list := p.parseTypeList()
				if list == nil {
					return nil
				}
				if p.arrowAhead() {
					if _, ok := p.functionTypeRest(); !ok {
						return nil
					}
					lit.ReturnType, lit.ReturnToken = codes.TypeFunction.String(), open
				} else {
					lit.ReturnTypes = list
				}
			} else if p.expectPeek(lexer.TokIdent) {
				lit.ReturnType, lit.ReturnToken = p.curToken().Literal, p.curToken()
				p.role(p.curToken(), RoleType)
				if lit.ReturnToken.Code == codes.TypeNoneType {
					p.returnNoneError(rparen, lit.ReturnToken)
					return nil
				}
			}
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
		param := ast.Param{Name: p.curToken().Literal, NameToken: p.curToken()}
		param.TypeName, param.TypeToken, param.DefaultValue = p.parseTypeAndDefault()
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

// parseTypeList '(' üzerindeki imleçten '(A, B)' tip listesini okur; imleç ')'de kalır. Öğe bir ad ya
// da fonksiyon tipidir; boş liste '()' parametresiz fonksiyon tipinin başıdır. Hatalıysa nil.
func (p *Parser) parseTypeList() []ast.TypeRef {
	types := []ast.TypeRef{}
	if p.peekTokenIs(lexer.TokRParen) {
		p.nextToken()
		return types
	}
	for {
		name, tok, ok := p.typeAfter()
		if !ok {
			return nil
		}
		types = append(types, ast.TypeRef{Name: name, Token: tok})
		if !p.peekTokenIs(lexer.TokComma) {
			break
		}
		p.nextToken() // ','
	}
	if !p.expectPeek(lexer.TokRParen) {
		return nil
	}
	return types
}

// typeAfter imlecin ardındaki tipi okur (6.9): ad (TR_Int) ya da fonksiyon tipi ((T1, T2) -> R,
// () -> R, (T) -> (A, B)); '->' sağdan birleşir. Fonksiyon tipinin adı TR_Function'dır, belirteci
// '('dir; parametre ve dönüş tipleri ayrıştırılır, denetimi 4a'da. İmleç tipin son belirtecinde kalır.
func (p *Parser) typeAfter() (string, lexer.Token, bool) {
	if p.peekTokenIs(lexer.TokLParen) {
		p.nextToken()
		open := p.curToken()
		if p.parseTypeList() == nil {
			return "", open, false
		}
		if !p.arrowAhead() {
			p.peekError(lexer.TokArrow)
			return "", open, false
		}
		if _, ok := p.functionTypeRest(); !ok {
			return "", open, false
		}
		return codes.TypeFunction.String(), open, true
	}
	if !p.expectPeek(lexer.TokIdent) {
		return "", p.curToken(), false
	}
	p.role(p.curToken(), RoleType)
	return p.curToken().Literal, p.curToken(), true
}

// arrowAhead ardından dönüş oku '->' ('-' ve '>') mı geliyor.
func (p *Parser) arrowAhead() bool {
	return p.peekTokenIs(lexer.TokMinus) && p.peekTokenN(2).Type == lexer.TokGt
}

// functionTypeRest fonksiyon tipinin '-> R' kısmını okur (R bir tip ya da sonuç listesi (A, B)).
func (p *Parser) functionTypeRest() (string, bool) {
	p.nextToken() // '-'
	p.nextToken() // '>'
	if p.peekTokenIs(lexer.TokLParen) {
		// (A, B) sonuç listesi ya da bir fonksiyon tipi
		p.nextToken()
		if p.parseTypeList() == nil {
			return "", false
		}
		if p.arrowAhead() {
			return p.functionTypeRest()
		}
		return "", true
	}
	name, _, ok := p.typeAfter()
	return name, ok
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
func (p *Parser) parseTypeAndDefault() (typeName string, typeTok lexer.Token, def ast.Expression) {
	if p.peekTokenIs(lexer.TokColon) {
		p.nextToken() // ':'
		typeName, typeTok, _ = p.typeAfter()
	}
	if p.peekTokenIs(lexer.TokAssign) {
		p.nextToken() // '='
		p.nextToken()
		def = p.parseExpression(precLowest)
	}
	return typeName, typeTok, def
}
