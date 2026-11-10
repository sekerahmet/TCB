package interp

import (
	"slices"
	"strings"
)

// --- PARÇA PARÇA METNİN MASKESİ (19.4a) ---
//
// Maske (19.4) her çıktıya ayrı uygulanır: iki parçaya bölünen gizli değeri görmez. MaskStream
// parçaları birleştirip 19.4'ün maskesinin kararı kesinleşen başını maskeli verir, gerisini tutar.
// Metin soldan taranır (19.4'ün örtüşme kuralı: bir konumda eşleşen en uzun gizli değer bütünüyle
// maskelenir, içi yeniden taranmaz); maskelenmemiş bir konumda kalan metin daha uzun bir gizli
// değerin eksik başıysa oradan sonrası tutulur. Söz: bütün Step ve Finish çıktılarının birleşimi,
// parçaların birleşiminin Mask'ıdır (akış açıkken yeni gizli değer eklenmezse; FuzzMaskStream).

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
	out, m.held = maskScan(m.vm.secretsLongestFirst(), m.held+piece)
	return out, true
}

// Finish tutulanı maskeleyip verir ve akışı bitirir. Bitmiş akışta tutulan yoktur, boş metin döner.
func (m *MaskStream) Finish() string {
	m.finished = true
	out := m.vm.Mask(m.held)
	m.held = ""
	return out
}

// secretsLongestFirst kayıtlı gizli değerlerdir, uzundan kısaya (Mask'ın denediği sıra).
func (v *VM) secretsLongestFirst() []string {
	v.isSecret.Lock()
	ordered := slices.Clone(v.secrets)
	v.isSecret.Unlock()
	slices.SortFunc(ordered, func(a, b string) int { return len(b) - len(a) })
	return ordered
}

// maskScan metni soldan tarar: bir konumda kalan metin daha uzun bir gizli değerin eksik başıysa
// oradan sonrası tutulur (held); değilse o konumda eşleşen en uzun gizli değer maskelenir.
func maskScan(ordered []string, text string) (out, held string) {
	if len(ordered) == 0 {
		return text, ""
	}
	longest := len(ordered[0])
	var b strings.Builder
	for i := 0; i < len(text); {
		rest := text[i:]
		if len(rest) < longest && incompleteSecret(ordered, rest) {
			return b.String(), rest
		}
		if n := matchAt(ordered, rest); n > 0 {
			b.WriteString(SecretMask)
			i += n
			continue
		}
		b.WriteByte(text[i])
		i++
	}
	return b.String(), ""
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
