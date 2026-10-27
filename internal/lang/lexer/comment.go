package lexer

import "github.com/sekerahmet/TCB/internal/lang/messages"

// skipWhitespaceAndComments boşlukları ve yorumları atlar. Yorum '#' ile başlar, satır sonuna
// kadar sürer (2.2); dosyanın başlığı (# TR 4) ve ilk satırdaki '#!' de birer yorumdur. '//' tam
// bölme işlecidir; satır başında yazılınca (C ailesinin yorumu) sözdizimi hatasıdır, '/*' de öyle
// (2.2): ileti '#'i önerir, satırın (ya da '*/'ya kadar yorumun) geri kalanı atlanır ki ardındaki
// sözcükler ikinci bir hata üretmesin.
func (l *Lexer) skipWhitespaceAndComments() {
	for {
		for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
			if l.ch == '\n' {
				l.lineStart = true
			}
			l.readChar()
		}
		switch {
		case l.ch == '#':
			l.skipLine()
		case l.lineStart && l.ch == '/' && l.peekChar() == '/':
			l.addError(l.line, l.column, messages.New(messages.H0015, "//"))
			l.skipLine()
		case l.lineStart && l.ch == '/' && l.peekChar() == '*':
			l.addError(l.line, l.column, messages.New(messages.H0015, "/*"))
			for l.ch != 0 && !(l.ch == '*' && l.peekChar() == '/') {
				l.readChar()
			}
			if l.ch != 0 {
				l.readChar()
				l.readChar()
			}
		default:
			return
		}
	}
}

// skipLine satır sonuna kadar (satır sonu hariç) atlar.
func (l *Lexer) skipLine() {
	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}
}
