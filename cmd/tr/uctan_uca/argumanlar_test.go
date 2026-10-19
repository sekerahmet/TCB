package uctanuca

import (
	"path/filepath"
	"strings"
	"testing"
)

// Betiğe dosya adından sonra verilen argümanlar programa iletilir (sessizce yok sayılmaz).
// Program onları okumazsa (argümanları komut_satırı okur) çıkışta standart hataya uyarı
// yazılır; çıkış kodu değişmez, standart çıktıya karışmaz.
func TestBetikArgumanlariIletilir(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "a.tr", "yazdır(\"çalıştı\")\n")
	uyari := "⚠️ Uyarı: programa verilen 3 argüman kullanılmadı (--öğrenme_oranı 0.01 veri); program komut satırı argümanlarını okumuyor"
	for _, komut := range [][]string{
		{"a.tr", "--öğrenme_oranı", "0.01", "veri"},
		{"çalıştır", "a.tr", "--öğrenme_oranı", "0.01", "veri"},
		{"hata_ayıkla", "a.tr", "--öğrenme_oranı", "0.01", "veri"},
	} {
		s := tr(t, klasor, komut...)
		bekle(t, s, 0, []string{"çalıştı"}, []string{uyari})
		if strings.Contains(s.cikti, "Uyarı") {
			t.Errorf("%v: uyarı standart çıktıya yazılmamalı: %q", komut, s.cikti)
		}
	}
	// Argümansız çalıştırmada uyarı yok
	if s := tr(t, klasor, "a.tr"); s.kod != 0 || strings.Contains(s.hata, "argüman") {
		t.Errorf("argümansız çalıştırmada uyarı olmamalı: %+v", s)
	}
	// Program hatayla biterse yalnızca hata yazılır
	dosyaYaz(t, klasor, "hata.tr", "yazdır(1 / 0)\n")
	if s := tr(t, klasor, "hata.tr", "x"); s.kod != 1 || strings.Contains(s.hata, "kullanılmadı") {
		t.Errorf("hatalı programda argüman uyarısı olmamalı: %+v", s)
	}
}

// 'tr derle' ile üretilen uygulamanın bütün argümanları programındır.
func TestDerlenmisUygulamaArgumanlari(t *testing.T) {
	klasor := t.TempDir()
	dosyaYaz(t, klasor, "uyg.tr", "yazdır(\"uygulama\")\n")
	cikis := filepath.Join(t.TempDir(), "uyg.exe")
	bekle(t, tr(t, klasor, "derle", "uyg.tr", cikis), 0, nil, nil)
	s := calistir(t, cikis, klasor, "", nil, "--adım", "5")
	bekle(t, s, 0, []string{"uygulama"}, []string{"programa verilen 2 argüman kullanılmadı (--adım 5)"})
}
