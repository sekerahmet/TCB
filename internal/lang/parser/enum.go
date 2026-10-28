package parser

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Durumlu tip (7.7) ve arayüz (7.8) yapının deyimiyle (ast.StructStatement, Kind) okunur: adları
// yapınınki gibi tanımlanır, metotları yapının metotları gibi denetlenir.

// typeDefinitionHead 'yapı Ad {', 'durumlu Ad {' ya da 'arayüz Ad {' başını okur; gövdenin ilk
// token'ında durur. Dönen token açılan '{'dir (kapanmazsa hata: requireClosingBrace).
func (p *Parser) typeDefinitionHead(kind ast.StructKind) (*ast.StructStatement, lexer.Token) {
	tok := p.curToken()
	if !p.expectPeek(lexer.TokIdent) {
		return nil, lexer.Token{}
	}
	stmt := &ast.StructStatement{
		Token:     tok,
		Name:      p.curToken().Literal,
		NameToken: p.curToken(),
		Fields:    []ast.StructField{},
		Methods:   make(map[string]*ast.FunctionLiteral),
		Kind:      kind,
	}
	// Kalıtım yoktur (7.1): 'yapı A(B)' sözdizimi hatasıdır, ileti ortak parçayı alanla kurmayı
	// söyler. Parantezin içi atlanır, gövde okunmaya devam eder.
	if p.peekTokenIs(lexer.TokLParen) {
		p.inheritanceError(stmt.Name)
	}
	if !p.expectPeek(lexer.TokLBrace) {
		return nil, lexer.Token{}
	}
	open := p.curToken()
	p.nextToken()
	return stmt, open
}

// parseEnumStatement 'durumlu Ad { Durum; Durum(alan: T); fonksiyon m() { ... } }' tanımını okur:
// önce durumlar, sonra metotlar. Durumun alanları 7.1'deki gibi yazılır; kurucu yoktur.
func (p *Parser) parseEnumStatement() *ast.StructStatement {
	stmt, open := p.typeDefinitionHead(ast.KindEnum)
	if stmt == nil {
		return nil
	}
	defer func() { p.requireClosingBrace(open) }()
	body := &structBody{p: p, stmt: stmt, defined: map[string]bool{}}
	methods := false
	for !p.curTokenIs(lexer.TokRBrace) && !p.curTokenIs(lexer.TokEOF) {
		if p.curTokenIs(lexer.TokSemicolon) || p.curTokenIs(lexer.TokComma) {
			p.nextToken()
			continue
		}
		read := body.variant
		if p.curTokenIs(lexer.TokFunction) {
			methods, read = true, body.enumMethod
		} else if methods {
			// durum metotlardan sonra yazılmaz: okuyan durumların hepsini başta görür
			body.invalidAs(messages.H0286, p.word(codes.KwFunc))
			break
		}
		if !read() {
			break
		}
		p.nextToken()
	}
	return stmt
}

// parseInterfaceStatement 'arayüz Ad { fonksiyon f(x: T) -> T }' tanımını okur: yalnız gövdesiz metot
// imzaları; en az bir metot yazılır (boş arayüz her tipi karşılayan genel bir tip olurdu, 3.11).
func (p *Parser) parseInterfaceStatement() *ast.StructStatement {
	stmt, open := p.typeDefinitionHead(ast.KindInterface)
	if stmt == nil {
		return nil
	}
	defer func() { p.requireClosingBrace(open) }()
	body := &structBody{p: p, stmt: stmt, defined: map[string]bool{}}
	for !p.curTokenIs(lexer.TokRBrace) && !p.curTokenIs(lexer.TokEOF) {
		if p.curTokenIs(lexer.TokSemicolon) || p.curTokenIs(lexer.TokComma) {
			p.nextToken()
			continue
		}
		if !p.curTokenIs(lexer.TokFunction) {
			body.invalidAs(messages.H0289, p.word(codes.KwFunc))
			break
		}
		if !body.signature() {
			break
		}
		p.nextToken()
	}
	if len(stmt.Methods) == 0 && p.curTokenIs(lexer.TokRBrace) {
		p.syntaxError(stmt.NameToken, messages.H0288, stmt.Name, p.word(codes.KwFunc))
	}
	return stmt
}

// invalidAs gövdede yerinde olmayan içeriği code ile bildirir ve gövdenin sonuna atlar.
func (b *structBody) invalidAs(code messages.Code, args ...any) {
	b.p.syntaxError(b.p.curToken(), code, args...)
	b.p.skipToBlockEnd()
}

// variant bir durumu okur: ad ve isteğe bağlı parantez içinde alanlar (7.1'deki gibi).
func (b *structBody) variant() bool {
	p := b.p
	if !p.curTokenIs(lexer.TokIdent) {
		return b.invalid(p.curToken(), messages.New(messages.H0148, p.tokenDescription(p.curToken())))
	}
	tok := p.curToken()
	p.role(tok, RoleStructField)
	v := ast.EnumVariant{Name: tok.Literal, NameToken: tok}
	if p.peekTokenIs(lexer.TokLParen) {
		p.nextToken() // '('
		fields, ok := p.variantFields(b.stmt.Name + "." + v.Name)
		if !ok {
			p.skipToBlockEnd()
			return false
		}
		v.Fields = fields
	}
	if b.checkName(tok, v.Name, true) {
		b.stmt.Variants = append(b.stmt.Variants, v)
	}
	return true
}

// variantFields bir durumun parantez içindeki alanlarını okur; ')'de durur. variant iletideki addır
// (Durum.Çalışıyor).
func (p *Parser) variantFields(variant string) ([]ast.StructField, bool) {
	fields := []ast.StructField{}
	seen := map[string]bool{}
	lastDefault := ""
	for !p.peekTokenIs(lexer.TokRParen) {
		if !p.expectPeek(lexer.TokIdent) {
			return nil, false
		}
		tok := p.curToken()
		f := ast.StructField{Name: tok.Literal, NameToken: tok}
		f.TypeName, f.TypeToken, f.DefaultValue = p.parseTypeAndDefault()
		switch {
		case seen[f.Name]:
			p.syntaxError(tok, messages.H0296, f.Name, variant)
		case f.DefaultValue != nil:
			lastDefault = f.Name
		case lastDefault != "":
			p.syntaxError(tok, messages.H0297, f.Name, lastDefault, variant)
		}
		seen[f.Name] = true
		fields = append(fields, f)
		if !p.peekTokenIs(lexer.TokComma) {
			break
		}
		p.nextToken() // ','
	}
	if !p.expectPeek(lexer.TokRParen) {
		return nil, false
	}
	return fields, true
}

// enumMethod durumlu tipin bir metodunu okur: kurucu yoktur, durum kendi alanlarıyla kurulur.
func (b *structBody) enumMethod() bool {
	p := b.p
	if p.peekTokenIs(lexer.TokIdent) && p.peekToken().Literal == p.l.Language().Name(codes.CtxInit) {
		p.syntaxError(p.peekToken(), messages.H0285, b.stmt.Name, p.word(codes.CtxInit))
	}
	return b.method()
}

// signature arayüzün gövdesiz bir metot imzasını okur: fonksiyon ad(parametreler) -> Tip.
func (b *structBody) signature() bool {
	p := b.p
	fnTok := p.curToken()
	lit := &ast.FunctionLiteral{Token: fnTok}
	if !p.expectPeek(lexer.TokIdent) {
		p.skipToBlockEnd()
		return false
	}
	lit.Name, lit.NameToken = p.curToken().Literal, p.curToken()
	p.role(p.curToken(), RoleStructMethod)
	if !p.expectPeek(lexer.TokLParen) {
		p.skipToBlockEnd()
		return false
	}
	lit.Parameters = p.parseFunctionParameters()
	if p.peekTokenIs(lexer.TokMinus) {
		p.nextToken() // '-'
		if p.expectPeek(lexer.TokGt) && p.expectPeek(lexer.TokIdent) {
			lit.ReturnType, lit.ReturnToken = p.curToken().Literal, p.curToken()
			p.role(p.curToken(), RoleType)
		}
	}
	if p.peekTokenIs(lexer.TokLBrace) {
		// arayüzün metodu yalnız imzadır; gövde onu karşılayan yapıdadır
		p.nextToken()
		b.invalidAs(messages.H0290, lit.Name, b.stmt.Name)
		return false
	}
	if b.checkName(fnTok, lit.Name, false) {
		b.stmt.Methods[lit.Name] = lit
	}
	return true
}

// enumPattern eşle kolunun desenini durumlu tip desenine çevirir (7.7): parantezsiz 'Durum.Durum(a, _)'
// çağrısı alanları bağlar. Desende değer karşılaştırılmaz ve adlı alan yazılmaz; koşullu kol
// ('... eğer koşul =>') yoktur: karşılaştırma kolun içinde yapılır. Parantezli ifade değeriyle
// karşılaştırılır (5.6), olduğu gibi kalır.
func (p *Parser) enumPattern(grouped bool, e ast.Expression) ast.Expression {
	if p.peekTokenIs(lexer.TokIf) {
		p.syntaxError(p.peekToken(), messages.H0292, p.word(codes.KwIf))
	}
	call, ok := e.(*ast.CallExpression)
	if !ok || grouped {
		return e
	}
	mem, ok := call.Function.(*ast.MemberExpression)
	if !ok {
		return e
	}
	pat := &ast.EnumPattern{Token: call.Token, Enum: mem.Object, Variant: mem.Property, VariantToken: mem.NameToken}
	label := mem.Property
	if id, ok := mem.Object.(*ast.Identifier); ok {
		label = id.Value + "." + mem.Property
	}
	if len(call.NamedArgs) > 0 {
		na := call.NamedArgs[0]
		value := "_"
		if id, ok := na.Value.(*ast.Identifier); ok {
			value = id.Value
		}
		p.syntaxError(na.Token, messages.H0287, na.Name, label, value)
		return pat
	}
	for _, a := range call.Arguments {
		id, ok := a.(*ast.Identifier)
		if !ok {
			p.syntaxError(call.Token, messages.H0291, label)
			return pat
		}
		pat.Names = append(pat.Names, id)
	}
	return pat
}
