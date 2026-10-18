package main

import (
	"os"
	"path/filepath"
	"testing"

	"tr_dili/internal/depo"
)

// geciciOrtam işletim sisteminin geçici klasörünü boş bir klasöre çeviren ortam değişkenleridir.
func geciciOrtam(t *testing.T) (string, []string) {
	t.Helper()
	gecici := t.TempDir()
	return gecici, []string{"TMP=" + gecici, "TEMP=" + gecici, "TMPDIR=" + gecici}
}

func geciciBosMu(t *testing.T, gecici, ne string) {
	t.Helper()
	girdiler, err := os.ReadDir(gecici)
	if err != nil {
		t.Fatal(err)
	}
	if len(girdiler) != 0 {
		var adlar []string
		for _, g := range girdiler {
			adlar = append(adlar, g.Name())
		}
		t.Errorf("%s: geçici klasörde bir şey kalmamalı: %v", ne, adlar)
	}
}

// Geçici klasör kullanan dil tanımı testleri (18, 19) koşudan sonra geçici klasörde ve
// depoda bir şey bırakmaz (18.14: tr test de program bitince geçici klasörleri siler).
func TestTestKomutuGeciciKlasorBirakmaz(t *testing.T) {
	for _, dosya := range []string{"testler/surec_ve_kayit_test.tr", "testler/dosya_ve_yol_test.tr"} {
		gecici, ortam := geciciOrtam(t)
		s := calistir(t, trIkilisi, depo.Kok(), "", ortam, "test", filepath.FromSlash(dosya))
		bekle(t, s, 0, []string{"0 kaldı"}, nil)
		geciciBosMu(t, gecici, dosya)
	}
	if eski, _ := filepath.Glob(depo.Yol("testler/gecici_*")); len(eski) != 0 {
		t.Errorf("testler depoda geçici klasör bırakmamalı: %v", eski)
	}
}

// Program nasıl biterse bitsin (başarı, çalışma hatası, komut satırı hatasıyla çıkış)
// geçici_klasör()'ün klasörleri içlerindeki açık satır okuyucusuyla birlikte silinir.
func TestGeciciKlasorProgramBitinceSilinir(t *testing.T) {
	klasor := t.TempDir()
	ortak := "değişken k = geçici_klasör()\ndosya_yaz(yol(k, \"a.txt\"), \"1\\n2\")\ndeğişken o = dosya_satırları(yol(k, \"a.txt\"))\no.sonraki()\nyazdır(k.var_mı)\n"
	durumlar := []struct {
		ad, son, arguman string
		kod              int
	}{
		{"başarı", "", "", 0},
		{"çalışma hatası", "yazdır(1 / 0)\n", "", 1},
		{"komut satırı çıkışı", "komut_satırı({adım: 1})\n", "--bilinmeyen", 2},
	}
	for _, d := range durumlar {
		gecici, ortam := geciciOrtam(t)
		dosyaYaz(t, klasor, "p.tr", ortak+d.son)
		args := []string{"p.tr"}
		if d.arguman != "" {
			args = append(args, d.arguman)
		}
		s := calistir(t, trIkilisi, klasor, "", ortam, args...)
		bekle(t, s, d.kod, []string{"doğru"}, nil)
		geciciBosMu(t, gecici, d.ad)
	}
}
