package lexer

import (
	"strings"
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Sayı yazımı (2.4, 2.5). Tam sayı ondalık ya da ön ekle yazılır: 0x1F, 0o17, 0b1010; ön ek yalnız
// küçük harftir. Rakamlar arasına ve ön ekten sonra '_' yazılabilir, sonda ve iki '_' yan yana
// yazılamaz. Ondalık sayıda noktanın iki yanında rakam vardır; üs e ya da E, isteğe bağlı işaret
// ve en az bir rakamdır. Sözcük biriminin metni '_'siz yazımdır; ön ekli tam sayıda ön ek kalır
// (0x1F), ayrıştırıcı tabanı ondan okur.

// prefixBases ön ekli tam sayının harfi → tabanıdır.
var prefixBases = map[rune]int{'x': 16, 'o': 8, 'b': 2}

// number l.ch'deki rakamla başlayan sayıyı okur.
func (l *Lexer) number(line, col int) Token {
	start := l.position
	next := l.peekChar()
	if l.ch == '0' && prefixBases[unicode.ToLower(next)] != 0 {
		return l.prefixedNumber(line, col, start)
	}
	var sb strings.Builder
	ok := l.readDigits(&sb, 10)
	isFloat := false
	// Ondalık kısım ('1..10' aralığıyla karışmasın diye noktadan sonra rakam gelmeli)
	if l.ch == '.' && isDigit(l.peekChar()) {
		isFloat = true
		sb.WriteRune(l.ch)
		l.readChar()
		ok = l.readDigits(&sb, 10) && ok
	} else if p := l.peekChar(); l.ch == '.' && p != '.' && !isLetter(p) {
		// '1.' sayı değildir (2.5): ileti '1.0'ı önerir, okuma öyle yazılmış gibi sürer (nokta ardından
		// gelen satırı üye sanmasın). Noktadan sonra harf üye erişimidir, ikinci nokta aralıktır.
		written := string(l.input[start:l.position]) + "."
		l.errors = append(l.errors, l.newErrorTo(line, col, l.position+1, messages.New(messages.H0032, written, written+"0"))) // up to the '.'
		l.readChar()
		isFloat = true
		sb.WriteString(".0")
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
			ok = l.readDigits(&sb, 10) && ok
		}
	}
	if !ok {
		l.addError(line, col, messages.New(messages.H0019, string(l.input[start:l.position])))
	}
	if isFloat {
		return Token{Type: TokFloat, Literal: sb.String(), Line: line, Column: col}
	}
	return Token{Type: TokInt, Literal: sb.String(), Line: line, Column: col}
}

// prefixedNumber 0x, 0o ya da 0b ön ekli tam sayıyı okur. Büyük harfli ön ek (0X1F) hatadır, ileti
// küçük harflisini önerir; okuma küçük harfliymiş gibi sürer. Ön ekten sonra harf ve rakamlar
// sonuna kadar okunur ki tabana uymayan rakam (0b102) ayrı bir sözcük sayılmasın.
func (l *Lexer) prefixedNumber(line, col, start int) Token {
	letter := l.peekChar()
	lower := unicode.ToLower(letter)
	base := prefixBases[lower]
	l.readChar() // '0'
	l.readChar() // ön ek harfi
	var sb strings.Builder
	sb.WriteString("0" + string(lower))
	digitsStart := l.position
	if l.ch == '_' {
		l.readChar() // ön ekten hemen sonra '_' yazılabilir (0x_FF)
	}
	ok := l.readDigits(&sb, base) && l.position > digitsStart && sb.Len() > 2
	bad := ""
	for isNameChar(l.ch) {
		if bad == "" {
			bad = string(l.ch)
		}
		l.readChar()
	}
	written := string(l.input[start:l.position])
	switch {
	case letter != lower:
		fixed := "0" + string(lower) + written[2:]
		// The literal written over again with a small prefix reads right only when nothing else is
		// wrong with it (a digit outside the base, a stray letter): then it applies as it stands.
		e := l.newError(line, col, messages.New(messages.H0018, written, fixed))
		e.WithFix(messages.Fix{Range: e.Range(), Replacement: fixed, Applicable: ok && bad == ""})
		l.errors = append(l.errors, e)
	case bad != "":
		l.addError(line, col, messages.New(messages.H0020, written, bad))
		return Token{Type: TokInt, Literal: "0", Line: line, Column: col}
	case !ok:
		l.addError(line, col, messages.New(messages.H0019, written))
		// Rakamsız ön ekin (0x_) değeri yok; ayrıştırıcı ikinci bir hata vermesin.
		return Token{Type: TokInt, Literal: "0", Line: line, Column: col}
	}
	return Token{Type: TokInt, Literal: sb.String(), Line: line, Column: col}
}

// pointFraction '.5' yazımını okur (2.5): sayı değildir, ileti '0.5'i önerir; okuma öyle yazılmış
// gibi sürer.
func (l *Lexer) pointFraction(line, col int) Token {
	start := l.position
	l.readChar() // '.'
	var sb strings.Builder
	sb.WriteString("0.")
	l.readDigits(&sb, 10)
	written := string(l.input[start:l.position])
	l.addError(line, col, messages.New(messages.H0032, written, "0"+written))
	return Token{Type: TokFloat, Literal: sb.String(), Line: line, Column: col}
}

// operandEnded son sözcük biriminin bir işleneni bitirip bitirmediğidir: ad, kapanan parantez ya da
// öğe indeksi (t.0). Ardından gelen '.' ve rakam üye erişimidir (t.0, f().1, t.0.1), '.5' gibi
// noktayla başlayan bir sayı değildir.
func (l *Lexer) operandEnded() bool {
	switch s := &l.places; s.prev {
	case TokIdent, TokSelf, TokRParen, TokRBracket:
		return true
	case TokInt:
		return s.prev2 == TokDot
	}
	return false
}

// elementIndex üye noktasından sonraki demet indeksini okur (3.6a): yalnız ondalık rakamlar,
// ayırıcısız ve baştaki sıfırsız ('0' kendisi olur). Nokta ve üs okunmaz: 't.0.1' iki indekstir,
// (t.0).1; ardındaki '.' yeniden bir üye noktasıdır. Rakamlara bitişik harf ya da '_' (x.0e1,
// t.1_0) ve baştaki sıfır (t.01) hatadır; okuma onları da indeksin metnine katıp sürer.
func (l *Lexer) elementIndex(line, col int) Token {
	start := l.position
	for isDigit(l.ch) {
		l.readChar()
	}
	digits := string(l.input[start:l.position])
	for isNameChar(l.ch) {
		l.readChar()
	}
	if written := string(l.input[start:l.position]); written != digits || len(digits) > 1 && digits[0] == '0' {
		l.addError(line, col, messages.New(messages.H0036, written))
	}
	return Token{Type: TokInt, Literal: digits, Line: line, Column: col}
}

// readDigits tabandaki rakamları (ve ayırıcı '_' işaretlerini, onları yazmadan) sb'ye okur. '_'
// iki rakam arasında olmalıdır: sonda ya da iki '_' yan yana gelirse false döner. Metottur,
// kapanış değil: sayı başına en çok üç çağrıdır ve sb yakalanıp yığından kaçmaz.
func (l *Lexer) readDigits(sb *strings.Builder, base int) bool {
	ok := true
	prevUnderscore := false
	for isBaseDigit(l.ch, base) || l.ch == '_' {
		if l.ch == '_' {
			if prevUnderscore {
				ok = false
			}
			prevUnderscore = true
		} else {
			sb.WriteRune(l.ch)
			prevUnderscore = false
		}
		l.readChar()
	}
	return ok && !prevUnderscore
}

// isBaseDigit ch'nin base tabanında bir rakam olduğudur (2, 8, 10 ya da 16).
func isBaseDigit(ch rune, base int) bool {
	switch base {
	case 2:
		return ch == '0' || ch == '1'
	case 8:
		return '0' <= ch && ch <= '7'
	case 16:
		return isHexDigit(ch)
	}
	return isDigit(ch)
}
