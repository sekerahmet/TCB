package parser

import (
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Ayrıştırıcının hata mesajları kullanıcıya iç token adlarını (DOSYA_SONU, SONUÇ,
// TANIMLAYICI) göstermez; bunların yerine "dosya sonu", "'sonuç' anahtar kelimesi",
// "'x' adı" gibi Türkçe anlatımlar kullanılır.

// typeDescription beklenen bir token türünü anlatır (ör. "'{'", "bir ad",
// "'içinde' anahtar kelimesi").
func typeDescription(t lexer.TokenType) string {
	switch t {
	case lexer.TOKEN_EOF:
		return messages.Text(messages.H0110)
	case lexer.TOKEN_IDENT:
		return messages.Text(messages.H0111)
	case lexer.TOKEN_INT:
		return messages.Text(messages.H0112)
	case lexer.TOKEN_FLOAT:
		return messages.Text(messages.H0113)
	case lexer.TOKEN_STRING:
		return messages.Text(messages.H0114)
	case lexer.TOKEN_ILLEGAL:
		return messages.Text(messages.H0115)
	}
	if spelling := keywordSpelling(t); spelling != "" {
		return messages.Text(messages.H0116, spelling)
	}
	return messages.Text(messages.H0117, string(t))
}

// tokenDescription kaynakta bulunan bir token'ı yazıldığı biçimiyle anlatır
// (ör. "'x' adı", "42 sayısı", "'sonuç' anahtar kelimesi", "dosya sonu").
func tokenDescription(tok lexer.Token) string {
	switch tok.Type {
	case lexer.TOKEN_EOF:
		return messages.Text(messages.H0110)
	case lexer.TOKEN_IDENT:
		return messages.Text(messages.H0118, tok.Literal)
	case lexer.TOKEN_INT, lexer.TOKEN_FLOAT:
		return messages.Text(messages.H0119, tok.Literal)
	case lexer.TOKEN_STRING:
		return messages.Text(messages.H0120, shortText(unescapeBraces(tok.Literal)))
	case lexer.TOKEN_ILLEGAL:
		return messages.Text(messages.H0121, tok.Literal)
	}
	if keywordSpelling(tok.Type) != "" {
		return messages.Text(messages.H0116, tok.Literal)
	}
	return messages.Text(messages.H0117, tok.Literal)
}

// keywordSpelling anahtar kelime türünün sözlükteki adını bulur; tür bir anahtar
// kelime değilse "" döner.
func keywordSpelling(t lexer.TokenType) string {
	return lexer.KeywordName(t)
}

// shortText hata mesajında gösterilecek metin sabitini kısaltır.
func shortText(s string) string {
	const limit = 30
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	return string([]rune(s)[:limit-1]) + "…"
}

// unexpectedHint sık yapılan yanlışlar için hata mesajına eklenecek ipucunu verir.
func unexpectedHint(expected lexer.TokenType, found lexer.Token) string {
	switch {
	case expected == lexer.TOKEN_LBRACE && found.Type == lexer.TOKEN_ASSIGN:
		return messages.Text(messages.H0130)
	case expected == lexer.TOKEN_RPAREN && found.Type == lexer.TOKEN_EOF,
		expected == lexer.TOKEN_RBRACKET && found.Type == lexer.TOKEN_EOF,
		expected == lexer.TOKEN_RBRACE && found.Type == lexer.TOKEN_EOF:
		return messages.Text(messages.H0131, string(expected))
	}
	return ""
}

// recordError bir sözdizimi hatası kaydeder. Bir deyimde yalnızca ilk hata kaydedilir:
// ardından gelenler çoğunlukla ilkinin sonucudur ve kullanıcıyı yanıltır. Deyim
// döngüleri senkronizeOl ile hatalı deyimin geri kalanını atlayıp kaydı yeniden açar.
func (p *Parser) recordError(tok lexer.Token, err error) {
	if p.errorMode || p.depthExceeded {
		return
	}
	p.errorMode = true
	p.errorLine = tok.Line
	p.errors = append(p.errors, messages.At(tok.Line, tok.Column, err))
}

// syntaxError, "Sözdizimi hatası — ..." önekli (H0100) bir sözdizimi hatası kaydeder.
func (p *Parser) syntaxError(tok lexer.Token, code messages.Code, args ...any) {
	p.recordError(tok, messages.New(messages.H0100, messages.New(code, args...)))
}

// skipToBlockEnd, içinde bulunulan '{ ... }' bloğunu kapatan '}' işaretine kadar
// (iç içe blokları sayarak) ilerler; dosya biterse dosya sonunda durur.
func (p *Parser) skipToBlockEnd() {
	nesting := 0
	for !p.curTokenIs(lexer.TOKEN_EOF) {
		if p.curTokenIs(lexer.TOKEN_LBRACE) {
			nesting++
		} else if p.curTokenIs(lexer.TOKEN_RBRACE) {
			if nesting == 0 {
				return
			}
			nesting--
		}
		p.nextToken()
	}
}

// isOpener ve kapayanMi parantez, köşeli parantez ve süslü parantezleri tanır.
func isOpener(t lexer.TokenType) bool {
	return t == lexer.TOKEN_LBRACE || t == lexer.TOKEN_LPAREN || t == lexer.TOKEN_LBRACKET
}

func isCloser(t lexer.TokenType) bool {
	return t == lexer.TOKEN_RBRACE || t == lexer.TOKEN_RPAREN || t == lexer.TOKEN_RBRACKET
}

// synchronize, hatalı bir deyimden sonra (deyim deyimBas konumundaki token'da
// başlamıştı) deyimin geri kalanını atlar: açılıp kapanan parantezleri ve blokları
// sayarak, yeni bir satırda başlayan ilk deyime, ';' işaretine ya da içinde bulunulan
// bloğu kapatan '}' işaretine kadar ilerler. 'değilse' ve 'yakala' ile başlayan
// satırlar hatalı deyimin devamı sayılır. ustDuzey dosyanın en üst düzeyidir: orada
// kapatılacak blok olmadığından sahipsiz '}' de atlanır.
func (p *Parser) synchronize(stmtStart int, topLevel bool) {
	if !p.errorMode {
		return
	}
	defer func() { p.errorMode = false }()

	// Hatalı deyim, içinde bulunduğu bloğu kapatan '}' işaretine kadar okuduysa
	// (ör. 'eğer x { yazdır( }'), o '}' bloğa geri bırakılır.
	if !topLevel && p.curTokenIs(lexer.TOKEN_RBRACE) && p.pos > stmtStart {
		brace := 0
		for i := stmtStart; i <= p.pos && i < len(p.tokens); i++ {
			switch p.tokens[i].Type {
			case lexer.TOKEN_LBRACE:
				brace++
			case lexer.TOKEN_RBRACE:
				brace--
			}
		}
		if brace < 0 {
			p.pos--
			return
		}
	}
	// Sayım hatanın ardından başlar: kapatılmamış bir '(' dosyanın geri kalanını yutmasın.
	nesting := 0
	for !p.peekTokenIs(lexer.TOKEN_EOF) {
		pk := p.peekToken()
		if nesting == 0 {
			if pk.Type == lexer.TOKEN_SEMICOLON || (pk.Type == lexer.TOKEN_RBRACE && !topLevel) {
				return
			}
			newline := pk.Line > p.errorLine && pk.Line > p.curToken().Line
			if newline && pk.Type != lexer.TOKEN_ELSE && pk.Type != lexer.TOKEN_CATCH {
				return
			}
		}
		if isOpener(pk.Type) {
			nesting++
		} else if isCloser(pk.Type) && nesting > 0 {
			nesting--
		}
		p.nextToken()
	}
}
