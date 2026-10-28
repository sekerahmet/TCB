package lexer

import (
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// readRawString ters tırnaklı ham metni (`...`, 2.6a) okur: içinde kaçış dizisi ve yerleştirme
// yoktur, ters bölü ve tırnaklar olduğu gibi kalır (düzenli ifadeler ve Windows yolları için).
// Birden çok satıra yayılabilir; satır sonu, çift tırnaklı metindeki gibi kaynağın biçiminden
// bağımsız olarak '\n'dir. Ham metnin içine ters tırnak yazılamaz.
func (l *Lexer) readRawString(startLine, startCol int) string {
	var sb strings.Builder
	l.readChar() // açılış ters tırnağını geç
	for l.ch != '`' {
		switch l.ch {
		case 0:
			if l.position >= len(l.input) {
				l.addError(startLine, startCol, messages.New(messages.H0007))
				return sb.String()
			}
			sb.WriteRune(l.ch)
		case '\r':
			if l.peekChar() != '\n' {
				sb.WriteRune('\r')
			}
		default:
			sb.WriteRune(l.ch)
		}
		l.readChar()
	}
	l.readChar() // kapanış ters tırnağını geç
	return sb.String()
}

// readEscape tırnaklı metindeki bir kaçış dizisini (ters bölü ve ardındaki karakter) okur ve
// karşılığını yazar (2.6): \n \t \r \" \\ ve \u{...}. Dosya sonunda ters bölüden sonra durur;
// readString hatayı bildirir.
func (l *Lexer) readEscape(sb *strings.Builder) {
	escLine, escCol := l.line, l.column
	begin := l.position // karşılığın kaynaktaki konumu: ters bölü
	l.readChar()
	var r rune
	switch l.ch {
	case 'n':
		r = '\n'
	case 't':
		r = '\t'
	case 'r':
		r = '\r'
	case '"':
		r = '"'
	case '\\':
		r = '\\'
	case 'u':
		var ok bool
		if r, ok = l.unicodeEscape(); !ok {
			l.addError(escLine, escCol, messages.New(messages.H0017, string(l.input[begin:l.position])))
			return
		}
		l.appendString(sb, r, begin)
		return
	case 0:
		return // dosya sonu: readString hatayı bildirir
	default:
		// Bilinmeyen kaçış hatadır: ters bölüyü sessizce atmak düzenli ifadeleri
		// ("\d") ve Windows yollarını ("C:\veri") fark edilmeden bozardı.
		l.addError(escLine, escCol, messages.New(messages.H0008, unknownEscape(l.ch)))
		l.appendString(sb, '\\', begin)
		if l.ch == '\r' && l.peekChar() == '\n' {
			l.readChar()
		}
		r, begin = l.ch, l.position
	}
	l.appendString(sb, r, begin)
	l.readChar()
}

// unicodeEscape 'u'da duran imleçten '\u{...}' kaçışının geri kalanını okur: '{', bir ile altı
// onaltılık rakam, '}'; değer geçerli bir Unicode karakteri olmalıdır (vekil yarım ya da
// U+10FFFF'ten büyük değil). İmleç okunan son karakterden sonradadır.
func (l *Lexer) unicodeEscape() (rune, bool) {
	l.readChar() // 'u'
	if l.ch != '{' {
		return 0, false
	}
	l.readChar()
	start := l.position
	for isHexDigit(l.ch) {
		l.readChar()
	}
	digits := string(l.input[start:l.position])
	if l.ch != '}' {
		return 0, false
	}
	l.readChar()
	n, err := strconv.ParseUint(digits, 16, 32)
	if err != nil || len(digits) > 6 || n > 0x10FFFF || n >= 0xD800 && n <= 0xDFFF {
		return 0, false
	}
	return rune(n), true
}

func isHexDigit(ch rune) bool {
	return isDigit(ch) || 'a' <= ch && ch <= 'f' || 'A' <= ch && ch <= 'F'
}

// unknownEscape tırnaklı metindeki tanınmayan '\c' kaçış dizisini hata iletisi için anlatır; satır
// sonu ve sekme ileti parçasıdır (dış iletinin dilinde yazılır).
func unknownEscape(c rune) any {
	switch c {
	case '\n', '\r':
		return messages.New(messages.H0168)
	case '\t':
		return messages.New(messages.H0169)
	}
	return "'\\" + string(c) + "'"
}
