package interp

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

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
func (v *VM) ResolvePath(yol string) (cozulen string, bakilanlar []string) {
	if yol == "" || filepath.IsAbs(yol) {
		return yol, nil
	}
	adaylar := []string{yol}
	bakilanlar = []string{mutlakYol(yol)}
	for _, klasor := range v.aramaKlasorleri() {
		aday := filepath.Join(klasor, yol)
		if mutlak := mutlakYol(aday); !slices.Contains(bakilanlar, mutlak) {
			adaylar = append(adaylar, aday)
			bakilanlar = append(bakilanlar, mutlak)
		}
	}
	// 1. Dosyanın (ya da klasörün) kendisinin bulunduğu ilk yer
	for _, aday := range adaylar {
		if _, err := os.Stat(aday); err == nil {
			return aday, bakilanlar
		}
	}
	// 2. Yoksa, bulunacağı klasörün var olduğu ilk yer: yeni bir "çıktı/rapor.txt"
	// programın yanındaki çıktı/ klasörüne yazılır; okumada "dosya yok" hatası o
	// klasördeki eksik dosyayı söyler (yanıltıcı biçimde "klasör yok" demez).
	for _, aday := range adaylar {
		if bilgi, err := os.Stat(filepath.Dir(aday)); err == nil && bilgi.IsDir() {
			return aday, bakilanlar
		}
	}
	return yol, bakilanlar
}

// SearchedPlaces turns the places returned by ResolvePath into the error message suffix.
func SearchedPlaces(bakilanlar []string) string {
	switch len(bakilanlar) {
	case 0:
		return ""
	case 1:
		return "bakılan yer: " + bakilanlar[0]
	}
	return "bakılan yerler: " + strings.Join(bakilanlar, ", ")
}

// ReadError explains a read error with turkish.FileError; when the file was not found it adds
// the places ResolvePath tried. fn is the builtin reporting the error, yol the path the user
// wrote, islem an outcome such as "okunamadı".
func ReadError(fn, yol, islem string, err error, bakilanlar []string) error {
	ek := ""
	if errors.Is(err, fs.ErrNotExist) {
		ek = SearchedPlaces(bakilanlar)
	}
	return turkish.FileError(fn, yol, islem, err, ek)
}

// CodeDirs returns the directories of the running code: the module's directory first when the
// code runs in a module, then the program's directory (where relative paths are searched after
// the working directory; sil never deletes these or their parents). Empty when the program does
// not run from a file.
func (v *VM) CodeDirs() []string { return v.aramaKlasorleri() }

// aramaKlasorleri göreli yolların çalışma klasöründen sonra arandığı klasörlerdir: kod bir
// modülde çalışıyorsa önce o modülün klasörü, sonra programın klasörü. Program bir
// dosyadan çalışmıyorsa (etkileşimli kabuk, derlenmiş uygulama: "<...>") onun klasörü yoktur;
// dilin gömülü kitaplığındaki bir modülün (11.6) diskte klasörü yoktur.
func (v *VM) aramaKlasorleri() []string {
	if v == nil {
		return nil
	}
	var klasorler []string
	if v.kaynak != nil && !v.kaynak.Embedded && v.kaynak.Path != "" && !strings.HasPrefix(v.kaynak.Path, "<") {
		klasorler = append(klasorler, filepath.Dir(v.kaynak.Path))
	}
	if v.SourceFile != "" && !strings.HasPrefix(v.SourceFile, "<") {
		klasorler = append(klasorler, filepath.Dir(v.SourceFile))
	}
	return klasorler
}

func mutlakYol(yol string) string {
	if m, err := filepath.Abs(yol); err == nil {
		return m
	}
	return yol
}
