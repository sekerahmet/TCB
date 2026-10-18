package parcalayici

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ============================================================================
// Ek (özel) tokenları metin içinde bulma
// ============================================================================
//
// Ek tokenlar ön bölmeden ÖNCE ham metinde aranır. Arama "en soldaki, en
// uzun" kuralıyla yapılır: metinde en erken başlayan eşleşme, o konumda
// birden çok aday varsa en uzunu seçilir; arama eşleşmenin bitiminden devam
// eder (eşleşmeler çakışmaz). Seçenekler:
//   - single_word: tokenın hemen önünde/arkasında harf ya da rakam varsa
//     eşleşme yok sayılır (arama yine eşleşme bitiminden sürer).
//   - lstrip / rstrip: tokenın solundaki / sağındaki boşluklar tokena katılır.

type ekEslestirici struct {
	ilkBayt [256]bool
	// ilk bayta göre adaylar, uzundan kısaya sıralı
	adaylar [256][]EkToken
	bos     bool
}

func yeniEkEslestirici(ekler []EkToken) *ekEslestirici {
	e := &ekEslestirici{bos: len(ekler) == 0}
	for _, t := range ekler {
		c := t.Icerik[0]
		e.ilkBayt[c] = true
		e.adaylar[c] = append(e.adaylar[c], t)
	}
	for c := range e.adaylar {
		a := e.adaylar[c]
		sort.SliceStable(a, func(i, j int) bool { return len(a[i].Icerik) > len(a[j].Icerik) })
	}
	return e
}

// ekParca metnin bir bölümüdür: id >= 0 ise ek tokendır, değilse düz metin.
type ekParca struct {
	bas, son int
	id       int32
}

// bol metni ek tokenlara göre böler. ozelleriAtla doğruysa özel tokenlar
// tanınmaz (düz metin sayılır); sıradan ek tokenlar yine tanınır.
func (e *ekEslestirici) bol(s string, ozelleriAtla bool, f func(ekParca)) {
	if e.bos || s == "" {
		f(ekParca{0, len(s), -1})
		return
	}
	baslangic := 0 // henüz yayımlanmamış düz metnin başı
	i := 0
	for i < len(s) {
		if !e.ilkBayt[s[i]] {
			i++
			continue
		}
		var bulunan *EkToken
		for k := range e.adaylar[s[i]] {
			t := &e.adaylar[s[i]][k]
			if strings.HasPrefix(s[i:], t.Icerik) {
				bulunan = t
				break
			}
		}
		if bulunan == nil {
			i++
			continue
		}
		bas, son := i, i+len(bulunan.Icerik)
		i = son
		if ozelleriAtla && bulunan.Ozel {
			continue
		}
		if bulunan.TekSozcuk {
			if bas > 0 {
				r, _ := utf8.DecodeLastRuneInString(s[:bas])
				if sozcukKarakteri(r) {
					continue
				}
			}
			if son < len(s) {
				r, _ := utf8.DecodeRuneInString(s[son:])
				if sozcukKarakteri(r) {
					continue
				}
			}
		}
		if bulunan.SolKirp {
			yeni := len(strings.TrimRightFunc(s[:bas], unicode.IsSpace))
			if yeni < baslangic {
				yeni = baslangic
			}
			bas = yeni
		}
		if bulunan.SagKirp {
			son = len(s) - len(strings.TrimLeftFunc(s[son:], unicode.IsSpace))
		}
		if baslangic < bas {
			f(ekParca{baslangic, bas, -1})
		}
		f(ekParca{bas, son, int32(bulunan.ID)})
		baslangic = son
	}
	if baslangic < len(s) {
		f(ekParca{baslangic, len(s), -1})
	}
}
