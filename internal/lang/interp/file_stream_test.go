package interp

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// AtomikYaz bağlantının gösterdiği dosyayı yazar (bağlantı korunur) ve var olan dosyanın izin
// bitlerini korur.
func TestAtomikYazBaglantiVeIzin(t *testing.T) {
	k := t.TempDir()
	gercek := filepath.Join(k, "gercek.txt")
	os.WriteFile(gercek, []byte("eski"), 0o600)
	if runtime.GOOS != "windows" {
		if err := WriteAtomic(gercek, func(w io.Writer) error { _, e := w.Write([]byte("yeni")); return e }); err != nil {
			t.Fatal(err)
		}
		if b, _ := os.Stat(gercek); b.Mode().Perm() != 0o600 {
			t.Errorf("izin korunmadı: %v", b.Mode())
		}
	}
	bag := filepath.Join(k, "bag.txt")
	if err := os.Symlink(gercek, bag); err != nil {
		t.Skipf("sembolik bağlantı kurulamadı: %v", err)
	}
	if err := WriteAtomic(bag, func(w io.Writer) error { _, e := w.Write([]byte("bağlantıdan")); return e }); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.Lstat(bag); b.Mode()&os.ModeSymlink == 0 {
		t.Error("bağlantı sıradan dosyayla değiştirildi")
	}
	if b, _ := os.ReadFile(gercek); string(b) != "bağlantıdan" {
		t.Errorf("gösterilen dosya yazılmadı: %q", b)
	}
}

func TestGzipYardimcilari(t *testing.T) {
	k := t.TempDir()
	yol := filepath.Join(k, "a.TXT.GZ")
	if err := WriteFileContent(yol, []byte("merhaba")); err != nil {
		t.Fatal(err)
	}
	if b, err := ReadFileContent(yol); err != nil || string(b) != "merhaba" {
		t.Fatalf("%q %v", b, err)
	}
	// Bozuk ve sıkıştırılmamış içerik Türkçe nedenle reddedilir.
	ham, _ := os.ReadFile(yol)
	bozuk := bytes.Clone(ham)
	bozuk[len(bozuk)-5] ^= 0xff
	os.WriteFile(yol, bozuk, 0o644)
	if _, err := ReadFileContent(yol); err == nil || !strings.Contains(turkish.FileErrorReason(yol, err), "gzip içeriği bozuk") {
		t.Errorf("bozuk gzip: %v", err)
	}
	os.WriteFile(yol, []byte("düz"), 0o644)
	if _, err := ReadFileContent(yol); err == nil || !strings.Contains(err.Error(), "uzantısı .gz ama içerik gzip değil") {
		t.Errorf("düz içerik: %v", err)
	}
	// Açılmış içerik sınırı.
	eski := gzipIcerikSiniri
	gzipIcerikSiniri = 1000
	t.Cleanup(func() { gzipIcerikSiniri = eski })
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	gz.Write(bytes.Repeat([]byte("a"), 5000))
	gz.Close()
	os.WriteFile(yol, b.Bytes(), 0o644)
	if _, err := ReadFileContent(yol); err == nil || !strings.Contains(err.Error(), "1 GB'tan büyük") || !strings.Contains(err.Error(), "dosya_satırları(...)") {
		t.Errorf("sınır: %v", err)
	}
}
