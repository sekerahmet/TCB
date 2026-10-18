package sozcuk

import (
	"fmt"
	"strings"
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
			l.addError(startLine, startCol, "kapatılmamış ham metin: ` ile başlayan metin dosya sonuna kadar kapanmadı")
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

// kacisOku tırnaklı metindeki bir kaçış dizisini (ters bölü ve ardındaki karakter) okur ve
// karşılığını yazar. Dosya sonunda ters bölüden sonra durur; readString hatayı bildirir.
func (l *Lexer) kacisOku(sb *strings.Builder) {
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
		return // dosya sonu: readString hatayı bildirir
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
}

// bilinmeyenKacis tırnaklı metindeki tanınmayan '\c' kaçış dizisinin hata mesajıdır.
func bilinmeyenKacis(c rune) string {
	dizi := "'\\" + string(c) + "'"
	switch c {
	case '\n', '\r':
		dizi = "'\\' ve satır sonu"
	case '\t':
		dizi = "'\\' ve sekme"
	}
	return fmt.Sprintf("bilinmeyen kaçış dizisi %s (bilinenler: \\n \\t \\r \\\" \\' \\\\ \\{ \\}); "+
		"ters bölüyü olduğu gibi yazmak için ham metin (`C:\\veri`, `\\d+`) kullanın ya da ters bölüyü ikileyin (\"\\\\\")", dizi)
}
