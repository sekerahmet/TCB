package parca

import "sort"

// karakterTablosu sıralı, çakışmayan [ilk, son] kod noktası aralıklarıdır.
// Tablolar karaktertablo.go'da başvuru uygulamasından çıkarılarak üretilir
// (bkz. testdata/karakter_tablolari_uret.py).
type karakterTablosu [][2]rune

func (t karakterTablosu) icinde(r rune) bool {
	i := sort.Search(len(t), func(i int) bool { return t[i][1] >= r })
	return i < len(t) && t[i][0] <= r
}

// sozcukKarakteri başvuru uygulamasının "sözcük karakteri" tanımıdır (Unicode
// UTS #18 \w: Alphabetic, M, Nd, Pc, Join_Control). Whitespace ön bölücüsü ve
// ek tokenların single_word seçeneği bunu kullanır.
func sozcukKarakteri(r rune) bool {
	if r < 0x80 {
		return r == '_' || (r >= '0' && r <= '9') || (r|0x20 >= 'a' && r|0x20 <= 'z')
	}
	return tabloSozcuk.icinde(r)
}

// ifadeSozcuk düzenli ifadelerdeki \w sınıfıdır (L, M, N, Pc).
func ifadeSozcuk(r rune) bool {
	if r < 0x80 {
		return r == '_' || (r >= '0' && r <= '9') || (r|0x20 >= 'a' && r|0x20 <= 'z')
	}
	return tabloIfadeSozcuk.icinde(r)
}

// ifadeBosluk düzenli ifadelerdeki \s sınıfıdır.
func ifadeBosluk(r rune) bool {
	if r < 0x80 {
		return r == ' ' || (r >= '\t' && r <= '\r')
	}
	return ifadeBoslukTablo.icinde(r)
}

var (
	ifadeHarfTablo   = ifadeKategori["l"]
	ifadeSayiTablo   = ifadeKategori["n"]
	ifadeBoslukTablo = ifadeKategori["bosluk"]
)
