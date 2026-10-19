package tokenize

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

// ============================================================================
// Unicode normalleştirme (NFD, NFC, NFKD, NFKC)
// ============================================================================
//
// Tablolar normtablo.go'dadır (üretilmiş). Algoritma Unicode Standart Ek #15:
// tam (özyinelemeli) ayrıştırma, kanonik sıralama (sıfır olmayan birleşim
// sınıflı karakter dizilerinin sınıfa göre kararlı sıralanması) ve gerekirse
// kanonik birleştirme. Hangıl heceleri algoritmik olarak işlenir.

type unormData struct {
	canonical  map[rune][]rune // tam kanonik ayrıştırma
	compatible map[rune][]rune // tam uyumluluk ayrıştırması (kanonik dahil)
	union      map[uint64]rune // (ilk, ikinci) -> birincil birleşik
	class      map[rune]uint8
}

var (
	unormOne   sync.Once
	unormTable *unormData
)

const (
	hangulS      = 0xAC00
	hangulL      = 0x1100
	hangulV      = 0x1161
	hangulT      = 0x11A7
	hangulLCount = 19
	hangulVCount = 21
	hangulTCount = 28
	hangulNCount = hangulVCount * hangulTCount
	hangulSCount = hangulLCount * hangulNCount
)

func loadUnorm() *unormData {
	unormOne.Do(func() {
		v := &unormData{
			canonical:  make(map[rune][]rune),
			compatible: make(map[rune][]rune),
			union:      make(map[uint64]rune),
			class:      make(map[rune]uint8),
		}
		hex := func(s string) rune {
			n, _ := strconv.ParseUint(s, 16, 32)
			return rune(n)
		}
		type input struct {
			kind byte
			pt   []rune
		}
		raw := make(map[rune]input)
		for _, line := range strings.Split(parseData, "\n") {
			field := strings.Fields(line)
			if len(field) < 3 {
				continue
			}
			kn := hex(field[0])
			var p []rune
			for _, a := range field[2:] {
				p = append(p, hex(a))
			}
			raw[kn] = input{field[1][0], p}
			if field[1][0] == 'c' {
				v.union[uint64(p[0])<<32|uint64(p[1])] = kn
			}
		}
		var split func(r rune, compat bool, out []rune) []rune
		split = func(r rune, compat bool, out []rune) []rune {
			g, ok := raw[r]
			if !ok || (g.kind == 'k' && !compat) {
				return append(out, r)
			}
			for _, x := range g.pt {
				out = split(x, compat, out)
			}
			return out
		}
		for kn, g := range raw {
			if g.kind != 'k' {
				v.canonical[kn] = split(kn, false, nil)
			}
			v.compatible[kn] = split(kn, true, nil)
		}
		for _, line := range strings.Split(unionClassData, "\n") {
			field := strings.Fields(line)
			if len(field) != 3 {
				continue
			}
			s, _ := strconv.Atoi(field[2])
			for r := hex(field[0]); r <= hex(field[1]); r++ {
				v.class[r] = uint8(s)
			}
		}
		unormTable = v
	})
	return unormTable
}

// unicodeNormallestir metni verilen biçime çevirir. uyum: K biçimleri,
// birlestir: C biçimleri.
func unicodeNormalize(s string, compat, joinFn bool) string {
	// Hızlı yol: yalnızca ASCII içeren metin her biçimde değişmez.
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		return s
	}
	v := loadUnorm()
	table := v.canonical
	if compat {
		table = v.compatible
	}
	// 1) Ayrıştırma
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= hangulS && r < hangulS+hangulSCount {
			si := r - hangulS
			out = append(out, hangulL+si/hangulNCount, hangulV+(si%hangulNCount)/hangulTCount)
			if t := si % hangulTCount; t != 0 {
				out = append(out, hangulT+t)
			}
			continue
		}
		if d, ok := table[r]; ok {
			out = append(out, d...)
		} else {
			out = append(out, r)
		}
	}
	// 2) Kanonik sıralama
	for i := 0; i < len(out); {
		if v.class[out[i]] == 0 {
			i++
			continue
		}
		j := i
		for j < len(out) && v.class[out[j]] != 0 {
			j++
		}
		if j-i > 1 {
			piece := out[i:j]
			sort.SliceStable(piece, func(a, b int) bool { return v.class[piece[a]] < v.class[piece[b]] })
		}
		i = j
	}
	if !joinFn {
		return string(out)
	}
	// 3) Kanonik birleştirme
	result := out[:0]
	start := -1
	lastClass := uint8(0)
	for _, r := range out {
		s := v.class[r]
		if start >= 0 {
			adjacent := len(result)-1 == start
			if adjacent || (lastClass != 0 && lastClass < s) {
				if c, ok := v.joinFn(result[start], r); ok {
					result[start] = c
					continue
				}
			}
		}
		if s == 0 {
			start = len(result)
			lastClass = 0
		} else {
			lastClass = s
		}
		result = append(result, r)
	}
	return string(result)
}

func (v *unormData) joinFn(a, b rune) (rune, bool) {
	// Hangıl: L + V -> LV, LV + T -> LVT
	if a >= hangulL && a < hangulL+hangulLCount && b >= hangulV && b < hangulV+hangulVCount {
		return hangulS + ((a-hangulL)*hangulVCount+(b-hangulV))*hangulTCount, true
	}
	if a >= hangulS && a < hangulS+hangulSCount && (a-hangulS)%hangulTCount == 0 && b > hangulT && b < hangulT+hangulTCount {
		return a + (b - hangulT), true
	}
	c, ok := v.union[uint64(a)<<32|uint64(b)]
	return c, ok
}
