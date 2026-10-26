package parser

import (
	"github.com/sekerahmet/TCB/internal/lang/codes"
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
	case lexer.TokEOF:
		return messages.Text(messages.H0110)
	case lexer.TokIdent:
		return messages.Text(messages.H0111)
	case lexer.TokInt:
		return messages.Text(messages.H0112)
	case lexer.TokFloat:
		return messages.Text(messages.H0113)
	case lexer.TokString:
		return messages.Text(messages.H0114)
	case lexer.TokIllegal:
		return messages.Text(messages.H0115)
	}
	if spelling := lexer.KeywordName(t); spelling != "" {
		return messages.Text(messages.H0116, spelling)
	}
	return messages.Text(messages.H0117, string(t))
}

// tokenDescription kaynakta bulunan bir token'ı yazıldığı biçimiyle anlatır
// (ör. "'x' adı", "42 sayısı", "'sonuç' anahtar kelimesi", "dosya sonu").
func tokenDescription(tok lexer.Token) string {
	switch tok.Type {
	case lexer.TokEOF:
		return messages.Text(messages.H0110)
	case lexer.TokIdent:
		return messages.Text(messages.H0118, tokenName(tok))
	case lexer.TokInt, lexer.TokFloat:
		return messages.Text(messages.H0119, tok.Literal)
	case lexer.TokString:
		return messages.Text(messages.H0120, shortText(unescapeBraces(tok.Literal)))
	case lexer.TokIllegal:
		return messages.Text(messages.H0121, tok.Literal)
	}
	if lexer.KeywordName(tok.Type) != "" {
		return messages.Text(messages.H0116, tokenName(tok))
	}
	return messages.Text(messages.H0117, tok.Literal)
}

// tokenName iletide bir sözcük biriminin adıdır: dilin sözcüğü (kod taşıyan) iletinin dilinde
// adlandırılır, içerideki kimliğiyle değil; kullanıcının adı olduğu gibi.
func tokenName(tok lexer.Token) any {
	if tok.Code != codes.None {
		return codes.NameRef(tok.Code)
	}
	return tok.Literal
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
	case expected == lexer.TokLBrace && found.Type == lexer.TokAssign:
		return messages.Text(messages.H0130)
	case expected == lexer.TokRParen && found.Type == lexer.TokEOF,
		expected == lexer.TokRBracket && found.Type == lexer.TokEOF,
		expected == lexer.TokRBrace && found.Type == lexer.TokEOF:
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
	for !p.curTokenIs(lexer.TokEOF) {
		if p.curTokenIs(lexer.TokLBrace) {
			nesting++
		} else if p.curTokenIs(lexer.TokRBrace) {
			if nesting == 0 {
				return
			}
			nesting--
		}
		p.nextToken()
	}
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
	if !topLevel && p.curTokenIs(lexer.TokRBrace) && p.pos > stmtStart {
		brace := 0
		for i := stmtStart; i <= p.pos && i < len(p.tokens); i++ {
			switch p.tokens[i].Type {
			case lexer.TokLBrace:
				brace++
			case lexer.TokRBrace:
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
	for !p.peekTokenIs(lexer.TokEOF) {
		pk := p.peekToken()
		if nesting == 0 {
			if pk.Type == lexer.TokSemicolon || (pk.Type == lexer.TokRBrace && !topLevel) {
				return
			}
			newline := pk.Line > p.errorLine && pk.Line > p.curToken().Line
			if newline && pk.Type != lexer.TokElse && pk.Type != lexer.TokCatch {
				return
			}
		}
		if pk.Type.IsOpener() {
			nesting++
		} else if pk.Type.IsCloser() && nesting > 0 {
			nesting--
		}
		p.nextToken()
	}
}
