package files

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// rename os.Rename'dir; testler disk değiştiren (başka sürücüye) taşımayı bununla
// benzetir.
var rename = os.Rename

// run denetlenmiş işi yapar (yorumlayıcı kilidi bırakılmışken; TR değerlerine dokunmaz).
func (job *moveJob) run() error {
	if job.move {
		err := job.replace(job.src)
		if err == nil || !otherDevice(err) {
			return err
		}
		// Başka bir diske: kopyala, doğrula, sonra kaynağı sil.
	}
	if !job.srcInfo.IsDir() {
		return job.copyFile()
	}
	return job.copyDir()
}

// otherDevice yeniden adlandırma kaynakla hedef başka disklerde olduğu için mi olmadı?
func otherDevice(err error) bool {
	var no syscall.Errno
	if !errors.As(err, &no) {
		return false
	}
	if runtime.GOOS == "windows" {
		return no == 17 // ERROR_NOT_SAME_DEVICE
	}
	return no == syscall.EXDEV
}

// replace yeni (hazırlanmış kopya ya da taşınan kaynak) yolu son hedefe yeniden adlandırarak
// koyar. Hedef bir dosyaysa yeniden adlandırma onu tek adımda değiştirir; var olan bir
// klasörse önce geçici bir ada çekilir, yeni klasör yerine konunca silinir (yerine koyma
// olmazsa eski klasör geri konur).
func (job *moveJob) replace(fresh string) error {
	if !job.existingDir {
		return rename(fresh, job.last)
	}
	old, err := os.MkdirTemp(filepath.Dir(job.last), "."+filepath.Base(job.last)+".*.eski")
	if err != nil {
		return err
	}
	if err := os.Remove(old); err != nil {
		return err
	}
	if err := rename(job.last, old); err != nil {
		return err
	}
	if err := rename(fresh, job.last); err != nil {
		_ = rename(old, job.last)
		return err
	}
	if b, err := os.Lstat(old); err == nil {
		_, _ = deleteTree(old, b)
	}
	return nil
}

// copyFile dosyayı atomik olarak (geçici ad + yerine koyma) kopyalar; izinleri ve
// değiştirilme zamanını korur. Taşımada boyutu doğrulayıp kaynağı siler.
func (job *moveJob) copyFile() error {
	k, err := os.Open(job.src)
	if err != nil {
		return err
	}
	err = interp.WriteAtomic(job.last, func(w io.Writer) error {
		_, e := io.Copy(w, k)
		return e
	})
	k.Close()
	if err != nil {
		return err
	}
	keepTimesAndPerms(job.last, job.srcInfo)
	if !job.move {
		return nil
	}
	if b, err := os.Stat(job.last); err != nil || b.Size() != job.srcInfo.Size() {
		return turkish.Reason(fmt.Sprintf("kopyanın boyutu kaynağınkiyle aynı değil; kaynak silinmedi (%q)", filepath.ToSlash(job.last)))
	}
	_, err = deleteTree(job.src, job.srcInfo)
	return err
}

func keepTimesAndPerms(fpath string, b fs.FileInfo) {
	_ = os.Chmod(fpath, b.Mode().Perm())
	_ = os.Chtimes(fpath, time.Now(), b.ModTime())
}

// copyDir klasörü hedefin yanındaki geçici bir klasöre kopyalar, sonra yerine koyar
// (yarıda kalan kopya var olan hedefi bozmaz). Taşımada kopyayı dosya sayısı ve toplam
// boyutla doğrular, sonra kaynağı siler; kaynakta kopyalanamayan bir bağlantı varsa kaynağa
// dokunmaz.
func (job *moveJob) copyDir() (err error) {
	tmp, err := os.MkdirTemp(filepath.Dir(job.last), "."+filepath.Base(job.last)+".*.kopyalaniyor")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(tmp)
		}
	}()
	digest, err := copyTree(job.src, tmp)
	job.skipped = digest.skipped
	if err != nil {
		return err
	}
	if job.move {
		if len(digest.skipped) > 0 {
			return turkish.Reason(fmt.Sprintf("kaynakta %d bağlantı ya da özel dosya var; başka bir diske taşınamaz (önce kopyalayıp bağlantıları elle kurun)", len(digest.skipped)))
		}
		if targetDigest, err := treeDigest(tmp); err != nil || targetDigest != digest.tally {
			return turkish.Reason("kopya kaynakla aynı değil (dosya sayısı ya da toplam boyut tutmuyor); kaynak silinmedi")
		}
	}
	if err = job.replace(tmp); err != nil {
		return err
	}
	if job.move {
		_, err = deleteTree(job.src, job.srcInfo)
	}
	return err
}

// tally bir ağaçtaki sıradan dosyaların sayısı ve toplam boyutudur.
type tally struct {
	file   int
	nBytes int64
}

type copySummary struct {
	tally
	skipped []string // kopyalanmayan bağlantılar ve özel dosyalar (kaynağa göre, / ile)
}

// copyTree kaynak klasörün içini hedef klasöre kopyalar. Kaynak os.Root ile gezilir:
// bağlantıların içine girilmez, gezinme kaynağın dışına çıkmaz. Klasörlerin izinleri ve
// zamanları en sonda (içleri yazıldıktan sonra) kurulur.
func copyTree(src, target string) (copySummary, error) {
	var own copySummary
	root, err := os.OpenRoot(src)
	if err != nil {
		return own, err
	}
	defer root.Close()
	type dirInfo struct {
		fpath string
		b     fs.FileInfo
	}
	var dirs []dirInfo
	err = fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		h := filepath.Join(target, filepath.FromSlash(p))
		switch {
		case d.IsDir():
			b, err := d.Info()
			if err != nil {
				return err
			}
			dirs = append(dirs, dirInfo{h, b})
			if p == "." {
				return nil
			}
			return os.Mkdir(h, 0o700)
		case d.Type().IsRegular():
			n, err := rootCopyFile(root, p, h)
			own.file++
			own.nBytes += n
			return err
		}
		own.skipped = append(own.skipped, p)
		return nil
	})
	for i := len(dirs) - 1; i >= 0 && err == nil; i-- {
		keepTimesAndPerms(dirs[i].fpath, dirs[i].b)
	}
	return own, err
}

// rootCopyFile kökteki p dosyasını h'ye kopyalar (izin ve zamanla); kopyalanan bayt
// sayısını verir.
func rootCopyFile(root *os.Root, p, h string) (int64, error) {
	k, err := root.Open(p)
	if err != nil {
		return 0, err
	}
	defer k.Close()
	b, err := k.Stat()
	if err != nil {
		return 0, err
	}
	f, err := os.OpenFile(h, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(f, k)
	if e := f.Close(); err == nil {
		err = e
	}
	if err == nil {
		keepTimesAndPerms(h, b)
	}
	return n, err
}

// treeDigest klasördeki sıradan dosyaları sayar ve boyutlarını toplar (doğrulama için).
func treeDigest(dir string) (tally, error) {
	var s tally
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		b, err := d.Info()
		if err != nil {
			return err
		}
		s.file++
		s.nBytes += b.Size()
		return nil
	})
	return s, err
}
