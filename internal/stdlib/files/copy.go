package files

import (
	"errors"
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
		m.WriteOutput(m.Err, messages.Text(messages.H7106, job.fn, len(job.skipped), job.skipped[0])+"\n")
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
		return nil, interp.ArgCountError(kd, "2", len(args), messages.Text(messages.H2316))
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
		return messages.New(messages.H7107, job.fn, job.srcRaw)
	case !b.IsDir() && !b.Mode().IsRegular() && !isLink(b.Mode()):
		return messages.New(messages.H7108, job.fn, job.srcRaw)
	}
	if job.move {
		if err := openFileError(job.fn, job.srcRaw, realPath(resolved)); err != nil {
			return err
		}
		if reason := deleteProtection(m, realPath(resolved)); reason != nil {
			return messages.New(messages.H7109, job.fn, job.srcRaw, filepath.ToSlash(realPath(resolved)), reason)
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
		return messages.New(messages.H7110, job.fn, targetRaw, codes.DisplayName(codes.G123))
	case err == nil && wantDir:
		return messages.New(messages.H7111, job.fn, targetRaw)
	case err != nil && !bind.IsNotExist(err):
		return turkish.FileError(job.fn, targetRaw, messages.H2103, err)
	case err != nil && wantDir:
		return messages.New(messages.H7112, job.fn, targetRaw, codes.DisplayName(codes.G122), strings.TrimRight(targetRaw, `/\`))
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
		return messages.New(messages.H7113, job.fn, job.lastRaw)
	}
	if job.srcInfo.IsDir() && isInside(gs, gk) {
		return messages.New(messages.H7114, job.fn, job.srcRaw, job.lastRaw)
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
		return messages.New(messages.H7115, job.fn, job.lastRaw, nameOverwrite.TR+":")
	case isLink(v.Mode()):
		return messages.New(messages.H7116, job.fn, job.lastRaw, codes.DisplayName(codes.G123))
	case v.IsDir() && !job.srcInfo.IsDir():
		return messages.New(messages.H7117, job.fn, job.lastRaw)
	case !v.IsDir() && job.srcInfo.IsDir():
		return messages.New(messages.H7118, job.fn, job.lastRaw)
	case v.IsDir():
		if isInside(gk, gs) {
			return messages.New(messages.H7119, job.fn, job.lastRaw)
		}
		if reason := deleteProtection(m, gs); reason != nil {
			return messages.New(messages.H7120, job.fn, job.lastRaw, filepath.ToSlash(gs), reason)
		}
		job.existingDir = true
	}
	return nil
}
