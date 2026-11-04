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

// ResolvePath applies the reading rule for relative file paths in TR programs (13.1a, 18.16):
// every builtin that reads a file or asks about it (TR_read_file, TR_read_lines, TR_list_dir,
// the properties of 18.3, the source of TR_copy, the engines' load functions) resolves paths
// with it. Builtins that change the disk resolve with ResolveWritePath. TR_import resolves
// module paths with its own rule (ResolveModulePath: the working directory is not part of the
// module search, 11.1).
//
// The rule: a relative path is tried first against the working directory, then (when the code
// runs from a file) against the directory of the code's module and the program's directory,
// and the first place where the file exists is used. If the file exists nowhere, the first
// place whose directory exists is used, so that the "dosya yok" error names the missing file
// in that directory (not a missing directory); failing that the path is returned as is
// (relative to the working directory).
//
// Rationale: a program finds the data next to it (or next to its module) whichever directory
// it runs from. Absolute paths are used as is.
//
// searched are the absolute paths of the places tried, shown as "bakılan yerler" in error
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
	// 2. Yoksa, bulunacağı klasörün var olduğu ilk yer: "dosya yok" hatası o klasördeki
	// eksik dosyayı söyler (yanıltıcı biçimde "klasör yok" demez).
	for _, candidate := range candidates {
		if info, err := os.Stat(filepath.Dir(candidate)); err == nil && info.IsDir() {
			return candidate, searched
		}
	}
	return filePath, searched
}

// ResolveWritePath resolves the path of a builtin that changes the disk (18.16: TR_write_file,
// TR_save_tensors, the run folder of TR_Checkpoint, the file of TR_Logger, TR_make_dir,
// TR_remove, the source and target of TR_move, the target of TR_copy, the cache folder of
// TR_Dataset): a relative path is relative to the working directory only, so a file of the
// same name next to the module or the program is never overwritten, deleted or moved (13.1a).
// A program that writes next to itself builds that path explicitly (18). searched is the one
// place looked at, nil for an absolute or empty path, which is returned as is. v may be nil.
func (v *VM) ResolveWritePath(filePath string) (resolved string, searched []string) {
	if filePath == "" || filepath.IsAbs(filePath) {
		return filePath, nil
	}
	return filePath, []string{absPath(filePath)}
}

// WriteError explains why a builtin that changes the disk could not find filePath in the
// working directory (the place ResolveWritePath gave). When the reading rule finds a file of
// that name elsewhere (next to the module or the program), the error names it and the path
// that reaches it from the working directory (H2630): the builtin does not touch it, a program
// that means that file writes its path out (18). Otherwise it is ReadError.
func (v *VM) WriteError(fn any, filePath string, operation messages.Code, err error, searched []string) error {
	if errors.Is(err, fs.ErrNotExist) && len(searched) == 1 {
		if other, _ := v.ResolvePath(filePath); other != filePath {
			if _, statErr := os.Lstat(other); statErr == nil {
				return messages.New(messages.H2630, fn, filePath, searched[0], absPath(other), explicitPath(other))
			}
		}
	}
	return ReadError(fn, filePath, operation, err, searched)
}

// explicitPath is the path of found as written from the working directory ("../modul/a.txt"),
// absolute when there is no such path (another drive).
func explicitPath(found string) string {
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, absPath(found)); err == nil {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(absPath(found))
}

// SearchedPlaces turns the places returned by ResolvePath into the error message suffix.
func SearchedPlaces(searched []string) *messages.Error {
	switch len(searched) {
	case 0:
		return nil
	case 1:
		return messages.New(messages.H2030, searched[0])
	}
	return messages.New(messages.H2031, strings.Join(searched, ", "))
}

// ReadError explains a read error with turkish.FileError; when the file was not found it adds
// the places ResolvePath tried. fn is the builtin reporting the error, yol the path the user
// wrote, operation an outcome message code such as messages.H2100 ("okunamadı").
func ReadError(fn any, filePath string, operation messages.Code, err error, searched []string) error {
	var extra *messages.Error
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
