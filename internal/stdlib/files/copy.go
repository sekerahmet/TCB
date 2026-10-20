package files

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// copyBuiltin kopyala(kaynak, hedef, üzerine_yaz: yanlış) yerleşiğidir.
func copyBuiltin(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	return doMove(m, codes.G124, args, named, false)
}

// moveBuiltin taşı(kaynak, hedef, üzerine_yaz: yanlış) yerleşiğidir.
func moveBuiltin(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	return doMove(m, codes.G125, args, named, true)
}

// moveJob bir kopyalama ya da taşımanın denetlenmiş hâlidir; Blocking içinde yapılır.
type moveJob struct {
	fn          string
	move        bool
	srcRaw      string
	src         string // çözülmüş
	srcInfo     fs.FileInfo
	last        string // çözülmüş son hedef
	lastRaw     string // kullanıcının yazımıyla son hedef
	existingDir bool   // son hedef var olan bir klasör (üzerine_yaz ile yerine konacak)
	skipped     []string
}

func doMove(m *interp.VM, kd codes.Code, args []interp.Value, named map[string]interp.Value, move bool) (interp.Value, error) {
	job, err := prepareMove(m, kd, args, named, move)
	if err != nil {
		return nil, err
	}
	m.Blocking(func() { err = job.run() })
	if len(job.skipped) > 0 {
		m.WriteOutput(m.Err, fmt.Sprintf("Uyarı: %s: %d bağlantı ya da özel dosya kopyalanmadı (ilki: %q); TR bağlantıları izlemez\n", job.fn, len(job.skipped), job.skipped[0]))
	}
	if err != nil {
		op := messages.H2110 // kopyalanamadı
		if move {
			op = messages.H2111 // taşınamadı
		}
		return nil, turkish.FileError(job.fn, job.srcRaw, op, err)
	}
	return NewPath(job.lastRaw), nil
}

// prepareMove argümanları okur, kaynağı ve son hedefi bulur, güvenlik kurallarını denetler.
func prepareMove(m *interp.VM, kd codes.Code, args []interp.Value, named map[string]interp.Value, move bool) (*moveJob, error) {
	fn := interp.CallPrefix(kd)
	if err := bind.CheckNamed(fn, named, nameOverwrite); err != nil {
		return nil, err
	}
	if len(args) != 2 {
		return nil, interp.ArgCountError(kd, "2", len(args), "kaynak, hedef")
	}
	srcRaw, err := readPath(kd, 1, args[0])
	if err != nil {
		return nil, err
	}
	targetRaw, err := readPath(kd, 2, args[1])
	if err != nil {
		return nil, err
	}
	overwrite, err := boolNamed(kd, named, nameOverwrite)
	if err != nil {
		return nil, err
	}
	job := &moveJob{fn: fn, move: move, srcRaw: srcRaw}
	if err := job.findSource(m); err != nil {
		return nil, err
	}
	if err := job.findTarget(m, targetRaw, dirWanted(args[1])); err != nil {
		return nil, err
	}
	if err := job.checkConflict(m, overwrite); err != nil {
		return nil, err
	}
	return job, nil
}

// dirWanted hedef metni / ile bitiyor mu ("yedek/": var olan bir klasörün içine)?
func dirWanted(d interp.Value) bool {
	s, ok := d.(*interp.StringVal)
	if !ok {
		return false
	}
	return strings.HasSuffix(s.Val, "/") || (runtime.GOOS == "windows" && strings.HasSuffix(s.Val, `\`))
}

func (job *moveJob) findSource(m *interp.VM) error {
	resolved, tried := m.ResolvePath(filepath.FromSlash(job.srcRaw))
	b, err := os.Lstat(resolved)
	if err != nil {
		return interp.ReadError(job.fn, job.srcRaw, messages.H2113, err, tried)
	}
	switch {
	case isLink(b.Mode()) && !job.move:
		return fmt.Errorf("%s: %q bir bağlantı; TR bağlantıları izlemez ve kopyalamaz (bağlantının gösterdiği yolu verin)", job.fn, job.srcRaw)
	case !b.IsDir() && !b.Mode().IsRegular() && !isLink(b.Mode()):
		return fmt.Errorf("%s: %q sıradan bir dosya ya da klasör değil (aygıt, boru ...); kopyalanmaz", job.fn, job.srcRaw)
	}
	if job.move {
		if err := openFileError(job.fn, job.srcRaw, realPath(resolved)); err != nil {
			return err
		}
		if reason := deleteProtection(m, realPath(resolved)); reason != "" {
			return fmt.Errorf("%s: %q (%s) %s; TR bunu yerinden kaldırmaz", job.fn, job.srcRaw, filepath.ToSlash(realPath(resolved)), reason)
		}
	}
	job.src, job.srcInfo = resolved, b
	return nil
}

// findTarget son hedefi bulur: hedef var olan bir klasörse kaynak onun içine aynı adla gider.
func (job *moveJob) findTarget(m *interp.VM, targetRaw string, wantDir bool) error {
	target, _ := m.ResolvePath(filepath.FromSlash(targetRaw))
	b, err := os.Lstat(target)
	switch {
	case err == nil && b.IsDir():
		name := filepath.Base(job.src)
		job.last, job.lastRaw = filepath.Join(target, name), path.Join(clean(targetRaw), name)
		return nil
	case err == nil && isLink(b.Mode()):
		return fmt.Errorf("%s: hedef %q bir bağlantı; TR bağlantıların gösterdiği yere yazmaz (önce %s(...) ile kaldırın)", job.fn, targetRaw, codes.Name(codes.G123))
	case err == nil && wantDir:
		return fmt.Errorf("%s: hedef %q bir dosya, klasör değil", job.fn, targetRaw)
	case err != nil && !bind.IsNotExist(err):
		return turkish.FileError(job.fn, targetRaw, messages.H2103, err)
	case err != nil && wantDir:
		return fmt.Errorf("%s: hedef klasör %q yok; önce %s(%q) ile oluşturun", job.fn, targetRaw, codes.Name(codes.G122), strings.TrimRight(targetRaw, `/\`))
	}
	if _, err := os.Stat(filepath.Dir(target)); err != nil {
		var pe *fs.PathError
		if errors.As(err, &pe) {
			err = &fs.PathError{Op: pe.Op, Path: target, Err: pe.Err}
		}
		return turkish.FileError(job.fn, targetRaw, messages.H2101, err)
	}
	job.last, job.lastRaw = target, targetRaw
	return nil
}

// checkConflict son hedef zaten varsa üzerine_yaz'ı ve türlerin uyuşmasını, klasörün kendi
// içine kopyalanmamasını denetler.
func (job *moveJob) checkConflict(m *interp.VM, overwrite bool) error {
	gk, gs := realPath(job.src), realPath(job.last)
	if samePath(gk, gs) {
		return fmt.Errorf("%s: kaynak ile hedef aynı (%q)", job.fn, job.lastRaw)
	}
	if job.srcInfo.IsDir() && isInside(gs, gk) {
		return fmt.Errorf("%s: %q klasörü kendi içine (%q) konamaz", job.fn, job.srcRaw, job.lastRaw)
	}
	v, err := os.Lstat(job.last)
	if err != nil {
		return nil
	}
	if err := openFileError(job.fn, job.lastRaw, gs); err != nil {
		return err
	}
	switch {
	case !overwrite:
		return fmt.Errorf("%s: hedef %q zaten var; yerine koymak için %s doğru verin", job.fn, job.lastRaw, nameOverwrite.TR+":")
	case isLink(v.Mode()):
		return fmt.Errorf("%s: hedef %q bir bağlantı; TR bağlantıların yerine yazmaz (önce %s(...) ile kaldırın)", job.fn, job.lastRaw, codes.Name(codes.G123))
	case v.IsDir() && !job.srcInfo.IsDir():
		return fmt.Errorf("%s: hedef %q bir klasör, kaynak bir dosya; dosya bir klasörün yerine konmaz", job.fn, job.lastRaw)
	case !v.IsDir() && job.srcInfo.IsDir():
		return fmt.Errorf("%s: hedef %q bir dosya, kaynak bir klasör; klasör bir dosyanın yerine konmaz", job.fn, job.lastRaw)
	case v.IsDir():
		if isInside(gk, gs) {
			return fmt.Errorf("%s: hedef %q kaynağı içeriyor; yerine konamaz", job.fn, job.lastRaw)
		}
		if reason := deleteProtection(m, gs); reason != "" {
			return fmt.Errorf("%s: hedef %q (%s) %s; TR onun yerine bir şey koymaz", job.fn, job.lastRaw, filepath.ToSlash(gs), reason)
		}
		job.existingDir = true
	}
	return nil
}
