package files

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// deleteBuiltin sil(yol, içindekilerle: yanlış) yerleşiğidir: dosyayı, bağlantıyı ya da boş
// klasörü (içindekilerle: doğru ile dolu klasörü) siler, silinen dosya sayısını verir.
func deleteBuiltin(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.G123)
	if err := bind.CheckNamed(fn, named, nameWithContents); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.G123, "1", len(args), "dosya ya da klasör")
	}
	raw, err := readPath(codes.G123, 1, args[0])
	if err != nil {
		return nil, err
	}
	withContents, err := boolNamed(codes.G123, named, nameWithContents)
	if err != nil {
		return nil, err
	}
	resolved, tried := m.ResolvePath(filepath.FromSlash(raw))
	info, err := os.Lstat(resolved)
	if err != nil {
		return nil, interp.ReadError(fn, raw, messages.H2104, err, tried)
	}
	real := realPath(resolved)
	if reason := deleteProtection(m, real); reason != "" {
		return nil, fmt.Errorf("%s: %q (%s) %s; TR bunu silmez", fn, raw, filepath.ToSlash(real), reason)
	}
	if err := openFileError(fn, raw, real); err != nil {
		return nil, err
	}
	if info.IsDir() && !withContents {
		empty, err := isEmptyDir(resolved)
		if err != nil {
			return nil, turkish.FileError(fn, raw, messages.H2104, err)
		}
		if !empty {
			return nil, fmt.Errorf("%s: %q klasörü boş değil; içindekilerle birlikte silmek için %s doğru verin", fn, raw, nameWithContents.TR+":")
		}
	}
	var number int
	m.Blocking(func() { number, err = deleteTree(resolved, info) })
	if err != nil {
		return nil, turkish.FileError(fn, raw, messages.H2104, err)
	}
	return interp.NewInt(int64(number)), nil
}

func isEmptyDir(fpath string) (bool, error) {
	f, err := os.Open(fpath)
	if err != nil {
		return false, err
	}
	defer f.Close()
	names, err := f.Readdirnames(1)
	if len(names) > 0 {
		return false, nil
	}
	if err == nil || errors.Is(err, io.EOF) {
		return true, nil
	}
	return false, err
}

// readOnlyError salt okunur bir dosyanın silinmediğini bildirir. Salt okunur dosyalar zorla
// silinmez (Windows'ta işletim sisteminin silme çağrısı salt okunur bayrağını kendisi
// kaldırırdı); klasörde böyle bir dosya varsa hiçbir şey silinmez.
func readOnlyError(fpath, inner string) error {
	reason := turkish.NewReason(messages.H7103)
	if inner != "" {
		reason = turkish.NewReason(messages.H7104, inner)
	}
	return &fs.PathError{Op: "remove", Path: fpath, Err: reason}
}

// deleteTree yolu siler; klasörse içindekilerle birlikte. Bağlantıların gösterdiği yere
// dokunulmaz: klasörün içi os.Root ile gezilir ve silinir, gezinme sırasında bir alt klasör
// bağlantıyla değiştirilse de işlem klasörün dışına çıkmaz. Önce bütün ağaç denetlenir:
// salt okunur bir dosya varsa hiçbir şey silinmez. Silinen dosya (klasör olmayan girdi)
// sayısını verir.
func deleteTree(fpath string, info fs.FileInfo) (int, error) {
	if !info.IsDir() {
		if readOnly(info) {
			return 0, readOnlyError(fpath, "")
		}
		return 1, os.Remove(fpath)
	}
	root, err := os.OpenRoot(fpath)
	if err != nil {
		return 0, err
	}
	number, parents, err := checkTree(root, fpath)
	if err == nil {
		for _, name := range parents {
			if err = root.RemoveAll(name); err != nil {
				break
			}
		}
	}
	root.Close()
	if err != nil {
		return 0, err
	}
	return number, os.Remove(fpath)
}

// checkDeletable deleteTree'nin yolu silebileceğini (salt okunur dosya olmadığını) önceden
// denetler; hiçbir şeyi değiştirmez.
func checkDeletable(fpath string) error {
	info, err := os.Lstat(fpath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		if readOnly(info) {
			return readOnlyError(fpath, "")
		}
		return nil
	}
	root, err := os.OpenRoot(fpath)
	if err != nil {
		return err
	}
	defer root.Close()
	_, _, err = checkTree(root, fpath)
	return err
}

// checkTree kökün altını (bağlantıların içine girmeden) gezer: klasör olmayan girdileri
// sayar, salt okunur dosya arar ve kökün doğrudan altındaki adları verir.
func checkTree(root *os.Root, fpath string) (int, []string, error) {
	number := 0
	var parents []string
	err := fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "." {
			return nil
		}
		if filepath.Dir(filepath.FromSlash(p)) == "." {
			parents = append(parents, p)
		}
		if d.IsDir() {
			return nil
		}
		number++
		if d.Type().IsRegular() {
			b, err := d.Info()
			if err != nil {
				return err
			}
			if readOnly(b) {
				return readOnlyError(fpath, p)
			}
		}
		return nil
	})
	return number, parents, err
}

// readOnly sıradan bir dosya yazmaya kapalı mı? Windows'ta salt okunur bayrağı, öteki
// sistemlerde sahibin yazma izni (kabukta da silmeden önce onay sorulan durum).
func readOnly(b fs.FileInfo) bool {
	return b.Mode().IsRegular() && b.Mode().Perm()&0o200 == 0
}
