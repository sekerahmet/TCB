package tokenize

import (
	"fmt"
	"unicode/utf8"
)

// ============================================================================
// Bayt düzeyi eşleme ve kayıplı UTF-8 çözme
// ============================================================================
//
// Bayt düzeyi BPE'de her bayt (0-255) "görünür" bir Unicode karakterine
// eşlenir: yazdırılabilir Latin-1 baytları kendilerine, geri kalanlar (kontrol
// karakterleri, boşluk, 0x7F-0xA0, 0xAD) sırayla U+0100'den başlayan
// karakterlere. Böylece sözlükteki her token, boşluk ve kontrol karakteri
// içermeyen düz bir metin olarak saklanabilir (ör. boşluk -> "Ġ", U+0120).

// baytKarakter[b], b baytını temsil eden karakterdir.
var byteChar [256]rune

// charByte, temsil karakterinden bayta ters eşlemedir.
var charByte map[rune]byte

func init() {
	var direct [256]bool
	for b := '!'; b <= '~'; b++ {
		direct[b] = true
	}
	for b := 0xA1; b <= 0xAC; b++ {
		direct[b] = true
	}
	for b := 0xAE; b <= 0xFF; b++ {
		direct[b] = true
	}
	n := 0
	for b := 0; b < 256; b++ {
		if direct[b] {
			byteChar[b] = rune(b)
		} else {
			byteChar[b] = rune(256 + n)
			n++
		}
	}
	charByte = make(map[rune]byte, 256)
	for b := 0; b < 256; b++ {
		charByte[byteChar[b]] = byte(b)
	}
}

// bytesToChars bayt dizisini temsil karakterlerinden oluşan metne çevirir.
// initFallbackTable sözlükteki "<0xXX>" bayt tokenlarının kimliklerini bayt değerine göre dizer (-1: yok).
func initFallbackTable(vocab map[string]int32) (table [256]int32) {
	for c := range table {
		table[c] = -1
		if id, ok := vocab[fmt.Sprintf("<0x%02X>", c)]; ok {
			table[c] = id
		}
	}
	return table
}

// appendByteFallback s'nin her baytı için "<0xXX>" tokenı varsa hepsini dst'ye ekler ve doğru döner;
// biri eksikse dst'ye hiçbir şey eklemez ve yanlış döner.
func appendByteFallback(dst []int32, table *[256]int32, s string) ([]int32, bool) {
	for k := 0; k < len(s); k++ {
		if table[s[k]] < 0 {
			return dst, false
		}
	}
	for k := 0; k < len(s); k++ {
		dst = append(dst, table[s[k]])
	}
	return dst, true
}

func bytesToChars(b []byte) string {
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = byteChar[c]
	}
	return string(r)
}

// tokenBytes bir token metnini çözme sırasında üreteceği baytlara çevirir.
// Tokenın bütün karakterleri bayt eşlemesinde varsa karşılık gelen baytlar,
// en az bir karakter eşlemede yoksa (ör. bayt düzeyi dışı bir ek token)
// tokenın kendi UTF-8 baytları kullanılır.
func tokenBytes(tok string) []byte {
	if out, ok := decodeByteChars(tok); ok {
		return out
	}
	return []byte(tok)
}

// decodeByteChars bir tokenın bayt düzeyi karakterlerini baytlara çevirir; bir karakter eşlemede
// yoksa (bayt düzeyi dışı token) yanlış döner.
func decodeByteChars(tok string) ([]byte, bool) {
	out := make([]byte, 0, len(tok))
	for _, r := range tok {
		b, ok := charByte[r]
		if !ok {
			return nil, false
		}
		out = append(out, b)
	}
	return out, true
}

// lossyUTF8 bayt dizisini metne çevirir; geçersiz UTF-8 dizilerini U+FFFD
// ile değiştirir. Unicode'un önerdiği "en uzun geçerli önek" kuralı
// uygulanır: yarım kalmış çok baytlı bir dizinin geçerli öneki TEK bir U+FFFD
// olur, kurala uymayan bayt ise yeni bir dizinin başı sayılır. (Standart
// kütüphanenin her geçersiz bayt için ayrı değiştirme yapan davranışından
// farklıdır.)
func lossyUTF8(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	out := make([]byte, 0, len(b)+8)
	i := 0
	for i < len(b) {
		c := b[i]
		if c < 0x80 {
			out = append(out, c)
			i++
			continue
		}
		// Gereken devam baytı sayısı ve ilk devam baytının izinli aralığı.
		var need int
		sub, parent := byte(0x80), byte(0xBF)
		switch {
		case c >= 0xC2 && c <= 0xDF:
			need = 1
		case c == 0xE0:
			need, sub = 2, 0xA0
		case c >= 0xE1 && c <= 0xEC, c == 0xEE, c == 0xEF:
			need = 2
		case c == 0xED:
			need, parent = 2, 0x9F
		case c == 0xF0:
			need, sub = 3, 0x90
		case c >= 0xF1 && c <= 0xF3:
			need = 3
		case c == 0xF4:
			need, parent = 3, 0x8F
		default:
			out = append(out, "�"...)
			i++
			continue
		}
		j := i + 1
		done := true
		for k := 0; k < need; k++ {
			if j >= len(b) {
				done = false
				break
			}
			d := b[j]
			lo, hi := byte(0x80), byte(0xBF)
			if k == 0 {
				lo, hi = sub, parent
			}
			if d < lo || d > hi {
				done = false
				break
			}
			j++
		}
		if done {
			out = append(out, b[i:j]...)
		} else {
			out = append(out, "�"...)
		}
		i = j
	}
	return string(out)
}

// validText geçersiz UTF-8 içeren girdiyi kayıplı çözmeyle onarır.
func validText(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return lossyUTF8([]byte(s))
}
