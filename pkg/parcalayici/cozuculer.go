package parcalayici

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ============================================================================
// Çözücüler (decoder)
// ============================================================================

func init() {
	CozucuKaydet("ByteLevel", func(json.RawMessage) (Cozucu, error) { return baytCozucu{}, nil })
	CozucuKaydet("Metaspace", func(ham json.RawMessage) (Cozucu, error) {
		m, err := metaspaceOku(ham)
		if err != nil {
			return nil, err
		}
		return metaspaceCozucu{yerine: []rune(m.yerine)[0], kural: m.kural}, nil
	})
	CozucuKaydet("WordPiece", func(ham json.RawMessage) (Cozucu, error) {
		t := struct {
			Prefix  *string `json:"prefix"`
			Cleanup *bool   `json:"cleanup"`
		}{}
		if err := katiCoz(ham, &t); err != nil {
			return nil, err
		}
		c := wordPieceCozucu{onek: "##", temizle: true}
		if t.Prefix != nil {
			c.onek = *t.Prefix
		}
		if t.Cleanup != nil {
			c.temizle = *t.Cleanup
		}
		return c, nil
	})
	CozucuKaydet("BPEDecoder", func(ham json.RawMessage) (Cozucu, error) {
		t := struct {
			Suffix *string `json:"suffix"`
		}{}
		if err := katiCoz(ham, &t); err != nil {
			return nil, err
		}
		c := sonekCozucu{"</w>"}
		if t.Suffix != nil {
			c.sonek = *t.Suffix
		}
		return c, nil
	})
	CozucuKaydet("ByteFallback", func(json.RawMessage) (Cozucu, error) { return baytYedegiCozucu{}, nil })
	CozucuKaydet("Fuse", func(json.RawMessage) (Cozucu, error) { return kaynastir{}, nil })
	CozucuKaydet("Strip", func(ham json.RawMessage) (Cozucu, error) {
		var t struct {
			Content string `json:"content"`
			Start   int    `json:"start"`
			Stop    int    `json:"stop"`
		}
		if err := katiCoz(ham, &t); err != nil {
			return nil, err
		}
		if utf8.RuneCountInString(t.Content) != 1 {
			return nil, fmt.Errorf("content tek karakter olmalı: %q", t.Content)
		}
		return kirpCozucu{[]rune(t.Content)[0], t.Start, t.Stop}, nil
	})
	CozucuKaydet("Replace", func(ham json.RawMessage) (Cozucu, error) {
		d, err := degistiriciKur(ham)
		if err != nil {
			return nil, err
		}
		return degistirCozucu{d}, nil
	})
	CozucuKaydet("Sequence", func(ham json.RawMessage) (Cozucu, error) {
		var t struct {
			Decoders []json.RawMessage `json:"decoders"`
		}
		if err := katiCoz(ham, &t); err != nil {
			return nil, err
		}
		var s CozucuSirasi
		for i, h := range t.Decoders {
			c, err := CozucuKur(h)
			if err != nil {
				return nil, fmt.Errorf("%d. adım: %w", i, err)
			}
			if c != nil {
				s = append(s, c)
			}
		}
		return s, nil
	})
}

// CozucuSirasi çözücüleri sırayla uygular.
type CozucuSirasi []Cozucu

func (s CozucuSirasi) CozZincir(t []string) []string {
	for _, c := range s {
		t = c.CozZincir(t)
	}
	return t
}

// baytCozucu tokenları bayt düzeyi karakterlerden baytlara çevirir ve tümünü
// tek bir (kayıplı UTF-8 ile onarılmış) metne birleştirir.
type baytCozucu struct{}

func (baytCozucu) CozZincir(t []string) []string {
	var buf []byte
	for _, s := range t {
		buf = append(buf, tokenBaytlari(s)...)
	}
	return []string{kayipliUTF8(buf)}
}

type metaspaceCozucu struct {
	yerine rune
	kural  onekKurali
}

func (m metaspaceCozucu) CozZincir(t []string) []string {
	out := make([]string, len(t))
	for i, s := range t {
		var sb strings.Builder
		for _, r := range s {
			if r == m.yerine {
				if i == 0 && m.kural != onekYok {
					continue
				}
				sb.WriteByte(' ')
				continue
			}
			sb.WriteRune(r)
		}
		out[i] = sb.String()
	}
	return out
}

type wordPieceCozucu struct {
	onek    string
	temizle bool
}

func wordPieceTemizle(s string) string {
	for _, c := range [][2]string{{" .", "."}, {" ?", "?"}, {" !", "!"}, {" ,", ","}, {" ' ", "'"},
		{" n't", "n't"}, {" 'm", "'m"}, {" do not", " don't"}, {" 's", "'s"}, {" 've", "'ve"}, {" 're", "'re"}} {
		s = strings.ReplaceAll(s, c[0], c[1])
	}
	return s
}

func (w wordPieceCozucu) CozZincir(t []string) []string {
	out := make([]string, len(t))
	for i, s := range t {
		if i != 0 {
			if strings.HasPrefix(s, w.onek) {
				s = strings.Replace(s, w.onek, "", 1)
			} else {
				s = " " + s
			}
		}
		if w.temizle {
			s = wordPieceTemizle(s)
		}
		out[i] = s
	}
	return out
}

type sonekCozucu struct{ sonek string }

func (c sonekCozucu) CozZincir(t []string) []string {
	out := make([]string, len(t))
	for i, s := range t {
		yeni := " "
		if i == len(t)-1 {
			yeni = ""
		}
		out[i] = strings.ReplaceAll(s, c.sonek, yeni)
	}
	return out
}

// baytYedegiCozucu "<0xXX>" tokenlarını baytlara çevirir; ardışık baytlar
// geçerli UTF-8 değilse her bayt için bir U+FFFD üretilir.
type baytYedegiCozucu struct{}

func yedekBayti(s string) (byte, bool) {
	if len(s) != 6 || !strings.HasPrefix(s, "<0x") || s[5] != '>' {
		return 0, false
	}
	n, err := strconv.ParseUint(s[3:5], 16, 8)
	if err != nil {
		return 0, false
	}
	return byte(n), true
}

func (baytYedegiCozucu) CozZincir(t []string) []string {
	var out []string
	var bekleyen []byte
	bosalt := func() {
		if len(bekleyen) == 0 {
			return
		}
		if utf8.Valid(bekleyen) {
			out = append(out, string(bekleyen))
		} else {
			for range bekleyen {
				out = append(out, "�")
			}
		}
		bekleyen = bekleyen[:0]
	}
	for _, s := range t {
		if b, ok := yedekBayti(s); ok {
			bekleyen = append(bekleyen, b)
			continue
		}
		bosalt()
		out = append(out, s)
	}
	bosalt()
	return out
}

type kaynastir struct{}

func (kaynastir) CozZincir(t []string) []string { return []string{strings.Join(t, "")} }

type kirpCozucu struct {
	icerik     rune
	bas, bitis int
}

func (k kirpCozucu) CozZincir(t []string) []string {
	out := make([]string, len(t))
	for i, s := range t {
		r := []rune(s)
		bas := 0
		for j := 0; j < k.bas && j < len(r); j++ {
			if r[j] != k.icerik {
				break
			}
			bas = j + 1
		}
		son := len(r)
		for j := 0; j < k.bitis; j++ {
			x := len(r) - j - 1
			if x < 0 || r[x] != k.icerik {
				break
			}
			son = x
		}
		if son < bas {
			son = bas
		}
		out[i] = string(r[bas:son])
	}
	return out
}

type degistirCozucu struct{ d *degistirici }

func (c degistirCozucu) CozZincir(t []string) []string {
	out := make([]string, len(t))
	for i, s := range t {
		out[i] = c.d.uygula(s)
	}
	return out
}
