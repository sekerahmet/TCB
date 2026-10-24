package compute_test

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// computeDirs hesap çekirdeğinin (test dışı) Go dosyası olan klasörleridir; yollar mutlaktır.
func computeDirs(t *testing.T) []string {
	t.Helper()
	base := filepath.Join(reporoot.Root(), "internal", "compute")
	var dirs []string
	err := reporoot.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".go") && !strings.HasSuffix(d.Name(), "_test.go") {
			if dir := filepath.Dir(path); len(dirs) == 0 || dirs[len(dirs)-1] != dir {
				dirs = append(dirs, dir)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) < 4 {
		t.Fatalf("yalnızca %d hesap paketi bulundu; depo yapısı değişmiş olabilir", len(dirs))
	}
	return dirs
}
