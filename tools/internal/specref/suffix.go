package specref

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// Bu dosya madde numarasından sonra gelen kesme işaretli ekin (`15.9'daki`) ses uyumunu denetler. Numara
// okunuşuyla değerlendirilir: "15.9" sonu "dokuz" diye okunur, eki "'daki"; "15.3" sonu "üç": "'teki".
// Okunuş bilgisi (son ünlü, ünlüyle bitiş, ötümsüzlük) yeniden yazılmaz, internal/lang/turkish'in
// hâl ekleri paketinden çıkarılır: bildirme hâlinin ünsüzü ötümsüzlüğü, yönelme hâlindeki kaynaştırma
// ünlüyle bitişi, tamlayan hâlinin ünlüsü de son ünlünün yönünü ve yuvarlaklığını verir. Denetim büyük
// ünlü uyumunu (a/e, ı/i/u/ü), ünsüz benzeşmesini (d/t) ve kaynaştırma ünsüzünü (y/n) kapsar; ek
// bunları tanımayan biçimdeyse (o/ö içeren, harfli numara) denetlenmez.

// reading numaranın son sözcüğünün okunuşudur.
type reading struct {
	vowel     rune // son ünlünün dört yönlü karşılığı (ı, i, u, ü)
	endsVowel bool // sözcük ünlüyle bitiyor
	voiceless bool // sözcük ötümsüz ünsüzle bitiyor
}

// readingOf "15.27" için madde sırasının ("27") okunuşunu verir; harfli numarada (15.3b) ok=false.
func readingOf(id string) (reading, bool) {
	dot := strings.IndexByte(id, '.')
	if dot < 0 {
		return reading{}, false
	}
	n := id[dot+1:]
	for _, c := range n {
		if c < '0' || c > '9' {
			return reading{}, false
		}
	}
	loc := caseEnding(n, turkish.Locative)
	gen := caseEnding(n, turkish.Genitive)
	acc := caseEnding(n, turkish.Accusative)
	r := reading{endsVowel: strings.HasPrefix(acc, "y"), voiceless: strings.HasPrefix(loc, "t")}
	for _, c := range gen {
		if isVowel(c) {
			r.vowel = c
			break
		}
	}
	return r, r.vowel != 0
}

// caseEnding sayıya hâl ekini getirir ve kesme işaretinden sonrasını verir ("27", Locative → "de").
func caseEnding(n string, c turkish.Case) string {
	s := turkish.Suffix(n, c)
	return s[strings.IndexByte(s, '\'')+1:]
}

func isVowel(c rune) bool {
	switch c {
	case 'a', 'e', 'ı', 'i', 'o', 'ö', 'u', 'ü':
		return true
	}
	return false
}

func isBack(c rune) bool {
	switch c {
	case 'a', 'ı', 'o', 'u':
		return true
	}
	return false
}

func isRound(c rune) bool {
	switch c {
	case 'o', 'ö', 'u', 'ü':
		return true
	}
	return false
}

// CheckSuffix "15.9" numarasından sonra gelen kesme sonrası eki (`daki`) denetler. ok=true ek doğru ya da
// denetlenemez demektir; ok=false ve want boş değilse doğrusu want, want boşsa düzeltilemeyen bir hatadır
// (kaynaştırma ünsüzü eksik ya da fazla).
func CheckSuffix(id, suffix string) (want string, ok bool) {
	r, known := readingOf(id)
	body := strings.ToLower(suffix)
	if !known || body == "" {
		return "", true
	}
	ki := ""
	if len(body) > 2 && strings.HasSuffix(body, "ki") {
		body, ki = body[:len(body)-2], "ki" // -ki ünlü uyumuna girmez
	}
	rs := []rune(body)
	for _, c := range rs {
		if c == 'o' || c == 'ö' {
			return "", true
		}
	}
	if r.endsVowel && isVowel(rs[0]) {
		return "", false // iki'e: kaynaştırma ünsüzü (y, n, s) eksik
	}
	if !r.endsVowel && len(rs) > 1 && (rs[0] == 'y' || rs[0] == 'n') && isVowel(rs[1]) {
		return "", false // üç'ye: ünsüzden sonra kaynaştırma ünsüzü fazla
	}
	want = string(realize(r, rs)) + ki
	return want, want == strings.ToLower(suffix)
}

// realize ekin ünlülerini ve baştaki d/t'yi numaranın okunuşuna uydurur.
func realize(r reading, rs []rune) []rune {
	out := make([]rune, len(rs))
	vowel := r.vowel
	for i, c := range rs {
		switch {
		case i == 0 && (c == 'd' || c == 't') && len(rs) > 1 && isVowel(rs[1]):
			out[i] = 'd'
			if r.voiceless {
				out[i] = 't'
			}
		case c == 'a' || c == 'e':
			out[i] = 'e'
			if isBack(vowel) {
				out[i] = 'a'
			}
			vowel = out[i]
		case isVowel(c): // ı i u ü
			out[i] = pickFour(isBack(vowel), isRound(vowel))
			vowel = out[i]
		default:
			out[i] = c
		}
	}
	return out
}

// pickFour dört yönlü ünlüyü seçer.
func pickFour(back, round bool) rune {
	switch {
	case back && round:
		return 'u'
	case back:
		return 'ı'
	case round:
		return 'ü'
	}
	return 'i'
}
