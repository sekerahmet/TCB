package lexer

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// readRawString ters tırnaklı ham metni (`...`) okur: içinde kaçış dizisi ve '{ifade}'
// yerleştirmesi yoktur, ters bölü ve süslü parantez olduğu gibi kalır (düzenli ifadeler ve
// Windows yolları için). Birden çok satıra yayılabilir; satır sonu, çift tırnaklı metindeki
// gibi kaynağın biçiminden bağımsız olarak '\n'dir. Ham metnin içine ters tırnak yazılamaz.
//
// Süslü parantezler, '\{' ve '\}' kaçışlarının işaretçileriyle taşınır; ayrıştırıcı onları
// yerleştirme saymadan düz süslü parantezlere çevirir. Böylece ham metin ayrıştırıcıya
// sıradan bir metin sözcük birimi olarak gider.
func (l *Lexer) readRawString(startLine, startCol int) string {
	var sb strings.Builder
	l.readChar() // açılış ters tırnağını geç
	for l.ch != '`' {
		switch l.ch {
		case 0:
			l.addError(startLine, startCol, messages.New(messages.H0007))
			return sb.String()
		case '{':
			sb.WriteRune(EscapedLBrace)
		case '}':
			sb.WriteRune(EscapedRBrace)
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
// karşılığını yazar. Dosya sonunda ters bölüden sonra durur; readString hatayı bildirir.
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
	case '\'':
		r = '\''
	case '\\':
		r = '\\'
	case '{':
		// Kaçışlı süslü parantez (interpolasyon değil)
		r = EscapedLBrace
	case '}':
		r = EscapedRBrace
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

// unknownEscape tırnaklı metindeki tanınmayan '\c' kaçış dizisini hata iletisi için anlatır.
func unknownEscape(c rune) string {
	arr := "'\\" + string(c) + "'"
	switch c {
	case '\n', '\r':
		arr = messages.Text(messages.H0168)
	case '\t':
		arr = messages.Text(messages.H0169)
	}
	return arr
}
