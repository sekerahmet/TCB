package debugger

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// runningFile çalışan deyimin dosyasıdır: hata ayıklanan dosya ya da 'aktar' ile yüklenen bir
// modül. Deyim satırları bu dosyanın satırlarıdır.
type runningFile struct {
	path  string
	lines []string
	// own hata ayıklanan dosya mı: kesme noktaları ve zıplama satırları bu dosyadadır.
	own bool
}

// sourcePathOf kaynağın yoludur; nil ana programdır.
func sourcePathOf(v *interp.VM, src *interp.Source) string {
	if src == nil {
		return v.SourceFile
	}
	return src.Path
}

// inOwnFile çalışan deyim hata ayıklanan dosyada mı (her deyimde sorulur; ucuz tutulur).
func (d *Debugger) inOwnFile(v *interp.VM) bool {
	return sourcePathOf(v, v.CurrentSource()) == d.SourceFile
}

// fileOf kaynağın dosyasını verir; başka dosyaların satırları ilk gösterimde bölünüp saklanır.
func (d *Debugger) fileOf(v *interp.VM, src *interp.Source) runningFile {
	path := sourcePathOf(v, src)
	if path == d.SourceFile {
		return runningFile{path: path, lines: d.SourceLines, own: true}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	lines, ok := d.otherLines[path]
	if !ok {
		code := v.SourceCode
		if src != nil {
			code = src.Code
		}
		lines = strings.Split(strings.ReplaceAll(code, "\r\n", "\n"), "\n")
		if d.otherLines == nil {
			d.otherLines = make(map[string][]string)
		}
		d.otherLines[path] = lines
	}
	return runningFile{path: path, lines: lines}
}

// quietly fn'i hata ayıklayıcıyı tetiklemeden çalıştırır: izleme koşulu, kesme noktası koşulu
// ya da konsolda hesaplanan ifade bir kullanıcı fonksiyonunu çağırınca gövdesinin deyimleri
// yeniden duraklatmaz, izleme noktalarını yeniden denetlemez (sonsuz özyineleme olurdu).
func (d *Debugger) quietly(fn func() (interp.Value, error)) (interp.Value, error) {
	was := d.Active
	d.Active = false
	defer func() { d.Active = was }()
	return fn()
}
