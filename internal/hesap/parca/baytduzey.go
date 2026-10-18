package parca

import "unicode/utf8"

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
var baytKarakter [256]rune

// karakterBayt, temsil karakterinden bayta ters eşlemedir.
var karakterBayt map[rune]byte

func init() {
	var dogrudan [256]bool
	for b := '!'; b <= '~'; b++ {
		dogrudan[b] = true
	}
	for b := 0xA1; b <= 0xAC; b++ {
		dogrudan[b] = true
	}
	for b := 0xAE; b <= 0xFF; b++ {
		dogrudan[b] = true
	}
	n := 0
	for b := 0; b < 256; b++ {
		if dogrudan[b] {
			baytKarakter[b] = rune(b)
		} else {
			baytKarakter[b] = rune(256 + n)
			n++
		}
	}
	karakterBayt = make(map[rune]byte, 256)
	for b := 0; b < 256; b++ {
		karakterBayt[baytKarakter[b]] = byte(b)
	}
}

// baytlariKarakterle bayt dizisini temsil karakterlerinden oluşan metne çevirir.
func baytlariKarakterle(b []byte) string {
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = baytKarakter[c]
	}
	return string(r)
}

// tokenBaytlari bir token metnini çözme sırasında üreteceği baytlara çevirir.
// Tokenın bütün karakterleri bayt eşlemesinde varsa karşılık gelen baytlar,
// en az bir karakter eşlemede yoksa (ör. bayt düzeyi dışı bir ek token)
// tokenın kendi UTF-8 baytları kullanılır.
func tokenBaytlari(tok string) []byte {
	out := make([]byte, 0, len(tok))
	for _, r := range tok {
		b, ok := karakterBayt[r]
		if !ok {
			return []byte(tok)
		}
		out = append(out, b)
	}
	return out
}

// kayipliUTF8 bayt dizisini metne çevirir; geçersiz UTF-8 dizilerini U+FFFD
// ile değiştirir. Unicode'un önerdiği "en uzun geçerli önek" kuralı
// uygulanır: yarım kalmış çok baytlı bir dizinin geçerli öneki TEK bir U+FFFD
// olur, kurala uymayan bayt ise yeni bir dizinin başı sayılır. (Standart
// kütüphanenin her geçersiz bayt için ayrı değiştirme yapan davranışından
// farklıdır.)
func kayipliUTF8(b []byte) string {
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
		var gerek int
		alt, ust := byte(0x80), byte(0xBF)
		switch {
		case c >= 0xC2 && c <= 0xDF:
			gerek = 1
		case c == 0xE0:
			gerek, alt = 2, 0xA0
		case c >= 0xE1 && c <= 0xEC, c == 0xEE, c == 0xEF:
			gerek = 2
		case c == 0xED:
			gerek, ust = 2, 0x9F
		case c == 0xF0:
			gerek, alt = 3, 0x90
		case c >= 0xF1 && c <= 0xF3:
			gerek = 3
		case c == 0xF4:
			gerek, ust = 3, 0x8F
		default:
			out = append(out, "�"...)
			i++
			continue
		}
		j := i + 1
		tamam := true
		for k := 0; k < gerek; k++ {
			if j >= len(b) {
				tamam = false
				break
			}
			d := b[j]
			lo, hi := byte(0x80), byte(0xBF)
			if k == 0 {
				lo, hi = alt, ust
			}
			if d < lo || d > hi {
				tamam = false
				break
			}
			j++
		}
		if tamam {
			out = append(out, b[i:j]...)
		} else {
			out = append(out, "�"...)
		}
		i = j
	}
	return string(out)
}

// gecerliMetin geçersiz UTF-8 içeren girdiyi kayıplı çözmeyle onarır.
func gecerliMetin(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return kayipliUTF8([]byte(s))
}
