package ogrenme

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// Bulgu 9: kayıt atomiktir: içerik önce aynı klasörde geçici dosyaya yazılır, sonra yerine
// konur. Yazma yarıda hata verirse eski dosya bozulmaz ve geçici dosya kalmaz.
func TestAtomikYaz(t *testing.T) {
	klasor := t.TempDir()
	yol := filepath.Join(klasor, "model.trt")
	if err := os.WriteFile(yol, []byte("eski"), 0o644); err != nil {
		t.Fatal(err)
	}
	hata := errors.New("disk doldu")
	err := atomikYaz(yol, func(w io.Writer) error {
		w.Write([]byte("yarım"))
		return hata
	})
	if !errors.Is(err, hata) {
		t.Fatalf("hata: %v", err)
	}
	if b, _ := os.ReadFile(yol); string(b) != "eski" {
		t.Fatalf("yarım yazma eski dosyayı bozdu: %q", b)
	}
	if g, _ := os.ReadDir(klasor); len(g) != 1 {
		t.Fatalf("geçici dosya kaldı: %v", g)
	}
	if err := atomikYaz(yol, func(w io.Writer) error { _, e := w.Write([]byte("yeni")); return e }); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(yol); string(b) != "yeni" {
		t.Fatalf("içerik: %q", b)
	}
	if g, _ := os.ReadDir(klasor); len(g) != 1 {
		t.Fatalf("geçici dosya kaldı: %v", g)
	}
}
