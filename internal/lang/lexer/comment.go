package lexer

import "github.com/sekerahmet/TCB/internal/lang/messages"

func (l *Lexer) skipWhitespaceAndComments() {
	for {
		// Boşluk karakterlerini atla
		for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
			l.readChar()
		}
		// '#' yorum değildir (her işin tek yazımı: satır yorumu yalnızca '//'). Başka
		// dillerden alışkanlıkla yazılmış olabileceği için açık bir hata verilir; satırın
		// geri kalanı atlanır ki ardından gelen sözcükler ikinci bir hata üretmesin.
		if l.ch == '#' {
			l.addError(l.line, l.column, messages.New(messages.H0005))
			for l.ch != '\n' && l.ch != 0 {
				l.readChar()
			}
			continue
		}
		// '//' ile başlayan C/Go tarzı yorum satırı
		if l.ch == '/' && l.peekChar() == '/' {
			for l.ch != '\n' && l.ch != 0 {
				l.readChar()
			}
			continue
		}
		// '/* ... */' çok satırlı yorum
		if l.ch == '/' && l.peekChar() == '*' {
			startLine, startCol := l.line, l.column
			l.readChar()
			l.readChar()
			for !(l.ch == '*' && l.peekChar() == '/') && l.ch != 0 {
				l.readChar()
			}
			if l.ch != 0 {
				l.readChar()
				l.readChar()
			} else {
				l.addError(startLine, startCol, messages.New(messages.H0006))
			}
			continue
		}
		break
	}
}
