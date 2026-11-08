package interp

import (
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// LibraryPrefix is the 'aktar' prefix reserved for the language's own library (11.6):
// "tr/layers" is loaded from the library embedded in the binary (lib/layers.tr in the repo),
// not from disk. The user's files on disk cannot shadow this prefix; a user folder named "tr"
// is imported with "./tr/...".
const LibraryPrefix = "tr/"

// libraryFiles dilin gömülü kitaplığıdır; komut satırı (cmd/trlang) KitaplikBagla ile bağlar.
// Yorumlayıcı kitaplık paketini içe aktarmaz (katman kuralı, docs/architecture.md).
var libraryFiles fs.FS

// SetLibrary installs the language's library written in TR (the embedded files of the lib
// package). Call it once before the program starts; nil removes the library.
func SetLibrary(f fs.FS) { libraryFiles = f }

// LibraryModule, when the path points into the language's library (tr/ prefix), returns the
// module's name relative to the library root with the ".tr" extension: "tr/layers" →
// "layers.tr". A backslash also counts as a separator.
func LibraryModule(filePath string) (name string, ok bool) {
	y := strings.ReplaceAll(filePath, `\`, "/")
	if !strings.HasPrefix(y, LibraryPrefix) {
		return "", false
	}
	name = strings.TrimPrefix(y, LibraryPrefix)
	if !strings.HasSuffix(name, ".tr") {
		name += ".tr"
	}
	return name, true
}

// ReadLibrary reads the source of a library module. If the module does not exist the error
// lists the library's modules and, if any, the closest name. Test files (*_test.tr) are not
// modules.
func ReadLibrary(name string) ([]byte, error) {
	if libraryFiles == nil {
		return nil, messages.New(messages.H2480)
	}
	valid := fs.ValidPath(name) && pathpkg.Clean(name) == name && !strings.HasSuffix(name, "_test.tr")
	if valid {
		if payload, err := fs.ReadFile(libraryFiles, name); err == nil {
			return payload, nil
		}
	}
	modules := LibraryModules()
	wanted := LibraryPrefix + strings.TrimSuffix(name, ".tr")
	var suggestion *messages.Error
	if closest := turkish.Closest(wanted, modules); closest != "" {
		suggestion = messages.New(messages.H2479, closest)
	}
	return nil, messages.New(messages.H2478, wanted, strings.Join(modules, ", "), suggestion)
}

// LibraryModules returns the 'aktar' paths of the library's modules ("tr/layers", ...),
// sorted.
func LibraryModules() []string {
	if libraryFiles == nil {
		return nil
	}
	var nameList []string
	fs.WalkDir(libraryFiles, ".", func(filePath string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(filePath, ".tr") && !strings.HasSuffix(filePath, "_test.tr") {
			nameList = append(nameList, LibraryPrefix+strings.TrimSuffix(filePath, ".tr"))
		}
		return nil
	})
	sort.Strings(nameList)
	return nameList
}

// ResolveModulePath is the SINGLE rule for a relative 'aktar' path (11.1): the path is resolved
// only against the importing file's directory (dirs[0]); the working directory and the program's
// directory (dirs[1:], FileDirs) are not searched. Without directories (REPL, a program with no
// file) it is relative to the working directory. The path's letter case must be the name on disk,
// on every system: a path that differs only in case is not found (one file must not load as two
// modules). Absolute paths are tried as is. The interpreter, 'trlang build' and 'trlang check' use
// this function; ModulePathError says why a path was not found. searched are the places tried,
// for the error message.
func ResolveModulePath(filePath string, dirs []string) (resolved string, searched []string, ok bool) {
	if filepath.IsAbs(filePath) {
		_, err := os.Stat(filePath)
		return filePath, []string{filePath}, err == nil
	}
	dir := "."
	if len(dirs) > 0 {
		dir = dirs[0]
	}
	resolved = filepath.Join(dir, filePath)
	disk, found := diskSpelling(dir, filePath)
	return resolved, []string{absPath(resolved)}, found && disk == filepath.ToSlash(filePath)
}

// ModulePathError is why ResolveModulePath did not find the relative 'aktar' path written (as
// written, with or without .tr) among dirs (11.1): when only the letter case differs from the
// name on disk the message gives the name on disk (H2721); when the file is not next to the
// importing file but the program's directory has it, the message gives the explicit path there
// (H2722, "../ad"); otherwise fs.ErrNotExist. Both suggestions are 'aktar' paths that load.
func ModulePathError(written string, dirs []string) error {
	filePath := written
	if !strings.HasSuffix(filePath, ".tr") {
		filePath += ".tr"
	}
	if filepath.IsAbs(filePath) {
		return fs.ErrNotExist
	}
	dir := "."
	if len(dirs) > 0 {
		dir = dirs[0]
	}
	if disk, found := diskSpelling(dir, filePath); found {
		return messages.New(messages.H2721, writtenLike(written, disk))
	}
	for _, other := range dirs[min(1, len(dirs)):] {
		if disk, found := diskSpelling(other, filePath); found && absPath(other) != absPath(dir) {
			if rel, err := filepath.Rel(dir, filepath.Join(other, disk)); err == nil {
				return messages.New(messages.H2722, writtenLike(written, filepath.ToSlash(rel)))
			}
		}
	}
	return fs.ErrNotExist
}

// writtenLike path'i yazılan yol gibi yazar: yazılan yolda .tr yoksa öneride de olmaz.
func writtenLike(written, path string) string {
	if !strings.HasSuffix(written, ".tr") {
		return strings.TrimSuffix(path, ".tr")
	}
	return path
}

// diskSpelling rel yolunu dir'den başlayarak bileşen bileşen diskte arar: found yol diskte, harf
// büyüklüğüne bakılmadan, bir dosyaysa doğrudur; disk yolun diskteki adlarla yazımıdır ("/" ile).
// Bileşenin aynı yazımı varsa o, yoksa harf büyüklüğüyle ayrılan ilk ad (klasör sırasıyla) alınır.
func diskSpelling(dir, rel string) (disk string, found bool) {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	cur := dir
	for i, p := range parts {
		if p == "" || p == "." || p == ".." {
			cur = filepath.Join(cur, p)
			continue
		}
		entries, err := os.ReadDir(cur)
		if err != nil {
			return "", false
		}
		name := ""
		for _, e := range entries {
			if e.Name() == p {
				name = p
				break
			}
			if name == "" && strings.EqualFold(e.Name(), p) {
				name = e.Name()
			}
		}
		if name == "" {
			return "", false
		}
		parts[i] = name
		cur = filepath.Join(cur, name)
	}
	if info, err := os.Stat(cur); err != nil || info.IsDir() {
		return "", false
	}
	return strings.Join(parts, "/"), true
}

// FileDirs returns the directories of file paths; empty sources and those named "<...>" (with
// no file) are skipped. It builds the directory list for ResolveModulePath from the importing
// and program files.
func FileDirs(files ...string) []string {
	var dirs []string
	for _, d := range files {
		if d != "" && !strings.HasPrefix(d, "<") {
			dirs = append(dirs, filepath.Dir(d))
		}
	}
	return dirs
}

// LibraryFileName is the embedded library's file name ("layers.tr") when text is that file read
// from path (its test files too); otherwise "". A file is the library's only when its text is the
// embedded text: a user's file of the same name is not (6.1, 7.1: the library's exceptions).
func LibraryFileName(filePath, text string) string {
	if libraryFiles == nil {
		return ""
	}
	name := pathpkg.Base(strings.ReplaceAll(filePath, `\`, "/"))
	if payload, err := fs.ReadFile(libraryFiles, name); err == nil && string(payload) == text {
		return name
	}
	return ""
}
