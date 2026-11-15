package interp

import (
	"slices"
	"strings"
)

// --- PARÇA PARÇA METNİN MASKESİ (19.4a) ---
//
// Maske (19.4) her çıktıya ayrı uygulanır: iki parçaya bölünen gizli değeri ya da adresi görmez.
// MaskStream parçaları birleştirip 19.4'ün maskesinin kararı kesinleşen başını maskeli verir, gerisini
// tutar. Değer maskesi ve bağlamlı maske aynı metinde bulunur (maskWith): değer eşleşmeleri soldan
// taranır (19.4'ün örtüşme kuralı: bir konumda eşleşen en uzun gizli değer bütünüyle maskelenir, içi
// yeniden taranmaz), adres bölgeleri contextSpans'le. Tutulan, şunların en solundan başlar: daha uzun
// bir gizli değerin eksik başı; sınırı henüz gelmemiş bir adres bölgesi, "://"'dan başlayarak
// (uzunluğu sınırsız); metnin sonundaki ":" ya da ":/"; bunlardan biri bir aralığın ortasına düşerse o
// aralığın başı. Söz: bütün Step ve Finish çıktılarının birleşimi, parçaların birleşiminin Mask'ıdır
// (akış açıkken yeni gizli değer eklenmezse; FuzzMaskStream).

// MaskStream parça parça gelen metni insanın göreceği çıktıya maskeli verir (19.4a). Bir görevden
// kullanılır; eşzamanlı Step çağrıları için kilidi yoktur.
type MaskStream struct {
	vm       *VM
	held     string
	finished bool
}

// NewMaskStream boş bir akış verir.
func (v *VM) NewMaskStream() *MaskStream { return &MaskStream{vm: v} }

// Step tutulanla parçayı birleştirir ve kararı kesinleşen başı maskeli verir; akış bittiyse ok false.
func (m *MaskStream) Step(piece string) (out string, ok bool) {
	if m.finished {
		return "", false
	}
	text := m.held + piece
	spans, open := contextSpans(text, false)
	out, m.held = maskWith(m.vm.mask.Load().longestFirst(), text, spans, open, false)
	return out, true
}

// Finish tutulanı maskeleyip verir ve akışı bitirir. Bitmiş akışta tutulan yoktur, boş metin döner.
func (m *MaskStream) Finish() string {
	m.finished = true
	out := m.vm.Mask(m.held)
	m.held = ""
	return out
}

// maskWith masks text with the value mask (ordered: the secrets longest first) and the contextual
// spans together (19.4): value matches are found in the same original text as spans, and ranges
// sharing a byte become one SecretMask. atEnd false (a stream's Step): open is the "://" offset of
// the first address region with no boundary yet (-1: none), and the end whose decision is not final
// is returned as held instead of being masked.
func maskWith(ordered []string, text string, spans []span, open int, atEnd bool) (out, held string) {
	spans, cut := maskCut(ordered, text, spans, open, atEnd)
	return applySpans(text, spans, cut), text[cut:]
}

// maskCut maskWith'in kararıdır: birleşmiş aralıklar ve kararı kesinleşen başın sonu (atEnd'de
// len(text)); metni yazmaz.
func maskCut(ordered []string, text string, spans []span, open int, atEnd bool) ([]span, int) {
	cut := len(text)
	if len(ordered) > 0 {
		longest := len(ordered[0])
		for i := 0; i < len(text); {
			rest := text[i:]
			if !atEnd && len(rest) < longest && incompleteSecret(ordered, rest) {
				cut = i
				break
			}
			if n := matchAt(ordered, rest); n > 0 {
				spans = append(spans, span{i, i + n})
				i += n
				continue
			}
			i++
		}
	}
	if !atEnd {
		switch {
		case open >= 0:
			cut = min(cut, open)
		case strings.HasSuffix(text, ":/"):
			cut = min(cut, len(text)-2)
		case strings.HasSuffix(text, ":"):
			cut = min(cut, len(text)-1)
		}
	}
	spans = joinSpans(spans)
	for _, s := range spans {
		if s.start < cut && cut < s.end {
			cut = s.start
		}
	}
	return spans, cut
}

// MaskJoined olay akışının unutulan maskesidir (28.7): held aynı yolun önceki olaydaki metninden
// tutulan, text yeni olayın metni. out text'in maskelisidir: kendi maskesi (Mask) ile birleşimin
// (held+text) maskesinin text'e düşen kısmı birlikte; held boşsa Mask(text)'tir. caught: birleşim
// text'in kendi maskesinin görmediği bir aralık buldu (iki olaya bölünen gizli değer ya da adres), ya
// da sınırı gelmemiş bölge sınırı aştı. next 19.4a'nın akışının birleşimden tutacağıdır; en çok en
// uzun gizli değerin boyu (kayıttaki kaçışlı biçimler dahil, LongestSecret) + pad bayttır: aşarsa
// tutulanın text'e düşen kısmı *** olur, next boştur ve caught true'dur. Her goroutine'den
// çağrılabilir; maske tek kez okunur.
func (v *VM) MaskJoined(held, text string, pad int) (out, next string, caught bool) {
	ordered := v.mask.Load().longestFirst()
	n := len(held)
	joined := held + text
	var spans []span // nil: text'in maskesi Mask'ınkidir
	if n > 0 {
		own := fullSpans(ordered, text)
		spans = own
		for _, s := range fullSpans(ordered, joined) {
			if s.end > n {
				spans = append(spans, span{max(s.start-n, 0), s.end - n})
			}
		}
		spans = joinSpans(spans)
		caught = !slices.Equal(spans, own)
	}
	ctx, open := contextSpans(joined, false)
	_, cut := maskCut(ordered, joined, ctx, open, false)
	next = joined[cut:]
	longest := 0
	if len(ordered) > 0 {
		longest = len(ordered[0])
	}
	if len(next) > longest+pad {
		if spans == nil {
			spans = fullSpans(ordered, text)
		}
		spans = joinSpans(append(spans, span{max(cut-n, 0), len(text)}))
		next, caught = "", true
	}
	if spans == nil {
		return v.Mask(text), next, false
	}
	return applySpans(text, spans, len(text)), next, caught
}

// fullSpans text'in 19.4 maskesinin birleşmiş aralıklarıdır (Mask'ın kararı): adres bölgeleri metnin
// sonunda biter, gizli değerler soldan taranır.
func fullSpans(ordered []string, text string) []span {
	ctx, _ := contextSpans(text, true)
	spans, _ := maskCut(ordered, text, ctx, -1, true)
	return spans
}

// joinSpans aralıkları başlarına göre sıralar; ortak baytı olan aralıklar tek aralık olur (bitişik
// iki aralık ayrı kalır).
func joinSpans(spans []span) []span {
	slices.SortFunc(spans, func(a, b span) int { return a.start - b.start })
	var out []span
	for _, s := range spans {
		if n := len(out); n > 0 && s.start < out[n-1].end {
			out[n-1].end = max(out[n-1].end, s.end)
			continue
		}
		out = append(out, s)
	}
	return out
}

// applySpans text[:upTo]'yu yazar, upTo'dan önce başlayan her aralığın yerine SecretMask.
func applySpans(text string, spans []span, upTo int) string {
	var b strings.Builder
	last := 0
	for _, s := range spans {
		if s.start >= upTo {
			break
		}
		b.WriteString(text[last:s.start])
		b.WriteString(SecretMask)
		last = s.end
	}
	b.WriteString(text[last:upTo])
	return b.String()
}

// incompleteSecret rest daha uzun bir gizli değerin eksik başı mı?
func incompleteSecret(ordered []string, rest string) bool {
	for _, g := range ordered {
		if len(g) > len(rest) && strings.HasPrefix(g, rest) {
			return true
		}
	}
	return false
}

// matchAt rest'in başında eşleşen en uzun gizli değerin bayt uzunluğudur (yoksa 0).
func matchAt(ordered []string, rest string) int {
	for _, g := range ordered {
		if strings.HasPrefix(rest, g) {
			return len(g)
		}
	}
	return 0
}
