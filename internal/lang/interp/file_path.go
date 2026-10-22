package interp

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// ResolvePath applies the SINGLE rule for relative file paths in TR programs; every builtin
// that reads or writes files (dosya_oku, dosya_yaz and the engines' load/save functions) must
// resolve paths with it. 'aktar' resolves module paths with its own rule (ResolveModulePath:
// the working directory is not part of the module search, 11.1).
//
// The rule (the same for reading and writing): a relative path is tried first against the
// working directory, then (when the program runs from a file) against the program's
// directory, and the first place where the file exists is used. If the file exists nowhere,
// the first place whose directory exists is used (for a plain "a.txt" that is always the
// working directory); failing that the path is returned as is (relative to the working
// directory). Reading a missing file gives a "dosya yok" error; writing creates the file in
// the chosen place.
//
// Rationale: the same path names the same file for reading and writing (a sayaç.txt next to
// the program is read, updated and written back to the same file; separate read/write rules
// would read one and write the other), a program finds the data next to it whichever
// directory it runs from, and new files appear in the working directory as shell habits
// expect. Absolute paths are used as is.
//
// bakilanlar are the absolute paths of the places tried, shown as "bakılan yerler" in error
// messages (nil for an absolute or empty path).
func (v *VM) ResolvePath(filePath string) (resolved string, searched []string) {
	if filePath == "" || filepath.IsAbs(filePath) {
		return filePath, nil
	}
	candidates := []string{filePath}
	searched = []string{absPath(filePath)}
	for _, folder := range v.searchDirs() {
		candidates, searched = addCandidate(candidates, searched, folder, filePath)
	}
	// 1. Dosyanın (ya da klasörün) kendisinin bulunduğu ilk yer
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, searched
		}
	}
	// 2. Yoksa, bulunacağı klasörün var olduğu ilk yer: yeni bir "çıktı/rapor.txt"
	// programın yanındaki çıktı/ klasörüne yazılır; okumada "dosya yok" hatası o
	// klasördeki eksik dosyayı söyler (yanıltıcı biçimde "klasör yok" demez).
	for _, candidate := range candidates {
		if info, err := os.Stat(filepath.Dir(candidate)); err == nil && info.IsDir() {
			return candidate, searched
		}
	}
	return filePath, searched
}

// SearchedPlaces turns the places returned by ResolvePath into the error message suffix.
func SearchedPlaces(searched []string) string {
	switch len(searched) {
	case 0:
		return ""
	case 1:
		return messages.Text(messages.H2030, searched[0])
	}
	return messages.Text(messages.H2031, strings.Join(searched, ", "))
}

// ReadError explains a read error with turkish.FileError; when the file was not found it adds
// the places ResolvePath tried. fn is the builtin reporting the error, yol the path the user
// wrote, operation an outcome message code such as messages.H2100 ("okunamadı").
func ReadError(fn, filePath string, operation messages.Code, err error, searched []string) error {
	extra := ""
	if errors.Is(err, fs.ErrNotExist) {
		extra = SearchedPlaces(searched)
	}
	return turkish.FileError(fn, filePath, operation, err, extra)
}

// CodeDirs returns the directories of the running code: the module's directory first when the
// code runs in a module, then the program's directory (where relative paths are searched after
// the working directory; sil never deletes these or their parents). Empty when the program does
// not run from a file.
func (v *VM) CodeDirs() []string { return v.searchDirs() }

// searchDirs göreli yolların çalışma klasöründen sonra arandığı klasörlerdir: kod bir
// modülde çalışıyorsa önce o modülün klasörü, sonra programın klasörü. Program bir
// dosyadan çalışmıyorsa (etkileşimli kabuk, derlenmiş uygulama: "<...>") onun klasörü yoktur;
// dilin gömülü kitaplığındaki bir modülün (11.6) diskte klasörü yoktur.
func (v *VM) searchDirs() []string {
	if v == nil {
		return nil
	}
	src := ""
	if v.source != nil && !v.source.Embedded {
		src = v.source.Path
	}
	return FileDirs(src, v.SourceFile)
}

// addCandidate folder/filePath yolunu, mutlak karşılığı daha önce bakılmadıysa aday listesine
// ve bakılan yerlere ekler. Birinci adayın (çalışma klasörü ya da boş) seçimi çağıranındır.
func addCandidate(candidates, searched []string, folder, filePath string) ([]string, []string) {
	candidate := filepath.Join(folder, filePath)
	absolute := absPath(candidate)
	if slices.Contains(searched, absolute) {
		return candidates, searched
	}
	return append(candidates, candidate), append(searched, absolute)
}

func absPath(filePath string) string {
	if m, err := filepath.Abs(filePath); err == nil {
		return m
	}
	return filePath
}
