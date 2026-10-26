package main

import (
	"encoding/binary"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural: 'tr derle' paketinin JSON anahtarları dilden bağımsız İngilizce tanımlayıcılardır (dil
// tanımı 17.3); eski (Türkçe anahtarlı, sürüm 2) paket okunmaz, sürümüyle reddedilir.
func TestPackageKeys(t *testing.T) {
	testkit.FormatJSONKeys(t, pkg{})
	body := []byte(`{"ana":{"kod":"yazdır(1)"}}`)
	footer := make([]byte, footerSize)
	binary.LittleEndian.PutUint64(footer[0:8], uint64(len(body)))
	binary.LittleEndian.PutUint32(footer[8:12], 2)
	copy(footer[12:], packageSignature)
	if _, _, found, err := extractPackage(append(body, footer...)); !found || !messages.Has(err, messages.H8751) {
		t.Errorf("sürüm 2 paketi reddedilmeli: bulundu=%v hata=%v", found, err)
	}
}

// Kural: gömülü paketteki modül adı hangi biçimde olursa olsun ("..", "ust/..", ".", boş)
// modül, geçici klasörün modül alt klasörüne yazılır; klasörün kökünün ya da dışının yolu
// olamaz (SECURITY.md: yol geçişi ayıklanır). Neden: filepath.Base ".." verir ve Join onu
// alt klasörden çıkarıp kökü hedef yapar; kök yoksa dosya olarak yazılır, varsa ham işletim
// sistemi hatası döner.
func TestWriteEmbeddedModulesKeepsDotNamesInsideTheModuleFolder(t *testing.T) {
	for _, name := range []string{"..", "ust/..", ".", "", "a/b/.."} {
		pk := &pkg{
			Main:    packageFile{Name: "ana.tr", Code: "değişken a = 1\n"},
			Modules: []packageFile{{Name: name, Code: "değişken x = 1\n"}},
		}
		for _, dir := range []string{t.TempDir(), filepath.Join(t.TempDir(), "paket")} {
			paths, err := writeEmbeddedModules(pk, dir)
			if err != nil {
				t.Errorf("ad %q: beklenmeyen hata: %v", name, err)
				continue
			}
			for _, y := range paths {
				if rel, err := filepath.Rel(dir, y); err != nil || rel == "." || strings.HasPrefix(rel, "..") || !strings.HasPrefix(rel, "0"+string(filepath.Separator)) {
					t.Errorf("ad %q: modül modül alt klasörünün dışına yazıldı: %q (rel=%q)", name, y, rel)
				}
			}
		}
	}
}
