package interp

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Bulgu 9: kayıt atomiktir: içerik önce aynı klasörde geçici dosyaya yazılır, sonra yerine
// konur. Yazma yarıda hata verirse eski dosya bozulmaz ve geçici dosya kalmaz.
func TestWriteAtomic(t *testing.T) {
	folder := t.TempDir()
	filePath := filepath.Join(folder, "model.trt")
	if err := os.WriteFile(filePath, []byte("eski"), 0o644); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("disk doldu")
	err := WriteAtomic(filePath, func(w io.Writer) error {
		w.Write([]byte("yarım"))
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("hata: %v", err)
	}
	if b, _ := os.ReadFile(filePath); string(b) != "eski" {
		t.Fatalf("yarım yazma eski dosyayı bozdu: %q", b)
	}
	if g, _ := reporoot.ReadDir(folder); len(g) != 1 {
		t.Fatalf("geçici dosya kaldı: %v", g)
	}
	if err := WriteAtomic(filePath, func(w io.Writer) error { _, e := w.Write([]byte("yeni")); return e }); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filePath); string(b) != "yeni" {
		t.Fatalf("içerik: %q", b)
	}
	if g, _ := reporoot.ReadDir(folder); len(g) != 1 {
		t.Fatalf("geçici dosya kaldı: %v", g)
	}
}
