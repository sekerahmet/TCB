package interp

import (
	"io"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// Çıktı noktaları: insanın göreceği her çıktı (yazdır, girdi istemi, hata mesajları,
// uyarılar, ilerleme çubuğunun düz satırları) CiktiYaz'dan geçer. Burada iki kanca vardır:
//
//   - Gizli değer süzgeci: kayıtlı gizli değerler (ortam değişkeninden ya da komut satırından
//     gelen anahtar, parola ...) çıktıda *** ile maskelenir. Bu bir sızıntı azaltma önlemidir,
//     garanti değildir (metin dönüştürülünce eşleşme kaybolur). Veri çıktıları (dosya_yaz,
//     JSON, yazdır'ın döndürdüğü değer) değiştirilmez.
//   - Canlı satır: terminalin son satırında yeniden çizilen bir çıktı (ilerleme çubuğu)
//     etkinse, metin yazılmadan önce silinir ve sonra yeniden çizilir; çıktılar çubuğa
//     karışmaz.
//
// Kanca kurulmamışsa (gizli değer yok, canlı satır yok) maliyet iki atomik okumadır.

// SecretMask is the text written in place of secret values in output.
const SecretMask = "***"

// MinSecretLength is the minimum length of a secret value taken into the filter; shorter
// values also occur in ordinary text, so they are not masked (to avoid mangling output).
const MinSecretLength = 6

// LiveLine is output redrawn on the terminal's last line (e.g. a progress bar). Clear and Draw
// write to its own stream (usually standard error); the VM calls them under the output lock.
type LiveLine interface {
	Clear() // satırı ekrandan siler
	Draw()  // satırı yeniden çizer
}

// AddSecret registers a value with the secret-value filter; a value shorter than
// MinSecretLength characters is not registered and it returns false. It may be called from
// any goroutine.
func (v *VM) AddSecret(value string) bool {
	if utf8.RuneCountInString(value) < MinSecretLength {
		return false
	}
	v.addSecrets([]string{value}, false)
	return true
}

// addSecrets registers the values of at least MinSecretLength characters, with their escaped forms
// (escapedForms), with the filter and with a part of the program-wide set (start: the start part).
// New values are gathered first and the filter is built once, only when one of them is new: a value
// with many addresses does not rebuild it per address.
func (v *VM) addSecrets(values []string, start bool) {
	values = slices.DeleteFunc(slices.Clone(values), func(s string) bool {
		return utf8.RuneCountInString(s) < MinSecretLength
	})
	if len(values) == 0 {
		return
	}
	values = escapedForms(values)
	programSecrets.add(values, start)
	v.isSecret.Lock()
	defer v.isSecret.Unlock()
	known := make(map[string]bool, len(v.secrets)+len(values))
	for _, s := range v.secrets {
		known[s] = true
	}
	added := false
	for _, s := range values {
		if !known[s] {
			known[s] = true
			v.secrets = append(v.secrets, s)
			added = true
		}
	}
	if added {
		v.mask.Store(longestFirst(v.secrets))
		v.maskBuilds++
	}
}

// escapedForms is values with, after each value and when different from it, the forms the value
// takes when the text holding it is escaped before the mask: inside a container's text (QuoteText,
// without its quotes), inside TR_JSON's text (appendJSONString, without its quotes) and in a message
// or warning (turkish.Printable). The three escapes go character by character, so a secret inside a
// longer text escapes to its own escaped form (for valid UTF-8). Kural (19.4): maskeden önce metni
// değiştiren yeni bir dönüşüm eklenirse kayda onun biçimi de eklenir; testi
// TestEscapedSecretIsMasked'tedir.
func escapedForms(values []string) []string {
	out := make([]string, 0, len(values))
	for _, s := range values {
		quoted := QuoteText(s)
		json := string(appendJSONString(nil, s))
		out = append(out, s)
		for _, form := range []string{quoted[1 : len(quoted)-1], json[1 : len(json)-1], turkish.Printable(s)} {
			if form != s {
				out = append(out, form)
			}
		}
	}
	return out
}

// valueMask is the value mask of a set of secrets: the replacer and the same values longest first,
// for the scan that masks them together with the contextual mask (maskWith).
type valueMask struct {
	replacer *strings.Replacer
	ordered  []string
}

// longestFirst builds the value mask of secrets; longer values are tried first, so that when one
// contains another the longer is masked whole (strings.Replacer takes the earlier of two patterns
// that match at the same place).
func longestFirst(secrets []string) *valueMask {
	ordered := slices.Clone(secrets)
	slices.Sort(ordered)
	slices.SortStableFunc(ordered, func(a, b string) int { return len(b) - len(a) })
	pairs := make([]string, 0, 2*len(ordered))
	for _, g := range ordered {
		pairs = append(pairs, g, SecretMask)
	}
	return &valueMask{replacer: strings.NewReplacer(pairs...), ordered: ordered}
}

// apply masks text with the values of m (nil: none) and the contextual mask (19.4). Text with no
// address userinfo takes the replacer alone (one strings.Index more); otherwise the value matches
// and the contextual spans are found in the same text and joined (maskWith).
func (m *valueMask) apply(text string) string {
	spans, _ := contextSpans(text, true)
	if len(spans) == 0 {
		if m == nil {
			return text
		}
		return m.replacer.Replace(text)
	}
	out, _ := maskWith(m.longestFirst(), text, spans, -1, true)
	return out
}

// longestFirst m'nin değerleridir, uzundan kısaya (m nil ise yok).
func (m *valueMask) longestFirst() []string {
	if m == nil {
		return nil
	}
	return m.ordered
}

// LongestSecret is the length in bytes of the longest registered secret value (0 when there is
// none): a bounded buffer that may be cut keeps this much more, so that no secret is cut before it
// is masked.
func (v *VM) LongestSecret() int {
	v.isSecret.Lock()
	defer v.isSecret.Unlock()
	longest := 0
	for _, s := range v.secrets {
		longest = max(longest, len(s))
	}
	return longest
}

// HasSecrets reports whether any secret value is registered. Places where scanning the whole
// output is costly (e.g. debug messages) check this first; the contextual mask needs no registered
// value, so such a place masks text with "://" even when this is false (19.4).
func (v *VM) HasSecrets() bool { return v.mask.Load() != nil }

// Mask replaces the registered secret values in text, with their escaped forms, and the userinfo
// secret of every address in text (the contextual mask, 19.4) with SecretMask.
func (v *VM) Mask(text string) string { return v.mask.Load().apply(text) }

// maskValues is Mask without the contextual mask: only the registered values and their escaped
// forms. TR_write_file's H7600 check uses it (18.13): an address with a password in the written
// text is data, not a registered secret.
func (v *VM) maskValues(text string) string {
	if m := v.mask.Load(); m != nil {
		return m.replacer.Replace(text)
	}
	return text
}

// MaskBeforeCut masks text with the program-wide secret set and the contextual mask (19.4). A place
// that cuts text meant for a person and has no VM (the debugger's value text, the progress bar's
// text measure) calls it before cutting, so a cut does not leave the start of a secret.
func MaskBeforeCut(text string) string { return programSecrets.mask(text) }

// programSecrets süreç boyu gizli değer kümesidir; yalnız hata iletilerindeki değer özeti
// (summaryIn) onu kullanır: özet metni kesmeden önce bu kümeyle maskeler (19.4, önce maske sonra
// kesim). Özeti kuran TypeAndSummary'nin ve ileti metnini yazan MessageText'in elinde VM yoktur
// (çağrı yerlerinin çoğunda yoktur); bu yüzden küme VM başına değil süreç başınadır. Programın
// durumu VM başınadır ilkesinden yalnız bu sapar: çıktının maskesi (Mask) VM başına kalır.
// Bedeli: tek süreçte birden çok VM koşan yolda (trrun, Go testleri) bir VM'nin kaydı öbürünün
// yakalanan hata metninde *** olarak görünebilir; gizli değer sızmaz, yalnız fazla örtülür.
var programSecrets secretSet

// secretSet iki parçadır: süreç başı parçası (program başlarken taranan ortam ve argüman değerleri,
// hiç boşalmaz) ve VM'lerin kaydettiği parça (ClearRegisteredSecrets boşaltır). Değiştirici
// tembel kurulur: kayıt yalnız kümeyi değiştirir, değiştirici maske istendiğinde ve küme
// değişmişse bir kez kurulur.
type secretSet struct {
	mu         sync.Mutex
	start      map[string]struct{}
	registered map[string]struct{}
	changed    atomic.Bool
	replacer   atomic.Pointer[valueMask]
}

// add değerleri bir parçaya ekler; yalnız kümede olmayan değer kümeyi "değişti" diye işaretler.
// Süreç başı parçasına giren değer öbür parçadan çıkar: boşaltmada kaybolmasın.
func (s *secretSet) add(values []string, start bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, value := range values {
		if _, ok := s.start[value]; ok {
			continue
		}
		_, registered := s.registered[value]
		switch {
		case start:
			if s.start == nil {
				s.start = map[string]struct{}{}
			}
			s.start[value] = struct{}{}
			delete(s.registered, value)
		case registered:
			continue
		default:
			if s.registered == nil {
				s.registered = map[string]struct{}{}
			}
			s.registered[value] = struct{}{}
		}
		if !registered {
			s.changed.Store(true)
		}
	}
}

// mask metni kümeyle ve bağlamlı maskeyle maskeler (VM.Mask gibi); değiştirici küme değiştiyse
// burada bir kez kurulur.
func (s *secretSet) mask(text string) string {
	if s.changed.Load() {
		s.mu.Lock()
		if s.changed.Load() {
			s.replacer.Store(s.build())
			s.changed.Store(false)
		}
		s.mu.Unlock()
	}
	return s.replacer.Load().apply(text)
}

// build iki parçanın değerlerinden değer maskesini kurar (uzun değer önce); küme boşsa nil.
func (s *secretSet) build() *valueMask {
	values := make([]string, 0, len(s.start)+len(s.registered))
	for value := range s.start {
		values = append(values, value)
	}
	for value := range s.registered {
		values = append(values, value)
	}
	if len(values) == 0 {
		return nil
	}
	return longestFirst(values)
}

// ClearRegisteredSecrets empties the part of the program-wide secret set that VMs registered
// (AddSecret, AddAddressSecrets); the start part (AddStartAddressSecrets) stays. The test runner
// calls it before each test's VM, so a test's error text does not depend on an earlier test's
// secret (12.5); FuzzMaskStream calls it for each input. It would also empty the part of a VM
// running at the same time in the same process, so only those two places call it.
func ClearRegisteredSecrets() {
	programSecrets.mu.Lock()
	defer programSecrets.mu.Unlock()
	if len(programSecrets.registered) > 0 {
		programSecrets.registered = nil
		programSecrets.changed.Store(true)
	}
}

// SetLiveLine sets the active live line (nil removes it). There is one live line at a time; a
// new one replaces the old.
func (v *VM) SetLiveLine(c LiveLine) {
	v.isOutput.Lock()
	defer v.isOutput.Unlock()
	v.live = c
	v.hasLive.Store(c != nil)
}

// UpdateLiveLine runs fn under the output lock, so the live line drawing itself does not
// interleave with other output.
func (v *VM) UpdateLiveLine(fn func()) {
	v.isOutput.Lock()
	defer v.isOutput.Unlock()
	fn()
}

// WriteOutput writes text meant for a person to w: secret values are masked, and the active
// live line is cleared first and redrawn afterwards.
func (v *VM) WriteOutput(w io.Writer, text string) {
	text = v.Mask(text)
	if !v.hasLive.Load() {
		_, _ = io.WriteString(w, text)
		return
	}
	v.isOutput.Lock()
	defer v.isOutput.Unlock()
	c := v.live
	if c != nil {
		c.Clear()
	}
	_, _ = io.WriteString(w, text)
	if c != nil {
		c.Draw()
	}
}
