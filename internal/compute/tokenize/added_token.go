package tokenize

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

type addedMatcher struct {
	firstByte [256]bool
	// ilk bayta göre adaylar, uzundan kısaya sıralı
	candidates [256][]AddedToken
	empty      bool
}

func newAddedMatcher(extras []AddedToken) *addedMatcher {
	e := &addedMatcher{empty: len(extras) == 0}
	for _, t := range extras {
		c := t.Content[0]
		e.firstByte[c] = true
		e.candidates[c] = append(e.candidates[c], t)
	}
	for c := range e.candidates {
		a := e.candidates[c]
		sort.SliceStable(a, func(i, j int) bool { return len(a[i].Content) > len(a[j].Content) })
	}
	return e
}

// ekParca metnin bir bölümüdür: id >= 0 ise ek tokendır, değilse düz metin.
type addedPiece struct {
	begin, last int
	id          int32
}

// bol metni ek tokenlara göre böler. ozelleriAtla doğruysa özel tokenlar
// tanınmaz (düz metin sayılır); sıradan ek tokenlar yine tanınır.
func (e *addedMatcher) splitFn(s string, skipSpecial bool, f func(addedPiece)) {
	if e.empty || s == "" {
		f(addedPiece{0, len(s), -1})
		return
	}
	start := 0 // henüz yayımlanmamış düz metnin başı
	i := 0
	for i < len(s) {
		if !e.firstByte[s[i]] {
			i++
			continue
		}
		var found *AddedToken
		for k := range e.candidates[s[i]] {
			t := &e.candidates[s[i]][k]
			if strings.HasPrefix(s[i:], t.Content) {
				found = t
				break
			}
		}
		if found == nil {
			i++
			continue
		}
		begin, last := i, i+len(found.Content)
		i = last
		if skipSpecial && found.Special {
			continue
		}
		if found.SingleWord {
			if begin > 0 {
				r, _ := utf8.DecodeLastRuneInString(s[:begin])
				if wordChar(r) {
					continue
				}
			}
			if last < len(s) {
				r, _ := utf8.DecodeRuneInString(s[last:])
				if wordChar(r) {
					continue
				}
			}
		}
		if found.LStrip {
			fresh := len(strings.TrimRightFunc(s[:begin], unicode.IsSpace))
			if fresh < start {
				fresh = start
			}
			begin = fresh
		}
		if found.RStrip {
			last = len(s) - len(strings.TrimLeftFunc(s[last:], unicode.IsSpace))
		}
		if start < begin {
			f(addedPiece{start, begin, -1})
		}
		f(addedPiece{begin, last, int32(found.ID)})
		start = last
	}
	if start < len(s) {
		f(addedPiece{start, len(s), -1})
	}
}
