package tokenize

import (
	"encoding/json"
	"strings"
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// Normalleştiriciler
// ============================================================================

func init() {
	RegisterNormalizer("Sequence", newSeqNorm)
	for _, t := range []struct {
		ad            string
		compat, merge bool
	}{{"NFC", false, true}, {"NFD", false, false}, {"NFKC", true, true}, {"NFKD", true, false}} {
		n := unicodeNorm{t.compat, t.merge}
		RegisterNormalizer(t.ad, func(json.RawMessage) (Normalizer, error) { return n, nil })
	}
	RegisterNormalizer("Lowercase", func(json.RawMessage) (Normalizer, error) { return lowerCase{}, nil })
	RegisterNormalizer("StripAccents", func(json.RawMessage) (Normalizer, error) { return stripAccent{}, nil })
	RegisterNormalizer("Nmt", func(json.RawMessage) (Normalizer, error) { return nmtNorm{}, nil })
	RegisterNormalizer("ByteLevel", func(json.RawMessage) (Normalizer, error) { return byteNorm{}, nil })
	RegisterNormalizer("Strip", newStripNorm)
	RegisterNormalizer("Replace", newReplaceNorm)
	RegisterNormalizer("Prepend", newPrefixNorm)
	RegisterNormalizer("BertNormalizer", newBertNorm)
}

// NormalizerSequence normalleştiricileri sırayla uygular.
type NormalizerSequence []Normalizer

func (s NormalizerSequence) Normalize(m string) string { return s.normalizeB(m, nil) }

func (s NormalizerSequence) normalizeB(m string, b *reBudget) string {
	if b == nil {
		b = newBudget(len(m))
	}
	for _, n := range s {
		m = normalizeBudgeted(n, m, b)
	}
	return m
}

// budgetedNormalizer düzenli ifade kullanan normalleştiricilerdir: bir
// kodlama çağrısının paylaşılan adım bütçesiyle çalışabilirler.
type budgetedNormalizer interface {
	normalizeB(m string, b *reBudget) string
}

// normalizeBudgeted n'yi, destekliyorsa b bütçesiyle uygular. Bütçeyi kendisi kullanmayan
// normalleştiricilerin çıktı büyütmesi burada bütçeden düşülür.
func normalizeBudgeted(n Normalizer, m string, b *reBudget) string {
	if bn, ok := n.(budgetedNormalizer); ok && b != nil {
		return bn.normalizeB(m, b)
	}
	out := n.Normalize(m)
	b.chargeGrowth(len(m), len(out))
	return out
}

func newSeqNorm(raw json.RawMessage) (Normalizer, error) {
	var t struct {
		Normalizers []json.RawMessage `json:"normalizers"`
	}
	if err := strictDecode(raw, &t); err != nil {
		return nil, err
	}
	var s NormalizerSequence
	for i, h := range t.Normalizers {
		n, err := NewNormalizer(h)
		if err != nil {
			return nil, messages.New(messages.H5213, i, err)
		}
		if n != nil {
			s = append(s, n)
		}
	}
	return s, nil
}

type unicodeNorm struct{ compat, joinFn bool }

func (n unicodeNorm) Normalize(m string) string {
	return unicodeNormalize(m, n.compat, n.joinFn)
}

// lowerCase her karakteri tek tek küçük harfe çevirir (bağlama bakmadan).
// 'İ' (U+0130) Unicode'un koşulsuz özel durumuna göre "i̇" (i + U+0307) olur.
type lowerCase struct{}

func (lowerCase) Normalize(m string) string {
	var sb strings.Builder
	sb.Grow(len(m))
	for _, r := range m {
		if r == 'İ' {
			sb.WriteString("i̇")
			continue
		}
		if s, ok := lowerCaseExceptions[r]; ok {
			sb.WriteString(s)
			continue
		}
		sb.WriteRune(unicode.ToLower(r))
	}
	return sb.String()
}

// stripAccent birleşen işaretleri (M) siler; genellikle NFD'den sonra kullanılır.
type stripAccent struct{}

func (stripAccent) Normalize(m string) string {
	return strings.Map(func(r rune) rune {
		if tableAccent.inside(r) {
			return -1
		}
		return r
	}, m)
}

type nmtNorm struct{}

func (nmtNorm) Normalize(m string) string {
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

type byteNorm struct{}

func (byteNorm) Normalize(m string) string { return bytesToChars([]byte(m)) }

type stripNorm struct{ left, right bool }

func (k stripNorm) Normalize(m string) string {
	if k.left {
		m = strings.TrimLeftFunc(m, unicode.IsSpace)
	}
	if k.right {
		m = strings.TrimRightFunc(m, unicode.IsSpace)
	}
	return m
}

func newStripNorm(raw json.RawMessage) (Normalizer, error) {
	var t struct {
		Left  bool `json:"strip_left"`
		Right bool `json:"strip_right"`
	}
	if err := strictDecode(raw, &t); err != nil {
		return nil, err
	}
	return stripNorm{t.Left, t.Right}, nil
}

// replacer bir deseni (düz metin ya da düzenli ifade) içerikle değiştirir.
type replacer struct {
	plain   string
	expr    *Regex
	content string
}

func newReplacer(raw json.RawMessage) (*replacer, error) {
	var t struct {
		Pattern patternForm `json:"pattern"`
		Content string      `json:"content"`
	}
	if err := strictDecode(raw, &t); err != nil {
		return nil, err
	}
	d := &replacer{content: t.Content}
	switch {
	case t.Pattern.String != nil:
		d.plain = *t.Pattern.String
	case t.Pattern.Regex != nil:
		expr, err := CompileRegex(*t.Pattern.Regex)
		if err != nil {
			return nil, err
		}
		d.expr = expr
	default:
		return nil, messages.New(messages.H5227)
	}
	return d, nil
}

// apply değiştirmeyi yapar; b nil değilse düzenli ifade adımları o
// bütçeden düşülür.
func (d *replacer) apply(m string, b *reBudget) string {
	if b == nil {
		b = newBudget(len(m))
	}
	if d.expr == nil {
		if d.plain == "" {
			return m
		}
		if grow := len(d.content) - len(d.plain); grow > 0 {
			// Üretmeden önce düşülür: çıktı hiç ayrılmadan bütçe aşımı bildirilir.
			b.charge(int64(strings.Count(m, d.plain))*int64(grow), messages.H5289)
		}
		return strings.ReplaceAll(m, d.plain, d.content)
	}
	var sb strings.Builder
	prev := 0
	d.expr.allMatches(m, func(begin, last int) {
		if grow := len(d.content) - (last - begin); grow > 0 {
			b.charge(int64(grow), messages.H5289)
		}
		sb.WriteString(m[prev:begin])
		sb.WriteString(d.content)
		prev = last
	}, b)
	sb.WriteString(m[prev:])
	return sb.String()
}

type replaceNorm struct{ d *replacer }

func (n replaceNorm) Normalize(m string) string { return n.d.apply(m, nil) }

func (n replaceNorm) normalizeB(m string, b *reBudget) string { return n.d.apply(m, b) }

func newReplaceNorm(raw json.RawMessage) (Normalizer, error) {
	d, err := newReplacer(raw)
	if err != nil {
		return nil, err
	}
	return replaceNorm{d}, nil
}

type prefixNorm struct{ prefix string }

func (n prefixNorm) Normalize(m string) string {
	if m == "" {
		return m
	}
	return n.prefix + m
}

func newPrefixNorm(raw json.RawMessage) (Normalizer, error) {
	var t struct {
		Prepend string `json:"prepend"`
	}
	if err := strictDecode(raw, &t); err != nil {
		return nil, err
	}
	return prefixNorm{t.Prepend}, nil
}

// bertNorm: denetim karakterlerini siler, boşlukları ' ' yapar, Çince
// karakterlerin çevresine boşluk koyar, isteğe bağlı aksan siler ve küçültür.
type bertNorm struct {
	clean, chinese, accent, shrink bool
}

func newBertNorm(raw json.RawMessage) (Normalizer, error) {
	var t struct {
		CleanText          *bool `json:"clean_text"`
		HandleChineseChars *bool `json:"handle_chinese_chars"`
		StripAccents       *bool `json:"strip_accents"`
		Lowercase          *bool `json:"lowercase"`
	}
	if err := strictDecode(raw, &t); err != nil {
		return nil, err
	}
	value := func(b *bool, v bool) bool {
		if b == nil {
			return v
		}
		return *b
	}
	n := bertNorm{clean: value(t.CleanText, true), chinese: value(t.HandleChineseChars, true), shrink: value(t.Lowercase, true)}
	n.accent = value(t.StripAccents, n.shrink)
	return n, nil
}

func isChinese(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) || (r >= 0x20000 && r <= 0x2A6DF) ||
		(r >= 0x2A700 && r <= 0x2B73F) || (r >= 0x2B740 && r <= 0x2B81F) || (r >= 0x2B920 && r <= 0x2CEAF) ||
		(r >= 0xF900 && r <= 0xFAFF) || (r >= 0x2F800 && r <= 0x2FA1F)
}

func (n bertNorm) Normalize(m string) string {
	if n.clean {
		m = strings.Map(func(r rune) rune {
			if tableBertControl.inside(r) {
				return -1
			}
			if r == '\t' || r == '\n' || r == '\r' || unicode.IsSpace(r) {
				return ' '
			}
			return r
		}, m)
	}
	if n.chinese {
		var sb strings.Builder
		for _, r := range m {
			if isChinese(r) {
				sb.WriteByte(' ')
				sb.WriteRune(r)
				sb.WriteByte(' ')
			} else {
				sb.WriteRune(r)
			}
		}
		m = sb.String()
	}
	if n.accent {
		m = strings.Map(func(r rune) rune {
			if tableBertAccent.inside(r) {
				return -1
			}
			return r
		}, unicodeNormalize(m, false, false))
	}
	if n.shrink {
		m = lowerCase{}.Normalize(m)
	}
	return m
}
