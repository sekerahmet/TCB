package tokenize

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// Çözücüler (decoder)
// ============================================================================

func init() {
	RegisterDecoder("ByteLevel", func(json.RawMessage) (Decoder, error) { return byteDecoder{}, nil })
	RegisterDecoder("Metaspace", func(raw json.RawMessage) (Decoder, error) {
		m, err := readMetaspace(raw)
		if err != nil {
			return nil, err
		}
		return metaspaceDecoder{instead: []rune(m.instead)[0], rule: m.rule}, nil
	})
	RegisterDecoder("WordPiece", func(raw json.RawMessage) (Decoder, error) {
		t := struct {
			Prefix  *string `json:"prefix"`
			Cleanup *bool   `json:"cleanup"`
		}{}
		if err := strictDecode(raw, &t); err != nil {
			return nil, err
		}
		c := wordPieceDecoder{prefix: "##", clean: true}
		if t.Prefix != nil {
			c.prefix = *t.Prefix
		}
		if t.Cleanup != nil {
			c.clean = *t.Cleanup
		}
		return c, nil
	})
	RegisterDecoder("BPEDecoder", func(raw json.RawMessage) (Decoder, error) {
		t := struct {
			Suffix *string `json:"suffix"`
		}{}
		if err := strictDecode(raw, &t); err != nil {
			return nil, err
		}
		c := suffixDecoder{"</w>"}
		if t.Suffix != nil {
			c.suffix = *t.Suffix
		}
		return c, nil
	})
	RegisterDecoder("ByteFallback", func(json.RawMessage) (Decoder, error) { return byteFallbackDecoder{}, nil })
	RegisterDecoder("Fuse", func(json.RawMessage) (Decoder, error) { return fuse{}, nil })
	RegisterDecoder("Strip", func(raw json.RawMessage) (Decoder, error) {
		var t struct {
			Content string `json:"content"`
			Start   int    `json:"start"`
			Stop    int    `json:"stop"`
		}
		if err := strictDecode(raw, &t); err != nil {
			return nil, err
		}
		if utf8.RuneCountInString(t.Content) != 1 {
			return nil, messages.New(messages.H5212, t.Content)
		}
		return stripDecoder{[]rune(t.Content)[0], t.Start, t.Stop}, nil
	})
	RegisterDecoder("Replace", func(raw json.RawMessage) (Decoder, error) {
		d, err := newReplacer(raw)
		if err != nil {
			return nil, err
		}
		return replaceDecoder{d}, nil
	})
	RegisterDecoder("Sequence", func(raw json.RawMessage) (Decoder, error) {
		var t struct {
			Decoders []json.RawMessage `json:"decoders"`
		}
		if err := strictDecode(raw, &t); err != nil {
			return nil, err
		}
		s, err := buildSequence(t.Decoders, NewDecoder)
		if err != nil {
			return nil, err
		}
		return DecoderSequence(s), nil
	})
}

// DecoderSequence çözücüleri sırayla uygular.
type DecoderSequence []Decoder

func (s DecoderSequence) DecodeChain(t []string) []string { return s.decodeChainB(t, nil) }

func (s DecoderSequence) decodeChainB(t []string, b *reBudget) []string {
	if b == nil {
		b = newBudget(listBytes(t))
	}
	for _, c := range s {
		t = decodeBudgeted(c, t, b)
	}
	return t
}

// budgetedDecoder düzenli ifade kullanan çözücülerdir: bir çözme çağrısının
// paylaşılan adım bütçesiyle çalışabilirler.
type budgetedDecoder interface {
	decodeChainB(t []string, b *reBudget) []string
}

// decodeBudgeted c'yi, destekliyorsa b bütçesiyle uygular.
func decodeBudgeted(c Decoder, t []string, b *reBudget) []string {
	if bc, ok := c.(budgetedDecoder); ok && b != nil {
		return bc.decodeChainB(t, b)
	}
	out := c.DecodeChain(t)
	if b != nil {
		b.chargeGrowth(listBytes(t), listBytes(out))
	}
	return out
}

// listBytes token listesinin toplam bayt sayısıdır.
func listBytes(t []string) int {
	n := 0
	for _, s := range t {
		n += len(s)
	}
	return n
}

// byteDecoder tokenları bayt düzeyi karakterlerden baytlara çevirir ve tümünü
// tek bir (kayıplı UTF-8 ile onarılmış) metne birleştirir.
type byteDecoder struct{}

func (byteDecoder) DecodeChain(t []string) []string {
	var buf []byte
	for _, s := range t {
		buf = append(buf, tokenBytes(s)...)
	}
	return []string{lossyUTF8(buf)}
}

type metaspaceDecoder struct {
	instead rune
	rule    prefixRule
}

func (m metaspaceDecoder) DecodeChain(t []string) []string {
	out := make([]string, len(t))
	for i, s := range t {
		var sb strings.Builder
		for _, r := range s {
			if r == m.instead {
				if i == 0 && m.rule != prefixNever {
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

type wordPieceDecoder struct {
	prefix string
	clean  bool
}

func wordPieceCleanup(s string) string {
	for _, c := range [][2]string{{" .", "."}, {" ?", "?"}, {" !", "!"}, {" ,", ","}, {" ' ", "'"},
		{" n't", "n't"}, {" 'm", "'m"}, {" do not", " don't"}, {" 's", "'s"}, {" 've", "'ve"}, {" 're", "'re"}} {
		s = strings.ReplaceAll(s, c[0], c[1])
	}
	return s
}

func (w wordPieceDecoder) DecodeChain(t []string) []string {
	out := make([]string, len(t))
	for i, s := range t {
		if i != 0 {
			if strings.HasPrefix(s, w.prefix) {
				s = strings.Replace(s, w.prefix, "", 1)
			} else {
				s = " " + s
			}
		}
		if w.clean {
			s = wordPieceCleanup(s)
		}
		out[i] = s
	}
	return out
}

type suffixDecoder struct{ suffix string }

func (c suffixDecoder) DecodeChain(t []string) []string {
	out := make([]string, len(t))
	for i, s := range t {
		fresh := " "
		if i == len(t)-1 {
			fresh = ""
		}
		out[i] = strings.ReplaceAll(s, c.suffix, fresh)
	}
	return out
}

// byteFallbackDecoder "<0xXX>" tokenlarını baytlara çevirir; ardışık baytlar
// geçerli UTF-8 değilse her bayt için bir U+FFFD üretilir.
type byteFallbackDecoder struct{}

func fallbackByte(s string) (byte, bool) {
	if len(s) != 6 || !strings.HasPrefix(s, "<0x") || s[5] != '>' {
		return 0, false
	}
	n, err := strconv.ParseUint(s[3:5], 16, 8)
	if err != nil {
		return 0, false
	}
	return byte(n), true
}

func (byteFallbackDecoder) DecodeChain(t []string) []string {
	var out []string
	var pending []byte
	flush := func() {
		if len(pending) == 0 {
			return
		}
		if utf8.Valid(pending) {
			out = append(out, string(pending))
		} else {
			for range pending {
				out = append(out, "�")
			}
		}
		pending = pending[:0]
	}
	for _, s := range t {
		if b, ok := fallbackByte(s); ok {
			pending = append(pending, b)
			continue
		}
		flush()
		out = append(out, s)
	}
	flush()
	return out
}

type fuse struct{}

func (fuse) DecodeChain(t []string) []string { return []string{strings.Join(t, "")} }

type stripDecoder struct {
	content     rune
	begin, stop int
}

func (k stripDecoder) DecodeChain(t []string) []string {
	out := make([]string, len(t))
	for i, s := range t {
		r := []rune(s)
		begin := 0
		for j := 0; j < k.begin && j < len(r); j++ {
			if r[j] != k.content {
				break
			}
			begin = j + 1
		}
		last := len(r)
		for j := 0; j < k.stop; j++ {
			x := len(r) - j - 1
			if x < 0 || r[x] != k.content {
				break
			}
			last = x
		}
		if last < begin {
			last = begin
		}
		out[i] = string(r[begin:last])
	}
	return out
}

type replaceDecoder struct{ d *replacer }

func (c replaceDecoder) DecodeChain(t []string) []string { return c.decodeChainB(t, nil) }

func (c replaceDecoder) decodeChainB(t []string, b *reBudget) []string {
	if b == nil {
		b = newBudget(listBytes(t))
	}
	out := make([]string, len(t))
	for i, s := range t {
		out[i] = c.d.apply(s, b)
	}
	return out
}
