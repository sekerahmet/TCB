package turkce

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestDosyaHatasiTurkceVeYolGosterici(t *testing.T) {
	d := t.TempDir()
	yok := filepath.Join(d, "yok.txt")
	_, errYok := os.ReadFile(yok)
	_, errKlasorYok := os.ReadFile(filepath.Join(d, "a", "b", "c.txt"))
	errYazKlasorYok := os.WriteFile(filepath.Join(d, "a", "c.txt"), nil, 0o644)
	_, errKlasor := os.ReadFile(d)
	errKlasoreYaz := os.WriteFile(d, nil, 0o644)

	durumlar := []struct {
		ad, islem string
		err       error
		ister     string
	}{
		{"yok", "okunamadı", errYok, "böyle bir dosya ya da klasör yok"},
		{"klasör yok", "okunamadı", errKlasorYok, filepath.ToSlash(filepath.Join(d, "a")) + `" klasörü yok`},
		{"yazmada klasör yok", "yazılamadı", errYazKlasorYok, `klasörü yok`},
		{"klasörü okuma", "okunamadı", errKlasor, "bu bir klasör, dosya değil"},
		{"klasöre yazma", "yazılamadı", errKlasoreYaz, "bu bir klasör, dosya değil"},
		{"izin", "yazılamadı", &fs.PathError{Op: "open", Path: yok, Err: fs.ErrPermission}, "erişim izni yok"},
	}
	for _, d := range durumlar {
		if d.err == nil {
			t.Fatalf("%s: işletim sistemi hata vermedi", d.ad)
		}
		e := DosyaHatasi("dosya_oku()", "x.txt", d.islem, d.err, "ek bilgi")
		m := e.Error()
		if !strings.HasPrefix(m, `dosya_oku(): "x.txt" `+d.islem+": ") || !strings.Contains(m, d.ister) || !strings.HasSuffix(m, "; ek bilgi") {
			t.Errorf("%s: beklenmeyen mesaj: %s", d.ad, m)
		}
		if strings.Contains(m, "cannot") || strings.Contains(m, "system") || strings.Contains(m, "Incorrect") {
			t.Errorf("%s: işletim sisteminin İngilizce metni sızdı: %s", d.ad, m)
		}
		if !errors.Is(e, d.err) && !errors.Is(e, errors.Unwrap(d.err)) {
			t.Errorf("%s: özgün hata Unwrap ile korunmadı", d.ad)
		}
	}
	if !errors.Is(DosyaHatasi("", "x", "okunamadı", errYok), os.ErrNotExist) {
		t.Error("errors.Is(..., os.ErrNotExist) çalışmalı")
	}
	if DosyaHatasi("f", "x", "okunamadı", nil) != nil {
		t.Error("nil hata nil kalmalı")
	}
}

func TestDosyaHatasiBilinmeyenDurumKoduVerir(t *testing.T) {
	err := &fs.PathError{Op: "open", Path: "x", Err: syscall.Errno(7777)}
	m := DosyaHatasi("", "x", "okunamadı", err).Error()
	if !strings.HasPrefix(m, `"x" okunamadı: işletim sistemi beklenmeyen bir hata bildirdi (hata kodu 7777`) {
		t.Errorf("beklenmeyen mesaj: %s", m)
	}
	m = DosyaHatasi("", "x", "okunamadı", errors.New("tuhaf")).Error()
	if m != `"x" okunamadı: beklenmeyen bir hata oluştu (özgün hata: tuhaf)` {
		t.Errorf("beklenmeyen mesaj: %s", m)
	}
}
