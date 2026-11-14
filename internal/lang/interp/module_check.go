package interp

import (
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Çalışmadan önce modül denetimi (1.3): programın aktardığı bütün modüller (özyinelemeli)
// çalışmadan önce ayrıştırılır ve CheckModule ile tip denetiminden geçer; hata varsa hiçbir satır
// çalışmaz. 'trlang check', 'trlang build' (cmd/trlang) ve çalıştıran her yol aynı CheckModule'ü
// kullanır; modül aktaranın adlarını görmez (5.14). Yürütme sırasında modül denetlenmiş ağacıyla
// çalışır (checked_modules.go); dosya denetimden sonra değiştiyse yalnız okunup ayrıştırılır
// (runModule), ikinci bir denetim yolu yoktur. Dilin gömülü kitaplığının aktarılan modülleri de
// (ve onların aktardıkları) aynı yoldan, süreç başına bir kez denetlenir: denetleyicinin ağaca
// yazdığı bilgi kitaplıkta da çalışsın (1.2).

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
	name, library := LibraryModule(path)
	if !library && importer != nil && importer.Embedded && !filepath.IsAbs(path) {
		// Kitaplık modülünün göreli aktarımı kitaplıkta çözülür (moduleTarget ile aynı).
		name, library = pathpkg.Join(pathpkg.Dir(strings.TrimPrefix(importer.Path, LibraryPrefix)), filepath.ToSlash(path)), true
	}
	if library {
		return c.library(importer, name, im)
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
	rememberCheckedModule(resolved, string(data), checked)
	return c.check(modSource, filepath.Clean(resolved), checked.Program)
}

// library dilin gömülü kitaplığındaki name modülünü denetler ve onun aktardıklarına iner. Olmayan
// kitaplık modülü çalışmadan önce bulunur (1.3). Kitaplığın metni süreç boyunca değişmediği için
// modül süreç başına bir kez denetlenir (checkedLibraryModule); kitaplık dosya değildir, files'a
// girmez.
func (c *moduleChecker) library(importer *Source, name string, im *ast.ImportStatement) error {
	key := LibraryPrefix + name
	if c.seen[key] {
		return nil
	}
	payload, err := ReadLibrary(name)
	if err != nil {
		return messages.At(im.Line(), 0, err)
	}
	c.seen[key] = true
	checked := checkedLibraryModule(name, string(payload))
	modSource := &Source{Path: key, Name: key, Code: checked.Text, Embedded: true}
	if checked.Stage != "" {
		return &ModuleError{Source: modSource, Kind: checked.Stage, Errors: checked.Errors,
			ModulePath: im.Path, ImportLine: im.Line(), Importer: importer}
	}
	return c.check(modSource, key, checked.Program)
}
