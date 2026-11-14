package messages

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// systemError işletim sisteminin ya da Go'nun ham hatasını kodlu neden iletisiyle sarar; özgün hata
// Unwrap ile korunur ama Error() ve MessageText metnine yalnız kodlu ileti girer (8.5).
type systemError struct {
	reason *Error
	orig   error
}

func (e *systemError) Error() string { return e.reason.Error() }

// MessageText nedenin lang dilindeki metnidir (Localized).
func (e *systemError) MessageText(lang string) string { return ErrorIn(e.reason, lang) }

func (e *systemError) Unwrap() []error { return []error{e.reason, e.orig} }

// SystemError wraps an operating system or Go I/O error with a coded, language-independent reason
// message so the operating system's raw text never enters a program value (8.5), while keeping the
// original error reachable through Unwrap for errors.Is and errors.As. A coded message that carries
// no system error is returned unchanged; nil returns nil.
func SystemError(path string, err error) error {
	if err == nil {
		return nil
	}
	if _, ok := As(err); ok && !HasSystemError(err) {
		return err
	}
	return &systemError{reason: SystemReason(path, err), orig: err}
}

// SystemReason classifies err into a coded reason message (H2130-H2158, H2628 or H2629). A coded
// message that wraps no operating system error is returned as it is.
func SystemReason(path string, err error) *Error {
	if m, ok := As(err); ok && !HasSystemError(err) {
		return m
	}
	// Denetimler işletim sisteminin gerçekten denediği yolla yapılır (göreli yol programın
	// klasörüne göre çözülmüş olabilir).
	var pe *fs.PathError
	if errors.As(err, &pe) && pe.Path != "" {
		path = pe.Path
	} else if path == "" {
		var le *os.LinkError
		if errors.As(err, &le) {
			if le.New != "" {
				path = le.New
			} else {
				path = le.Old
			}
		}
	}
	// Yol var ve bir klasörse işletim sistemleri bunu farklı biçimlerde bildirir
	// (Windows'ta "Incorrect function", Unix'te EISDIR); hepsi aynı cümleye iner.
	if path != "" {
		if info, e := os.Stat(path); e == nil && info.IsDir() {
			return New(H2130)
		}
	}
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if dir := missingDir(path); dir != "" {
			return New(H2131, dir)
		}
		return New(H2132)
	case errors.Is(err, fs.ErrPermission):
		return New(H2133)
	case errors.Is(err, fs.ErrExist):
		return New(H2134)
	case errors.Is(err, syscall.EISDIR):
		return New(H2130)
	case errors.Is(err, io.ErrUnexpectedEOF):
		return New(H2153)
	}
	var code syscall.Errno
	if errors.As(err, &code) {
		if reason := systemErrorReason(code); reason != 0 {
			return New(reason)
		}
		return New(H2628, uintptr(code)) // sistemin metni değere girmez (8.5)
	}
	return New(H2629) // işletim sistemi dışındaki kendi kodu olmayan öbür hata (8.5)
}

// HasSystemError reports whether err's chain carries an operating system or unclassified I/O error
// (*fs.PathError, *os.LinkError, *os.SyscallError, syscall.Errno, fs sentinel errors or
// io.ErrUnexpectedEOF).
func HasSystemError(err error) bool {
	var se *systemError
	var pe *fs.PathError
	var le *os.LinkError
	var sce *os.SyscallError
	var code syscall.Errno
	return errors.As(err, &se) || errors.As(err, &pe) || errors.As(err, &le) ||
		errors.As(err, &sce) || errors.As(err, &code) ||
		errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) ||
		errors.Is(err, fs.ErrExist) || errors.Is(err, io.ErrUnexpectedEOF)
}

// missingDir yolun klasör kısımlarından ilk olmayanını kullanıcının yazdığı biçimde
// döndürür; klasörler tamamsa (eksik olan dosyanın kendisiyse) "" döner.
func missingDir(path string) string {
	if path == "" {
		return ""
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == path {
		return ""
	}
	if _, err := os.Stat(dir); err == nil {
		return ""
	}
	// En üstteki eksik klasörü bul: "a/b/c.txt" için 'a' yoksa "a" söylenir.
	for {
		parentDir := filepath.Dir(dir)
		if parentDir == "." || parentDir == dir {
			break
		}
		if _, err := os.Stat(parentDir); err == nil {
			break
		}
		dir = parentDir
	}
	return filepath.ToSlash(dir)
}
