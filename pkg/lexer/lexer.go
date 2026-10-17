package lexer

import (
	"strings"
	"unicode"
)

type Lexer struct {
	input        []rune
	position     int  // mevcut karakterin indeksi
	readPosition int  // okunacak sonraki karakterin indeksi
	ch           rune // incelenen mevcut karakter
	line         int
	column       int
}

func New(input string) *Lexer {
	l := &Lexer{
		input:  []rune(input),
		line:   1,
		column: 0,
	}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition++
	if l.ch == '\n' {
		l.line++
		l.column = 0
	} else {
		l.column++
	}
}

func (l *Lexer) peekChar() rune {
	if l.readPosition >= len(l.input) {
		return 0
	}
	return l.input[l.readPosition]
}

func (l *Lexer) peekCharN(n int) rune {
	idx := l.position + n
	if idx >= len(l.input) {
		return 0
	}
	return l.input[idx]
}

func (l *Lexer) NextToken() Token {
	l.skipWhitespaceAndComments()

	line := l.line
	col := l.column
	var tok Token

	switch l.ch {
	case '=':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TOKEN_EQ, Literal: "==", Line: line, Column: col}
		} else if l.peekChar() == '>' {
			l.readChar()
			tok = Token{Type: TOKEN_ARROW, Literal: "=>", Line: line, Column: col}
		} else {
			tok = Token{Type: TOKEN_ASSIGN, Literal: "=", Line: line, Column: col}
		}
	case ':':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TOKEN_WALRUS, Literal: ":=", Line: line, Column: col}
		} else {
			tok = Token{Type: TOKEN_COLON, Literal: ":", Line: line, Column: col}
		}
	case '+':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TOKEN_PLUS_ASSIGN, Literal: "+=", Line: line, Column: col}
		} else {
			tok = Token{Type: TOKEN_PLUS, Literal: "+", Line: line, Column: col}
		}
	case '-':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TOKEN_MINUS_ASSIGN, Literal: "-=", Line: line, Column: col}
		} else {
			tok = Token{Type: TOKEN_MINUS, Literal: "-", Line: line, Column: col}
		}
	case '*':
		if l.peekChar() == '*' {
			l.readChar()
			tok = Token{Type: TOKEN_POWER, Literal: "**", Line: line, Column: col}
		} else if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TOKEN_MUL_ASSIGN, Literal: "*=", Line: line, Column: col}
		} else {
			tok = Token{Type: TOKEN_ASTERISK, Literal: "*", Line: line, Column: col}
		}
	case '/':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TOKEN_DIV_ASSIGN, Literal: "/=", Line: line, Column: col}
		} else {
			tok = Token{Type: TOKEN_SLASH, Literal: "/", Line: line, Column: col}
		}
	case '%':
		tok = Token{Type: TOKEN_PERCENT, Literal: "%", Line: line, Column: col}
	case '@':
		tok = Token{Type: TOKEN_MATMUL, Literal: "@", Line: line, Column: col}
	case '!':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TOKEN_NOT_EQ, Literal: "!=", Line: line, Column: col}
		} else {
			tok = Token{Type: TOKEN_BANG, Literal: "!", Line: line, Column: col}
		}
	case '<':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TOKEN_LTE, Literal: "<=", Line: line, Column: col}
		} else {
			tok = Token{Type: TOKEN_LT, Literal: "<", Line: line, Column: col}
		}
	case '>':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TOKEN_GTE, Literal: ">=", Line: line, Column: col}
		} else {
			tok = Token{Type: TOKEN_GT, Literal: ">", Line: line, Column: col}
		}
	case '&':
		if l.peekChar() == '&' {
			l.readChar()
			tok = Token{Type: TOKEN_AND, Literal: "&&", Line: line, Column: col}
		} else {
			tok = Token{Type: TOKEN_ILLEGAL, Literal: string(l.ch), Line: line, Column: col}
		}
	case '|':
		if l.peekChar() == '|' {
			l.readChar()
			tok = Token{Type: TOKEN_OR, Literal: "||", Line: line, Column: col}
		} else if l.peekChar() == '>' {
			l.readChar()
			tok = Token{Type: TOKEN_PIPE, Literal: "|>", Line: line, Column: col}
		} else {
			tok = Token{Type: TOKEN_ILLEGAL, Literal: string(l.ch), Line: line, Column: col}
		}
	case '?':
		if l.peekChar() == '?' {
			l.readChar()
			tok = Token{Type: TOKEN_NULL_COALESC, Literal: "??", Line: line, Column: col}
		} else if l.peekChar() == '.' {
			l.readChar()
			tok = Token{Type: TOKEN_SAFE_DOT, Literal: "?.", Line: line, Column: col}
		} else {
			tok = Token{Type: TOKEN_QUESTION, Literal: "?", Line: line, Column: col}
		}
	case '.':
		if l.peekChar() == '.' {
			l.readChar()
			if l.peekChar() == '=' {
				l.readChar()
				tok = Token{Type: TOKEN_RANGE_INCL, Literal: "..=", Line: line, Column: col}
			} else {
				tok = Token{Type: TOKEN_RANGE, Literal: "..", Line: line, Column: col}
			}
		} else {
			tok = Token{Type: TOKEN_DOT, Literal: ".", Line: line, Column: col}
		}
	case ',':
		tok = Token{Type: TOKEN_COMMA, Literal: ",", Line: line, Column: col}
	case ';':
		tok = Token{Type: TOKEN_SEMICOLON, Literal: ";", Line: line, Column: col}
	case '(':
		tok = Token{Type: TOKEN_LPAREN, Literal: "(", Line: line, Column: col}
	case ')':
		tok = Token{Type: TOKEN_RPAREN, Literal: ")", Line: line, Column: col}
	case '{':
		tok = Token{Type: TOKEN_LBRACE, Literal: "{", Line: line, Column: col}
	case '}':
		tok = Token{Type: TOKEN_RBRACE, Literal: "}", Line: line, Column: col}
	case '[':
		tok = Token{Type: TOKEN_LBRACKET, Literal: "[", Line: line, Column: col}
	case ']':
		tok = Token{Type: TOKEN_RBRACKET, Literal: "]", Line: line, Column: col}
	case '"', '\'':
		quote := l.ch
		str := l.readString(quote)
		tok = Token{Type: TOKEN_STRING, Literal: str, Line: line, Column: col}
		return tok
	case 0:
		tok = Token{Type: TOKEN_EOF, Literal: "", Line: line, Column: col}
		return tok
	default:
		if isLetter(l.ch) {
			lit := l.readIdentifier()
			tokType := LookupIdent(lit)
			return Token{Type: tokType, Literal: lit, Line: line, Column: col}
		} else if isDigit(l.ch) {
			lit, isFloat := l.readNumber()
			if isFloat {
				return Token{Type: TOKEN_FLOAT, Literal: lit, Line: line, Column: col}
			}
			return Token{Type: TOKEN_INT, Literal: lit, Line: line, Column: col}
		} else {
			tok = Token{Type: TOKEN_ILLEGAL, Literal: string(l.ch), Line: line, Column: col}
		}
	}

	l.readChar()
	return tok
}

func (l *Lexer) skipWhitespaceAndComments() {
	for {
		// Boşluk karakterlerini atla
		for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
			l.readChar()
		}
		// '#' ile başlayan Python tarzı yorum satırı
		if l.ch == '#' {
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
			l.readChar()
			l.readChar()
			for !(l.ch == '*' && l.peekChar() == '/') && l.ch != 0 {
				l.readChar()
			}
			if l.ch != 0 {
				l.readChar()
				l.readChar()
			}
			continue
		}
		break
	}
}

func (l *Lexer) readIdentifier() string {
	start := l.position
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return string(l.input[start:l.position])
}

func (l *Lexer) readNumber() (string, bool) {
	var sb strings.Builder
	isFloat := false

	for isDigit(l.ch) || l.ch == '_' {
		if l.ch != '_' {
			sb.WriteRune(l.ch)
		}
		l.readChar()
	}

	// Ondalık kısım kontrolü ('1..10' aralığı ile karışmaması için peekChar() != '.' kontrolü)
	if l.ch == '.' && isDigit(l.peekChar()) {
		isFloat = true
		sb.WriteRune(l.ch)
		l.readChar()
		for isDigit(l.ch) || l.ch == '_' {
			if l.ch != '_' {
				sb.WriteRune(l.ch)
			}
			l.readChar()
		}
	}

	return sb.String(), isFloat
}

func (l *Lexer) readString(quote rune) string {
	var sb strings.Builder
	l.readChar() // açılış tırnağını geç
	braceDepth := 0

	for l.ch != 0 {
		if l.ch == '\\' {
			l.readChar()
			switch l.ch {
			case 'n':
				sb.WriteRune('\n')
			case 't':
				sb.WriteRune('\t')
			case 'r':
				sb.WriteRune('\r')
			case '"':
				sb.WriteRune('"')
			case '\'':
				sb.WriteRune('\'')
			case '\\':
				sb.WriteRune('\\')
			case '{':
				// Kaçışlı süslü parantez (interpolasyon değil)
				sb.WriteString("\\{")
			case '}':
				sb.WriteString("\\}")
			default:
				sb.WriteRune(l.ch)
			}
			l.readChar()
			continue
		}

		if l.ch == '{' {
			braceDepth++
		} else if l.ch == '}' && braceDepth > 0 {
			braceDepth--
		} else if l.ch == quote && braceDepth == 0 {
			l.readChar() // kapanış tırnağını geç
			break
		}

		sb.WriteRune(l.ch)
		l.readChar()
	}
	return sb.String()
}

func isLetter(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

func isDigit(ch rune) bool {
	return '0' <= ch && ch <= '9'
}
