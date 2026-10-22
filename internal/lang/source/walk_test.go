package source

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural: WalkFiles dosya hedefini olduğu gibi verir; klasörde yalnızca sonek uyanları, sıralı,
// gizli klasörler ve node_modules dışında toplar; hedef yoksa çağıranın hatasını döndürür.
// Neden: testrunner.TestFiles ve cmd/tr.trFiles aynı yürüyüşü ayrı ayrı yazıyordu.
func TestWalkFilesCollectsSuffixedFilesInOrder(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"b_test.tr", "a_test.tr", "x.tr", filepath.Join("alt", "c_test.tr"), filepath.Join(".git", "g_test.tr"), filepath.Join("node_modules", "n_test.tr")} {
		testkit.WritePath(t, filepath.Join(dir, name), "")
	}
	got, err := WalkFiles(dir, "_test.tr", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(dir, "a_test.tr"), filepath.Join(dir, "alt", "c_test.tr"), filepath.Join(dir, "b_test.tr")}
	if len(got) != len(want) {
		t.Fatalf("dosyalar %v, beklenen %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dosyalar %v, beklenen %v", got, want)
		}
	}
	single := filepath.Join(dir, "x.tr")
	if got, _ := WalkFiles(single, "_test.tr", nil, nil); len(got) != 1 || got[0] != single {
		t.Errorf("dosya hedefi olduğu gibi dönmeli: %v", got)
	}
	sentinel := errors.New("yok")
	if _, err := WalkFiles(filepath.Join(dir, "yok"), ".tr", func(string, error) error { return sentinel }, nil); err != sentinel {
		t.Errorf("hedef hatası çağıranın hatası olmalı: %v", err)
	}
}
