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
const BolmeDeseni = `[’']\p{L}+| ?\p{L}+| ?\p{N}| ?[^\s\p{L}\p{N}]+|\s+(?!\S)|\s+`

// Karakter sınıfı bitleri (ASCII için önceden hesaplanır).
const (
	sinifHarf   = 1
	sinifSayi   = 2
	sinifBosluk = 4
)

// Sınıflar başvuru uygulamasının düzenli ifade motorundan çıkarılmış
// tablolardan gelir (karaktertablo.go); sık kullanılan ilk 0x800 kod noktası
// için önceden hesaplanmış bir dizi kullanılır.
var dusukSinif [0x800]uint8

func init() {
	for c := 0; c < len(dusukSinif); c++ {
		dusukSinif[c] = sinifHesapla(rune(c))
	}
}

func sinifHesapla(r rune) uint8 {
	switch {
	case ifadeHarfTablo.icinde(r):
		return sinifHarf
	case ifadeSayiTablo.icinde(r):
		return sinifSayi
	case ifadeBoslukTablo.icinde(r):
		return sinifBosluk
	}
	return 0
}

// sinif bir karakterin sınıf bitini verir (0: "diğer").
func sinif(r rune) uint8 {
	if r >= 0 && r < rune(len(dusukSinif)) {
		return dusukSinif[r]
	}
	return sinifHesapla(r)
}

// karakter s[i] konumundaki karakteri ve bayt genişliğini verir.
func karakter(s string, i int) (rune, int) {
	if c := s[i]; c < utf8.RuneSelf {
		return rune(c), 1
	}
	return utf8.DecodeRuneInString(s[i:])
}

// harfSonu i'den başlayan en uzun harf dizisinin bitiş konumunu verir.
func harfSonu(s string, i int) int {
	for i < len(s) {
		r, w := karakter(s, i)
		if sinif(r) != sinifHarf {
			break
		}
		i += w
	}
	return i
}

// digerSonu i'den başlayan, harf/sayı/boşluk olmayan en uzun dizinin sonunu verir.
func digerSonu(s string, i int) int {
	for i < len(s) {
		r, w := karakter(s, i)
		if sinif(r) != 0 {
			break
		}
		i += w
	}
	return i
}

// eslesmeSonu i konumunda başlayan eşleşmenin bitiş konumunu verir (i < len(s)).
func eslesmeSonu(s string, i int) int {
	r, w := karakter(s, i)
	n := len(s)
	// 1) [’']\p{L}+
	if r == '\'' || r == '’' {
		if i+w < n {
			if r2, _ := karakter(s, i+w); sinif(r2) == sinifHarf {
				return harfSonu(s, i+w)
			}
		}
	}
	// 2-4) Başta isteğe bağlı tek boşluk: " ?\p{L}+", " ?\p{N}", " ?[^\s\p{L}\p{N}]+"
	if r == ' ' && i+1 < n {
		r2, w2 := karakter(s, i+1)
		switch sinif(r2) {
		case sinifHarf:
			return harfSonu(s, i+1)
		case sinifSayi:
			return i + 1 + w2
		case 0:
			return digerSonu(s, i+1)
		}
		// Ardından boşluk geliyorsa boşluk kurallarına düşülür.
	}
	switch sinif(r) {
	case sinifHarf:
		return harfSonu(s, i)
	case sinifSayi:
		return i + w
	case 0:
		return digerSonu(s, i)
	}
	// 5-6) Boşluk dizisi: "\s+(?!\S)" önce en uzun diziyi alır; dizi metnin
	// sonuna kadar sürüyorsa tamamı eşleşir. Değilse (ardından boşluk olmayan
	// bir karakter geliyorsa) son boşluk karakteri geri bırakılır; böylece o
	// karakter sonraki sözcüğe (ör. " kelime") katılabilir. Dizi tek
	// karakterse ileriye bakma tutmaz ve "\s+" tek karakteri alır.
	j := i + w
	son := i // dizideki son karakterin başlangıcı
	for j < n {
		r2, w2 := karakter(s, j)
		if sinif(r2) != sinifBosluk {
			break
		}
		son = j
		j += w2
	}
	if j == n || son == i {
		return j
	}
	return son
}

// onBol metni desene göre parçalara böler ve her parça için f'yi çağırır.
func onBol(s string, f func(parca string)) {
	for i := 0; i < len(s); {
		j := eslesmeSonu(s, i)
		f(s[i:j])
		i = j
	}
}

// DesenleBol metni BolmeDeseni'ne göre parçalara ayırır (sınama ve inceleme için).
func DesenleBol(metin string) []string {
	metin = gecerliMetin(metin)
	var out []string
	onBol(metin, func(p string) { out = append(out, p) })
	return out
}
