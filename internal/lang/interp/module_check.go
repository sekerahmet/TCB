package interp

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Çalışmadan önce modül denetimi (1.3): programın aktardığı bütün modüller (özyinelemeli)
// çalışmadan önce ayrıştırılır ve CheckModule ile tip denetiminden geçer; hata varsa hiçbir satır
// çalışmaz. 'tr kontrol', 'tr derle' (cmd/tr) ve çalıştıran her yol aynı CheckModule'ü aynı
// genel adlarla (ana programın en dış adları) kullanır; yürütme sırasında modül yalnız okunup
// ayrıştırılır (runModule), ikinci bir denetim yolu yoktur. Dilin gömülü kitaplık modülleri
// derlemeyle gelir, denetlenmez.

// CheckImportedModules mainFile adlı prog'un aktardığı diskteki bütün modülleri çalıştırmadan
// denetler. İlk hatayı döndürür: denetim hatası için *ModuleError (aktar satırı ve modülün
// hataları), bulunamayan ya da okunamayan modül için o hatanın kendisi.
func CheckImportedModules(mainFile string, prog *ast.Program) error {
	names, dynamic := checker.TopLevelNames(prog)
	c := &moduleChecker{mainFile: mainFile, names: names, dynamic: dynamic, seen: map[string]bool{}}
	return c.check(nil, mainFile, prog)
}

// moduleChecker bir denetimin durumudur.
type moduleChecker struct {
	mainFile string
	names    []string
	dynamic  bool
	seen     map[string]bool
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
	if _, library := LibraryModule(path); library {
		return nil
	}
	resolved, searched, ok := ResolveModulePath(path, FileDirs(file, c.mainFile))
	abs := absPath(resolved)
	if ok && c.seen[abs] {
		return nil
	}
	data, err := os.ReadFile(resolved)
	if !ok || err != nil {
		if err == nil {
			err = os.ErrNotExist
		}
		return messages.At(im.Line(), 0, ReadError(codes.NameRef(codes.KwImport), path, messages.H2112, err, searched))
	}
	c.seen[abs] = true
	checked := CheckModule(string(data), c.names, c.dynamic)
	modSource := &Source{Path: resolved, Name: shownPath(resolved), Code: checked.Text, Language: checked.Language}
	if checked.Stage != "" {
		return &ModuleError{Source: modSource, Kind: checked.Stage, Errors: checked.Errors,
			ModulePath: im.Path, ImportLine: im.Line(), Importer: importer}
	}
	return c.check(modSource, filepath.Clean(resolved), checked.Program)
}
