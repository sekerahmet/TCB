package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// Betiğe dosya adından sonra verilen argümanlar programa iletilir (sessizce yok sayılmaz).
// Program onları okumazsa (argümanları komut_satırı okur) çıkışta standart hataya uyarı
// yazılır; çıkış kodu değişmez, standart çıktıya karışmaz.
func TestScriptArgumentsPassed(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.tr", "yazdır(\"çalıştı\")\n")
	warning := "⚠️ Uyarı: programa verilen 3 argüman kullanılmadı (--öğrenme_oranı 0.01 veri); program komut satırı argümanlarını okumuyor"
	for _, command := range [][]string{
		{"a.tr", "--öğrenme_oranı", "0.01", "veri"},
		{"çalıştır", "a.tr", "--öğrenme_oranı", "0.01", "veri"},
		{"hata_ayıkla", "a.tr", "--öğrenme_oranı", "0.01", "veri"},
	} {
		s := tr(t, dir, command...)
		wait(t, s, 0, []string{"çalıştı"}, []string{warning})
		if strings.Contains(s.stdout, "Uyarı") {
			t.Errorf("%v: uyarı standart çıktıya yazılmamalı: %q", command, s.stdout)
		}
	}
	// Argümansız çalıştırmada uyarı yok
	if s := tr(t, dir, "a.tr"); s.code != 0 || strings.Contains(s.stderr, "argüman") {
		t.Errorf("argümansız çalıştırmada uyarı olmamalı: %+v", s)
	}
	// Program hatayla biterse yalnızca hata yazılır
	writeFile(t, dir, "hata.tr", "yazdır(1 / 0)\n")
	if s := tr(t, dir, "hata.tr", "x"); s.code != 1 || strings.Contains(s.stderr, "kullanılmadı") {
		t.Errorf("hatalı programda argüman uyarısı olmamalı: %+v", s)
	}
}

// 'tr derle' ile üretilen uygulamanın bütün argümanları programındır.
func TestBuiltAppArguments(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "uyg.tr", "yazdır(\"uygulama\")\n")
	exit := filepath.Join(t.TempDir(), "uyg.exe")
	wait(t, tr(t, dir, "derle", "uyg.tr", exit), 0, nil, nil)
	s := run(t, exit, dir, "", nil, "--adım", "5")
	wait(t, s, 0, []string{"uygulama"}, []string{"programa verilen 2 argüman kullanılmadı (--adım 5)"})
}
