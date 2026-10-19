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
func TestWriteAtomicSymlinkAndPerm(t *testing.T) {
	k := t.TempDir()
	actual := filepath.Join(k, "gercek.txt")
	os.WriteFile(actual, []byte("eski"), 0o600)
	if runtime.GOOS != "windows" {
		if err := WriteAtomic(actual, func(w io.Writer) error { _, e := w.Write([]byte("yeni")); return e }); err != nil {
			t.Fatal(err)
		}
		if b, _ := os.Stat(actual); b.Mode().Perm() != 0o600 {
			t.Errorf("izin korunmadı: %v", b.Mode())
		}
	}
	link := filepath.Join(k, "bag.txt")
	if err := os.Symlink(actual, link); err != nil {
		t.Skipf("sembolik bağlantı kurulamadı: %v", err)
	}
	if err := WriteAtomic(link, func(w io.Writer) error { _, e := w.Write([]byte("bağlantıdan")); return e }); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.Lstat(link); b.Mode()&os.ModeSymlink == 0 {
		t.Error("bağlantı sıradan dosyayla değiştirildi")
	}
	if b, _ := os.ReadFile(actual); string(b) != "bağlantıdan" {
		t.Errorf("gösterilen dosya yazılmadı: %q", b)
	}
}

func TestGzipHelpers(t *testing.T) {
	k := t.TempDir()
	filePath := filepath.Join(k, "a.TXT.GZ")
	if err := WriteFileContent(filePath, []byte("merhaba")); err != nil {
		t.Fatal(err)
	}
	if b, err := ReadFileContent(filePath); err != nil || string(b) != "merhaba" {
		t.Fatalf("%q %v", b, err)
	}
	// Bozuk ve sıkıştırılmamış içerik Türkçe nedenle reddedilir.
	rawStr, _ := os.ReadFile(filePath)
	broken := bytes.Clone(rawStr)
	broken[len(broken)-5] ^= 0xff
	os.WriteFile(filePath, broken, 0o644)
	if _, err := ReadFileContent(filePath); err == nil || !strings.Contains(turkish.FileErrorReason(filePath, err), "gzip içeriği bozuk") {
		t.Errorf("bozuk gzip: %v", err)
	}
	os.WriteFile(filePath, []byte("düz"), 0o644)
	if _, err := ReadFileContent(filePath); err == nil || !strings.Contains(err.Error(), "uzantısı .gz ama içerik gzip değil") {
		t.Errorf("düz içerik: %v", err)
	}
	// Açılmış içerik sınırı.
	prevVal := gzipContentLimit
	gzipContentLimit = 1000
	t.Cleanup(func() { gzipContentLimit = prevVal })
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	gz.Write(bytes.Repeat([]byte("a"), 5000))
	gz.Close()
	os.WriteFile(filePath, b.Bytes(), 0o644)
	if _, err := ReadFileContent(filePath); err == nil || !strings.Contains(err.Error(), "1 GB'tan büyük") || !strings.Contains(err.Error(), "dosya_satırları(...)") {
		t.Errorf("sınır: %v", err)
	}
}
