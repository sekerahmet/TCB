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
	return applySpans(text, spans, cut), text[cut:]
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
