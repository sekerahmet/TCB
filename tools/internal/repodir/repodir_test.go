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

// FindFrom iç içe go.mod'lara takılmaz: depo kökü, go.mod'u TR modülünü söyleyen klasördür.
func TestFindFromSkipsNestedModules(t *testing.T) {
	top := t.TempDir()
	if err := os.WriteFile(filepath.Join(top, "go.mod"), []byte("module "+Module+"\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(top, "tools", "testdata", "mod")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "go.mod"), []byte("module example.com/fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := FindFrom(nested)
	if err != nil || got != top {
		t.Errorf("FindFrom = %q, %v; want %q", got, err, top)
	}
}
