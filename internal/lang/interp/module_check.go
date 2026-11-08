package interp

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Çalışmadan önce modül denetimi (1.3): programın aktardığı bütün modüller (özyinelemeli)
// çalışmadan önce ayrıştırılır ve CheckModule ile tip denetiminden geçer; hata varsa hiçbir satır
// çalışmaz. 'trlang check', 'trlang build' (cmd/trlang) ve çalıştıran her yol aynı CheckModule'ü aynı
// genel adlarla (ana programın en dış adları) kullanır; yürütme sırasında modül yalnız okunup
// ayrıştırılır (runModule), ikinci bir denetim yolu yoktur. Dilin gömülü kitaplık modülleri
// derlemeyle gelir, denetlenmez.

// CheckImportedModules mainFile adlı prog'un aktardığı diskteki bütün modülleri çalıştırmadan
// denetler. İlk hatayı döndürür: denetim hatası için *ModuleError (aktar satırı ve modülün
// hataları), bulunamayan ya da okunamayan modül için o hatanın kendisi.
func CheckImportedModules(mainFile string, prog *ast.Program) error {
	_, err := ImportedModuleFiles(mainFile, prog)
	return err
}

// ImportedModuleFiles CheckImportedModules'ün denetimini yapar ve denetlediği modül dosyalarını, dolaylı
// aktarılanlarla, aktarım sırasıyla verir: aktarılan modülün testleri de koşar (12.1). Dilin gömülü
// kitaplığı dosya değildir, listede yoktur.
func ImportedModuleFiles(mainFile string, prog *ast.Program) ([]string, error) {
	c := &moduleChecker{mainFile: mainFile, seen: map[string]bool{}}
	err := c.check(nil, mainFile, prog)
	return c.files, err
}

// moduleChecker bir denetimin durumudur.
type moduleChecker struct {
	mainFile string
	seen     map[string]bool
	files    []string // denetlenen modül dosyaları, aktarım sırasıyla
}

// check importerin (nil: ana program) prog'unun aktardığı modülleri denetler.
func (c *moduleChecker) check(importer *Source, file string, prog *ast.Program) error {
	var imports []*ast.ImportStatement
	ast.Walk(prog, func(n any) bool {
		if im, ok := n.(*ast.ImportStatement); ok {
			imports = append(imports, im)
		}
		return true
	})
	for _, im := range imports {
		if err := c.module(importer, file, im); err != nil {
			return err
		}
	}
	return nil
}

// module tek bir 'aktar' deyiminin modülünü denetler ve onun aktardıklarına iner.
func (c *moduleChecker) module(importer *Source, file string, im *ast.ImportStatement) error {
	path := im.Path
	if !strings.HasSuffix(path, ".tr") {
		path += ".tr"
	}
	if name, library := LibraryModule(path); library {
		// Dilin gömülü kitaplığı derlemeyle gelir, denetlenmez; ama olmayan bir kitaplık modülü
		// çalışmadan önce bulunur (1.3).
		if _, err := ReadLibrary(name); err != nil {
			return messages.At(im.Line(), 0, err)
		}
		return nil
	}
	dirs := FileDirs(file, c.mainFile)
	resolved, searched, ok := ResolveModulePath(path, dirs)
	abs := absPath(resolved)
	if ok && c.seen[abs] {
		return nil
	}
	if !ok {
		return messages.At(im.Line(), 0, ReadError(codes.NameRef(codes.KwImport), path, messages.H2112, ModulePathError(im.Path, dirs), searched))
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return messages.At(im.Line(), 0, ReadError(codes.NameRef(codes.KwImport), path, messages.H2112, err, searched))
	}
	c.seen[abs] = true
	c.files = append(c.files, resolved)
	checked := CheckModule(resolved, string(data))
	modSource := &Source{Path: resolved, Name: shownPath(resolved), Code: checked.Text}
	if checked.Stage != "" {
		return &ModuleError{Source: modSource, Kind: checked.Stage, Errors: checked.Errors,
			ModulePath: im.Path, ImportLine: im.Line(), Importer: importer}
	}
	return c.check(modSource, filepath.Clean(resolved), checked.Program)
}
