package turkish

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestFileErrorTurkishAndHelpful(t *testing.T) {
	d := t.TempDir()
	missing := filepath.Join(d, "yok.txt")
	_, errNotExist := os.ReadFile(missing)
	_, errDirMissing := os.ReadFile(filepath.Join(d, "a", "b", "c.txt"))
	errWriteDirMissing := os.WriteFile(filepath.Join(d, "a", "c.txt"), nil, 0o644)
	_, errDir := os.ReadFile(d)
	errWriteToDir := os.WriteFile(d, nil, 0o644)

	states := []struct {
		name, op string
		err      error
		wantSub  string
	}{
		{"yok", "okunamadı", errNotExist, "böyle bir dosya ya da klasör yok"},
		{"klasör yok", "okunamadı", errDirMissing, filepath.ToSlash(filepath.Join(d, "a")) + `" klasörü yok`},
		{"yazmada klasör yok", "yazılamadı", errWriteDirMissing, `klasörü yok`},
		{"klasörü okuma", "okunamadı", errDir, "bu bir klasör, dosya değil"},
		{"klasöre yazma", "yazılamadı", errWriteToDir, "bu bir klasör, dosya değil"},
		{"izin", "yazılamadı", &fs.PathError{Op: "open", Path: missing, Err: fs.ErrPermission}, "erişim izni yok"},
	}
	for _, d := range states {
		if d.err == nil {
			t.Fatalf("%s: işletim sistemi hata vermedi", d.name)
		}
		e := FileError("dosya_oku()", "x.txt", d.op, d.err, "ek bilgi")
		m := e.Error()
		if !strings.HasPrefix(m, `dosya_oku(): "x.txt" `+d.op+": ") || !strings.Contains(m, d.wantSub) || !strings.HasSuffix(m, "; ek bilgi") {
			t.Errorf("%s: beklenmeyen mesaj: %s", d.name, m)
		}
		if strings.Contains(m, "cannot") || strings.Contains(m, "system") || strings.Contains(m, "Incorrect") {
			t.Errorf("%s: işletim sisteminin İngilizce metni sızdı: %s", d.name, m)
		}
		if !errors.Is(e, d.err) && !errors.Is(e, errors.Unwrap(d.err)) {
			t.Errorf("%s: özgün hata Unwrap ile korunmadı", d.name)
		}
	}
	if !errors.Is(FileError("", "x", "okunamadı", errNotExist), os.ErrNotExist) {
		t.Error("errors.Is(..., os.ErrNotExist) çalışmalı")
	}
	if FileError("f", "x", "okunamadı", nil) != nil {
		t.Error("nil hata nil kalmalı")
	}
}

func TestFileErrorUnknownStatusCode(t *testing.T) {
	err := &fs.PathError{Op: "open", Path: "x", Err: syscall.Errno(7777)}
	m := FileError("", "x", "okunamadı", err).Error()
	if !strings.HasPrefix(m, `"x" okunamadı: işletim sistemi beklenmeyen bir hata bildirdi (hata kodu 7777`) {
		t.Errorf("beklenmeyen mesaj: %s", m)
	}
	m = FileError("", "x", "okunamadı", errors.New("tuhaf")).Error()
	if m != `"x" okunamadı: beklenmeyen bir hata oluştu (özgün hata: tuhaf)` {
		t.Errorf("beklenmeyen mesaj: %s", m)
	}
}
