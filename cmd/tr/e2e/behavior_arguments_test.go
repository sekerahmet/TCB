package e2e

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Betiğe dosya adından sonra verilen argümanlar programa iletilir (sessizce yok sayılmaz).
// Program onları okumazsa (argümanları komut_satırı okur) çıkışta standart hataya uyarı
// yazılır; çıkış kodu değişmez, standart çıktıya karışmaz.
func TestScriptArgumentsPassed(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "a.tr", "yazdır(\"çalıştı\")\n")
	warning := "⚠️ Uyarı: programa verilen 3 argüman kullanılmadı (--öğrenme_oranı 0.01 veri); program komut satırı argümanlarını okumuyor"
	for _, command := range [][]string{
		{"a.tr", "--öğrenme_oranı", "0.01", "veri"},
		{"çalıştır", "a.tr", "--öğrenme_oranı", "0.01", "veri"},
		{"hata_ayıkla", "a.tr", "--öğrenme_oranı", "0.01", "veri"},
	} {
		s := tr(t, dir, command...)
		testkit.Wait(t, s, 0, []string{"çalıştı"}, []string{warning})
		if strings.Contains(s.Stdout, "Uyarı") {
			t.Errorf("%v: uyarı standart çıktıya yazılmamalı: %q", command, s.Stdout)
		}
	}
	// Argümansız çalıştırmada uyarı yok
	if s := tr(t, dir, "a.tr"); s.Code != 0 || strings.Contains(s.Stderr, "argüman") {
		t.Errorf("argümansız çalıştırmada uyarı olmamalı: %+v", s)
	}
	// Program hatayla biterse yalnızca hata yazılır
	testkit.WriteFile(t, dir, "hata.tr", "yazdır(1 / 0)\n")
	if s := tr(t, dir, "hata.tr", "x"); s.Code != 1 || strings.Contains(s.Stderr, "kullanılmadı") {
		t.Errorf("hatalı programda argüman uyarısı olmamalı: %+v", s)
	}
}

// 'tr derle' ile üretilen uygulamanın bütün argümanları programındır.
func TestBuiltAppArguments(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "uyg.tr", "yazdır(\"uygulama\")\n")
	exit := filepath.Join(t.TempDir(), "uyg.exe")
	testkit.Wait(t, tr(t, dir, "derle", "uyg.tr", exit), 0, nil, nil)
	s := testkit.RunBinary(t, exit, dir, "", nil, "--adım", "5")
	testkit.Wait(t, s, 0, []string{"uygulama"}, []string{"programa verilen 2 argüman kullanılmadı (--adım 5)"})
}
