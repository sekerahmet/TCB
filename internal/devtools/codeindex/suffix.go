package codeindex

import "unicode"

// Bu dosya Türkçe ek ayrımıdır: "baş" aranırken "başı", "başları", "başında" aynı sözcüğün ekli biçimleri
// sayılır; "başka", "başla", "başlık", "başarı" ayrı sözcüklerdir. Kapalı bir ek dizisi üretilir
// (çoğul, iyelik, hâl; ünlü uyumu ve kaynaştırma ünsüzleriyle); sözcüğün sonuna bu dizilerden
// biri gelmiş olan ekli sayılır, gerisi sayılmaz. Kapsam sınırı: ünsüz yumuşaması ve düşmesi
// (kitap → kitabı, ağız → ağzı) ve türetme ekleri (-lık, -sız, -la) yoktur; ek, aranan sözcüğün
// kendi biçimine eklenir.

// harmony bir ünlünün uyum sınıfıdır: iki yönlü ek ünlüsü (a/e) ve dört yönlü ek ünlüsü (ı/i/u/ü).
type harmony struct{ two, four rune }

func harmonyOf(v rune) (harmony, bool) {
	switch v {
	case 'a', 'ı':
		return harmony{'a', 'ı'}, true
	case 'o', 'u':
		return harmony{'a', 'u'}, true
	case 'e', 'i':
		return harmony{'e', 'i'}, true
	case 'ö', 'ü':
		return harmony{'e', 'ü'}, true
	}
	return harmony{}, false
}

func isVowel(r rune) bool {
	_, ok := harmonyOf(r)
	return ok
}

// suffixForm ek dizisinin bir aşamasıdır: o ana kadarki ek metni, ünlüyle bitip bitmediği (kaynaştırma
// ünsüzü gerekir), uyum sınıfı ve 3. kişi iyeliği alıp almadığı (hâl eklerinde "n" kaynaştırması).
type suffixForm struct {
	text  string
	endsV bool
	h     harmony
	poss3 bool
}

// suffixTails katlanmış (Türkçe küçük harf) bir sözcüğe gelebilen boş olmayan ek dizilerinin kümesidir;
// sözcükte ünlü yoksa nil.
func suffixTails(stem string) map[string]bool {
	runes := []rune(stem)
	var h harmony
	found := false
	for i := len(runes) - 1; i >= 0 && !found; i-- {
		h, found = harmonyOf(runes[i])
	}
	if !found {
		return nil
	}
	start := suffixForm{endsV: isVowel(runes[len(runes)-1]), h: h}
	forms := []suffixForm{start, pluralOf(start)}
	var withPoss []suffixForm
	for _, f := range forms {
		withPoss = append(withPoss, f)
		withPoss = append(withPoss, possessives(f)...)
	}
	tails := map[string]bool{}
	for _, f := range withPoss {
		if f.text != "" {
			tails[f.text] = true
		}
		for _, c := range cases(f) {
			tails[f.text+c] = true
		}
		for _, c := range copulas(f) {
			tails[f.text+c] = true // başıdır, baştır, başlardır
		}
	}
	return tails
}

func pluralOf(f suffixForm) suffixForm {
	h, _ := harmonyOf(f.h.two)
	return suffixForm{text: f.text + "l" + string(f.h.two) + "r", h: h}
}

// possessives iyelik eklerinin her biriyle f'den çıkan aşamalardır (ben, sen, o, biz, siz).
func possessives(f suffixForm) []suffixForm {
	four := string(f.h.four)
	after, _ := harmonyOf(f.h.four)
	join := func(vowelBuffer, consonantBuffer string) string {
		if f.endsV {
			return vowelBuffer
		}
		return consonantBuffer
	}
	return []suffixForm{
		{text: f.text + join("m", four+"m"), h: f.h},
		{text: f.text + join("n", four+"n"), h: f.h},
		{text: f.text + join("s"+four, four), endsV: true, h: after, poss3: true},
		{text: f.text + join("m"+four+"z", four+"m"+four+"z"), h: after},
		{text: f.text + join("n"+four+"z", four+"n"+four+"z"), h: after},
	}
}

// cases f'ye gelebilen hâl eklerinin metinleridir (yalın hâl için boş ek ayrıca vardır).
func cases(f suffixForm) []string {
	two, four := string(f.h.two), string(f.h.four)
	buffer := ""
	switch {
	case f.poss3:
		buffer = "n"
	case f.endsV:
		buffer = "y"
	}
	pronominal := ""
	if f.poss3 || f.endsV {
		pronominal = "n"
	}
	loc := ""
	if f.poss3 {
		loc = "n"
	}
	return []string{
		buffer + four,           // -ı hâli
		buffer + two,            // -a hâli
		loc + "d" + two,         // -da
		loc + "t" + two,         // -ta (sert ünsüzden sonra)
		loc + "d" + two + "n",   // -dan
		loc + "t" + two + "n",   // -tan
		pronominal + four + "n", // -ın (tamlayan)
	}
}

// foldedRunes bir sözcüğün Türkçe küçük harfli kod noktalarıdır.
func foldedRunes(s string) []rune { return []rune(fold(s)) }

// wordRune bir tanımlayıcı ya da sözcük karakteri mi (harf, rakam, alt çizgi).
func wordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

// derivedTails sözcükten türeyen sıfat ekleridir (-lı/-li/-lu/-lü, -sız/-siz/-suz/-süz): aynı
// kavramı taşıdığı için ("çok başlı") olası listede ayrı etiketlenir; -lık, -la gibi anlamı
// değişenler değildir.
func derivedTails(stem string) map[string]bool {
	h, ok := lastHarmony(stem)
	if !ok {
		return nil
	}
	four := string(h.four)
	return map[string]bool{"l" + four: true, "s" + four + "z": true}
}

// copulas ek fiilin bildirme ekidir (-dır/-dir/-dur/-dür, sert ünsüzden sonra -tır ...).
func copulas(f suffixForm) []string {
	four := string(f.h.four)
	return []string{"d" + four + "r", "t" + four + "r"}
}

// soften çok heceli bir sözcüğün sondaki sert ünsüzünün ünlüyle başlayan ekten önceki yumuşak biçimini
// verir (çekirdek -> çekirdeğ, kitap -> kitab, ağaç -> ağac, kanat -> kanad; -nk -> -ng); tek heceli
// sözcükler (ok -> oku) ve yumuşamayan ünsüzler için ok=false. Yumuşamayan çok heceli istisnalar
// (ahenk, hukuk...) kapsam dışıdır: yalnızca fazladan bir aday üretir.
func soften(word string) (string, bool) {
	runes := []rune(word)
	vowels := 0
	for _, r := range runes {
		if isVowel(r) {
			vowels++
		}
	}
	if vowels < 2 || len(runes) < 2 {
		return "", false
	}
	last := runes[len(runes)-1]
	switch last {
	case 'k':
		if runes[len(runes)-2] == 'n' {
			return string(runes[:len(runes)-1]) + "g", true
		}
		return string(runes[:len(runes)-1]) + string('ğ'), true
	case 'p':
		return string(runes[:len(runes)-1]) + "b", true
	case 'ç':
		return string(runes[:len(runes)-1]) + "c", true
	case 't':
		return string(runes[:len(runes)-1]) + "d", true
	}
	return "", false
}

// vowelStart s bir ünlüyle mi başlıyor.
func vowelStart(s string) bool {
	for _, r := range s {
		return isVowel(r)
	}
	return false
}
