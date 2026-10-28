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

func (p *Parser) parseStructStatement() *ast.StructStatement {
	tok := p.curToken()
	if !p.expectPeek(lexer.TokIdent) {
		return nil
	}
	stmt := &ast.StructStatement{
		Token:     tok,
		Name:      p.curToken().Literal,
		NameToken: p.curToken(),
		Fields:    []ast.StructField{},
		Methods:   make(map[string]*ast.FunctionLiteral),
	}

	// Kalıtım yoktur (7.1): 'yapı A(B)' sözdizimi hatasıdır, ileti ortak parçayı alanla kurmayı
	// söyler. Parantezin içi atlanır, gövde okunmaya devam eder.
	if p.peekTokenIs(lexer.TokLParen) {
		p.inheritanceError(stmt.Name)
	}
	if !p.expectPeek(lexer.TokLBrace) {
		return nil
	}
	structOpen := p.curToken()
	defer func() { p.requireClosingBrace(structOpen) }()
	p.nextToken()

	// Yapı gövdesinde yalnızca alanlar ve fonksiyon tanımları bulunur; her ad bir kez
	// tanımlanır. Başka içerik sessizce alan sayılmaz ya da atlanmaz: sözdizimi hatasıdır.
	// Yinelenen ad kaydedilmez ama gövde okunmaya devam eder: tek yanlışa tek ileti.
	defined := map[string]bool{} // ad → alan mı
	checkName := func(tok lexer.Token, name string, field bool) bool {
		if wasField, ok := defined[name]; ok {
			switch {
			case wasField != field:
				p.syntaxError(tok, messages.H0277, name, stmt.Name)
			case field:
				p.syntaxError(tok, messages.H0146, name, stmt.Name)
			default:
				p.syntaxError(tok, messages.H0145, name, stmt.Name)
			}
			return false
		}
		defined[name] = field
		return true
	}
	lastDefault := "" // varsayılanı olan son alan: ondan sonraki alanların da varsayılanı olur (7.1)
	invalid := func(tok lexer.Token, what *messages.Error) *ast.StructStatement {
		p.syntaxError(tok, messages.H0147, what)
		p.skipToBlockEnd()
		return stmt
	}
	for !p.curTokenIs(lexer.TokRBrace) && !p.curTokenIs(lexer.TokEOF) {
		if p.curTokenIs(lexer.TokSemicolon) || p.curTokenIs(lexer.TokComma) {
			p.nextToken()
			continue
		}
		if p.curTokenIs(lexer.TokFunction) {
			fnTok := p.curToken()
			p.role(p.peekToken(), RoleStructMethod)
			// Hatalı metod tanımında parseFunctionLiteral nil döner; tip dönüşümü panik yapmamalı
			fn, _ := p.parseFunctionLiteral().(*ast.FunctionLiteral)
			if fn == nil {
				// Hata zaten kaydedildi; yapının kapanışına kadar atla
				p.skipToBlockEnd()
				break
			}
			if fn.Name == "" {
				p.syntaxError(fnTok, messages.H0150)
				p.skipToBlockEnd()
				break
			}
			if checkName(fnTok, fn.Name, false) {
				stmt.Methods[fn.Name] = fn
			}
			p.nextToken()
			continue
		}
		if p.curTokenIs(lexer.TokVar) {
			p.nextToken()
		}
		if !p.curTokenIs(lexer.TokIdent) {
			return invalid(p.curToken(), messages.New(messages.H0148, p.tokenDescription(p.curToken())))
		}
		fieldTok := p.curToken()
		p.role(fieldTok, RoleStructField)
		field := ast.StructField{Name: fieldTok.Literal, NameToken: fieldTok}
		field.TypeName, field.TypeToken, field.DefaultValue = p.parseTypeAndDefault()
		// Alan virgül, ';', satır sonu ya da '}' ile biter: aynı satırda başka bir şey
		// (ör. 'yazdır("...")' çağrısının '(' işareti) alan değildir.
		if next := p.peekToken(); next.Type != lexer.TokRBrace && next.Type != lexer.TokComma &&
			next.Type != lexer.TokSemicolon && next.Type != lexer.TokEOF && next.Line == p.curToken().Line {
			return invalid(fieldTok, messages.New(messages.H0149, p.tokenName(fieldTok), p.tokenDescription(next)))
		}
		if field.DefaultValue != nil {
			lastDefault = field.Name
		} else if lastDefault != "" {
			p.syntaxError(fieldTok, messages.H0276, field.Name, lastDefault, stmt.Name)
		}
		if checkName(fieldTok, field.Name, true) {
			stmt.Fields = append(stmt.Fields, field)
		}
		p.nextToken()
	}

	return stmt
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
	p.syntaxError(open, messages.H0275, p.word(codes.KwStruct), name, base.String(), "")
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
