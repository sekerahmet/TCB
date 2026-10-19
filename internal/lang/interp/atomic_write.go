package interp

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// WriteAtomic writes the content produced by yaz to yol atomically: the content first goes
// to a temporary file in the same directory, is flushed to disk (Sync), and the file is
// renamed over the old one. If writing stops halfway (error, full disk, crash) the existing
// file stays intact; on error the temporary file is removed. dosya_yaz, tensor saving and the
// checkpoints of long training runs write through it.
//
// If the path is a symbolic link, the file it points to is written (the link is kept); the
// permission bits of an existing file carry over to the new one (new files get 0644). The
// costs: other hard links of the file keep showing the old content, ownership and ACLs fall
// back to the directory's defaults, and the directory must be writable.
//
// Errors carry the target path (so the file error message explains the user's path, not the
// temporary file's).
func WriteAtomic(yol string, yaz func(w io.Writer) error) (err error) {
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
