package interp

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// LibraryPrefix is the 'aktar' prefix reserved for the language's own library (11.6):
// "tr/katman" is loaded from the library embedded in the binary (lib/katman.tr in the repo),
// not from disk. The user's files on disk cannot shadow this prefix; a user folder named "tr"
// is imported with "./tr/...".
const LibraryPrefix = "tr/"

// kitaplikDosyalari dilin gömülü kitaplığıdır; komut satırı (cmd/tr) KitaplikBagla ile bağlar.
// Yorumlayıcı kitaplık paketini içe aktarmaz (katman kuralı, docs/mimari.md).
var kitaplikDosyalari fs.FS

// SetLibrary installs the language's library written in TR (the embedded files of the lib
// package). Call it once before the program starts; nil removes the library.
func SetLibrary(f fs.FS) { kitaplikDosyalari = f }

// LibraryModule, when the path points into the language's library (tr/ prefix), returns the
// module's name relative to the library root with the ".tr" extension: "tr/katman" →
// "katman.tr". A backslash also counts as a separator.
func LibraryModule(yol string) (ad string, ok bool) {
	y := strings.ReplaceAll(yol, `\`, "/")
	if !strings.HasPrefix(y, LibraryPrefix) {
		return "", false
	}
	ad = strings.TrimPrefix(y, LibraryPrefix)
	if !strings.HasSuffix(ad, ".tr") {
		ad += ".tr"
	}
	return ad, true
}

// ReadLibrary reads the source of a library module. If the module does not exist the error
// lists the library's modules and, if any, the closest name. Test files (*_test.tr) are not
// modules.
func ReadLibrary(ad string) ([]byte, error) {
	if kitaplikDosyalari == nil {
		return nil, errors.New("dilin kitaplığı bu programa bağlı değil")
	}
	gecerli := fs.ValidPath(ad) && pathpkg.Clean(ad) == ad && !strings.HasSuffix(ad, "_test.tr")
	if gecerli {
		if veri, err := fs.ReadFile(kitaplikDosyalari, ad); err == nil {
			return veri, nil
		}
	}
	moduller := LibraryModules()
	aranan := LibraryPrefix + strings.TrimSuffix(ad, ".tr")
	mesaj := fmt.Sprintf("%q dilin kitaplığında yok; kitaplıktaki modüller: %s", aranan, strings.Join(moduller, ", "))
	if oneri := turkish.Closest(aranan, moduller); oneri != "" {
		mesaj += fmt.Sprintf(". Bunu mu demek istediniz: %q?", oneri)
	}
	return nil, errors.New(mesaj)
}

// LibraryModules returns the 'aktar' paths of the library's modules ("tr/katman", ...),
// sorted.
func LibraryModules() []string {
	if kitaplikDosyalari == nil {
		return nil
	}
	var adlar []string
	fs.WalkDir(kitaplikDosyalari, ".", func(yol string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(yol, ".tr") && !strings.HasSuffix(yol, "_test.tr") {
			adlar = append(adlar, LibraryPrefix+strings.TrimSuffix(yol, ".tr"))
		}
		return nil
	})
	sort.Strings(adlar)
	return adlar
}

// ResolveModulePath is the SINGLE rule for a relative 'aktar' path (11.1): the path is tried
// against klasorler in order (first the importing file's directory, then the program's
// directory) and the first place where the file exists is used; without directories (REPL,
// a program with no file) it is relative to the working directory. For a program with a file
// the working directory is not one of the places searched: a module run from another directory
// must not load a same-named file from the working directory instead of the one next to it.
// Absolute paths are tried as is. The interpreter, 'tr derle' and 'tr kontrol' use this
// function. bakilanlar are the places tried, for the error message.
func ResolveModulePath(yol string, klasorler []string) (cozulen string, bakilanlar []string, ok bool) {
	if filepath.IsAbs(yol) {
		_, err := os.Stat(yol)
		return yol, []string{yol}, err == nil
	}
	if len(klasorler) == 0 {
		klasorler = []string{"."}
	}
	var adaylar []string
	for _, k := range klasorler {
		aday := filepath.Join(k, yol)
		mutlak := mutlakYol(aday)
		if slices.Contains(bakilanlar, mutlak) {
			continue
		}
		adaylar = append(adaylar, aday)
		bakilanlar = append(bakilanlar, mutlak)
	}
	for _, aday := range adaylar {
		if bilgi, err := os.Stat(aday); err == nil && !bilgi.IsDir() {
			return aday, bakilanlar, true
		}
	}
	return adaylar[0], bakilanlar, false
}

// FileDirs returns the directories of file paths; empty sources and those named "<...>" (with
// no file) are skipped. It builds the directory list for ResolveModulePath from the importing
// and program files.
func FileDirs(dosyalar ...string) []string {
	var klasorler []string
	for _, d := range dosyalar {
		if d != "" && !strings.HasPrefix(d, "<") {
			klasorler = append(klasorler, filepath.Dir(d))
		}
	}
	return klasorler
}
