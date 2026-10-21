// Package lexer is TR's lexer: it splits source text into tokens (keywords, names, numbers,
// strings, operators, comments) with line and column information. Keyword spellings come
// from the code list (internal/lang/codes); the parser and the formatter work on the tokens
// this package produces.
//
// Layer: language core (internal/lang); imports only internal/lang packages.
package lexer

import (
	"strings"
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
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
	errors       []error
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
	// lang ve words kökte, kodsuz gövdenin yazıldığı dil ve onun anahtar kelimeleridir
	// (NewBodyIn); nil ise kanonik dil (codes.Active).
	lang  *codes.Language
	words map[string]TokenType
}

// Language returns the language whose words the lexer reads: the language of an uncoded body
// (NewBodyIn), otherwise the active language (coded bodies are read in the active language).
func (l *Lexer) Language() *codes.Language {
	if l.root.lang != nil {
		return l.root.lang
	}
	return codes.Active()
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

// Errs returns the errors found while lexing (e.g. an unterminated string), each at its
// position (messages.Pos).
func (l *Lexer) Errs() []error {
	return l.errors
}

// Errors returns the texts of Errs ("Satır N, Sütun M: ...").
func (l *Lexer) Errors() []string {
	out := make([]string, len(l.errors))
	for i, e := range l.errors {
		out[i] = e.Error()
	}
	return out
}

func (l *Lexer) addError(line, col int, err error) {
	l.errors = append(l.errors, messages.At(line, col, err))
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
			tok = Token{Type: TokEq, Literal: "==", Line: line, Column: col}
		} else if l.peekChar() == '>' {
			l.readChar()
			tok = Token{Type: TokArrow, Literal: "=>", Line: line, Column: col}
		} else {
			tok = Token{Type: TokAssign, Literal: "=", Line: line, Column: col}
		}
	case ':':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TokWalrus, Literal: ":=", Line: line, Column: col}
		} else {
			tok = Token{Type: TokColon, Literal: ":", Line: line, Column: col}
		}
	case '+':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TokPlusAssign, Literal: "+=", Line: line, Column: col}
		} else {
			tok = Token{Type: TokPlus, Literal: "+", Line: line, Column: col}
		}
	case '-':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TokMinusAssign, Literal: "-=", Line: line, Column: col}
		} else {
			tok = Token{Type: TokMinus, Literal: "-", Line: line, Column: col}
		}
	case '*':
		if l.peekChar() == '*' {
			l.readChar()
			tok = Token{Type: TokPower, Literal: "**", Line: line, Column: col}
		} else if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TokMulAssign, Literal: "*=", Line: line, Column: col}
		} else {
			tok = Token{Type: TokAsterisk, Literal: "*", Line: line, Column: col}
		}
	case '/':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TokDivAssign, Literal: "/=", Line: line, Column: col}
		} else {
			tok = Token{Type: TokSlash, Literal: "/", Line: line, Column: col}
		}
	case '%':
		tok = Token{Type: TokPercent, Literal: "%", Line: line, Column: col}
	case '@':
		tok = Token{Type: TokMatmul, Literal: "@", Line: line, Column: col}
	case '!':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TokNotEq, Literal: "!=", Line: line, Column: col}
		} else {
			tok = Token{Type: TokBang, Literal: "!", Line: line, Column: col}
		}
	case '<':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TokLte, Literal: "<=", Line: line, Column: col}
		} else {
			tok = Token{Type: TokLt, Literal: "<", Line: line, Column: col}
		}
	case '>':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TokGte, Literal: ">=", Line: line, Column: col}
		} else {
			tok = Token{Type: TokGt, Literal: ">", Line: line, Column: col}
		}
	case '&':
		if l.peekChar() == '&' {
			l.readChar()
			tok = Token{Type: TokAnd, Literal: "&&", Line: line, Column: col}
		} else {
			tok = Token{Type: TokIllegal, Literal: string(l.ch), Line: line, Column: col}
		}
	case '|':
		if l.peekChar() == '|' {
			l.readChar()
			tok = Token{Type: TokOr, Literal: "||", Line: line, Column: col}
		} else if l.peekChar() == '>' {
			l.readChar()
			tok = Token{Type: TokPipe, Literal: "|>", Line: line, Column: col}
		} else {
			tok = Token{Type: TokIllegal, Literal: string(l.ch), Line: line, Column: col}
		}
	case '?':
		if l.peekChar() == '?' {
			l.readChar()
			tok = Token{Type: TokNullCoalesc, Literal: "??", Line: line, Column: col}
		} else if l.peekChar() == '.' {
			l.readChar()
			tok = Token{Type: TokSafeDot, Literal: "?.", Line: line, Column: col}
		} else {
			tok = Token{Type: TokQuestion, Literal: "?", Line: line, Column: col}
		}
	case '.':
		if l.peekChar() == '.' {
			l.readChar()
			if l.peekChar() == '=' {
				l.readChar()
				tok = Token{Type: TokRangeIncl, Literal: "..=", Line: line, Column: col}
			} else {
				tok = Token{Type: TokRange, Literal: "..", Line: line, Column: col}
			}
		} else {
			tok = Token{Type: TokDot, Literal: ".", Line: line, Column: col}
		}
	case ',':
		tok = Token{Type: TokComma, Literal: ",", Line: line, Column: col}
	case ';':
		tok = Token{Type: TokSemicolon, Literal: ";", Line: line, Column: col}
	case '(':
		tok = Token{Type: TokLParen, Literal: "(", Line: line, Column: col}
	case ')':
		tok = Token{Type: TokRParen, Literal: ")", Line: line, Column: col}
	case '{':
		tok = Token{Type: TokLBrace, Literal: "{", Line: line, Column: col}
	case '}':
		tok = Token{Type: TokRBrace, Literal: "}", Line: line, Column: col}
	case '[':
		tok = Token{Type: TokLBracket, Literal: "[", Line: line, Column: col}
	case ']':
		tok = Token{Type: TokRBracket, Literal: "]", Line: line, Column: col}
	case '"', '\'':
		quote := l.ch
		str := l.readString(quote, line, col)
		tok = Token{Type: TokString, Literal: str, Line: line, Column: col}
		return tok
	case '`':
		return Token{Type: TokString, Literal: l.readRawString(line, col), Line: line, Column: col}
	case 0:
		if l.position < len(l.input) {
			return l.nulChar(line, col)
		}
		return Token{Type: TokEOF, Literal: "", Line: line, Column: col}
	case '$':
		if isLetter(l.peekChar()) {
			return l.escapedName(line, col)
		}
		tok = Token{Type: TokIllegal, Literal: string(l.ch), Line: line, Column: col}
	default:
		if isLetter(l.ch) {
			name := l.readIdentifier()
			l.checkName(name, line, col)
			return l.nameToken(name, line, col)
		} else if isDigit(l.ch) {
			lit, isFloat := l.readNumber()
			if isFloat {
				return Token{Type: TokFloat, Literal: lit, Line: line, Column: col}
			}
			return Token{Type: TokInt, Literal: lit, Line: line, Column: col}
		} else {
			tok = Token{Type: TokIllegal, Literal: string(l.ch), Line: line, Column: col}
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
