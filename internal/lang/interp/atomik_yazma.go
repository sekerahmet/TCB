package interp

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// AtomikYaz, yaz'ın ürettiği içeriği yol'a atomik olarak yazar: içerik önce aynı klasörde
// geçici bir dosyaya yazılır, diske işlenir (Sync) ve dosya yeniden adlandırılarak eski
// dosyanın yerine konur. Yazma yarıda kalırsa (hata, disk dolması, çökme) var olan dosya
// bozulmadan kalır; hata durumunda geçici dosya silinir. dosya_yaz, tensör kaydı ve uzun
// eğitimlerin kontrol noktaları bununla yazar.
//
// Yol bir sembolik bağlantıysa bağlantının gösterdiği dosya yazılır (bağlantı korunur); var
// olan dosyanın izin bitleri yeni dosyaya aktarılır (yeni dosya 0644). Bedelleri: dosyanın
// başka sabit bağlantıları eski içeriği göstermeye devam eder, sahiplik ve erişim listeleri
// klasörün varsayılanına döner, klasöre yazma izni gerekir.
//
// Hatalar hedef yolu taşır (Türkçe dosya hatası iletisi geçici dosyanın değil kullanıcının
// verdiği yolun nedenini anlatsın).
func AtomikYaz(yol string, yaz func(w io.Writer) error) (err error) {
	hedef, mod, err := atomikHedef(yol)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(hedef), "."+filepath.Base(hedef)+".*.yaziliyor")
	if err != nil {
		return yolHatasi("open", yol, err)
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
	_ = os.Chmod(gecici, mod) // geçici dosya 0600 açılır; eski dosyanın izinleri (yoksa 0644)
	if err = os.Rename(gecici, hedef); err != nil {
		return yolHatasi("rename", yol, err)
	}
	return nil
}

// atomikHedef yazılacak gerçek dosyayı (bağlantı ise gösterdiği yeri) ve izin bitlerini bulur.
func atomikHedef(yol string) (string, fs.FileMode, error) {
	hedef := yol
	if b, e := os.Lstat(yol); e == nil && b.Mode()&fs.ModeSymlink != 0 {
		if g, e := filepath.EvalSymlinks(yol); e == nil {
			hedef = g
		}
	}
	mod := fs.FileMode(0o644)
	if b, e := os.Stat(hedef); e == nil {
		if b.IsDir() {
			return "", 0, &fs.PathError{Op: "open", Path: yol, Err: errors.New("bir klasör")}
		}
		mod = b.Mode().Perm()
	}
	return hedef, mod, nil
}

// yolHatasi işletim sisteminin hatasını kullanıcının verdiği yolla yeniden sarar (geçici
// dosyanın adı mesaja sızmasın).
func yolHatasi(op, yol string, err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return &fs.PathError{Op: op, Path: yol, Err: pe.Err}
	}
	var le *os.LinkError
	if errors.As(err, &le) {
		return &fs.PathError{Op: op, Path: yol, Err: le.Err}
	}
	return err
}
