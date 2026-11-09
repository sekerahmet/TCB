//go:build windows

package httpserver

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestFileShortNameIsRefused: Windows'un kısa adı (INDEX~1.HTM) klasörün listesinde olmadığı için
// bulunmaz, 404'tür (28.3a). Kısa ad yalnız 8.3 adları açık birimde oluşur: atlama ölçütü testin kendi
// dosyasından ölçülür (fsutil yönetici ister).
func TestFileShortNameIsRefused(t *testing.T) {
	root := t.TempDir()
	long := filepath.Join(root, "uzun_dosya_adi.html")
	if err := os.WriteFile(long, []byte("<p>"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ := syscall.UTF16PtrFromString(long)
	buf := make([]uint16, 512)
	n, err := syscall.GetShortPathName(p, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 {
		t.Skipf("kısa ad okunamadı: %v", err)
	}
	short := filepath.Base(syscall.UTF16ToString(buf[:n]))
	if strings.EqualFold(short, filepath.Base(long)) {
		t.Skipf("bu birimde 8.3 kısa adları kapalı: %q", short)
	}
	if _, err := os.Stat(filepath.Join(root, short)); err != nil {
		t.Fatalf("Windows kısa adla açmalı (yoksa test bir şey sınamıyor): %v", err)
	}
	if r := serveFile(os.ReadDir, root, "/"+short); r.status != 404 {
		t.Fatalf("kısa ad %q 404 almalı: %d", short, r.status)
	}
	if r := serveFile(os.ReadDir, root, "/uzun_dosya_adi.html"); r.status != 200 {
		t.Fatalf("uzun ad 200 almalı: %d", r.status)
	}
}
