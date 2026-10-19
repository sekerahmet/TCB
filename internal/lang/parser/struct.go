package parser

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

func (p *Parser) parseStructStatement() *ast.StructStatement {
	tok := p.curToken()
	if !p.expectPeek(lexer.TOKEN_IDENT) {
		return nil
	}
	stmt := &ast.StructStatement{
		Token:   tok,
		Name:    p.curToken().Literal,
		Fields:  []ast.StructField{},
		Methods: make(map[string]*ast.FunctionLiteral),
	}

	if !p.expectPeek(lexer.TOKEN_LBRACE) {
		return nil
	}
	structOpen := p.curToken()
	defer func() { p.requireClosingBrace(structOpen) }()
	p.nextToken()

	// Yapı gövdesinde yalnızca alanlar ve fonksiyon tanımları bulunur; her ad bir kez
	// tanımlanır. Başka içerik sessizce alan sayılmaz ya da atlanmaz: sözdizimi hatasıdır.
	defined := map[string]bool{}
	checkName := func(tok lexer.Token, name, kind string) bool {
		if defined[name] {
			p.recordError(tok, fmt.Sprintf("Sözdizimi hatası — '%s' %s '%s' yapısında iki kez tanımlandı (her alan ve metot adı bir kez kullanılır)", name, kind, stmt.Name))
			return false
		}
		defined[name] = true
		return true
	}
	invalid := func(tok lexer.Token, what string) *ast.StructStatement {
		p.recordError(tok, "Sözdizimi hatası — yapı gövdesinde yalnızca alanlar (ad, ad: Tip, ad = değer) ve fonksiyon tanımları bulunabilir; "+what)
		p.skipToBlockEnd()
		return stmt
	}
	for !p.curTokenIs(lexer.TOKEN_RBRACE) && !p.curTokenIs(lexer.TOKEN_EOF) {
		if p.curTokenIs(lexer.TOKEN_SEMICOLON) || p.curTokenIs(lexer.TOKEN_COMMA) {
			p.nextToken()
			continue
		}
		if p.curTokenIs(lexer.TOKEN_FUNCTION) {
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
				p.recordError(fnTok, "Sözdizimi hatası — yapı metodunun bir adı olmalı: fonksiyon ad(...) { ... }")
				p.skipToBlockEnd()
				break
			}
			if !checkName(fnTok, fn.Name, "metodu") {
				p.skipToBlockEnd()
				break
			}
			stmt.Methods[fn.Name] = fn
			p.nextToken()
			continue
		}
		if p.curTokenIs(lexer.TOKEN_VAR) {
			p.nextToken()
		}
		if !p.curTokenIs(lexer.TOKEN_IDENT) {
			return invalid(p.curToken(), tokenDescription(p.curToken())+" bir alan tanımı değil")
		}
		fieldTok := p.curToken()
		p.role(fieldTok, RoleStructField)
		field := ast.StructField{Name: fieldTok.Literal}
		if p.peekTokenIs(lexer.TOKEN_COLON) {
			p.nextToken() // ':'
			if p.expectPeek(lexer.TOKEN_IDENT) {
				field.TypeName = p.curToken().Literal
				p.role(p.curToken(), RoleType)
			}
		}
		if p.peekTokenIs(lexer.TOKEN_ASSIGN) {
			p.nextToken() // '='
			p.nextToken()
			field.DefaultValue = p.parseExpression(LOWEST)
		}
		// Alan virgül, ';', satır sonu ya da '}' ile biter: aynı satırda başka bir şey
		// (ör. 'yazdır("...")' çağrısının '(' işareti) alan değildir.
		if next := p.peekToken(); next.Type != lexer.TOKEN_RBRACE && next.Type != lexer.TOKEN_COMMA &&
			next.Type != lexer.TOKEN_SEMICOLON && next.Type != lexer.TOKEN_EOF && next.Line == p.curToken().Line {
			return invalid(fieldTok, fmt.Sprintf("'%s' ardından gelen %s bir alan tanımı değil (alanlar virgül ya da satır sonuyla ayrılır)", fieldTok.Literal, tokenDescription(next)))
		}
		if !checkName(fieldTok, field.Name, "alanı") {
			p.skipToBlockEnd()
			break
		}
		stmt.Fields = append(stmt.Fields, field)
		p.nextToken()
	}

	return stmt
}
