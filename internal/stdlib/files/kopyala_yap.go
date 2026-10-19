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

// yenidenAdlandir os.Rename'dir; testler disk değiştiren (başka sürücüye) taşımayı bununla
// benzetir.
var yenidenAdlandir = os.Rename

// yap denetlenmiş işi yapar (yorumlayıcı kilidi bırakılmışken; TR değerlerine dokunmaz).
func (is *tasimaIsi) yap() error {
	if is.tasi {
		err := is.yerineKoy(is.kaynak)
		if err == nil || !farkliAygit(err) {
			return err
		}
		// Başka bir diske: kopyala, doğrula, sonra kaynağı sil.
	}
	if !is.kaynakBilgi.IsDir() {
		return is.dosyaKopyala()
	}
	return is.klasorKopyala()
}

// farkliAygit yeniden adlandırma kaynakla hedef başka disklerde olduğu için mi olmadı?
func farkliAygit(err error) bool {
	var no syscall.Errno
	if !errors.As(err, &no) {
		return false
	}
	if runtime.GOOS == "windows" {
		return no == 17 // ERROR_NOT_SAME_DEVICE
	}
	return no == syscall.EXDEV
}

// yerineKoy yeni (hazırlanmış kopya ya da taşınan kaynak) yolu son hedefe yeniden adlandırarak
// koyar. Hedef bir dosyaysa yeniden adlandırma onu tek adımda değiştirir; var olan bir
// klasörse önce geçici bir ada çekilir, yeni klasör yerine konunca silinir (yerine koyma
// olmazsa eski klasör geri konur).
func (is *tasimaIsi) yerineKoy(yeni string) error {
	if !is.varKlasor {
		return yenidenAdlandir(yeni, is.son)
	}
	eski, err := os.MkdirTemp(filepath.Dir(is.son), "."+filepath.Base(is.son)+".*.eski")
	if err != nil {
		return err
	}
	if err := os.Remove(eski); err != nil {
		return err
	}
	if err := yenidenAdlandir(is.son, eski); err != nil {
		return err
	}
	if err := yenidenAdlandir(yeni, is.son); err != nil {
		_ = yenidenAdlandir(eski, is.son)
		return err
	}
	if b, err := os.Lstat(eski); err == nil {
		_, _ = agaciSil(eski, b)
	}
	return nil
}

// dosyaKopyala dosyayı atomik olarak (geçici ad + yerine koyma) kopyalar; izinleri ve
// değiştirilme zamanını korur. Taşımada boyutu doğrulayıp kaynağı siler.
func (is *tasimaIsi) dosyaKopyala() error {
	k, err := os.Open(is.kaynak)
	if err != nil {
		return err
	}
	err = interp.AtomikYaz(is.son, func(w io.Writer) error {
		_, e := io.Copy(w, k)
		return e
	})
	k.Close()
	if err != nil {
		return err
	}
	zamanIzinKoru(is.son, is.kaynakBilgi)
	if !is.tasi {
		return nil
	}
	if b, err := os.Stat(is.son); err != nil || b.Size() != is.kaynakBilgi.Size() {
		return turkish.Neden(fmt.Sprintf("kopyanın boyutu kaynağınkiyle aynı değil; kaynak silinmedi (%q)", filepath.ToSlash(is.son)))
	}
	_, err = agaciSil(is.kaynak, is.kaynakBilgi)
	return err
}

func zamanIzinKoru(yol string, b fs.FileInfo) {
	_ = os.Chmod(yol, b.Mode().Perm())
	_ = os.Chtimes(yol, time.Now(), b.ModTime())
}

// klasorKopyala klasörü hedefin yanındaki geçici bir klasöre kopyalar, sonra yerine koyar
// (yarıda kalan kopya var olan hedefi bozmaz). Taşımada kopyayı dosya sayısı ve toplam
// boyutla doğrular, sonra kaynağı siler; kaynakta kopyalanamayan bir bağlantı varsa kaynağa
// dokunmaz.
func (is *tasimaIsi) klasorKopyala() (err error) {
	gecici, err := os.MkdirTemp(filepath.Dir(is.son), "."+filepath.Base(is.son)+".*.kopyalaniyor")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(gecici)
		}
	}()
	ozet, err := agaciKopyala(is.kaynak, gecici)
	is.atlanan = ozet.atlanan
	if err != nil {
		return err
	}
	if is.tasi {
		if len(ozet.atlanan) > 0 {
			return turkish.Neden(fmt.Sprintf("kaynakta %d bağlantı ya da özel dosya var; başka bir diske taşınamaz (önce kopyalayıp bağlantıları elle kurun)", len(ozet.atlanan)))
		}
		if hedefOzet, err := agacOzeti(gecici); err != nil || hedefOzet != ozet.sayim {
			return turkish.Neden("kopya kaynakla aynı değil (dosya sayısı ya da toplam boyut tutmuyor); kaynak silinmedi")
		}
	}
	if err = is.yerineKoy(gecici); err != nil {
		return err
	}
	if is.tasi {
		_, err = agaciSil(is.kaynak, is.kaynakBilgi)
	}
	return err
}

// sayim bir ağaçtaki sıradan dosyaların sayısı ve toplam boyutudur.
type sayim struct {
	dosya int
	bayt  int64
}

type kopyaOzeti struct {
	sayim
	atlanan []string // kopyalanmayan bağlantılar ve özel dosyalar (kaynağa göre, / ile)
}

// agaciKopyala kaynak klasörün içini hedef klasöre kopyalar. Kaynak os.Root ile gezilir:
// bağlantıların içine girilmez, gezinme kaynağın dışına çıkmaz. Klasörlerin izinleri ve
// zamanları en sonda (içleri yazıldıktan sonra) kurulur.
func agaciKopyala(kaynak, hedef string) (kopyaOzeti, error) {
	var oz kopyaOzeti
	kok, err := os.OpenRoot(kaynak)
	if err != nil {
		return oz, err
	}
	defer kok.Close()
	type klasorBilgisi struct {
		yol string
		b   fs.FileInfo
	}
	var klasorler []klasorBilgisi
	err = fs.WalkDir(kok.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		h := filepath.Join(hedef, filepath.FromSlash(p))
		switch {
		case d.IsDir():
			b, err := d.Info()
			if err != nil {
				return err
			}
			klasorler = append(klasorler, klasorBilgisi{h, b})
			if p == "." {
				return nil
			}
			return os.Mkdir(h, 0o700)
		case d.Type().IsRegular():
			n, err := rootDosyaKopyala(kok, p, h)
			oz.dosya++
			oz.bayt += n
			return err
		}
		oz.atlanan = append(oz.atlanan, p)
		return nil
	})
	for i := len(klasorler) - 1; i >= 0 && err == nil; i-- {
		zamanIzinKoru(klasorler[i].yol, klasorler[i].b)
	}
	return oz, err
}

// rootDosyaKopyala kökteki p dosyasını h'ye kopyalar (izin ve zamanla); kopyalanan bayt
// sayısını verir.
func rootDosyaKopyala(kok *os.Root, p, h string) (int64, error) {
	k, err := kok.Open(p)
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
		zamanIzinKoru(h, b)
	}
	return n, err
}

// agacOzeti klasördeki sıradan dosyaları sayar ve boyutlarını toplar (doğrulama için).
func agacOzeti(klasor string) (sayim, error) {
	var s sayim
	err := filepath.WalkDir(klasor, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		b, err := d.Info()
		if err != nil {
			return err
		}
		s.dosya++
		s.bayt += b.Size()
		return nil
	})
	return s, err
}
