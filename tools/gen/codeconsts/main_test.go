package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Üretecin aile harfleri kod listesindekiyle aynı sıradadır: yeni bir aile (ör. K) eklenince bu
// kopya da güncellenmezse üreteç o ailenin kodlarını geçersiz sayardı.
func TestFamilyLettersMatch(t *testing.T) {
	for i := 0; i < len(familyLetters); i++ {
		if got := codes.Code(1000 * (i + 1)).Letter(); got != familyLetters[i] {
			t.Errorf("%d. aile: üreteçte %c, kod listesinde %c", i+1, familyLetters[i], got)
		}
	}
	if codes.Code(1000*(len(familyLetters)+1)).Letter() != 0 {
		t.Error("kod listesinde üretecin bilmediği bir aile var")
	}
}

// Üretilen sabit dosyası kod listesinden üretilenle aynı olmalı: bir kod eklendiğinde,
// kaldırıldığında ya da taşındığında `go run ./tools/gen/codeconsts` çalıştırılmadıysa
// bu test düşer.
func TestConstantsUpToDate(t *testing.T) {
	root := reporoot.Root()
	fresh, err := Generate(root)
	if err != nil {
		t.Fatal(err)
	}
	old, err := os.ReadFile(filepath.Join(root, outputPath))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.ReplaceAll(old, []byte("\r\n"), []byte("\n")), fresh) {
		t.Fatalf("%s güncel değil; depo kökünde `go run ./tools/gen/codeconsts` çalıştırın", outputPath)
	}
}

// Go kaynaklarında (internal/lang/codes dışında) kodlar yalnızca adlı sabitlerle yazılır: aile + sayı
// biçimi (codes.G + 100) kalmamalı. Sözlükte olmayan bir sabit yazılırsa derleme zaten düşer.
func TestNoBareCodes(t *testing.T) {
	root := reporoot.Root()
	pattern := regexp.MustCompile(`\bcodes\.[AGTYVWH] *\+`)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == ".claude" || path == filepath.Join(root, "internal", "lang", "codes") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			if pattern.MatchString(line) && !strings.Contains(line, "desen") && !strings.HasPrefix(strings.TrimSpace(line), "//") {
				t.Errorf("%s:%d: kod adlı sabitle yazılmalı (codes.G100): %s", path, i+1, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
