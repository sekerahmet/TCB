package parcalayici

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

type unormVeri struct {
	kanonik  map[rune][]rune // tam kanonik ayrıştırma
	uyumlu   map[rune][]rune // tam uyumluluk ayrıştırması (kanonik dahil)
	birlesim map[uint64]rune // (ilk, ikinci) -> birincil birleşik
	sinif    map[rune]uint8
}

var (
	unormBir   sync.Once
	unormTablo *unormVeri
)

const (
	hangulS     = 0xAC00
	hangulL     = 0x1100
	hangulV     = 0x1161
	hangulT     = 0x11A7
	hangulLSayi = 19
	hangulVSayi = 21
	hangulTSayi = 28
	hangulNSayi = hangulVSayi * hangulTSayi
	hangulSSayi = hangulLSayi * hangulNSayi
)

func unormYukle() *unormVeri {
	unormBir.Do(func() {
		v := &unormVeri{
			kanonik:  make(map[rune][]rune),
			uyumlu:   make(map[rune][]rune),
			birlesim: make(map[uint64]rune),
			sinif:    make(map[rune]uint8),
		}
		hex := func(s string) rune {
			n, _ := strconv.ParseUint(s, 16, 32)
			return rune(n)
		}
		type girdi struct {
			tur  byte
			parc []rune
		}
		ham := make(map[rune]girdi)
		for _, satir := range strings.Split(ayristirmaVerisi, "\n") {
			alan := strings.Fields(satir)
			if len(alan) < 3 {
				continue
			}
			kn := hex(alan[0])
			var p []rune
			for _, a := range alan[2:] {
				p = append(p, hex(a))
			}
			ham[kn] = girdi{alan[1][0], p}
			if alan[1][0] == 'c' {
				v.birlesim[uint64(p[0])<<32|uint64(p[1])] = kn
			}
		}
		var ayir func(r rune, uyum bool, out []rune) []rune
		ayir = func(r rune, uyum bool, out []rune) []rune {
			g, ok := ham[r]
			if !ok || (g.tur == 'k' && !uyum) {
				return append(out, r)
			}
			for _, x := range g.parc {
				out = ayir(x, uyum, out)
			}
			return out
		}
		for kn, g := range ham {
			if g.tur != 'k' {
				v.kanonik[kn] = ayir(kn, false, nil)
			}
			v.uyumlu[kn] = ayir(kn, true, nil)
		}
		for _, satir := range strings.Split(birlesimSinifiVerisi, "\n") {
			alan := strings.Fields(satir)
			if len(alan) != 3 {
				continue
			}
			s, _ := strconv.Atoi(alan[2])
			for r := hex(alan[0]); r <= hex(alan[1]); r++ {
				v.sinif[r] = uint8(s)
			}
		}
		unormTablo = v
	})
	return unormTablo
}

// unicodeNormallestir metni verilen biçime çevirir. uyum: K biçimleri,
// birlestir: C biçimleri.
func unicodeNormallestir(s string, uyum, birlestir bool) string {
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
	v := unormYukle()
	tablo := v.kanonik
	if uyum {
		tablo = v.uyumlu
	}
	// 1) Ayrıştırma
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= hangulS && r < hangulS+hangulSSayi {
			si := r - hangulS
			out = append(out, hangulL+si/hangulNSayi, hangulV+(si%hangulNSayi)/hangulTSayi)
			if t := si % hangulTSayi; t != 0 {
				out = append(out, hangulT+t)
			}
			continue
		}
		if d, ok := tablo[r]; ok {
			out = append(out, d...)
		} else {
			out = append(out, r)
		}
	}
	// 2) Kanonik sıralama
	for i := 0; i < len(out); {
		if v.sinif[out[i]] == 0 {
			i++
			continue
		}
		j := i
		for j < len(out) && v.sinif[out[j]] != 0 {
			j++
		}
		if j-i > 1 {
			parca := out[i:j]
			sort.SliceStable(parca, func(a, b int) bool { return v.sinif[parca[a]] < v.sinif[parca[b]] })
		}
		i = j
	}
	if !birlestir {
		return string(out)
	}
	// 3) Kanonik birleştirme
	sonuc := out[:0]
	baslangic := -1
	sonSinif := uint8(0)
	for _, r := range out {
		s := v.sinif[r]
		if baslangic >= 0 {
			bitisik := len(sonuc)-1 == baslangic
			if bitisik || (sonSinif != 0 && sonSinif < s) {
				if c, ok := v.birlestir(sonuc[baslangic], r); ok {
					sonuc[baslangic] = c
					continue
				}
			}
		}
		if s == 0 {
			baslangic = len(sonuc)
			sonSinif = 0
		} else {
			sonSinif = s
		}
		sonuc = append(sonuc, r)
	}
	return string(sonuc)
}

func (v *unormVeri) birlestir(a, b rune) (rune, bool) {
	// Hangıl: L + V -> LV, LV + T -> LVT
	if a >= hangulL && a < hangulL+hangulLSayi && b >= hangulV && b < hangulV+hangulVSayi {
		return hangulS + ((a-hangulL)*hangulVSayi+(b-hangulV))*hangulTSayi, true
	}
	if a >= hangulS && a < hangulS+hangulSSayi && (a-hangulS)%hangulTSayi == 0 && b > hangulT && b < hangulT+hangulTSayi {
		return a + (b - hangulT), true
	}
	c, ok := v.birlesim[uint64(a)<<32|uint64(b)]
	return c, ok
}
