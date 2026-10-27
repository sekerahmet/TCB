package lexer

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// readString tırnaklı bir metni okur (2.6, 2.7). Kaçış dizileri çözülür; '\(ifade)' bir
// yerleştirmedir: ifadenin metni değere '(' ve ')' arasında olduğu gibi (kaçışları çözülmeden)
// yazılır, aralığı posBuf'a kaydedilir, ayrıştırıcı onu alt çözümleyiciyle okur
// (interpolation.go). Yerleştirme '\('in eşi olan parantezle kapanır; içindeki parantezler ve
// metinler onu kapatmaz; aynı satırda kapanmalıdır. Süslü parantez sıradan karakterdir. quote
// '"' dışında yalnız hata kurtarmada gelir (tek tırnaklı metin, 2.6).
func (l *Lexer) readString(quote rune, startLine, startCol int) string {
	var sb strings.Builder
	l.readChar() // açılış tırnağını geç
	// Değerdeki her karakterin kaynaktaki konumu biriktirilir; metin bir yerleştirme
	// içeriyorsa ayrıştırıcı ifadenin konumunu buradan okur (interpolation.go).
	l.strBuf, l.posBuf = l.strBuf[:0], l.posBuf[:0]
	defer func() {
		if len(l.posBuf) > 0 {
			l.saveStringPositions(startLine, startCol)
		}
	}()
	for {
		switch {
		case l.ch == 0 && l.position >= len(l.input):
			l.addError(startLine, startCol, messages.New(messages.H0010, string(quote)))
			return sb.String()
		case l.ch == quote:
			l.readChar() // kapanış tırnağını geç
			return sb.String()
		case l.ch == '\\' && l.peekChar() == '(':
			l.readInterpolation(&sb)
		case l.ch == '\\':
			l.readEscape(&sb)
		case l.ch == '\r' && l.peekChar() == '\n':
			// Çok satırlı metinde satır sonu kaynağın biçiminden bağımsız olarak '\n'dir:
			// CRLF'li dosyada metnin değerine '\r' girmez (yalnızca \r kaçışıyla yazılır).
			l.readChar()
		default:
			l.appendString(&sb, l.ch, l.position)
			l.readChar()
		}
	}
}

// readInterpolation '\(' ile başlayan yerleştirmeyi okur: değere '(' + ifadenin kaynaktaki metni
// + ')' yazılır, '(' ve ')'nin değerdeki indeksleri posBuf'a kaydedilir. Satır sonunda ya da dosya
// sonunda kapanmamışsa hata verir ve aralığın sonu açık kalır (-1).
func (l *Lexer) readInterpolation(sb *strings.Builder) {
	openLine, openCol := l.line, l.column
	l.readChar() // '\'
	l.posBuf = append(l.posBuf, [2]int{len(l.strBuf), -1})
	l.appendString(sb, '(', l.position)
	l.readChar()
	if !l.copyCode(sb) {
		l.addError(openLine, openCol, messages.New(messages.H0009))
		return
	}
	l.posBuf[len(l.posBuf)-1][1] = len(l.strBuf)
	l.appendString(sb, ')', l.position)
	l.readChar()
}

// copyCode yerleştirmenin içindeki kodu eşi olan ')'ye kadar (')' hariç) değere olduğu gibi
// yazar: iç içe parantezler sayılır, içteki tırnaklı ve ham metinler (kendi yerleştirmeleriyle)
// atlanır. Satır sonuna ya da dosya sonuna varırsa false döner.
func (l *Lexer) copyCode(sb *strings.Builder) bool {
	depth := 0
	for {
		switch l.ch {
		case 0, '\n':
			return false
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return true
			}
			depth--
		case '"', '`':
			if !l.copyQuoted(sb, l.ch) {
				return false
			}
			continue
		}
		l.appendString(sb, l.ch, l.position)
		l.readChar()
	}
}

// copyQuoted yerleştirmenin içindeki bir metni (tırnaklarıyla) değere olduğu gibi yazar; içindeki
// kaçışlar ve yerleştirmeler atlanır. Metin aynı satırda kapanmazsa false döner.
func (l *Lexer) copyQuoted(sb *strings.Builder, quote rune) bool {
	l.appendString(sb, l.ch, l.position)
	l.readChar()
	for l.ch != quote {
		switch {
		case l.ch == 0 || l.ch == '\n':
			return false
		case quote == '"' && l.ch == '\\' && l.peekChar() == '(':
			l.appendString(sb, l.ch, l.position)
			l.readChar()
			l.appendString(sb, l.ch, l.position)
			l.readChar()
			if !l.copyCode(sb) {
				return false
			}
		case quote == '"' && l.ch == '\\':
			l.appendString(sb, l.ch, l.position)
			l.readChar()
			if l.ch == 0 || l.ch == '\n' {
				return false
			}
		}
		l.appendString(sb, l.ch, l.position)
		l.readChar()
	}
	l.appendString(sb, l.ch, l.position)
	l.readChar()
	return true
}
