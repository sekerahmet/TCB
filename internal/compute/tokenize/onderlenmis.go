package tokenize

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"unicode"
	"unicode/utf8"
)

// ============================================================================
// Precompiled normalleştirici (önceden derlenmiş karakter eşlemesi)
// ============================================================================
//
// Cümle parçası tabanlı modellerden dönüştürülen
// parçalayıcılarda normalleştirme kuralları ikili bir "charsmap" olarak
// saklanır: bir çift dizili (double-array) önek ağacı ve sıfırla ayrılmış
// normalleştirilmiş metinler. Metin grafem kümelerine ayrılır; 6 bayttan kısa
// bir küme için ağaçta önek araması tutarsa kümenin TAMAMI bulunan metinle
// değiştirilir, aksi hâlde küme karakter karakter dönüştürülür.
//
// Grafem kümeleri Unicode UAX #29'un bu kullanım için etkili olan kurallarıyla
// belirlenir (CR LF, denetim karakterleri, Extend/SpacingMark/ZWJ eklenmesi,
// Prepend). 6 bayt ve daha uzun kümeler zaten karakter karakter işlendiğinden
// Hangıl, bölgesel gösterge çiftleri ve emoji ZWJ dizileri kuralları sonucu
// etkilemez.

func init() { NormallestiriciKaydet("Precompiled", precompiledKur) }

type precompiled struct {
	dizi  []uint32
	metin []byte
}

func precompiledKur(ham json.RawMessage) (Normallestirici, error) {
	var t struct {
		Charsmap *string `json:"precompiled_charsmap"`
	}
	if err := katiCoz(ham, &t); err != nil {
		return nil, err
	}
	if t.Charsmap == nil {
		return nil, fmt.Errorf("precompiled_charsmap yok")
	}
	veri, err := base64.StdEncoding.DecodeString(*t.Charsmap)
	if err != nil {
		return nil, fmt.Errorf("precompiled_charsmap base64 çözülemedi: %w", err)
	}
	if len(veri) < 4 {
		return nil, fmt.Errorf("precompiled_charsmap çok kısa")
	}
	boy := int(binary.LittleEndian.Uint32(veri))
	veri = veri[4:]
	if boy%4 != 0 || boy > len(veri) {
		return nil, fmt.Errorf("precompiled_charsmap ağaç boyutu geçersiz")
	}
	p := &precompiled{dizi: make([]uint32, boy/4)}
	for i := range p.dizi {
		p.dizi[i] = binary.LittleEndian.Uint32(veri[4*i:])
	}
	p.metin = veri[boy:]
	if len(p.dizi) == 0 {
		return nil, fmt.Errorf("precompiled_charsmap ağacı boş")
	}
	return p, nil
}

func daYaprakVar(u uint32) bool  { return (u>>8)&1 == 1 }
func daDeger(u uint32) uint32    { return u & (1<<31 - 1) }
func daEtiket(u uint32) uint32   { return u & (1<<31 | 0xFF) }
func daKaydirma(u uint32) uint32 { return (u >> 10) << ((u & (1 << 9)) >> 6) }

// ilkOnek anahtarın ağaçtaki İLK (en kısa) önek eşleşmesinin değerini verir.
func (p *precompiled) ilkOnek(anahtar string) (uint32, bool) {
	konum := uint32(0)
	birim := p.dizi[0]
	konum ^= daKaydirma(birim)
	for i := 0; i < len(anahtar); i++ {
		c := anahtar[i]
		if c == 0 {
			break
		}
		konum ^= uint32(c)
		if int(konum) >= len(p.dizi) {
			return 0, false
		}
		birim = p.dizi[konum]
		if daEtiket(birim) != uint32(c) {
			return 0, false
		}
		konum ^= daKaydirma(birim)
		if int(konum) >= len(p.dizi) {
			return 0, false
		}
		if daYaprakVar(birim) {
			return daDeger(p.dizi[konum]), true
		}
	}
	return 0, false
}

func (p *precompiled) donustur(parca string) (string, bool) {
	d, ok := p.ilkOnek(parca)
	if !ok || int(d) > len(p.metin) {
		return "", false
	}
	son := int(d)
	for son < len(p.metin) && p.metin[son] != 0 {
		son++
	}
	return string(p.metin[d:son]), true
}

func (p *precompiled) Normallestir(m string) string {
	out := make([]byte, 0, len(m))
	degisti := false
	for i := 0; i < len(m); {
		j := grafemSonu(m, i)
		kume := m[i:j]
		if len(kume) < 6 {
			if n, ok := p.donustur(kume); ok {
				out = append(out, n...)
				degisti = true
				i = j
				continue
			}
		}
		for k := 0; k < len(kume); {
			_, w := utf8.DecodeRuneInString(kume[k:])
			if n, ok := p.donustur(kume[k : k+w]); ok {
				out = append(out, n...)
				degisti = true
			} else {
				out = append(out, kume[k:k+w]...)
			}
			k += w
		}
		i = j
	}
	if !degisti {
		return m
	}
	return string(out)
}

// ---------------------------------------------------------------------------
// Grafem kümeleri (UAX #29, kısaltılmış)
// ---------------------------------------------------------------------------

func grafemDenetim(r rune) bool {
	if r == '\r' || r == '\n' {
		return true
	}
	if r == 0x200C || r == 0x200D {
		return false
	}
	if grafemOnEk(r) {
		return false
	}
	return unicode.In(r, unicode.Cc, unicode.Zl, unicode.Zp, unicode.Cf)
}

func grafemUzanti(r rune) bool {
	if r == 0x200D || r == 0x200C {
		return true
	}
	if r >= 0x1F3FB && r <= 0x1F3FF {
		return true // emoji ten rengi değiştiricileri
	}
	if r >= 0xE0020 && r <= 0xE007F {
		return true // etiket karakterleri
	}
	return unicode.In(r, unicode.Mn, unicode.Me, unicode.Other_Grapheme_Extend)
}

// grafemAralikIsareti GCB=SpacingMark (Mc'nin çoğu ve birkaç ek karakter).
func grafemAralikIsareti(r rune) bool {
	switch {
	case r == 0x0E33 || r == 0x0EB3:
		return true
	case r == 0x102B || r == 0x102C || r == 0x1038 || (r >= 0x1062 && r <= 0x1064) || (r >= 0x1067 && r <= 0x106D) ||
		r == 0x1083 || (r >= 0x1087 && r <= 0x108C) || r == 0x108F || (r >= 0x109A && r <= 0x109C) ||
		r == 0x1A61 || r == 0x1A63 || r == 0x1A64 || r == 0xAA7B || r == 0xAA7D || r == 0x11720 || r == 0x11721:
		return false
	}
	return unicode.Is(unicode.Mc, r) && !unicode.Is(unicode.Other_Grapheme_Extend, r)
}

func grafemOnEk(r rune) bool {
	return (r >= 0x0600 && r <= 0x0605) || r == 0x06DD || r == 0x070F || r == 0x0890 || r == 0x0891 ||
		r == 0x08E2 || r == 0x0D4E || r == 0x110BD || r == 0x110CD || r == 0x111C2 || r == 0x111C3 ||
		r == 0x1193F || r == 0x11941 || r == 0x11A3A || (r >= 0x11A84 && r <= 0x11A89) || r == 0x11D46 || r == 0x11F02
}

// grafemSonu i'de başlayan grafem kümesinin bitişini verir.
func grafemSonu(s string, i int) int {
	r, w := utf8.DecodeRuneInString(s[i:])
	j := i + w
	if r == '\r' {
		if j < len(s) && s[j] == '\n' {
			return j + 1
		}
		return j
	}
	if grafemDenetim(r) {
		return j
	}
	// Prepend: ardından gelen (denetim olmayan) karakterle birleşir.
	for grafemOnEk(r) && j < len(s) {
		r2, w2 := utf8.DecodeRuneInString(s[j:])
		if grafemDenetim(r2) {
			return j
		}
		r = r2
		j += w2
	}
	for j < len(s) {
		r2, w2 := utf8.DecodeRuneInString(s[j:])
		if !grafemUzanti(r2) && !grafemAralikIsareti(r2) {
			break
		}
		j += w2
	}
	return j
}
