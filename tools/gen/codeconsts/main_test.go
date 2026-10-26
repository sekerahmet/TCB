package main

import (
	"github.com/sekerahmet/TCB/tools/internal/genmain"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Üretilen sabit dosyası kod listesinden üretilenle aynı olmalı: bir kod eklendiğinde,
// silindiğinde ya da adı değiştiğinde `go run ./tools/gen/codeconsts` çalıştırılmadıysa
// bu test düşer.
func TestConstantsUpToDate(t *testing.T) {
	genmain.AssertUpToDate(t, reporoot.Root(), "codeconsts", outputPath, Generate)
}

// Kitaplık kodlarının modül tablosu (libraryModules) definitions.json'un "module" alanından üretilir;
// modülsüz kod tabloda yoktur.
func TestLibraryModulesGenerated(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, filepath.Dir(trPath))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, data := range map[string]string{
		trPath:   `{"TR_fn_print": {"name": "yazdır"}, "TR_fn_sin": {"name": "sin"}}`,
		defsPath: `{"TR_fn_print": {"kind": "function"}, "TR_fn_sin": {"kind": "function", "module": "tr/deneme"}}`,
	} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err := Generate(root)
	if err != nil {
		t.Fatal(err)
	}
	_, table, _ := strings.Cut(string(out), "var libraryModules")
	if !strings.Contains(table, `FnSin: "tr/deneme",`) || strings.Contains(table, "FnPrint") {
		t.Errorf("libraryModules yalnız FnSin'i tr/deneme ile içermeli:\n%s", table)
	}
}

// Go kaynaklarında (internal/lang/codes dışında) kodlar yalnızca adlı sabitlerle yazılır: kodun
// sayısı (codes.Code(57)) dil dosyasındaki sırasıdır, bir kod eklenince kayar. Sözlükte olmayan bir
// sabit yazılırsa derleme zaten düşer.
func TestNoBareCodes(t *testing.T) {
	root := reporoot.Root()
	pattern := regexp.MustCompile(`\bcodes\.Code\(\s*\d`)
	err := reporoot.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
				t.Errorf("%s:%d: kod adlı sabitle yazılmalı (codes.FnPrint): %s", path, i+1, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
