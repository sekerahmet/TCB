package lexer

import "strings"

// readString bir metin sabitini okur. Metin içindeki '{ifade}' bölümleri
// (interpolasyon) ayrıştırıcıya olduğu gibi aktarılır; bu bölümlerin içinde
// tırnaklı alt metinler bulunabilir ("{p['ad']}"). Bir interpolasyon aynı satırda
// kapanmalıdır; kapanmazsa veya metnin kapanış tırnağı hiç gelmezse açık bir
// hata kaydedilir. Düz süslü parantez için '\{' ve '\}' kullanılır.
func (l *Lexer) readString(quote rune, startLine, startCol int) string {
	var sb strings.Builder
	l.readChar() // açılış tırnağını geç
	// Değerdeki her karakterin kaynaktaki konumu biriktirilir; metin bir yerleştirme
	// içeriyorsa ayrıştırıcı ifadenin konumunu buradan okur (interpolation.go).
	l.strBuf, l.posBuf = l.strBuf[:0], l.posBuf[:0]
	interpolated := false
	defer func() {
		if interpolated {
			l.saveStringPositions(startLine, startCol)
		}
	}()
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
			l.readEscape(&sb)
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
				l.posBuf = append(l.posBuf, [2]int{len(l.strBuf), -1})
			}
			braceDepth++
			interpolated = true
		} else if l.ch == '}' && braceDepth > 0 {
			if braceDepth--; braceDepth == 0 {
				l.posBuf[len(l.posBuf)-1][1] = len(l.strBuf)
			}
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
		l.appendString(&sb, l.ch, l.position)
		l.readChar()
	}
}
