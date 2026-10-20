package files

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// tempDirs bu sürecin geçici_klasör() ile oluşturduğu klasörlerdir; program bitince
// (CloseAll) içindekilerle silinirler. VM, silme korumasının programın ve modüllerin
// klasörlerini bilmesi içindir (deleteProtection).
var tempDirs = struct {
	sync.Mutex
	l []tempDir
}{}

type tempDir struct {
	fpath string // oluşturulduğu mutlak yol
	m     *interp.VM
}

// tempDirBuiltin geçici_klasör() yerleşiğidir: işletim sisteminin geçici klasörünün
// altında bu süreç için yeni, boş ve yalnızca kullanıcıya açık (0700) bir klasör oluşturur ve
// Yol olarak döndürür.
func tempDirBuiltin(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.G126)
	if len(args) != 0 {
		return nil, interp.ArgCountError(codes.G126, "0", len(args))
	}
	parent := os.TempDir()
	fpath, err := os.MkdirTemp(parent, fmt.Sprintf("tr_%d_*", os.Getpid()))
	if err != nil {
		return nil, turkish.FileError(fn, filepath.ToSlash(parent), messages.H2106, err)
	}
	if absPath, err := filepath.Abs(fpath); err == nil {
		fpath = absPath
	}
	tempDirs.Lock()
	tempDirs.l = append(tempDirs.l, tempDir{fpath: fpath, m: m})
	tempDirs.Unlock()
	return NewPath(filepath.ToSlash(fpath)), nil
}

// deleteTempDirs geçici_klasör() ile oluşturulan klasörleri içindekilerle siler (program
// bitince; açık satır okuyucuları önce kapatılmış olmalıdır). sil'in kuralları geçerlidir:
// korunan bir yer silinmez, bağlantıların gösterdiği yere dokunulmaz, salt okunur bir dosya
// bulunan klasör silinmez. Silinemeyen klasörler için Türkçe bir uyarı döner.
func deleteTempDirs() []error {
	tempDirs.Lock()
	list := tempDirs.l
	tempDirs.l = nil
	tempDirs.Unlock()
	var errs []error
	fn := interp.CallPrefix(codes.G126)
	for _, g := range list {
		info, err := os.Lstat(g.fpath)
		if errors.Is(err, fs.ErrNotExist) {
			continue // program klasörü kendisi silmiş ya da taşımış
		}
		raw := filepath.ToSlash(g.fpath)
		if err != nil {
			errs = append(errs, turkish.FileError(fn, raw, messages.H2104, err))
			continue
		}
		if reason := deleteProtection(g.m, realPath(g.fpath)); reason != "" {
			errs = append(errs, fmt.Errorf("%s: %q %s; TR bunu silmez", fn, raw, reason))
			continue
		}
		if _, err := deleteTree(g.fpath, info); err != nil {
			errs = append(errs, turkish.FileError(fn, raw, messages.H2104, err))
		}
	}
	return errs
}
