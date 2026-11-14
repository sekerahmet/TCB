package download

import (
	"errors"
	"io"
	"os"
)

// errBusy kilidi başka bir indirmenin (bu süreçte ya da başka süreçte) tuttuğunu söyler.
var errBusy = errors.New("part locked")

// lockAttempts kilit alınınca yolun hâlâ kilitlenen dosyayı gösterip göstermediğinin en çok kaç kez
// deneneceğidir: ad başka dosyaya geçtiyse dosya yeniden açılır (28.10), sonsuz döngü olmasın.
const lockAttempts = 3

// part kilitli bir .part dosyasıdır. Kilit dosya kapanana kadar tutulur; ad yalnız kilit tutulurken
// değişir ve dosya hiçbir zaman silinmez (28.10).
type part struct {
	path string
	f    *os.File
}

// openPart .part'ı açar, kilitler ve yolun kilitlenen dosyayı gösterdiğini denetler. Kilit başka
// birindeyse errBusy döner.
func openPart(path string) (*part, error) {
	for range lockAttempts {
		f, err := openLocked(path)
		if err != nil {
			return nil, err
		}
		held, err1 := f.Stat()
		named, err2 := os.Stat(path)
		if err1 == nil && err2 == nil && os.SameFile(held, named) {
			return &part{path: path, f: f}, nil
		}
		f.Close()
	}
	return nil, errBusy
}

// size .part'taki bayt sayısıdır.
func (p *part) size() (int64, error) {
	fi, err := p.f.Stat()
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// reset içeriği sıfırlar; dosyanın kimliği ve adı aynı kalır.
func (p *part) reset() error {
	if err := p.f.Truncate(0); err != nil {
		return err
	}
	_, err := p.f.Seek(0, io.SeekStart)
	return err
}

// readFrom0 içeriği baştan okur. Windows'ta aynı tutamaktan okunur: DELETE erişimli tutamak
// FILE_SHARE_DELETE vermeyen ikinci açışı reddeder (K7, defter 8).
func (p *part) readFrom0(w io.Writer) error {
	if _, err := p.f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	_, err := io.Copy(w, p.f)
	return err
}

// close kilidi bırakır ve dosyayı kapatır.
func (p *part) close() { p.f.Close() }

// place .part'ı kilit tutulurken hedefin yerine koyar: önce diske işlenir (18.13), sonra ad tek
// adımda değişir (18.10). Var olan hedef ezilir; ezmenin izni çağıranın denetimidir.
func (p *part) place(target string) error {
	if err := p.f.Sync(); err != nil {
		return err
	}
	return renameLocked(p.f, p.path, target)
}
