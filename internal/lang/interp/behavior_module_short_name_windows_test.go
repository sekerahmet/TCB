//go:build windows

package interp

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// Kural (11.3): Windows'un 8.3 kısa adıyla (UZUN_K~1) ulaşılan modül dosyası uzun adla
// aktarılanla aynı modüldür ("1 2"). Kısa ad mutlak yazımla kurulur: göreli kısa ad diskteki adla
// (11.1) bulunmaz. Kısa ad yalnız 8.3 adları açık birimde oluşur: atlama ölçütü testin kendi
// klasöründen ölçülür (fsutil yönetici ister).
func TestAModuleThroughItsShortNameLoadsOnce(t *testing.T) {
	root := t.TempDir()
	long := filepath.Join(root, "uzun_kitaplik_klasoru")
	writeCounter(t, filepath.Join(long, "sayac.tr"))
	p, _ := syscall.UTF16PtrFromString(long)
	buf := make([]uint16, 1024)
	n, err := syscall.GetShortPathName(p, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 {
		t.Skipf("kısa ad okunamadı: %v", err)
	}
	shortDir := syscall.UTF16ToString(buf[:n])
	if strings.EqualFold(filepath.Base(shortDir), filepath.Base(long)) {
		t.Skipf("bu birimde 8.3 kısa adları kapalı: %q", shortDir)
	}
	second := filepath.ToSlash(filepath.Join(shortDir, "sayac"))
	out, err := runMain(t, root, twoImports("uzun_kitaplik_klasoru/sayac", second), true)
	if err != nil || out != "1\n2\n" {
		t.Fatalf("kısa ad %q uzun adla aynı modül olmalı: çıktı %q, hata %v", second, out, err)
	}
}
