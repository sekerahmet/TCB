package ogrenme

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// atomikYaz, yaz'ın ürettiği içeriği yol'a atomik olarak yazar: içerik önce aynı klasörde
// geçici bir dosyaya yazılır, diske işlenir (Sync) ve dosya yeniden adlandırılarak eski
// dosyanın yerine konur. Yazma yarıda kalırsa (hata, disk dolması, çökme) var olan dosya
// bozulmadan kalır; hata durumunda geçici dosya silinir. Uzun eğitimlerin kontrol
// noktaları için.
//
// Hatalar hedef yolu taşır (Türkçe dosya hatası iletisi geçici dosyanın değil kullanıcının
// verdiği yolun nedenini anlatsın).
func atomikYaz(yol string, yaz func(w io.Writer) error) (err error) {
	if b, e := os.Stat(yol); e == nil && b.IsDir() {
		return &fs.PathError{Op: "open", Path: yol, Err: errors.New("bir klasör")}
	}
	f, err := os.CreateTemp(filepath.Dir(yol), "."+filepath.Base(yol)+".*.yaziliyor")
	if err != nil {
		var pe *fs.PathError
		if errors.As(err, &pe) {
			return &fs.PathError{Op: "open", Path: yol, Err: pe.Err}
		}
		return err
	}
	gecici := f.Name()
	kapali := false
	defer func() {
		if err != nil {
			if !kapali {
				f.Close()
			}
			os.Remove(gecici)
		}
	}()
	if err = yaz(f); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	kapali = true
	if err = f.Close(); err != nil {
		return err
	}
	_ = os.Chmod(gecici, 0o644) // geçici dosya 0600 açılır; kayıtlar herkesçe okunabilir olsun
	return os.Rename(gecici, yol)
}
