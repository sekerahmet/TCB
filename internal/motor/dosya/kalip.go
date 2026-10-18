package dosya

import (
	"errors"
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"
)

// klasörü_listele kalıbı: / ile ayrılmış bileşenler. Bir bileşenin içinde * (sıfır ya da çok
// karakter), ? (tek karakter) ve [a-z] / [!a-z] / [^a-z] (karakter kümesi) özeldir; ** yalnızca
// tam bir bileşen olarak sıfır ya da çok klasör demektir. {a,b} seçeneği yoktur (TR metninde
// { yerleştirmedir). Nokta ile başlayan bir ad, kalıbın o bileşeni de nokta ile başlamadıkça
// eşlenmez; ** gizli klasörlere girmez. Windows ve macOS'ta harf büyüklüğü fark etmez
// (işletim sistemi gibi), öteki sistemlerde eder.

// harfDuyarsiz eşlemenin büyük/küçük harfi ayırmadığı sistemlerdir.
var harfDuyarsiz = runtime.GOOS == "windows" || runtime.GOOS == "darwin"

// kalipBileseni kalıbın bir bileşenidir.
type kalipBileseni struct {
	cift  bool   // ** (sıfır ya da çok klasör)
	metin string // bileşenin kalıbı (cift değilse)
	sabit bool   // özel karakter yok: ad olduğu gibi eşlenir
}

// kalip derlenmiş bir klasörü_listele kalıbıdır.
type kalip struct {
	bilesenler []kalipBileseni
	gizli      bool // bir bileşen nokta ile başlıyor (gizli adlara bakmak gerekir)
}

// Kalıp hataları (mesajlar yerleşiğin adıyla sarılır).
var (
	errKalipBos     = errors.New("kalıp boş olamaz")
	errKalipMutlak  = errors.New("kalıp göreli olmalı; taban klasörü birinci argümanla verin")
	errKalipUst     = errors.New("kalıpta .. olamaz; üst klasörü taban klasör olarak verin")
	errKalipCift    = errors.New("** yalnızca tam bir bileşen olabilir (ör. \"**/*.npy\"; \"a**\" yazılamaz)")
	errKalipKume    = errors.New("[ ile açılan karakter kümesi ] ile kapanmıyor")
	errKalipBosKume = errors.New("boş karakter kümesi []")
)

// kalipDerle kalıbı bileşenlerine ayırır ve denetler.
func kalipDerle(s string) (*kalip, error) {
	if s == "" {
		return nil, errKalipBos
	}
	if runtime.GOOS == "windows" {
		s = strings.ReplaceAll(s, `\`, "/")
	}
	if mutlakParcaMi(s) {
		return nil, errKalipMutlak
	}
	k := &kalip{}
	for _, b := range strings.Split(s, "/") {
		switch {
		case b == "" || b == ".":
			continue
		case b == "..":
			return nil, errKalipUst
		case b == "**":
			if n := len(k.bilesenler); n > 0 && k.bilesenler[n-1].cift {
				continue // **/** tek ** ile aynıdır
			}
			k.bilesenler = append(k.bilesenler, kalipBileseni{cift: true})
			continue
		case strings.Contains(b, "**"):
			return nil, errKalipCift
		}
		if err := bilesenDenetle(b); err != nil {
			return nil, err
		}
		k.bilesenler = append(k.bilesenler, kalipBileseni{metin: b, sabit: !strings.ContainsAny(b, "*?[")})
		if b[0] == '.' {
			k.gizli = true
		}
	}
	if len(k.bilesenler) == 0 {
		return nil, errKalipBos
	}
	return k, nil
}

// bilesenDenetle karakter kümelerinin kapandığını denetler.
func bilesenDenetle(b string) error {
	for i := 0; i < len(b); i++ {
		if b[i] != '[' {
			continue
		}
		son, ok := kumeSonu(b, i)
		if !ok {
			return errKalipKume
		}
		if son == i+1 || (son == i+2 && (b[i+1] == '!' || b[i+1] == '^')) {
			return errKalipBosKume
		}
		i = son
	}
	return nil
}

// kumeSonu b[i]'deki [ ile açılan kümenin ] konumunu verir. Kümenin ilk karakteri (olumsuzlama
// işaretinden sonra) ] ise kapanış değil, kümenin bir elemanıdır.
func kumeSonu(b string, i int) (int, bool) {
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

// gizliEslesir bileşen nokta ile başlayan bir adı eşleyebilir mi?
func (b kalipBileseni) gizliEslesir() bool { return !b.cift && strings.HasPrefix(b.metin, ".") }

// eslesir ad bileşenle eşleşiyor mu (** için kullanılmaz)?
func (b kalipBileseni) eslesir(ad string) bool {
	if strings.HasPrefix(ad, ".") && !b.gizliEslesir() {
		return false
	}
	if b.sabit {
		if harfDuyarsiz {
			return strings.EqualFold(b.metin, ad)
		}
		return b.metin == ad
	}
	return joker(b.metin, ad)
}

// joker tek bir bileşen kalıbını adla eşler (* ve ? ve [küme]). Yıldızda geri izleme son
// yıldıza döner; süre kalıp × ad ile sınırlıdır (üstel patlama yok).
func joker(k, ad string) bool {
	ki, ai := 0, 0
	yildizK, yildizA := -1, 0
	for ai < len(ad) {
		if ki < len(k) {
			switch k[ki] {
			case '*':
				yildizK, yildizA = ki, ai
				ki++
				continue
			case '?':
				_, n := utf8.DecodeRuneInString(ad[ai:])
				ki++
				ai += n
				continue
			case '[':
				r, n := utf8.DecodeRuneInString(ad[ai:])
				if son, ok := kumeSonu(k, ki); ok && kumeEslesir(k[ki+1:son], r) {
					ki = son + 1
					ai += n
					continue
				}
			default:
				kr, kn := utf8.DecodeRuneInString(k[ki:])
				ar, an := utf8.DecodeRuneInString(ad[ai:])
				if harfEsit(kr, ar) {
					ki += kn
					ai += an
					continue
				}
			}
		}
		if yildizK < 0 {
			return false
		}
		_, n := utf8.DecodeRuneInString(ad[yildizA:])
		yildizA += n
		ki, ai = yildizK+1, yildizA
	}
	for ki < len(k) && k[ki] == '*' {
		ki++
	}
	return ki == len(k)
}

// kumeEslesir r, kümenin içi olan ic'e (köşeli ayraçlar olmadan) uyuyor mu?
func kumeEslesir(ic string, r rune) bool {
	olumsuz := false
	if ic != "" && (ic[0] == '!' || ic[0] == '^') {
		olumsuz, ic = true, ic[1:]
	}
	uyar := false
	for i := 0; i < len(ic); {
		bas, n := utf8.DecodeRuneInString(ic[i:])
		i += n
		son := bas
		if i+1 < len(ic) && ic[i] == '-' {
			s, m := utf8.DecodeRuneInString(ic[i+1:])
			son = s
			i += 1 + m
		}
		if aralikta(r, bas, son) {
			uyar = true
		}
	}
	return uyar != olumsuz
}

func aralikta(r, bas, son rune) bool {
	if bas <= r && r <= son {
		return true
	}
	if !harfDuyarsiz {
		return false
	}
	for _, d := range []rune{unicode.ToUpper(r), unicode.ToLower(r)} {
		if bas <= d && d <= son {
			return true
		}
	}
	return false
}

func harfEsit(a, b rune) bool {
	return a == b || (harfDuyarsiz && unicode.ToUpper(a) == unicode.ToUpper(b))
}
