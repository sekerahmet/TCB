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
	trackedRoot string
	tracked     *trackedSet // nil: git yok, depo bir git kopyası değil ya da kök bulunamadı
)

// trackedFiles depo kökü ve izlenen kümedir, ilk çağrıda bir kez kurulur. Root'tan farklı olarak paniklemez: çalışma
// klasörünü depo dışına taşımış bir testte (t.Chdir) kök bulunamazsa küme yoktur ve her yol olduğu gibi görünür.
func trackedFiles() (string, *trackedSet) {
	trackedOnce.Do(func() {
		if r, ok := findRoot(); ok {
			trackedRoot, tracked = r, gitTracked(r)
		}
	})
	return trackedRoot, tracked
}

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
	root, set := trackedFiles()
	return walkTracked(root, set, dir, fn)
}

// walkTracked WalkDir'in kendisidir: root altındaki dir'de yalnızca set'teki dosyalar ve klasörler gezilir (set nil
// ya da dir root dışındaysa her şey).
func walkTracked(root string, set *trackedSet, dir string, fn fs.WalkDirFunc) error {
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		switch {
		case d == nil || visible(root, set, p, d.IsDir()):
		case d.IsDir():
			return filepath.SkipDir
		default:
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

// Glob filepath.Glob gibidir; deponun içindeki eşleşmelerden yalnızca git'in izlediği dosyaları ve onları içeren
// klasörleri verir (WalkDir'in kuralı; göreli kalıp çalışma klasörüne, yani paketin klasörüne göredir). Depo dışındaki
// ve git'siz kopyadaki eşleşmeler olduğu gibi kalır (bağımsız denetim grup 7: formatter TestRepoFiles izlenmeyen bir
// .tr ile düşüyordu).
func Glob(pattern string) ([]string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	root, set := trackedFiles()
	var out []string
	for _, m := range matches {
		info, statErr := os.Stat(m)
		if statErr == nil && visible(root, set, m, info.IsDir()) {
			out = append(out, m)
		}
	}
	return out, nil
}

// GlobAll filepath.Glob'un kendisidir, izlenmeyen dosyalar da: bir testin depoda geride dosya bırakmadığını
// denetlemek içindir (izlenen-dosya kuralının tek bilinçli istisnası, adıyla).
func GlobAll(pattern string) ([]string, error) { return filepath.Glob(pattern) }

// ReadDir os.ReadDir gibidir; deponun içindeki klasörde izlenmeyen girdiler yoktur (Glob'un kuralı).
func ReadDir(dir string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	root, set := trackedFiles()
	var out []os.DirEntry
	for _, e := range entries {
		if visible(root, set, filepath.Join(dir, e.Name()), e.IsDir()) {
			out = append(out, e)
		}
	}
	return out, err
}

// visible yol izlenen kümeye göre görünür mü: root dışındaki yol ve küme yoksa her yol görünür.
func visible(root string, set *trackedSet, p string, dir bool) bool {
	abs, err := filepath.Abs(p)
	if set == nil || err != nil {
		return true
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return true
	}
	rel = filepath.ToSlash(rel)
	if dir {
		return rel == "." || set.dirs[rel]
	}
	return set.files[rel]
}
