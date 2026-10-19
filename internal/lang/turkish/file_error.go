package turkish

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// fileError işletim sisteminin dosya hatasını Türkçe anlatır. Özgün hata Unwrap ile
// korunur: errors.Is(err, os.ErrNotExist) gibi denetimler çalışmaya devam eder.
type fileError struct {
	message string
	orig    error
}

func (h *fileError) Error() string { return h.message }
func (h *fileError) Unwrap() error { return h.orig }

// Reason is a file error carrying its own Turkish reason sentence (cases that do not come
// from the operating system: "uzantısı .gz ama içerik gzip değil", "salt okunur dosya" ...).
// FileError writes it as is.
type Reason string

func (n Reason) Error() string { return string(n) }

// FileError turns the error of a file operation into a Turkish message for the user; the
// operating system's English text ("The system cannot find the file specified") never
// reaches the user. The message is a single line:
//
//	<fn>: "<yol>" <islem>: <neden>[; <ek>...]
//
// fn is the builtin that failed (e.g. "dosya_oku()"; no prefix when empty), yol is the
// path as the user wrote it, islem is the outcome such as "okunamadı" or "yazılamadı".
// ekler are appended after the reason with "; " (e.g. "bakılan yerler: ..."). Known cases:
// file or folder missing, permission denied, path is a folder, disk full, file in use,
// invalid name, path too long, write-protected disk. Unknown cases get a Turkish lead
// sentence followed by the operating system's error code and original text (so it can be
// searched).
//
// Returns nil when err is nil. Standard library packages (tensör_kaydet/yükle,
// parçalayıcı_yükle, veri_kümesi) must report file errors through it as well.
func FileError(fn, path, op string, err error, suffixes ...string) error {
	if err == nil {
		return nil
	}
	var sb strings.Builder
	if fn != "" {
		sb.WriteString(fn)
		sb.WriteString(": ")
	}
	fmt.Fprintf(&sb, "%q %s: %s", path, op, FileErrorReason(path, err))
	for _, suffix := range suffixes {
		if suffix != "" {
			sb.WriteString("; ")
			sb.WriteString(suffix)
		}
	}
	return &fileError{message: sb.String(), orig: err}
}

// FileErrorReason returns only the Turkish reason sentence of the error (e.g. "böyle bir
// dosya ya da klasör yok"). For callers that build their own message.
func FileErrorReason(path string, err error) string {
	if err == nil {
		return ""
	}
	var n Reason
	if errors.As(err, &n) {
		return string(n)
	}
	// Denetimler işletim sisteminin gerçekten denediği yolla yapılır (göreli yol programın
	// klasörüne göre çözülmüş olabilir).
	var pe *fs.PathError
	if errors.As(err, &pe) && pe.Path != "" {
		path = pe.Path
	}
	// Yol var ve bir klasörse işletim sistemleri bunu farklı biçimlerde bildirir
	// (Windows'ta "Incorrect function", Unix'te EISDIR); hepsi aynı cümleye iner.
	if path != "" {
		if info, e := os.Stat(path); e == nil && info.IsDir() {
			return "bu bir klasör, dosya değil (klasörün içindeki bir dosyanın yolunu verin)"
		}
	}
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if dir := missingDir(path); dir != "" {
			return fmt.Sprintf("%q klasörü yok", dir)
		}
		return "böyle bir dosya ya da klasör yok (yolun yazımını denetleyin)"
	case errors.Is(err, fs.ErrPermission):
		return "erişim izni yok (dosya salt okunur, korumalı bir klasörde ya da başka bir kullanıcıya ait olabilir)"
	case errors.Is(err, fs.ErrExist):
		return "bu adda bir dosya zaten var"
	case errors.Is(err, syscall.EISDIR):
		return "bu bir klasör, dosya değil (klasörün içindeki bir dosyanın yolunu verin)"
	}
	var code syscall.Errno
	if errors.As(err, &code) {
		if reason := systemErrorReason(code); reason != "" {
			return reason
		}
		return fmt.Sprintf("işletim sistemi beklenmeyen bir hata bildirdi (hata kodu %d: %s)", uintptr(code), code.Error())
	}
	orig := err.Error()
	if pe != nil {
		orig = pe.Err.Error()
	}
	return fmt.Sprintf("beklenmeyen bir hata oluştu (özgün hata: %s)", orig)
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
