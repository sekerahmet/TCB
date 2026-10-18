package parca

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// ============================================================================
// Normalleştiriciler
// ============================================================================

func init() {
	NormallestiriciKaydet("Sequence", sirNormKur)
	for _, t := range []struct {
		ad          string
		uyum, birle bool
	}{{"NFC", false, true}, {"NFD", false, false}, {"NFKC", true, true}, {"NFKD", true, false}} {
		n := unicodeNorm{t.uyum, t.birle}
		NormallestiriciKaydet(t.ad, func(json.RawMessage) (Normallestirici, error) { return n, nil })
	}
	NormallestiriciKaydet("Lowercase", func(json.RawMessage) (Normallestirici, error) { return kucukHarf{}, nil })
	NormallestiriciKaydet("StripAccents", func(json.RawMessage) (Normallestirici, error) { return aksanSil{}, nil })
	NormallestiriciKaydet("Nmt", func(json.RawMessage) (Normallestirici, error) { return nmtNorm{}, nil })
	NormallestiriciKaydet("ByteLevel", func(json.RawMessage) (Normallestirici, error) { return baytNorm{}, nil })
	NormallestiriciKaydet("Strip", kirpNormKur)
	NormallestiriciKaydet("Replace", degistirNormKur)
	NormallestiriciKaydet("Prepend", onekNormKur)
	NormallestiriciKaydet("BertNormalizer", bertNormKur)
}

// NormallestiriciSirasi normalleştiricileri sırayla uygular.
type NormallestiriciSirasi []Normallestirici

func (s NormallestiriciSirasi) Normallestir(m string) string { return s.normallestirB(m, nil) }

func (s NormallestiriciSirasi) normallestirB(m string, b *dfButce) string {
	for _, n := range s {
		m = normallestirButceli(n, m, b)
	}
	return m
}

// butceliNormallestirici düzenli ifade kullanan normalleştiricilerdir: bir
// kodlama çağrısının paylaşılan adım bütçesiyle çalışabilirler.
type butceliNormallestirici interface {
	normallestirB(m string, b *dfButce) string
}

// normallestirButceli n'yi, destekliyorsa b bütçesiyle uygular.
func normallestirButceli(n Normallestirici, m string, b *dfButce) string {
	if bn, ok := n.(butceliNormallestirici); ok && b != nil {
		return bn.normallestirB(m, b)
	}
	return n.Normallestir(m)
}

func sirNormKur(ham json.RawMessage) (Normallestirici, error) {
	var t struct {
		Normalizers []json.RawMessage `json:"normalizers"`
	}
	if err := katiCoz(ham, &t); err != nil {
		return nil, err
	}
	var s NormallestiriciSirasi
	for i, h := range t.Normalizers {
		n, err := NormallestiriciKur(h)
		if err != nil {
			return nil, fmt.Errorf("%d. adım: %w", i, err)
		}
		if n != nil {
			s = append(s, n)
		}
	}
	return s, nil
}

type unicodeNorm struct{ uyum, birlestir bool }

func (n unicodeNorm) Normallestir(m string) string {
	return unicodeNormallestir(m, n.uyum, n.birlestir)
}

// kucukHarf her karakteri tek tek küçük harfe çevirir (bağlama bakmadan).
// 'İ' (U+0130) Unicode'un koşulsuz özel durumuna göre "i̇" (i + U+0307) olur.
type kucukHarf struct{}

func (kucukHarf) Normallestir(m string) string {
	var sb strings.Builder
	sb.Grow(len(m))
	for _, r := range m {
		if r == 'İ' {
			sb.WriteString("i̇")
			continue
		}
		if s, ok := kucukHarfIstisnalari[r]; ok {
			sb.WriteString(s)
			continue
		}
		sb.WriteRune(unicode.ToLower(r))
	}
	return sb.String()
}

// aksanSil birleşen işaretleri (M) siler; genellikle NFD'den sonra kullanılır.
type aksanSil struct{}

func (aksanSil) Normallestir(m string) string {
	return strings.Map(func(r rune) rune {
		if tabloAksan.icinde(r) {
			return -1
		}
		return r
	}, m)
}

type nmtNorm struct{}

func (nmtNorm) Normallestir(m string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 1 && r <= 8, r == 0x0B, r >= 0x0E && r <= 0x1F, r == 0x7F, r == 0x8F, r == 0x9F:
			return -1
		case r == 0x09, r == 0x0A, r == 0x0C, r == 0x0D, r == 0x1680, r >= 0x200B && r <= 0x200F,
			r == 0x2028, r == 0x2029, r == 0x2581, r == 0xFEFF, r == 0xFFFD:
			return ' '
		}
		return r
	}, m)
}

type baytNorm struct{}

func (baytNorm) Normallestir(m string) string { return baytlariKarakterle([]byte(m)) }

type kirpNorm struct{ sol, sag bool }

func (k kirpNorm) Normallestir(m string) string {
	if k.sol {
		m = strings.TrimLeftFunc(m, unicode.IsSpace)
	}
	if k.sag {
		m = strings.TrimRightFunc(m, unicode.IsSpace)
	}
	return m
}

func kirpNormKur(ham json.RawMessage) (Normallestirici, error) {
	var t struct {
		Sol bool `json:"strip_left"`
		Sag bool `json:"strip_right"`
	}
	if err := katiCoz(ham, &t); err != nil {
		return nil, err
	}
	return kirpNorm{t.Sol, t.Sag}, nil
}

// degistirici bir deseni (düz metin ya da düzenli ifade) içerikle değiştirir.
type degistirici struct {
	duz    string
	ifade  *DuzIfade
	icerik string
}

func degistiriciKur(ham json.RawMessage) (*degistirici, error) {
	var t struct {
		Pattern desenBicimi `json:"pattern"`
		Content string      `json:"content"`
	}
	if err := katiCoz(ham, &t); err != nil {
		return nil, err
	}
	d := &degistirici{icerik: t.Content}
	switch {
	case t.Pattern.String != nil:
		d.duz = *t.Pattern.String
	case t.Pattern.Regex != nil:
		ifade, err := DuzIfadeDerle(*t.Pattern.Regex)
		if err != nil {
			return nil, err
		}
		d.ifade = ifade
	default:
		return nil, fmt.Errorf("desen (pattern) String ya da Regex olmalı")
	}
	return d, nil
}

// uygula değiştirmeyi yapar; b nil değilse düzenli ifade adımları o
// bütçeden düşülür.
func (d *degistirici) uygula(m string, b *dfButce) string {
	if d.ifade == nil {
		if d.duz == "" {
			return m
		}
		return strings.ReplaceAll(m, d.duz, d.icerik)
	}
	var sb strings.Builder
	onceki := 0
	d.ifade.tumEslesmeler(m, func(bas, son int) {
		sb.WriteString(m[onceki:bas])
		sb.WriteString(d.icerik)
		onceki = son
	}, b)
	sb.WriteString(m[onceki:])
	return sb.String()
}

type degistirNorm struct{ d *degistirici }

func (n degistirNorm) Normallestir(m string) string { return n.d.uygula(m, nil) }

func (n degistirNorm) normallestirB(m string, b *dfButce) string { return n.d.uygula(m, b) }

func degistirNormKur(ham json.RawMessage) (Normallestirici, error) {
	d, err := degistiriciKur(ham)
	if err != nil {
		return nil, err
	}
	return degistirNorm{d}, nil
}

type onekNorm struct{ onek string }

func (n onekNorm) Normallestir(m string) string {
	if m == "" {
		return m
	}
	return n.onek + m
}

func onekNormKur(ham json.RawMessage) (Normallestirici, error) {
	var t struct {
		Prepend string `json:"prepend"`
	}
	if err := katiCoz(ham, &t); err != nil {
		return nil, err
	}
	return onekNorm{t.Prepend}, nil
}

// bertNorm: denetim karakterlerini siler, boşlukları ' ' yapar, Çince
// karakterlerin çevresine boşluk koyar, isteğe bağlı aksan siler ve küçültür.
type bertNorm struct {
	temizle, cince, aksan, kucult bool
}

func bertNormKur(ham json.RawMessage) (Normallestirici, error) {
	var t struct {
		CleanText          *bool `json:"clean_text"`
		HandleChineseChars *bool `json:"handle_chinese_chars"`
		StripAccents       *bool `json:"strip_accents"`
		Lowercase          *bool `json:"lowercase"`
	}
	if err := katiCoz(ham, &t); err != nil {
		return nil, err
	}
	deger := func(b *bool, v bool) bool {
		if b == nil {
			return v
		}
		return *b
	}
	n := bertNorm{temizle: deger(t.CleanText, true), cince: deger(t.HandleChineseChars, true), kucult: deger(t.Lowercase, true)}
	n.aksan = deger(t.StripAccents, n.kucult)
	return n, nil
}

func cinceMi(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) || (r >= 0x20000 && r <= 0x2A6DF) ||
		(r >= 0x2A700 && r <= 0x2B73F) || (r >= 0x2B740 && r <= 0x2B81F) || (r >= 0x2B920 && r <= 0x2CEAF) ||
		(r >= 0xF900 && r <= 0xFAFF) || (r >= 0x2F800 && r <= 0x2FA1F)
}

func (n bertNorm) Normallestir(m string) string {
	if n.temizle {
		m = strings.Map(func(r rune) rune {
			if tabloBertKontrol.icinde(r) {
				return -1
			}
			if r == '\t' || r == '\n' || r == '\r' || unicode.IsSpace(r) {
				return ' '
			}
			return r
		}, m)
	}
	if n.cince {
		var sb strings.Builder
		for _, r := range m {
			if cinceMi(r) {
				sb.WriteByte(' ')
				sb.WriteRune(r)
				sb.WriteByte(' ')
			} else {
				sb.WriteRune(r)
			}
		}
		m = sb.String()
	}
	if n.aksan {
		m = strings.Map(func(r rune) rune {
			if tabloBertAksan.icinde(r) {
				return -1
			}
			return r
		}, unicodeNormallestir(m, false, false))
	}
	if n.kucult {
		m = kucukHarf{}.Normallestir(m)
	}
	return m
}
