// Package lexer is TR's lexer: it splits source text into tokens (keywords, names, numbers,
// strings, operators, comments) with line and column information. Keyword spellings come
// from the code list (internal/lang/codes); the parser and the formatter work on the tokens
// this package produces.
//
// Layer: language core (internal/lang); imports only internal/lang packages.
package lexer

import (
	"fmt"
	"strings"
	"unicode"
)

// Braces escaped as '\{' and '\}' inside a string are carried in the token as these
// private-use characters; the parser turns them back into plain '{' and '}'. This keeps
// '\\{' (a backslash followed by an interpolation) apart from '\{' (a plain brace).
const (
	EscapedLBrace = '\uE000'
	EscapedRBrace = '\uE001'
)

type Lexer struct {
	input        []rune
	position     int  // mevcut karakterin indeksi
	readPosition int  // okunacak sonraki karakterin indeksi
	ch           rune // incelenen mevcut karakter
	line         int
	column       int
	errors       []string
	tokenStart   int // son okunan sözcük biriminin başladığı karakter (rune) indeksi

	// coded, kaynağın kodlu biçimde (17.10) yazıldığını söyler: kodlar (A001, G100) dilin
	// sözcükleri, kod olmayan her ad kullanıcının adıdır (coded.go).
	coded bool
	// root dosyanın çözümleyicisidir (kökte kendisi). Metin içi yerleştirmelerin alt
	// çözümleyicileri konumlarını, metin haritalarını ve kayıtlarını kökle paylaşır.
	root *Lexer
	// posMap alt çözümleyicide girdinin her karakterinin kökteki konumudur (interpolation.go).
	posMap []int32
	// stringInfos kökte, yerleştirme içeren metin sabitlerinin bilgisidir (sözcük biriminin satır
	// ve sütunuyla anahtarlı; interpolation.go); metinTampon ve yerTampon okunmakta olan metnin
	// konumlarını ve yerleştirme aralıklarını biriktirir.
	stringInfos map[[2]int]*stringInfo
	strBuf      []int32
	posBuf      [][2]int
	// lineStarts kökte satırların başladığı karakter indeksleridir (gerektiğinde kurulur).
	lineStarts []int
	// rec açıksa (Kaydet) kökte üretilen bütün sözcük birimleri kaynak aralıklarıyla tutulur.
	rec *[]Record
	// shifts kodlu kökte gösterimdeki sütun kaymalarıdır (coded.go); kaymaToplam birikimli
	// toplamları, kaymaSira kök çözümleyicinin sıradaki kayma noktasıdır.
	shifts   []shift
	shiftSum []int32
	shiftIdx int
}

// LastRange returns the source character (rune) range [bas, son) of the token returned by
// the last NextToken call. The formatter uses it to keep the raw text of tokens (escape
// sequences undecoded) and the comments between them.
func (l *Lexer) LastRange() (begin, end int) {
	end = l.position
	if end > len(l.input) {
		end = len(l.input)
	}
	return l.tokenStart, end
}

// Input returns the source the lexer reads (BOM removed, as runes).
func (l *Lexer) Input() []rune {
	return l.input
}

// Errors returns the errors found while lexing (e.g. an unterminated string).
func (l *Lexer) Errors() []string {
	return l.errors
}

func (l *Lexer) addError(line, col int, format string, args ...interface{}) {
	prefix := fmt.Sprintf("Satır %d, Sütun %d: ", line, col)
	l.errors = append(l.errors, prefix+fmt.Sprintf(format, args...))
}

// New returns a lexer for source text written in Turkish (a REPL line, an expression typed
// in the debugger). Files are read through internal/lang/source (NewBody).
func New(input string) *Lexer {
	// Bazı Windows editörlerinin dosya başına eklediği UTF-8 BOM'u (U+FEFF) atla;
	// aksi halde ilk satırda GEÇERSİZ bir sözcük birimi oluşur.
	l := newLexer(strings.TrimPrefix(input, string(rune(0xFEFF))))
	l.checkBidi()
	return l
}

func newLexer(input string) *Lexer {
	l := &Lexer{
		input:  []rune(input),
		line:   1,
		column: 0,
	}
	l.root = l
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
	if l.posMap != nil {
		// Alt çözümleyicinin konumu kökteki karşılığından okunur: kaçış dizileri metnin
		// değerinde kaynaktakinden kısadır.
		l.line, l.column = l.root.lineCol(int(l.sourcePos(l.position)))
		return
	}
	if l.ch == '\n' {
		l.line++
		l.column = 0
	} else {
		l.column++
	}
	if l.shifts != nil {
		l.applyShift()
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

// NextToken reads the next token; when recording is on (StartRecording) it adds the token
// with its source range to the root's record.
func (l *Lexer) NextToken() (result Token) {
	if l.root.rec != nil {
		defer func() { l.record(result) }()
	}
	l.skipWhitespaceAndComments()
	l.tokenStart = l.position

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
		str := l.readString(quote, line, col)
		tok = Token{Type: TOKEN_STRING, Literal: str, Line: line, Column: col}
		return tok
	case '`':
		return Token{Type: TOKEN_STRING, Literal: l.readRawString(line, col), Line: line, Column: col}
	case 0:
		if l.position < len(l.input) {
			return l.nulChar(line, col)
		}
		return Token{Type: TOKEN_EOF, Literal: "", Line: line, Column: col}
	case '$':
		if isLetter(l.peekChar()) {
			return l.escapedName(line, col)
		}
		tok = Token{Type: TOKEN_ILLEGAL, Literal: string(l.ch), Line: line, Column: col}
	default:
		if isLetter(l.ch) {
			name := l.readIdentifier()
			l.checkName(name, line, col)
			return l.nameToken(name, line, col)
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

	// Bilimsel yazım: 1e-5, 3.2E+4, 2e10 (üsten sonra en az bir rakam gelmelidir)
	if l.ch == 'e' || l.ch == 'E' {
		next := l.peekChar()
		marked := (next == '+' || next == '-') && isDigit(l.peekCharN(2))
		if isDigit(next) || marked {
			isFloat = true
			sb.WriteRune(l.ch)
			l.readChar()
			if marked {
				sb.WriteRune(l.ch)
				l.readChar()
			}
			for isDigit(l.ch) || l.ch == '_' {
				if l.ch != '_' {
					sb.WriteRune(l.ch)
				}
				l.readChar()
			}
		}
	}

	return sb.String(), isFloat
}

func isLetter(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

func isDigit(ch rune) bool {
	return '0' <= ch && ch <= '9'
}
