package reporoot

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Kural: WalkDir köke göre izlenen kümedeki dosyaları ve onları içeren klasörleri gezer; izlenmeyen dosya ve yalnızca
// izlenmeyen dosya taşıyan klasör görünmez; kök dışındaki klasör ve küme yoksa (git'siz kopya) her şey gezilir.
func TestWalkDirSeesOnlyTrackedFiles(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"a/x.go", "a/y.go", "b/z.go"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(f)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(f)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	walk := func(set *trackedSet, dir string) []string {
		var seen []string
		if err := walkTracked(root, set, dir, func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				r, _ := filepath.Rel(root, p)
				seen = append(seen, filepath.ToSlash(r))
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return seen
	}
	set := &trackedSet{files: map[string]bool{"a/x.go": true}, dirs: map[string]bool{"a": true}}
	if got := walk(set, root); !slices.Equal(got, []string{"a/x.go"}) {
		t.Errorf("izlenen küme: %v", got)
	}
	if got := walk(nil, root); len(got) != 3 {
		t.Errorf("küme yoksa her şey: %v", got)
	}
	if got := walk(set, filepath.Join(root, "b")); len(got) != 0 {
		t.Errorf("izlenmeyen klasör: %v", got)
	}
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "o.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var seen []string
	if err := walkTracked(root, set, other, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			seen = append(seen, filepath.Base(p))
		}
		return err
	}); err != nil || !slices.Equal(seen, []string{"o.txt"}) {
		t.Errorf("kök dışındaki klasör olduğu gibi gezilir: %v %v", seen, err)
	}
}
