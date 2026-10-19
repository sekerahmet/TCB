package interp

import (
	"io"
	"slices"
	"strings"
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
	if len([]rune(value)) < MinSecretLength {
		return false
	}
	v.isSecret.Lock()
	defer v.isSecret.Unlock()
	if slices.Contains(v.secrets, value) {
		return true
	}
	v.secrets = append(v.secrets, value)
	// Uzun değerler önce denenir: biri ötekini içerirse uzunu bütünüyle maskelensin.
	ordered := slices.Clone(v.secrets)
	slices.SortFunc(ordered, func(a, b string) int { return len(b) - len(a) })
	pairs := make([]string, 0, 2*len(ordered))
	for _, g := range ordered {
		pairs = append(pairs, g, SecretMask)
	}
	v.mask.Store(strings.NewReplacer(pairs...))
	return true
}

// HasSecrets reports whether any secret value is registered. Places where scanning the whole
// output is costly (e.g. debug messages) check this first.
func (v *VM) HasSecrets() bool { return v.mask.Load() != nil }

// Mask replaces the registered secret values in text with SecretMask.
func (v *VM) Mask(text string) string {
	if r := v.mask.Load(); r != nil {
		return r.Replace(text)
	}
	return text
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
