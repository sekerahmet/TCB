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

// libraryFiles dilin gömülü kitaplığıdır; komut satırı (cmd/tr) KitaplikBagla ile bağlar.
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

// ResolveModulePath is the SINGLE rule for a relative 'aktar' path (11.1): the path is tried
// against klasorler in order (first the importing file's directory, then the program's
// directory) and the first place where the file exists is used; without directories (REPL,
// a program with no file) it is relative to the working directory. For a program with a file
// the working directory is not one of the places searched: a module run from another directory
// must not load a same-named file from the working directory instead of the one next to it.
// Absolute paths are tried as is. The interpreter, 'tr build' and 'tr check' use this
// function. bakilanlar are the places tried, for the error message.
func ResolveModulePath(filePath string, dirs []string) (resolved string, searched []string, ok bool) {
	if filepath.IsAbs(filePath) {
		_, err := os.Stat(filePath)
		return filePath, []string{filePath}, err == nil
	}
	if len(dirs) == 0 {
		dirs = []string{"."}
	}
	var candidates []string
	for _, k := range dirs {
		candidates, searched = addCandidate(candidates, searched, k, filePath)
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, searched, true
		}
	}
	return candidates[0], searched, false
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
