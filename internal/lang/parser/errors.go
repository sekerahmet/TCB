package parser

import (
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Ayrıştırıcının hata mesajları kullanıcıya iç token adlarını (DOSYA_SONU, SONUÇ,
// TANIMLAYICI) göstermez; bunların yerine "dosya sonu", "'TR_return' anahtar kelimesi",
// "'x' adı" gibi anlatımlar kullanılır. Anlatımlar ileti parçasıdır (*messages.Error): dış iletinin
// dilinde yazılır. Dilin sözcüğü dosyada kimlik yazımıyla, yazılan satırda dilin sözcüğüyle
// adlandırılır (codes.WordRef).

// typeDescription beklenen bir token türünü anlatır (ör. "'{'", "bir ad",
// "'TR_in' anahtar kelimesi").
func (p *Parser) typeDescription(t lexer.TokenType) *messages.Error {
	switch t {
	case lexer.TokEOF:
		return messages.New(messages.H0110)
	case lexer.TokIdent:
		return messages.New(messages.H0111)
	case lexer.TokInt:
		return messages.New(messages.H0112)
	case lexer.TokFloat:
		return messages.New(messages.H0113)
	case lexer.TokString:
		return messages.New(messages.H0114)
	case lexer.TokIllegal:
		return messages.New(messages.H0115)
	}
	if k := lexer.KeywordCode(t); k != codes.None {
		return messages.New(messages.H0116, p.word(k))
	}
	return messages.New(messages.H0117, string(t))
}

// tokenDescription kaynakta bulunan bir token'ı yazıldığı biçimiyle anlatır
// (ör. "'x' adı", "42 sayısı", "'TR_return' anahtar kelimesi", "dosya sonu").
func (p *Parser) tokenDescription(tok lexer.Token) *messages.Error {
	switch tok.Type {
	case lexer.TokEOF:
		return messages.New(messages.H0110)
	case lexer.TokIdent:
		return messages.New(messages.H0118, p.tokenName(tok))
	case lexer.TokInt, lexer.TokFloat:
		return messages.New(messages.H0119, tok.Literal)
	case lexer.TokString:
		return messages.New(messages.H0120, shortText(tok.Literal))
	case lexer.TokIllegal:
		return messages.New(messages.H0121, tok.Literal)
	}
	if lexer.KeywordCode(tok.Type) != codes.None {
		return messages.New(messages.H0116, p.tokenName(tok))
	}
	return messages.New(messages.H0117, tok.Literal)
}

// tokenName iletide bir sözcük biriminin adıdır: dilin sözcüğü (kod taşıyan) dosyada kimlik
// yazımıyla, yazılan satırda iletinin dilinde (p.word); kullanıcının adı olduğu gibi.
func (p *Parser) tokenName(tok lexer.Token) any {
	if tok.Code != codes.None {
		return p.word(tok.Code)
	}
	return tok.Literal
}

// word dilin k sözcüğünün iletideki adıdır: dosyada yazımı (TR_break), yazılan satırda iletinin
// dilindeki sözcük (codes.WordRef).
func (p *Parser) word(k codes.Code) any { return codes.WordRef(k, p.l.WrittenIn()) }

// shortText hata mesajında gösterilecek metin sabitini kısaltır.
func shortText(s string) string {
	const limit = 30
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	return string([]rune(s)[:limit-1]) + "…"
}

// unexpectedHint sık yapılan yanlışlar için hata mesajına eklenecek ipucunu verir.
func unexpectedHint(expected lexer.TokenType, found lexer.Token) *messages.Error {
	switch {
	case expected == lexer.TokLBrace && found.Type == lexer.TokAssign:
		return messages.New(messages.H0130)
	case expected == lexer.TokRParen && found.Type == lexer.TokEOF,
		expected == lexer.TokRBracket && found.Type == lexer.TokEOF,
		expected == lexer.TokRBrace && found.Type == lexer.TokEOF:
		return messages.New(messages.H0131, string(expected))
	}
	return nil // öneri yok: iletide boş yazılır
}

// recordError bir sözdizimi hatası kaydeder. Bir deyimde yalnızca ilk hata kaydedilir:
// ardından gelenler çoğunlukla ilkinin sonucudur ve kullanıcıyı yanıltır. Deyim
// döngüleri senkronizeOl ile hatalı deyimin geri kalanını atlayıp kaydı yeniden açar.
//
// The record is at the token, from its start to its end (2.13); fixes are the changes that would put
// the error right (messages.Fix; the fix of an error that is mended by one known text, such as a
// '//' that starts a line becoming '#', is applicable).
func (p *Parser) recordError(tok lexer.Token, err error, fixes ...messages.Fix) {
	if p.errorMode || p.depthExceeded {
		return
	}
	p.errorMode = true
	p.errorLine = tok.Line
	p.errors = append(p.errors, messages.AtRange(tok.Range(), err).WithFix(fixes...))
}

// syntaxError, "Sözdizimi hatası — ..." önekli (H0100) bir sözdizimi hatası kaydeder.
func (p *Parser) syntaxError(tok lexer.Token, code messages.Code, args ...any) {
	p.syntaxErrorFix(tok, nil, code, args...)
}

// syntaxErrorFix is syntaxError with the fixes that mend it.
func (p *Parser) syntaxErrorFix(tok lexer.Token, fixes []messages.Fix, code messages.Code, args ...any) {
	p.recordError(tok, messages.New(messages.H0100, messages.New(code, args...)), fixes...)
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
