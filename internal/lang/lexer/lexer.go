// Package lexer is TR's lexer: it splits source text into tokens (keywords, names, numbers,
// strings, operators) with line and column information; comments are skipped. Keyword spellings
// come from the code list (internal/lang/codes); the parser and the formatter work on the tokens
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

type Lexer struct {
	input        []rune
	position     int  // mevcut karakterin indeksi
	readPosition int  // okunacak sonraki karakterin indeksi
	ch           rune // incelenen mevcut karakter
	line         int
	column       int
	errors       []error
	tokenStart   int // son okunan sözcük biriminin başladığı karakter (rune) indeksi

	// coded, gövdenin kimliklerle yazıldığını söyler: dilin sözcükleri TR_<ad> kimlikleridir
	// (TR_if, TR_print), başka her ad kullanıcının adıdır (identity.go).
	coded bool
	// root dosyanın çözümleyicisidir (kökte kendisi). Metin içi yerleştirmelerin alt
	// çözümleyicileri konumlarını, metin haritalarını ve kayıtlarını kökle paylaşır.
	root *Lexer
	// posMap alt çözümleyicide girdinin her karakterinin kökteki konumudur (interpolation.go).
	posMap []int32
	// stringInfos kökte, yerleştirme içeren metin sabitlerinin bilgisidir (sözcük biriminin satır
	// ve sütunuyla anahtarlı; interpolation.go); strBuf ve posBuf okunmakta olan metnin
	// konumlarını ve yerleştirme aralıklarını biriktirir.
	stringInfos map[[2]int]*stringInfo
	strBuf      []int32
	posBuf      [][2]int
	// lineStarts kökte satırların başladığı karakter indeksleridir (gerektiğinde kurulur).
	lineStarts []int
	// rec açıksa (Kaydet) kökte üretilen bütün sözcük birimleri kaynak aralıklarıyla tutulur.
	rec *[]Record
	// shifts kodlu kökte gösterimdeki sütun kaymalarıdır (coded.go); shiftSum birikimli
	// toplamları, shiftIdx kök çözümleyicinin sıradaki kayma noktasıdır.
	shifts   []shift
	shiftSum []int32
	shiftIdx int
	// lang ve words kökte, kodsuz gövdenin yazıldığı dil ve onun anahtar kelimeleridir
	// (NewBodyIn); nil ise kimlik listesi (codes.Active).
	lang  *codes.Language
	words map[string]TokenType
	// writtenIn kökte, yazılan metnin (kabuk satırı) yazım dilidir; dosyada nil (WrittenIn).
	writtenIn *codes.Language
	// places kimliklerin yerini (2.3a) bulmak için izlenen bağlamdır (identity.go).
	places placeState
	// lineStart, bu satırda henüz sözcük birimi okunmadığını söyler (satır başındaki '//', 2.2).
	lineStart bool
}

// Language returns the language whose words the lexer reads: the language of an uncoded body
// (NewBodyIn), otherwise the active language (coded bodies are read in the active language).
func (l *Lexer) Language() *codes.Language {
	if l.root.lang != nil {
		return l.root.lang
	}
	return codes.Active()
}

// WrittenIn returns the writing language of a typed text (a shell line, a debugger expression),
// whose words messages name in the message's language; nil for a file, which has no language
// (2.1): its words are named by their spelling (codes.WordRef).
func (l *Lexer) WrittenIn() *codes.Language { return l.root.writtenIn }

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
	return messages.Texts(l.errors)
}

// addError records err at (line, col). It ends where the lexer stands: the error is raised once
// the text it is about is read (an unclosed string ends at the end of the file); at the least it
// covers the character at the start. An error raised before the text is read to its end (a '//' not
// yet skipped, a '.' not yet taken) ends it with newErrorTo, or closes the record later with endError.
func (l *Lexer) addError(line, col int, err error) {
	l.errors = append(l.errors, l.newError(line, col, err))
}

// addErrorFix records err with one suggested fix: the text the error is at replaced by replacement.
// The suggestion is a guess (the closest name, the name without its prefix), so it is never
// applicable as it stands.
func (l *Lexer) addErrorFix(line, col int, err error, replacement string) {
	e := l.newError(line, col, err)
	l.errors = append(l.errors, e.WithFix(messages.Fix{Range: e.Range(), Replacement: replacement}))
}

// newError is addError's record, for an error that carries fixes (WithFix) before it is added.
func (l *Lexer) newError(line, col int, err error) *messages.PosError {
	return l.newErrorTo(line, col, l.position, err)
}

// newErrorTo is newError for an error whose text ends before the character at index end of the
// input (the lexer has not read that far yet).
func (l *Lexer) newErrorTo(line, col, end int, err error) *messages.PosError {
	e := messages.At(line, col, err)
	l.endError(e, end)
	return e
}

// endError ends e before the character at index end of the input, and at the least one character
// after its start.
func (l *Lexer) endError(e *messages.PosError, end int) {
	e.EndLine, e.EndCol = l.endAt(end)
	if e.EndLine < e.Line || e.EndLine == e.Line && e.EndCol <= e.Col {
		e.EndLine, e.EndCol = e.Line, e.Col+1
	}
}

// endAt is the line and column of the character at index i of the input, which is the position
// after the token that ends just before it (a token never ends with a newline, so a newline at i
// is still on the token's line). A lexer for the expression inside a string reads the position
// through its map (interpolation.go); one without a map has no place of its own.
func (l *Lexer) endAt(i int) (line, col int) {
	switch {
	case l.posMap != nil && i > 0:
		line, col = l.root.lineCol(int(l.sourcePos(i - 1)))
		return line, col + 1
	case l.posMap != nil || l.root != l:
		return l.line, l.column
	}
	return l.lineCol(i)
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
		input:     []rune(input),
		line:      1,
		column:    0,
		lineStart: true,
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
func (l *Lexer) NextToken() Token {
	tok := l.nextToken()
	tok.EndLine, tok.EndColumn = tok.Line, tok.Column
	if begin, end := l.LastRange(); end > begin {
		// A lexer without a place of its own (a string's text read apart) has no end to give:
		// Token.Range then ends the token after its text.
		if endLine, endCol := l.endAt(end); endLine > tok.Line || endLine == tok.Line && endCol > tok.Column {
			tok.EndLine, tok.EndColumn = endLine, endCol
		} else {
			tok.EndLine, tok.EndColumn = 0, 0
		}
	}
	l.track(tok)
	l.lineStart = false
	if l.root.rec != nil {
		l.record(tok)
	}
	return tok
}

func (l *Lexer) nextToken() Token {
	l.skipWhitespaceAndComments()
	l.tokenStart = l.position

	line := l.line
	col := l.column
	var tok Token

	switch l.ch {
	case '=', ':', '+', '-', '*', '/', '%', '@', '!', '<', '>', '&', '|', '^', '~', '?':
		tok = l.operatorToken(line, col)
	case '.':
		if isDigit(l.peekChar()) && !l.operandEnded() {
			return l.pointFraction(line, col)
		}
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
		tok = l.operator(line, col, TokComma)
	case ';':
		tok = l.operator(line, col, TokSemicolon)
	case '(':
		tok = l.operator(line, col, TokLParen)
	case ')':
		tok = l.operator(line, col, TokRParen)
	case '{':
		tok = l.operator(line, col, TokLBrace)
	case '}':
		tok = l.operator(line, col, TokRBrace)
	case '[':
		tok = l.operator(line, col, TokLBracket)
	case ']':
		tok = l.operator(line, col, TokRBracket)
	case '"':
		return Token{Type: TokString, Literal: l.readString('"', line, col), Line: line, Column: col}
	case '\'':
		// Metin yalnız çift tırnakla yazılır (2.6); tek tırnaklı metin yine okunur ki ardındaki
		// sözcükler ikinci bir hata üretmesin.
		e := l.newError(line, col, messages.New(messages.H0016))
		l.errors = append(l.errors, e) // the string's own errors follow it
		lit := l.readString('\'', line, col)
		l.endError(e, l.position) // the error is about the whole string
		return Token{Type: TokString, Literal: lit, Line: line, Column: col}
	case '`':
		return Token{Type: TokString, Literal: l.readRawString(line, col), Line: line, Column: col, Raw: true}
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
			return l.numberOrIndex(line, col)
		} else {
			tok = Token{Type: TokIllegal, Literal: string(l.ch), Line: line, Column: col}
		}
	}

	l.readChar()
	return tok
}

// readIdentifier bir adı okur (UAX #31, 2.3): ilk karakteri harf ya da '_' (isLetter), öbürleri
// harf, rakam, birleşen işaret ya da bağlayıcı noktalama (isNameChar).
func (l *Lexer) readIdentifier() string {
	start := l.position
	for isNameChar(l.ch) {
		l.readChar()
	}
	return string(l.input[start:l.position])
}

// follow, iki karakterli bir işlecin ikinci karakteri ve türüdür.
type follow struct {
	ch  rune
	typ TokenType
}

// operator, l.ch'de başlayan işleci verir: sonraki karakter follows'taki karakterlerden biriyse
// iki karakterli işleç (ilk karakter tüketilir; ikincisini NextToken'ın sonu tüketir), değilse
// single. go/scanner'ın switch2/switch3 yardımcılarının karşılığı: işleç tablosu tek yerde durur.
// operatorToken işleç ve noktalama karakteriyle (=, :, +, -, *, /, %, @, !, <, >, &, |, ^, ~, ?) başlayan
// sözcük birimini okur; iki ve üç karakterli işleçler (==, //=) en uzun eşleşmeyle.
func (l *Lexer) operatorToken(line, col int) Token {
	switch l.ch {
	case '=':
		return l.operator(line, col, TokAssign, follow{'=', TokEq}, follow{'>', TokArrow})
	case ':':
		return l.operator(line, col, TokColon, follow{'=', TokWalrus})
	case '+':
		return l.operator(line, col, TokPlus, follow{'=', TokPlusAssign})
	case '-':
		return l.operator(line, col, TokMinus, follow{'=', TokMinusAssign})
	case '*':
		if t, ok := l.doubled(line, col, TokPower, TokPowAssign); ok {
			return t
		} else {
			return l.operator(line, col, TokAsterisk, follow{'=', TokMulAssign})
		}
	case '/':
		if t, ok := l.doubled(line, col, TokSlashSlash, TokFloorDivAssign); ok {
			return t
		} else {
			return l.operator(line, col, TokSlash, follow{'=', TokDivAssign})
		}
	case '%':
		return l.operator(line, col, TokPercent, follow{'=', TokModAssign})
	case '@':
		return l.operator(line, col, TokMatmul)
	case '!':
		return l.operator(line, col, TokBang, follow{'=', TokNotEq})
	case '<':
		if t, ok := l.doubled(line, col, TokShl, TokShlAssign); ok {
			return t
		} else {
			return l.operator(line, col, TokLt, follow{'=', TokLte})
		}
	case '>':
		if t, ok := l.doubled(line, col, TokShr, TokShrAssign); ok {
			return t
		} else {
			return l.operator(line, col, TokGt, follow{'=', TokGte})
		}
	case '&':
		return l.operator(line, col, TokBitAnd, follow{'&', TokAnd}, follow{'=', TokBitAndAssign})
	case '|':
		return l.operator(line, col, TokBitOr, follow{'|', TokOr}, follow{'>', TokPipe}, follow{'=', TokBitOrAssign})
	case '^':
		return l.operator(line, col, TokBitXor, follow{'=', TokBitXorAssign})
	case '~':
		return l.operator(line, col, TokTilde)
	case '?':
		return l.operator(line, col, TokQuestion, follow{'?', TokNullCoalesc}, follow{'.', TokSafeDot})
	}
	return Token{Type: TokIllegal, Literal: string(l.ch), Line: line, Column: col} // yalnız işleç karakterinde çağrılır
}

// doubled iki kez yazılan işleci (//, **, <<, >>) ve ardından '=' geliyorsa bileşik atamasını (//=)
// okur; ok, karakterin ikinci kez yazıldığıdır.
func (l *Lexer) doubled(line, col int, two, assign TokenType) (Token, bool) {
	if l.peekChar() != l.ch {
		return Token{}, false
	}
	l.readChar()
	if l.peekChar() == '=' {
		l.readChar()
		return Token{Type: assign, Literal: string(assign), Line: line, Column: col}, true
	}
	return Token{Type: two, Literal: string(two), Line: line, Column: col}, true
}

func (l *Lexer) operator(line, col int, single TokenType, follows ...follow) Token {
	for _, f := range follows {
		if l.peekChar() == f.ch {
			lit := string(l.ch) + string(f.ch)
			l.readChar()
			return Token{Type: f.typ, Literal: lit, Line: line, Column: col}
		}
	}
	return Token{Type: single, Literal: string(l.ch), Line: line, Column: col}
}

// isLetter adın ilk karakteri olabilir mi (UAX #31 XID_Start ya da '_', 2.3): harf ya da harf
// sayı (Nl, Roma rakamı Ⅻ gibi). Go'nun unicode paketi XID özelliğini vermez; bu, XID_Start'ın
// genel kategorilerden kurulan karşılığıdır (NFKC'de farklı okunan birkaç karakter dışında aynı).
func isLetter(ch rune) bool {
	return ch == '_' || unicode.IsLetter(ch) || unicode.Is(unicode.Nl, ch)
}

// isNameChar adın ilk karakterinden sonra gelebilir mi (XID_Continue): harf, rakam (Nd), birleşen
// işaret (Mn, Mc) ya da bağlayıcı noktalama (Pc, '_' dahil).
func isNameChar(ch rune) bool {
	return isLetter(ch) || unicode.Is(unicode.Nd, ch) || unicode.In(ch, unicode.Mn, unicode.Mc, unicode.Pc)
}

// isDigit sayı yazımındaki ondalık rakamdır (yalnız ASCII; 2.4).
func isDigit(ch rune) bool {
	return '0' <= ch && ch <= '9'
}
