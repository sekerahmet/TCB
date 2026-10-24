package reporoot

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

var (
	trackedOnce sync.Once
	tracked     *trackedSet // nil: git yok ya da depo bir git kopyası değil
)

// trackedSet git'in izlediği dosyalar ve onları içeren klasörlerdir (köke göre, "/" ile).
type trackedSet struct {
	files, dirs map[string]bool
}

// WalkDir filepath.WalkDir gibidir, ama deponun içindeki bir yolda yalnızca git'in izlediği dosyaları ve onları içeren
// klasörleri gezer: depo testleri geliştiricinin izlenmeyen ya da yok sayılan dosyalarına (scratch/, kişisel
// denemeler, eski koşuların çıktıları) bağlı olmaz (bağımsız denetim grup 6: TestNoTurkishLiterals scratch/'taki bir
// denemeyle düşüyordu). Depo dışındaki yol (t.TempDir) ve git'siz kopya olduğu gibi gezilir. Yeni bir dosya `git add`
// ile görünür olur.
func WalkDir(dir string, fn fs.WalkDirFunc) error {
	trackedOnce.Do(func() { tracked = gitTracked(Root()) })
	return walkTracked(Root(), tracked, dir, fn)
}

// walkTracked WalkDir'in kendisidir: root altındaki dir'de yalnızca set'teki dosyalar ve klasörler gezilir (set nil
// ya da dir root dışındaysa her şey).
func walkTracked(root string, set *trackedSet, dir string, fn fs.WalkDirFunc) error {
	rel, err := filepath.Rel(root, dir)
	if set == nil || err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.WalkDir(dir, fn)
	}
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		r, _ := filepath.Rel(root, p)
		r = filepath.ToSlash(r)
		switch {
		case d == nil:
		case d.IsDir():
			if r != "." && !set.dirs[r] {
				return filepath.SkipDir
			}
		case !set.files[r]:
			return nil
		}
		return fn(p, d, err)
	})
}

// gitTracked kökün `git ls-files` listesidir (git yoksa nil).
func gitTracked(root string) *trackedSet {
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return nil
	}
	out, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		return nil
	}
	s := &trackedSet{files: map[string]bool{}, dirs: map[string]bool{}}
	for _, name := range bytes.Split(out, []byte{0}) {
		if len(name) == 0 {
			continue
		}
		f := string(name)
		s.files[f] = true
		for d := path.Dir(f); d != "." && !s.dirs[d]; d = path.Dir(d) {
			s.dirs[d] = true
		}
	}
	return s
}
