package turkish

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
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
		name    string
		op      messages.Code
		err     error
		wantSub string
	}{
		{"yok", messages.H2100, errNotExist, "böyle bir dosya ya da klasör yok"},
		{"klasör yok", messages.H2100, errDirMissing, filepath.ToSlash(filepath.Join(d, "a")) + `" klasörü yok`},
		{"yazmada klasör yok", messages.H2101, errWriteDirMissing, `klasörü yok`},
		{"klasörü okuma", messages.H2100, errDir, "bu bir klasör, dosya değil"},
		{"klasöre yazma", messages.H2101, errWriteToDir, "bu bir klasör, dosya değil"},
		{"izin", messages.H2101, &fs.PathError{Op: "open", Path: missing, Err: fs.ErrPermission}, "erişim izni yok"},
	}
	for _, d := range states {
		if d.err == nil {
			t.Fatalf("%s: işletim sistemi hata vermedi", d.name)
		}
		e := FileError("dosya_oku()", "x.txt", d.op, d.err, messages.New(messages.H2030, "y.txt"))
		m := e.Error()
		if !strings.HasPrefix(m, "dosya_oku(): ") || !strings.Contains(m, `"x.txt" `) || !messages.Has(e, d.op) || !strings.Contains(m, d.wantSub) || !strings.HasSuffix(m, "; "+messages.Text(messages.H2030, "y.txt")) {
			t.Errorf("%s: beklenmeyen mesaj: %s", d.name, m)
		}
		// İleti de ekleri de iletinin dilinde (messages.Localized; 4D denetimi, bulgu 1).
		if en := messages.ErrorIn(e, "en"); !strings.HasSuffix(en, "; "+messages.TextIn("en", messages.H2030, "y.txt")) || en == m {
			t.Errorf("%s: İngilizce metin: %s", d.name, en)
		}
		if strings.Contains(m, "cannot") || strings.Contains(m, "system") || strings.Contains(m, "Incorrect") {
			t.Errorf("%s: işletim sisteminin İngilizce metni sızdı: %s", d.name, m)
		}
		if !errors.Is(e, d.err) && !errors.Is(e, errors.Unwrap(d.err)) {
			t.Errorf("%s: özgün hata Unwrap ile korunmadı", d.name)
		}
	}
	if !errors.Is(FileError("", "x", messages.H2100, errNotExist), os.ErrNotExist) {
		t.Error("errors.Is(..., os.ErrNotExist) çalışmalı")
	}
	if FileError("f", "x", messages.H2100, nil) != nil {
		t.Error("nil hata nil kalmalı")
	}
	// İletinin, işlemin ve nedenin kodları hata zincirinde bulunur.
	e := FileError("f", "x", messages.H2100, errNotExist)
	if !messages.Has(e, messages.H2000) || !messages.Has(e, messages.H2100) || !messages.Has(e, messages.H2132) {
		t.Errorf("kodlar zincirde bulunmalı: %v", e)
	}
	if r := FileError("", "x", messages.H2100, NewReason(messages.H2042)); !messages.Has(r, messages.H2042) || !strings.HasSuffix(r.Error(), "gzip başlığı bozuk") {
		t.Errorf("Reason nedeni: %v", r)
	}
	// Kodlu bir hata da neden olarak olduğu gibi kullanılır ("beklenmeyen bir hata" değil).
	if r := FileError("", "x", messages.H2100, messages.New(messages.H7004, "bozuk")); !messages.Has(r, messages.H7004) || !strings.HasSuffix(r.Error(), "okunamadı: CSV okuma hatası: bozuk") {
		t.Errorf("kodlu neden: %v", r)
	}
}

func TestFileErrorUnknownStatusCode(t *testing.T) {
	err := &fs.PathError{Op: "open", Path: "x", Err: syscall.Errno(7777)}
	m := FileError("", "x", messages.H2100, err).Error()
	if !strings.HasPrefix(m, `"x" okunamadı: işletim sistemi beklenmeyen bir hata bildirdi (hata kodu 7777`) {
		t.Errorf("beklenmeyen mesaj: %s", m)
	}
	m = FileError("", "x", messages.H2100, errors.New("tuhaf")).Error()
	if m != `"x" okunamadı: beklenmeyen bir hata oluştu (özgün hata: tuhaf)` {
		t.Errorf("beklenmeyen mesaj: %s", m)
	}
}

// Kodlu bir sarmalayıcı işletim sistemi hatasını taşıyorsa neden sarmalayıcı değil işletim
// sistemi hatasıdır; işletim sistemi hatası yoksa kodlu ileti nedendir.
func TestFileErrorReasonThroughCodedWrapper(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "yok.npy")
	_, osErr := os.ReadFile(missing)
	wrapped := messages.New(messages.H2000, "x", osErr)
	if got := FileErrorReason(missing, wrapped); got != FileErrorReason(missing, osErr) {
		t.Errorf("sarmalayıcı işletim sistemi nedenini gizledi: %q", got)
	}
	if err := FileError("f()", missing, messages.H2100, wrapped); !messages.Has(err, messages.H2132) {
		t.Errorf("neden H2132 olmalı: %v", err)
	}
	coded := messages.New(messages.H2000, "digest.json çok büyük", "")
	if err := FileError("f()", missing, messages.H2100, coded); !messages.Has(err, messages.H2000) || messages.Has(err, messages.H2132) {
		t.Errorf("OS hatası olmayan kodlu ileti neden sayılmalı: %v", err)
	}
}
