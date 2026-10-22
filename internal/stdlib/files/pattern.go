package files

import (
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// klasörü_listele kalıbı: / ile ayrılmış bileşenler. Bir bileşenin içinde * (sıfır ya da çok
// karakter), ? (tek karakter) ve [a-z] / [!a-z] / [^a-z] (karakter kümesi) özeldir; ** yalnızca
// tam bir bileşen olarak sıfır ya da çok klasör demektir. {a,b} seçeneği yoktur (TR metninde
// { yerleştirmedir). Nokta ile başlayan bir ad, kalıbın o bileşeni de nokta ile başlamadıkça
// eşlenmez; ** gizli klasörlere girmez. Windows ve macOS'ta harf büyüklüğü fark etmez
// (işletim sistemi gibi), öteki sistemlerde eder.

// caseInsensitive eşlemenin büyük/küçük harfi ayırmadığı sistemlerdir.
var caseInsensitive = runtime.GOOS == "windows" || runtime.GOOS == "darwin"

// patternPart kalıbın bir bileşenidir.
type patternPart struct {
	double bool   // ** (sıfır ya da çok klasör)
	text   string // bileşenin kalıbı (cift değilse)
	fixed  bool   // özel karakter yok: ad olduğu gibi eşlenir
}

// pattern derlenmiş bir klasörü_listele kalıbıdır.
type pattern struct {
	components []patternPart
	hidden     bool // bir bileşen nokta ile başlıyor (gizli adlara bakmak gerekir)
}

// Kalıp hataları (mesajlar yerleşiğin adıyla sarılır).
var (
	errPatternEmpty    = messages.New(messages.H7141)
	errPatternAbsolute = messages.New(messages.H7142)
	errPatternParent   = messages.New(messages.H7143)
	errPatternDouble   = messages.New(messages.H7144)
	errPatternSet      = messages.New(messages.H7145)
	errPatternEmptySet = messages.New(messages.H7146)
)

// compilePattern kalıbı bileşenlerine ayırır ve denetler.
func compilePattern(s string) (*pattern, error) {
	if s == "" {
		return nil, errPatternEmpty
	}
	if runtime.GOOS == "windows" {
		s = strings.ReplaceAll(s, `\`, "/")
	}
	if isAbsPart(s) {
		return nil, errPatternAbsolute
	}
	k := &pattern{}
	for _, b := range strings.Split(s, "/") {
		switch {
		case b == "" || b == ".":
			continue
		case b == "..":
			return nil, errPatternParent
		case b == "**":
			if n := len(k.components); n > 0 && k.components[n-1].double {
				continue // **/** tek ** ile aynıdır
			}
			k.components = append(k.components, patternPart{double: true})
			continue
		case strings.Contains(b, "**"):
			return nil, errPatternDouble
		}
		if err := checkComponent(b); err != nil {
			return nil, err
		}
		k.components = append(k.components, patternPart{text: b, fixed: !strings.ContainsAny(b, "*?[")})
		if b[0] == '.' {
			k.hidden = true
		}
	}
	if len(k.components) == 0 {
		return nil, errPatternEmpty
	}
	return k, nil
}

// checkComponent karakter kümelerinin kapandığını denetler.
func checkComponent(b string) error {
	for i := 0; i < len(b); i++ {
		if b[i] != '[' {
			continue
		}
		last, ok := setEnd(b, i)
		if !ok {
			return errPatternSet
		}
		if last == i+1 || (last == i+2 && (b[i+1] == '!' || b[i+1] == '^')) {
			return errPatternEmptySet
		}
		i = last
	}
	return nil
}

// setEnd b[i]'deki [ ile açılan kümenin ] konumunu verir. Kümenin ilk karakteri (olumsuzlama
// işaretinden sonra) ] ise kapanış değil, kümenin bir elemanıdır.
func setEnd(b string, i int) (int, bool) {
	j := i + 1
	if j < len(b) && (b[j] == '!' || b[j] == '^') {
		j++
	}
	if j < len(b) && b[j] == ']' {
		j++
	}
	for ; j < len(b); j++ {
		if b[j] == ']' {
			return j, true
		}
	}
	return 0, false
}

// hiddenMatch bileşen nokta ile başlayan bir adı eşleyebilir mi?
func (b patternPart) hiddenMatch() bool { return !b.double && strings.HasPrefix(b.text, ".") }

// match ad bileşenle eşleşiyor mu (** için kullanılmaz)?
func (b patternPart) match(name string) bool {
	if strings.HasPrefix(name, ".") && !b.hiddenMatch() {
		return false
	}
	if b.fixed {
		return samePath(b.text, name)
	}
	return wildcard(b.text, name)
}

// wildcard tek bir bileşen kalıbını adla eşler (* ve ? ve [küme]). Yıldızda geri izleme son
// yıldıza döner; süre kalıp × ad ile sınırlıdır (üstel patlama yok).
func wildcard(k, name string) bool {
	ki, ai := 0, 0
	starK, starA := -1, 0
	for ai < len(name) {
		if ki < len(k) {
			switch k[ki] {
			case '*':
				starK, starA = ki, ai
				ki++
				continue
			case '?':
				_, n := utf8.DecodeRuneInString(name[ai:])
				ki++
				ai += n
				continue
			case '[':
				r, n := utf8.DecodeRuneInString(name[ai:])
				if last, ok := setEnd(k, ki); ok && setMatch(k[ki+1:last], r) {
					ki = last + 1
					ai += n
					continue
				}
			default:
				kr, kn := utf8.DecodeRuneInString(k[ki:])
				ar, an := utf8.DecodeRuneInString(name[ai:])
				if equalFold(kr, ar) {
					ki += kn
					ai += an
					continue
				}
			}
		}
		if starK < 0 {
			return false
		}
		_, n := utf8.DecodeRuneInString(name[starA:])
		starA += n
		ki, ai = starK+1, starA
	}
	for ki < len(k) && k[ki] == '*' {
		ki++
	}
	return ki == len(k)
}

// setMatch r, kümenin içi olan ic'e (köşeli ayraçlar olmadan) uyuyor mu?
func setMatch(inner string, r rune) bool {
	negated := false
	if inner != "" && (inner[0] == '!' || inner[0] == '^') {
		negated, inner = true, inner[1:]
	}
	warn := false
	for i := 0; i < len(inner); {
		head, n := utf8.DecodeRuneInString(inner[i:])
		i += n
		last := head
		if i+1 < len(inner) && inner[i] == '-' {
			s, m := utf8.DecodeRuneInString(inner[i+1:])
			last = s
			i += 1 + m
		}
		if inRange(r, head, last) {
			warn = true
		}
	}
	return warn != negated
}

func inRange(r, head, last rune) bool {
	if head <= r && r <= last {
		return true
	}
	if !caseInsensitive {
		return false
	}
	for _, d := range []rune{unicode.ToUpper(r), unicode.ToLower(r)} {
		if head <= d && d <= last {
			return true
		}
	}
	return false
}

func equalFold(a, b rune) bool {
	return a == b || (caseInsensitive && unicode.ToUpper(a) == unicode.ToUpper(b))
}
