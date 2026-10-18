package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"tr_dili/internal/depo"
)

// Üretilen sabit dosyası kod listesinden üretilenle aynı olmalı: bir kod eklendiğinde,
// kaldırıldığında ya da taşındığında `go run ./tools/uret/kod_sabitleri` çalıştırılmadıysa
// bu test düşer.
func TestSabitlerGuncel(t *testing.T) {
	kok := depo.Kok()
	yeni, err := Uret(kok)
	if err != nil {
		t.Fatal(err)
	}
	eski, err := os.ReadFile(filepath.Join(kok, ciktiYolu))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.ReplaceAll(eski, []byte("\r\n"), []byte("\n")), yeni) {
		t.Fatalf("%s güncel değil; depo kökünde `go run ./tools/uret/kod_sabitleri` çalıştırın", ciktiYolu)
	}
}

// Go kaynaklarında (internal/dil/kod dışında) kodlar yalnızca adlı sabitlerle yazılır: aile + sayı
// biçimi (kod.G + 100) kalmamalı. Sözlükte olmayan bir sabit yazılırsa derleme zaten düşer.
func TestCiplakKodYok(t *testing.T) {
	kok := depo.Kok()
	desen := regexp.MustCompile(`\bkod\.[AGTYVWH] *\+`)
	err := filepath.WalkDir(kok, func(yol string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == ".claude" || yol == filepath.Join(kok, "internal", "dil", "kod") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(yol, ".go") {
			return nil
		}
		veri, err := os.ReadFile(yol)
		if err != nil {
			return err
		}
		for i, satir := range strings.Split(string(veri), "\n") {
			if desen.MatchString(satir) && !strings.Contains(satir, "desen") && !strings.HasPrefix(strings.TrimSpace(satir), "//") {
				t.Errorf("%s:%d: kod adlı sabitle yazılmalı (kod.G100): %s", yol, i+1, strings.TrimSpace(satir))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
