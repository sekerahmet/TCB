package tokenize

import "unicode/utf8"

// ============================================================================
// Ön bölme (pre-tokenization)
// ============================================================================
//
// Desteklenen bölme deseni:
//
//	[’']\p{L}+| ?\p{L}+| ?\p{N}| ?[^\s\p{L}\p{N}]+|\s+(?!\S)|\s+
//
// Desendeki "\s+(?!\S)" ileriye bakma içerdiğinden standart kütüphanenin
// düzenli ifade motoruyla yazılamaz; eşleştirme burada elle yapılır. Kurallar
// geri izlemeli (backtracking) bir motorun davranışıyla birebir aynıdır:
// her konumda seçenekler soldan sağa denenir, ilk tutan seçenek alınır ve
// eşleşme bitiminden aramaya devam edilir. Desen her karakteri kapsadığından
// eşleşmeler arasında boşluk kalmaz; parçaların birleşimi girdinin kendisidir.
//
// Karakter sınıfları:
//   - \p{L}: harf (Lu, Ll, Lt, Lm, Lo)
//   - \p{N}: sayı (Nd, Nl, No)
//   - \s   : Unicode "White_Space" (9-13, 0x20, 0x85, 0xA0, Zs, Zl, Zp)

// BolmeDeseni bu paketin elle uyguladığı tek ön bölme desenidir.
const SplitPattern = `[’']\p{L}+| ?\p{L}+| ?\p{N}| ?[^\s\p{L}\p{N}]+|\s+(?!\S)|\s+`

// Karakter sınıfı bitleri (ASCII için önceden hesaplanır).
const (
	classLetter = 1
	classDigit  = 2
	classSpace  = 4
)

// Sınıflar başvuru uygulamasının düzenli ifade motorundan çıkarılmış
// tablolardan gelir (karaktertablo.go); sık kullanılan ilk 0x800 kod noktası
// için önceden hesaplanmış bir dizi kullanılır.
var lowClass [0x800]uint8

func init() {
	for c := 0; c < len(lowClass); c++ {
		lowClass[c] = computeClass(rune(c))
	}
}

func computeClass(r rune) uint8 {
	switch {
	case exprLetterTable.inside(r):
		return classLetter
	case exprDigitTable.inside(r):
		return classDigit
	case exprSpaceTable.inside(r):
		return classSpace
	}
	return 0
}

// sinif bir karakterin sınıf bitini verir (0: "diğer").
func class(r rune) uint8 {
	if r >= 0 && r < rune(len(lowClass)) {
		return lowClass[r]
	}
	return computeClass(r)
}

// karakter s[i] konumundaki karakteri ve bayt genişliğini verir.
func char(s string, i int) (rune, int) {
	if c := s[i]; c < utf8.RuneSelf {
		return rune(c), 1
	}
	return utf8.DecodeRuneInString(s[i:])
}

// harfSonu i'den başlayan en uzun harf dizisinin bitiş konumunu verir.
func letterEnd(s string, i int) int {
	for i < len(s) {
		r, w := char(s, i)
		if class(r) != classLetter {
			break
		}
		i += w
	}
	return i
}

// digerSonu i'den başlayan, harf/sayı/boşluk olmayan en uzun dizinin sonunu verir.
func otherEnd(s string, i int) int {
	for i < len(s) {
		r, w := char(s, i)
		if class(r) != 0 {
			break
		}
		i += w
	}
	return i
}

// eslesmeSonu i konumunda başlayan eşleşmenin bitiş konumunu verir (i < len(s)).
func matchEnd(s string, i int) int {
	r, w := char(s, i)
	n := len(s)
	// 1) [’']\p{L}+
	if r == '\'' || r == '’' {
		if i+w < n {
			if r2, _ := char(s, i+w); class(r2) == classLetter {
				return letterEnd(s, i+w)
			}
		}
	}
	// 2-4) Başta isteğe bağlı tek boşluk: " ?\p{L}+", " ?\p{N}", " ?[^\s\p{L}\p{N}]+"
	if r == ' ' && i+1 < n {
		r2, w2 := char(s, i+1)
		switch class(r2) {
		case classLetter:
			return letterEnd(s, i+1)
		case classDigit:
			return i + 1 + w2
		case 0:
			return otherEnd(s, i+1)
		}
		// Ardından boşluk geliyorsa boşluk kurallarına düşülür.
	}
	switch class(r) {
	case classLetter:
		return letterEnd(s, i)
	case classDigit:
		return i + w
	case 0:
		return otherEnd(s, i)
	}
	// 5-6) Boşluk dizisi: "\s+(?!\S)" önce en uzun diziyi alır; dizi metnin
	// sonuna kadar sürüyorsa tamamı eşleşir. Değilse (ardından boşluk olmayan
	// bir karakter geliyorsa) son boşluk karakteri geri bırakılır; böylece o
	// karakter sonraki sözcüğe (ör. " kelime") katılabilir. Dizi tek
	// karakterse ileriye bakma tutmaz ve "\s+" tek karakteri alır.
	j := i + w
	last := i // dizideki son karakterin başlangıcı
	for j < n {
		r2, w2 := char(s, j)
		if class(r2) != classSpace {
			break
		}
		last = j
		j += w2
	}
	if j == n || last == i {
		return j
	}
	return last
}

// onBol metni desene göre parçalara böler ve her parça için f'yi çağırır.
func preTokenize(s string, f func(piece string)) {
	for i := 0; i < len(s); {
		j := matchEnd(s, i)
		f(s[i:j])
		i = j
	}
}

// DesenleBol metni BolmeDeseni'ne göre parçalara ayırır (sınama ve inceleme için).
func SplitByPattern(text string) []string {
	text = validText(text)
	var out []string
	preTokenize(text, func(p string) { out = append(out, p) })
	return out
}
