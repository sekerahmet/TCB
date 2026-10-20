package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// docs/codes.md kod listesinden üretilenle aynı olmalı: bir kod, açıklama ya da tanım
// değiştiğinde `go run ./tools/gen/codesdoc` çalıştırılmadıysa bu test düşer.
func TestDocUpToDate(t *testing.T) {
	fresh, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	old, err := os.ReadFile(filepath.Join(reporoot.Root(), docPath))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.ReplaceAll(old, []byte("\r\n"), []byte("\n")), fresh) {
		t.Fatalf("%s güncel değil; depo kökünde `go run ./tools/gen/codesdoc` çalıştırın", docPath)
	}
}
