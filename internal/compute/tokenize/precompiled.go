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

func init() { RegisterNormalizer("Precompiled", newPrecompiled) }

type precompiled struct {
	arr  []uint32
	text []byte
}

func newPrecompiled(raw json.RawMessage) (Normalizer, error) {
	var t struct {
		Charsmap *string `json:"precompiled_charsmap"`
	}
	if err := strictDecode(raw, &t); err != nil {
		return nil, err
	}
	if t.Charsmap == nil {
		return nil, fmt.Errorf("precompiled_charsmap yok")
	}
	data, err := base64.StdEncoding.DecodeString(*t.Charsmap)
	if err != nil {
		return nil, fmt.Errorf("precompiled_charsmap base64 çözülemedi: %w", err)
	}
	if len(data) < 4 {
		return nil, fmt.Errorf("precompiled_charsmap çok kısa")
	}
	size := int(binary.LittleEndian.Uint32(data))
	data = data[4:]
	if size%4 != 0 || size > len(data) {
		return nil, fmt.Errorf("precompiled_charsmap ağaç boyutu geçersiz")
	}
	p := &precompiled{arr: make([]uint32, size/4)}
	for i := range p.arr {
		p.arr[i] = binary.LittleEndian.Uint32(data[4*i:])
	}
	p.text = data[size:]
	if len(p.arr) == 0 {
		return nil, fmt.Errorf("precompiled_charsmap ağacı boş")
	}
	return p, nil
}

func daHasLeaf(u uint32) bool { return (u>>8)&1 == 1 }
func daValue(u uint32) uint32 { return u & (1<<31 - 1) }
func daLabel(u uint32) uint32 { return u & (1<<31 | 0xFF) }
func daShift(u uint32) uint32 { return (u >> 10) << ((u & (1 << 9)) >> 6) }

// firstPrefix anahtarın ağaçtaki İLK (en kısa) önek eşleşmesinin değerini verir.
func (p *precompiled) firstPrefix(key string) (uint32, bool) {
	pos := uint32(0)
	unit := p.arr[0]
	pos ^= daShift(unit)
	for i := 0; i < len(key); i++ {
		c := key[i]
		if c == 0 {
			break
		}
		pos ^= uint32(c)
		if int(pos) >= len(p.arr) {
			return 0, false
		}
		unit = p.arr[pos]
		if daLabel(unit) != uint32(c) {
			return 0, false
		}
		pos ^= daShift(unit)
		if int(pos) >= len(p.arr) {
			return 0, false
		}
		if daHasLeaf(unit) {
			return daValue(p.arr[pos]), true
		}
	}
	return 0, false
}

func (p *precompiled) transform(piece string) (string, bool) {
	d, ok := p.firstPrefix(piece)
	if !ok || int(d) > len(p.text) {
		return "", false
	}
	last := int(d)
	for last < len(p.text) && p.text[last] != 0 {
		last++
	}
	return string(p.text[d:last]), true
}

func (p *precompiled) Normalize(m string) string {
	out := make([]byte, 0, len(m))
	changed := false
	for i := 0; i < len(m); {
		j := graphemeEnd(m, i)
		set := m[i:j]
		if len(set) < 6 {
			if n, ok := p.transform(set); ok {
				out = append(out, n...)
				changed = true
				i = j
				continue
			}
		}
		for k := 0; k < len(set); {
			_, w := utf8.DecodeRuneInString(set[k:])
			if n, ok := p.transform(set[k : k+w]); ok {
				out = append(out, n...)
				changed = true
			} else {
				out = append(out, set[k:k+w]...)
			}
			k += w
		}
		i = j
	}
	if !changed {
		return m
	}
	return string(out)
}

// ---------------------------------------------------------------------------
// Grafem kümeleri (UAX #29, kısaltılmış)
// ---------------------------------------------------------------------------

func graphemeControl(r rune) bool {
	if r == '\r' || r == '\n' {
		return true
	}
	if r == 0x200C || r == 0x200D {
		return false
	}
	if graphemePrepend(r) {
		return false
	}
	return unicode.In(r, unicode.Cc, unicode.Zl, unicode.Zp, unicode.Cf)
}

func graphemeExtend(r rune) bool {
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

// graphemeSpacingMark GCB=SpacingMark (Mc'nin çoğu ve birkaç ek karakter).
func graphemeSpacingMark(r rune) bool {
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

func graphemePrepend(r rune) bool {
	return (r >= 0x0600 && r <= 0x0605) || r == 0x06DD || r == 0x070F || r == 0x0890 || r == 0x0891 ||
		r == 0x08E2 || r == 0x0D4E || r == 0x110BD || r == 0x110CD || r == 0x111C2 || r == 0x111C3 ||
		r == 0x1193F || r == 0x11941 || r == 0x11A3A || (r >= 0x11A84 && r <= 0x11A89) || r == 0x11D46 || r == 0x11F02
}

// graphemeEnd i'de başlayan grafem kümesinin bitişini verir.
func graphemeEnd(s string, i int) int {
	r, w := utf8.DecodeRuneInString(s[i:])
	j := i + w
	if r == '\r' {
		if j < len(s) && s[j] == '\n' {
			return j + 1
		}
		return j
	}
	if graphemeControl(r) {
		return j
	}
	// Prepend: ardından gelen (denetim olmayan) karakterle birleşir.
	for graphemePrepend(r) && j < len(s) {
		r2, w2 := utf8.DecodeRuneInString(s[j:])
		if graphemeControl(r2) {
			return j
		}
		r = r2
		j += w2
	}
	for j < len(s) {
		r2, w2 := utf8.DecodeRuneInString(s[j:])
		if !graphemeExtend(r2) && !graphemeSpacingMark(r2) {
			break
		}
		j += w2
	}
	return j
}
