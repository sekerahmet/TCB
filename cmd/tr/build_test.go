package main

import (
	"path/filepath"
	"strings"
	"testing"
)

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
