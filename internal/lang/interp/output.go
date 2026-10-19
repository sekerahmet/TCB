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
func (v *VM) AddSecret(deger string) bool {
	if len([]rune(deger)) < MinSecretLength {
		return false
	}
	v.gizliMu.Lock()
	defer v.gizliMu.Unlock()
	if slices.Contains(v.gizliler, deger) {
		return true
	}
	v.gizliler = append(v.gizliler, deger)
	// Uzun değerler önce denenir: biri ötekini içerirse uzunu bütünüyle maskelensin.
	sirali := slices.Clone(v.gizliler)
	slices.SortFunc(sirali, func(a, b string) int { return len(b) - len(a) })
	ciftler := make([]string, 0, 2*len(sirali))
	for _, g := range sirali {
		ciftler = append(ciftler, g, SecretMask)
	}
	v.maske.Store(strings.NewReplacer(ciftler...))
	return true
}

// HasSecrets reports whether any secret value is registered. Places where scanning the whole
// output is costly (e.g. debug messages) check this first.
func (v *VM) HasSecrets() bool { return v.maske.Load() != nil }

// Mask replaces the registered secret values in text with SecretMask.
func (v *VM) Mask(metin string) string {
	if r := v.maske.Load(); r != nil {
		return r.Replace(metin)
	}
	return metin
}

// SetLiveLine sets the active live line (nil removes it). There is one live line at a time; a
// new one replaces the old.
func (v *VM) SetLiveLine(c LiveLine) {
	v.ciktiMu.Lock()
	defer v.ciktiMu.Unlock()
	v.canli = c
	v.canliVar.Store(c != nil)
}

// UpdateLiveLine runs fn under the output lock, so the live line drawing itself does not
// interleave with other output.
func (v *VM) UpdateLiveLine(fn func()) {
	v.ciktiMu.Lock()
	defer v.ciktiMu.Unlock()
	fn()
}

// WriteOutput writes text meant for a person to w: secret values are masked, and the active
// live line is cleared first and redrawn afterwards.
func (v *VM) WriteOutput(w io.Writer, metin string) {
	metin = v.Mask(metin)
	if !v.canliVar.Load() {
		_, _ = io.WriteString(w, metin)
		return
	}
	v.ciktiMu.Lock()
	defer v.ciktiMu.Unlock()
	c := v.canli
	if c != nil {
		c.Clear()
	}
	_, _ = io.WriteString(w, metin)
	if c != nil {
		c.Draw()
	}
}
