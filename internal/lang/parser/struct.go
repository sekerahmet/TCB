package parser

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// parseStructStatement 'yapı Ad { ... }' tanımını okur; durumlu tip ve arayüz kendi okuyucularına
// gider (enum.go).
func (p *Parser) parseStructStatement() *ast.StructStatement {
	switch p.curToken().Type {
	case lexer.TokEnum:
		return p.parseEnumStatement()
	case lexer.TokInterface:
		return p.parseInterfaceStatement()
	}
	stmt, open := p.typeDefinitionHead(ast.KindStruct)
	if stmt == nil {
		return nil
	}
	defer func() { p.requireClosingBrace(open) }()

	// Yapı gövdesinde yalnızca alanlar ve fonksiyon tanımları bulunur; her ad bir kez
	// tanımlanır. Başka içerik sessizce alan sayılmaz ya da atlanmaz: sözdizimi hatasıdır.
	body := &structBody{p: p, stmt: stmt, defined: map[string]bool{}}
	for !p.curTokenIs(lexer.TokRBrace) && !p.curTokenIs(lexer.TokEOF) {
		if p.curTokenIs(lexer.TokSemicolon) || p.curTokenIs(lexer.TokComma) {
			p.nextToken()
			continue
		}
		read := body.field
		if p.curTokenIs(lexer.TokFunction) {
			read = body.method
		}
		if !read() {
			break
		}
		p.nextToken()
	}
	return stmt
}

// structBody bir yapı gövdesinin okunmasıdır: tanımlanan adlar ve varsayılanı olan son alan.
type structBody struct {
	p    *Parser
	stmt *ast.StructStatement
	// defined tanımlanan adlardır (ad → alan mı). Yinelenen ad kaydedilmez ama gövde okunmaya
	// devam eder: tek yanlışa tek ileti.
	defined map[string]bool
	// lastDefault varsayılanı olan son alandır: ondan sonraki alanların da varsayılanı olur (7.1).
	lastDefault string
}

// checkName adın gövdede ilk kez tanımlandığını denetler.
func (b *structBody) checkName(tok lexer.Token, name string, field bool) bool {
	if wasField, ok := b.defined[name]; ok {
		switch {
		case b.stmt.Kind == ast.KindEnum && wasField != field:
			b.p.syntaxError(tok, messages.H0294, name, b.stmt.Name)
		case b.stmt.Kind == ast.KindEnum:
			kind := "state" // durumlu tipte "alan" durumdur (7.7)
			if !field {
				kind = "method"
			}
			b.p.syntaxError(tok, messages.H0293, kind, name, b.stmt.Name)
		case b.stmt.Kind == ast.KindInterface:
			b.p.syntaxError(tok, messages.H0295, name, b.stmt.Name)
		case wasField != field:
			b.p.syntaxError(tok, messages.H0277, name, b.stmt.Name)
		case field:
			b.p.syntaxError(tok, messages.H0146, name, b.stmt.Name)
		default:
			b.p.syntaxError(tok, messages.H0145, name, b.stmt.Name)
		}
		return false
	}
	b.defined[name] = field
	return true
}

// method gövdedeki bir metodu okur; okunamazsa gövdenin sonuna atlar ve false döner.
func (b *structBody) method() bool {
	p := b.p
	fnTok := p.curToken()
	p.role(p.peekToken(), RoleStructMethod)
	// Hatalı metod tanımında parseFunctionLiteral nil döner; tip dönüşümü panik yapmamalı
	fn, _ := p.parseFunctionLiteral().(*ast.FunctionLiteral)
	if fn == nil {
		// Hata zaten kaydedildi; yapının kapanışına kadar atla
		p.skipToBlockEnd()
		return false
	}
	if fn.Name == "" {
		p.syntaxError(fnTok, messages.H0150)
		p.skipToBlockEnd()
		return false
	}
	if b.checkName(fnTok, fn.Name, false) {
		b.stmt.Methods[fn.Name] = fn
	}
	if fn.Name == p.l.Language().Name(codes.CtxInit) {
		p.checkConstructor(fn)
	}
	return true
}

// field gövdedeki bir alanı okur; alan değilse hatayı kaydeder, gövdenin sonuna atlar ve false döner.
func (b *structBody) field() bool {
	p := b.p
	if p.curTokenIs(lexer.TokVar) {
		p.nextToken()
	}
	if !p.curTokenIs(lexer.TokIdent) {
		return b.invalid(p.curToken(), messages.New(messages.H0148, p.tokenDescription(p.curToken())))
	}
	fieldTok := p.curToken()
	p.role(fieldTok, RoleStructField)
	field := ast.StructField{Name: fieldTok.Literal, NameToken: fieldTok}
	field.TypeName, field.TypeToken, field.DefaultValue = p.parseTypeAndDefault()
	// Alan virgül, ';', satır sonu ya da '}' ile biter: aynı satırda başka bir şey
	// (ör. 'yazdır("...")' çağrısının '(' işareti) alan değildir.
	if next := p.peekToken(); next.Type != lexer.TokRBrace && next.Type != lexer.TokComma &&
		next.Type != lexer.TokSemicolon && next.Type != lexer.TokEOF && next.Line == p.curToken().Line {
		return b.invalid(fieldTok, messages.New(messages.H0149, p.tokenName(fieldTok), p.tokenDescription(next)))
	}
	if field.DefaultValue != nil {
		b.lastDefault = field.Name
	} else if b.lastDefault != "" {
		p.syntaxError(fieldTok, messages.H0276, field.Name, b.lastDefault, b.stmt.Name)
	}
	if b.checkName(fieldTok, field.Name, true) {
		b.stmt.Fields = append(b.stmt.Fields, field)
	}
	return true
}

// invalid gövdede alan ya da metot olmayan bir içeriği bildirir ve gövdenin sonuna atlar.
func (b *structBody) invalid(tok lexer.Token, what *messages.Error) bool {
	b.p.syntaxError(tok, messages.H0147, what)
	b.p.skipToBlockEnd()
	return false
}

// inheritanceError 'yapı A(B)' yazımını (kalıtım, 7.1) bildirir ve parantezi atlar; sonraki token '{'
// olmalıdır. B tek bir adsa ileti ortak parçayı B tipinde bir alanla kurmayı önerir: yapı A { b: B }.
// Alan tipi tek bir addır; 'nn.Module' gibi bir yazımda önerilecek çalışan bir yazım yoktur.
func (p *Parser) inheritanceError(name string) {
	p.nextToken() // '('
	open := p.curToken()
	var toks []lexer.Token
	for !p.peekTokenIs(lexer.TokRParen) && !p.peekTokenIs(lexer.TokEOF) && !p.peekTokenIs(lexer.TokLBrace) {
		p.nextToken()
		toks = append(toks, p.curToken())
	}
	if p.peekTokenIs(lexer.TokRParen) {
		p.nextToken()
	}
	if len(toks) == 1 && toks[0].Type == lexer.TokIdent {
		p.syntaxError(open, messages.H0275, p.word(codes.KwStruct), name, p.tokenName(toks[0]), fieldNameFor(toks[0]))
		return
	}
	var base strings.Builder
	for _, t := range toks {
		base.WriteString(t.Literal)
	}
	p.syntaxError(open, messages.H0280, p.word(codes.KwStruct), name, base.String())
}

// fieldNameFor kalıtım iletisinin önerdiği alanın adıdır: tipin adı küçük harfle başlar
// (B → b, TR_Linear → linear).
func fieldNameFor(tok lexer.Token) string {
	name := tok.Literal
	if tok.Code != codes.None {
		// kimliğin sınıfsız adı: TR_type_Linear → Linear
		name = codes.Name(tok.Code)
		name = strings.TrimPrefix(name, "TR_")
		name = name[strings.Index(name, "_")+1:]
	}
	r, size := utf8.DecodeRuneInString(name)
	return string(unicode.ToLower(r)) + name[size:]
}

// isPythonClass deyim Python'un sınıf tanımı mı: 'class A(B):' ya da 'class A:' (7.1). 'class' TR'de
// bir ad olabilir; yalnız ardından ad ve '(' ya da ':' gelirse tanım sayılır.
func (p *Parser) isPythonClass() bool {
	if k, ok := codes.FamiliarCode(p.curToken().Literal); !ok || k != codes.KwStruct || !p.peekTokenIs(lexer.TokIdent) {
		return false
	}
	if p.pos+2 >= len(p.tokens) {
		return false
	}
	next := p.tokens[p.pos+2].Type
	return next == lexer.TokLParen || next == lexer.TokColon
}

// pythonClassError Python'un sınıf tanımını tek iletiyle bildirir: TR'de yapı yazılır ve kalıtım yoktur;
// tek adlı tabanda ileti ortak parçayı alanla kurmayı önerir (H0275 gibi). Deyimin geri kalanı atlanır.
func (p *Parser) pythonClassError() ast.Statement {
	class := p.curToken()
	p.nextToken()
	name := p.curToken().Literal
	if !p.peekTokenIs(lexer.TokLParen) {
		p.syntaxError(class, messages.H0278, class.Literal, p.word(codes.KwStruct), name)
		return nil
	}
	p.nextToken() // '('
	var toks []lexer.Token
	for !p.peekTokenIs(lexer.TokRParen) && !p.peekTokenIs(lexer.TokEOF) && !p.peekTokenIs(lexer.TokColon) {
		p.nextToken()
		toks = append(toks, p.curToken())
	}
	if len(toks) == 1 && toks[0].Type == lexer.TokIdent {
		p.syntaxError(class, messages.H0279, class.Literal, p.word(codes.KwStruct), name, p.tokenName(toks[0]), fieldNameFor(toks[0]))
		return nil
	}
	var base strings.Builder
	for _, t := range toks {
		base.WriteString(t.Literal)
	}
	p.syntaxError(class, messages.H0281, class.Literal, p.word(codes.KwStruct), name, base.String())
	return nil
}

// checkConstructor kurucunun değer döndürmediğini denetler (7.4): dönüş tipi ve değerli 'sonuç'
// sözdizimi hatasıdır. Kurucunun içindeki fonksiyonların 'sonuç'ları kendilerinindir.
func (p *Parser) checkConstructor(fn *ast.FunctionLiteral) {
	if fn.ReturnType != "" {
		p.syntaxError(fn.ReturnToken, messages.H0283, p.word(codes.CtxInit))
		return
	}
	ast.Walk(fn.Body, func(n any) bool {
		switch x := n.(type) {
		case *ast.FunctionLiteral:
			return false
		case *ast.ReturnStatement:
			// değersiz 'sonuç'u ayrıştırıcı kendi token'ında bir yokluk olarak yazar
			if bare, ok := x.ReturnValue.(*ast.NullLiteral); !ok || bare.Token != x.Token {
				p.syntaxError(x.Token, messages.H0282, p.word(codes.CtxInit), p.word(codes.KwReturn))
			}
		}
		return true
	})
}

// selfParameterError 'kendi'nin parametre olarak yazılmasını (Python'un self'i, 7.3) bildirir; düzeltme
// onu ardındaki virgülle siler. Ardından başka parametre geliyorsa ona geçer ve true döner; 'kendi'
// listenin sonundaysa yerinde kalır ve false döner.
func (p *Parser) selfParameterError() bool {
	self := p.curToken()
	end := p.peekToken()
	more := p.peekTokenIs(lexer.TokComma)
	if more {
		p.nextToken() // ','
		end = p.peekToken()
	}
	cut := messages.Range{Line: self.Line, Col: self.Column, EndLine: end.Line, EndCol: end.Column}
	p.syntaxErrorFix(self, []messages.Fix{{Range: cut, Applicable: true}}, messages.H0284, p.tokenName(self))
	if more {
		p.nextToken()
	}
	return more
}
