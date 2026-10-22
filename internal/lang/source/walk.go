package source

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// WalkFiles komut satırı hedefinin (dosya ya da klasör) suffix ile biten dosyalarını sıralı
// toplar. Dosya hedefi olduğu gibi döner. Klasörde gizli klasörler (.git, .claude) ile
// node_modules taranmaz. Hedefin açılamaması statError, gezinti hatası walkError ile istenen
// hataya çevrilir (kodlar çağıranın katmanındadır); walkError nil ise ham hata döner.
func WalkFiles(target, suffix string, statError func(target string, err error) error, walkError func(path string, err error) error) ([]string, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, statError(target, err)
	}
	if !info.IsDir() {
		return []string{target}, nil
	}
	var files []string
	err = filepath.WalkDir(target, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if walkError != nil {
				return walkError(path, err)
			}
			return err
		}
		if d.IsDir() {
			if path != target && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), suffix) {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}
