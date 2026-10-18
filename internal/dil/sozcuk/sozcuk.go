// Package sozcuk TR'nin sözcük çözümleyicisidir: kaynak metni satır ve sütun bilgisiyle
// sözcük birimlerine (anahtar kelime, ad, sayı, metin, işleç, yorum) ayırır. Anahtar
// kelimelerin yazımları kod listesinden (internal/dil/kod) gelir; ayrıştırıcı ve
// biçimlendirici bu paketin ürettiği sözcük birimleriyle çalışır.
//
// Katman: dil çekirdeği (internal/dil); yalnızca internal/dil paketlerini içe aktarır.
package sozcuk

import (
	"fmt"
	"strings"
	"unicode"
)

// Metin içinde '\{' ve '\}' ile kaçışlanmış süslü parantezler, sözcük biriminde
// bu özel kullanım alanı karakterleriyle taşınır; ayrıştırıcı bunları düz '{' ve
// '}' karakterlerine çevirir. Böylece '\\{' (ters bölü + interpolasyon) ile '\{'
// (düz süslü parantez) birbirine karışmaz.
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
}

// SonAralik son NextToken çağrısının döndürdüğü sözcük biriminin kaynaktaki karakter
// (rune) aralığını verir: [bas, son). Biçimlendirici, sözcük birimlerinin ham metnini
// (kaçış dizileri çözülmemiş hâlde) ve aralarındaki yorumları bu sayede korur.
func (l *Lexer) SonAralik() (bas, son int) {
	son = l.position
	if son > len(l.input) {
		son = len(l.input)
	}
	return l.tokenStart, son
}

// Girdi sözcük çözümleyicinin okuduğu kaynağı (BOM atılmış, rune dizisi olarak) döndürür.
func (l *Lexer) Girdi() []rune {
	return l.input
}

// Errors sözcük çözümleme sırasında bulunan hataları (ör. kapatılmamış metin) döndürür.
func (l *Lexer) Errors() []string {
	return l.errors
}

func (l *Lexer) addError(line, col int, format string, args ...interface{}) {
	prefix := fmt.Sprintf("Satır %d, Sütun %d: ", line, col)
	l.errors = append(l.errors, prefix+fmt.Sprintf(format, args...))
}

func New(input string) *Lexer {
	// Bazı Windows editörlerinin dosya başına eklediği UTF-8 BOM'u (U+FEFF) atla;
	// aksi halde ilk satırda GEÇERSİZ bir sözcük birimi oluşur.
	input = strings.TrimPrefix(input, string(rune(0xFEFF)))
	l := &Lexer{
		input:  []rune(input),
		line:   1,
		column: 0,
	}
	l.readChar()
	return l
}

// NewAt, kaynağı verilen satır ve sütundan başlıyormuş gibi okuyan bir sözcük çözümleyici
// döndürür. Metin içi yerleştirmelerdeki ("{ifade}") ifadeler böylece hata mesajlarında
// dosyadaki gerçek konumlarıyla görünür.
func NewAt(input string, line, column int) *Lexer {
	l := &Lexer{input: []rune(input), line: line, column: column - 1}
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
		// '#' yorum değildir (her işin tek yazımı: satır yorumu yalnızca '//'). Başka
		// dillerden alışkanlıkla yazılmış olabileceği için açık bir hata verilir; satırın
		// geri kalanı atlanır ki ardından gelen sözcükler ikinci bir hata üretmesin.
		if l.ch == '#' {
			l.addError(l.line, l.column, "'#' TR'de yorum değil; yorum için '//' yazın")
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
				l.addError(startLine, startCol, "kapatılmamış yorum: '/*' ile başlayan yorum dosya sonuna kadar '*/' ile kapanmadı")
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

	// Bilimsel yazım: 1e-5, 3.2E+4, 2e10 (üsten sonra en az bir rakam gelmelidir)
	if l.ch == 'e' || l.ch == 'E' {
		sonraki := l.peekChar()
		isaretli := (sonraki == '+' || sonraki == '-') && isDigit(l.peekCharN(2))
		if isDigit(sonraki) || isaretli {
			isFloat = true
			sb.WriteRune(l.ch)
			l.readChar()
			if isaretli {
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

// readString bir metin sabitini okur. Metin içindeki '{ifade}' bölümleri
// (interpolasyon) ayrıştırıcıya olduğu gibi aktarılır; bu bölümlerin içinde
// tırnaklı alt metinler bulunabilir ("{p['ad']}"). Bir interpolasyon aynı satırda
// kapanmalıdır; kapanmazsa veya metnin kapanış tırnağı hiç gelmezse açık bir
// hata kaydedilir. Düz süslü parantez için '\{' ve '\}' kullanılır.
func (l *Lexer) readString(quote rune, startLine, startCol int) string {
	var sb strings.Builder
	l.readChar() // açılış tırnağını geç
	braceDepth := 0
	braceLine, braceCol := 0, 0
	var inner rune // interpolasyon içindeki alt metnin tırnağı (0 = alt metin yok)

	for {
		if l.ch == 0 || (braceDepth > 0 && l.ch == '\n') {
			if braceDepth > 0 {
				l.addError(braceLine, braceCol, "metin içindeki '{' kapatılmamış (interpolasyon aynı satırda '}' ile bitmeli; düz süslü parantez için '\\{' yazın)")
			} else {
				l.addError(startLine, startCol, "kapatılmamış metin: %c ile başlayan metin dosya sonuna kadar kapanmadı", quote)
			}
			return sb.String()
		}

		if l.ch == '\\' {
			kacisSatir, kacisSutun := l.line, l.column
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
				sb.WriteRune(EscapedLBrace)
			case '}':
				sb.WriteRune(EscapedRBrace)
			case 0:
				continue // dosya sonu: yukarıdaki kontrol hatayı bildirir
			default:
				// Bilinmeyen kaçış hatadır: ters bölüyü sessizce atmak düzenli ifadeleri
				// ("\d") ve Windows yollarını ("C:\veri") fark edilmeden bozardı.
				l.addError(kacisSatir, kacisSutun, "%s", bilinmeyenKacis(l.ch))
				sb.WriteRune('\\')
				if l.ch == '\r' && l.peekChar() == '\n' {
					l.readChar()
				}
				sb.WriteRune(l.ch)
			}
			l.readChar()
			continue
		}

		if inner != 0 {
			if l.ch == inner {
				inner = 0
			}
		} else if braceDepth > 0 && (l.ch == '"' || l.ch == '\'') {
			inner = l.ch
		} else if l.ch == '{' {
			if braceDepth == 0 {
				braceLine, braceCol = l.line, l.column
			}
			braceDepth++
		} else if l.ch == '}' && braceDepth > 0 {
			braceDepth--
		} else if l.ch == quote && braceDepth == 0 {
			l.readChar() // kapanış tırnağını geç
			return sb.String()
		}

		// Çok satırlı metinde satır sonu kaynağın biçiminden bağımsız olarak '\n'dir:
		// CRLF'li dosyada metnin değerine '\r' girmez (yalnızca \r kaçışıyla yazılır).
		if l.ch == '\r' && l.peekChar() == '\n' {
			l.readChar()
			continue
		}
		sb.WriteRune(l.ch)
		l.readChar()
	}
}

func isLetter(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

func isDigit(ch rune) bool {
	return '0' <= ch && ch <= '9'
}
