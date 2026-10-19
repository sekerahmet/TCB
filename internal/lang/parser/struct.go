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
	tanimli := map[string]bool{}
	adDenetle := func(tok lexer.Token, ad, tur string) bool {
		if tanimli[ad] {
			p.hataKaydet(tok, fmt.Sprintf("Sözdizimi hatası — '%s' %s '%s' yapısında iki kez tanımlandı (her alan ve metot adı bir kez kullanılır)", ad, tur, stmt.Name))
			return false
		}
		tanimli[ad] = true
		return true
	}
	gecersiz := func(tok lexer.Token, ne string) *ast.StructStatement {
		p.hataKaydet(tok, "Sözdizimi hatası — yapı gövdesinde yalnızca alanlar (ad, ad: Tip, ad = değer) ve fonksiyon tanımları bulunabilir; "+ne)
		p.blokSonunaAtla()
		return stmt
	}
	for !p.curTokenIs(lexer.TOKEN_RBRACE) && !p.curTokenIs(lexer.TOKEN_EOF) {
		if p.curTokenIs(lexer.TOKEN_SEMICOLON) || p.curTokenIs(lexer.TOKEN_COMMA) {
			p.nextToken()
			continue
		}
		if p.curTokenIs(lexer.TOKEN_FUNCTION) {
			fnTok := p.curToken()
			p.rol(p.peekToken(), RoleStructMethod)
			// Hatalı metod tanımında parseFunctionLiteral nil döner; tip dönüşümü panik yapmamalı
			fn, _ := p.parseFunctionLiteral().(*ast.FunctionLiteral)
			if fn == nil {
				// Hata zaten kaydedildi; yapının kapanışına kadar atla
				p.blokSonunaAtla()
				break
			}
			if fn.Name == "" {
				p.hataKaydet(fnTok, "Sözdizimi hatası — yapı metodunun bir adı olmalı: fonksiyon ad(...) { ... }")
				p.blokSonunaAtla()
				break
			}
			if !adDenetle(fnTok, fn.Name, "metodu") {
				p.blokSonunaAtla()
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
			return gecersiz(p.curToken(), tokenAciklamasi(p.curToken())+" bir alan tanımı değil")
		}
		alanTok := p.curToken()
		p.rol(alanTok, RoleStructField)
		field := ast.StructField{Name: alanTok.Literal}
		if p.peekTokenIs(lexer.TOKEN_COLON) {
			p.nextToken() // ':'
			if p.expectPeek(lexer.TOKEN_IDENT) {
				field.TypeName = p.curToken().Literal
				p.rol(p.curToken(), RoleType)
			}
		}
		if p.peekTokenIs(lexer.TOKEN_ASSIGN) {
			p.nextToken() // '='
			p.nextToken()
			field.DefaultValue = p.parseExpression(LOWEST)
		}
		// Alan virgül, ';', satır sonu ya da '}' ile biter: aynı satırda başka bir şey
		// (ör. 'yazdır("...")' çağrısının '(' işareti) alan değildir.
		if sonraki := p.peekToken(); sonraki.Type != lexer.TOKEN_RBRACE && sonraki.Type != lexer.TOKEN_COMMA &&
			sonraki.Type != lexer.TOKEN_SEMICOLON && sonraki.Type != lexer.TOKEN_EOF && sonraki.Line == p.curToken().Line {
			return gecersiz(alanTok, fmt.Sprintf("'%s' ardından gelen %s bir alan tanımı değil (alanlar virgül ya da satır sonuyla ayrılır)", alanTok.Literal, tokenAciklamasi(sonraki)))
		}
		if !adDenetle(alanTok, field.Name, "alanı") {
			p.blokSonunaAtla()
			break
		}
		stmt.Fields = append(stmt.Fields, field)
		p.nextToken()
	}

	return stmt
}
