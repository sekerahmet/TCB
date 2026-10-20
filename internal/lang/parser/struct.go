package parser

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func (p *Parser) parseStructStatement() *ast.StructStatement {
	tok := p.curToken()
	if !p.expectPeek(lexer.TokIdent) {
		return nil
	}
	stmt := &ast.StructStatement{
		Token:   tok,
		Name:    p.curToken().Literal,
		Fields:  []ast.StructField{},
		Methods: make(map[string]*ast.FunctionLiteral),
	}

	if !p.expectPeek(lexer.TokLBrace) {
		return nil
	}
	structOpen := p.curToken()
	defer func() { p.requireClosingBrace(structOpen) }()
	p.nextToken()

	// Yapı gövdesinde yalnızca alanlar ve fonksiyon tanımları bulunur; her ad bir kez
	// tanımlanır. Başka içerik sessizce alan sayılmaz ya da atlanmaz: sözdizimi hatasıdır.
	defined := map[string]bool{}
	checkName := func(tok lexer.Token, name string, kind messages.Code) bool {
		if defined[name] {
			p.syntaxError(tok, messages.H0144, name, messages.Text(kind), stmt.Name)
			return false
		}
		defined[name] = true
		return true
	}
	invalid := func(tok lexer.Token, what string) *ast.StructStatement {
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
			if !checkName(fnTok, fn.Name, messages.H0145) {
				p.skipToBlockEnd()
				break
			}
			stmt.Methods[fn.Name] = fn
			p.nextToken()
			continue
		}
		if p.curTokenIs(lexer.TokVar) {
			p.nextToken()
		}
		if !p.curTokenIs(lexer.TokIdent) {
			return invalid(p.curToken(), messages.Text(messages.H0148, tokenDescription(p.curToken())))
		}
		fieldTok := p.curToken()
		p.role(fieldTok, RoleStructField)
		field := ast.StructField{Name: fieldTok.Literal}
		if p.peekTokenIs(lexer.TokColon) {
			p.nextToken() // ':'
			if p.expectPeek(lexer.TokIdent) {
				field.TypeName = p.curToken().Literal
				p.role(p.curToken(), RoleType)
			}
		}
		if p.peekTokenIs(lexer.TokAssign) {
			p.nextToken() // '='
			p.nextToken()
			field.DefaultValue = p.parseExpression(precLowest)
		}
		// Alan virgül, ';', satır sonu ya da '}' ile biter: aynı satırda başka bir şey
		// (ör. 'yazdır("...")' çağrısının '(' işareti) alan değildir.
		if next := p.peekToken(); next.Type != lexer.TokRBrace && next.Type != lexer.TokComma &&
			next.Type != lexer.TokSemicolon && next.Type != lexer.TokEOF && next.Line == p.curToken().Line {
			return invalid(fieldTok, messages.Text(messages.H0149, fieldTok.Literal, tokenDescription(next)))
		}
		if !checkName(fieldTok, field.Name, messages.H0146) {
			p.skipToBlockEnd()
			break
		}
		stmt.Fields = append(stmt.Fields, field)
		p.nextToken()
	}

	return stmt
}
