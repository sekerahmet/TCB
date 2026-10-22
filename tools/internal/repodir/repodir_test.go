package repodir

import (
	"os"
	"path/filepath"
	"testing"
)

// Kural: Find çalışma klasöründen yukarı doğru go.mod'un bulunduğu ilk klasörü verir (test paket
// klasöründen çalışır; kök, iki üst klasördür... daha derin olabilir: go.mod'un varlığına bakılır).
func TestFindReturnsTheDirectoryWithGoMod(t *testing.T) {
	root := Find()
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("Find() = %q: go.mod yok: %v", root, err)
	}
}
