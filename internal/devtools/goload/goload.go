// Package goload type-checks a whole Go module (every package with its tests) using only the
// standard library: `go list` gives the files and, for dependencies outside the module, the export
// data. tools/impact and tools/codegraph share it.
//
// Layer: developer tools (internal/devtools)
package goload

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Unit birlikte tip denetimi yapılan dosya kümesidir: bir paket, kendi içindeki testleriyle birlikte
// paket ya da dış testleri. Aynı bildirimin farklı birimlerdeki kullanımları bildirimin konumunu paylaşır.
type Unit struct {
	PkgPath string // dosyaların ait olduğu paket (dış testlerde: sınanan paket + "_test")
	Files   []*ast.File
	Info    *types.Info
	Pkg     *types.Package
}

// Module tip denetimi bitmiş bütün modüldür.
type Module struct {
	Path  string
	Root  string
	Fset  *token.FileSet
	Units []*Unit
	Base  map[string]*types.Package // içe aktarma yolu -> testsiz paket
}

// Options yüklemenin araçlara göre değişen ayarlarıdır.
type Options struct {
	Comments   bool // yorumları da ayrıştır (belge yorumlarını okuyan araç)
	Selections bool // types.Info.Selections'ı doldur (alan ve metot seçimlerini okuyan araç)
	Types      bool // types.Info.Types'ı doldur (ifadelerin tipleri ve sabit değerleri)
}

// Load dir'deki modülün bütün paketlerini testleriyle birlikte yalnızca standart kitaplıkla tip
// denetiminden geçirir. Dışa aktarım verisi (derleme) yalnızca modül dışı paketler için istenir:
// modülün kendi paketleri kaynaktan denetlenir, onları derlemek soğuk önbellekte dakikalar tutar.
func Load(dir string, opts Options) (*Module, error) {
	pkgs, err := goList(dir)
	if err != nil {
		return nil, err
	}
	var own []*Listed
	var outside []string
	modPath := ""
	for _, p := range pkgs {
		if strings.Contains(p.ImportPath, " ") || strings.HasSuffix(p.ImportPath, ".test") {
			continue // test değişkenleri ve test main'leri; dosya listeleri yalın girdidedir
		}
		if p.Module != nil && p.Dir != "" && isUnder(p.Dir, dir) {
			own = append(own, p)
			modPath = p.Module.Path
		} else {
			outside = append(outside, p.ImportPath)
		}
	}
	if modPath == "" {
		return nil, fmt.Errorf("no Go packages under %s", dir)
	}
	exports, err := exportData(dir, outside)
	if err != nil {
		return nil, err
	}
	m := &Module{Path: modPath, Root: dir, Fset: token.NewFileSet(), Base: map[string]*types.Package{}}
	ext := importer.ForCompiler(m.Fset, "gc", func(path string) (io.ReadCloser, error) {
		if f, ok := exports[path]; ok {
			return os.Open(f)
		}
		return nil, fmt.Errorf("no export data for %s", path)
	})
	imp := importerFunc(func(path string) (*types.Package, error) {
		if b, ok := m.Base[path]; ok {
			return b, nil
		}
		return ext.Import(path)
	})
	// Önce paketler (içe aktarma sırasıyla), sonra testler: bir test sırada sonra gelen bir paketi
	// içe aktarabilir.
	sorted := order(own)
	bases := map[string]*Unit{}
	for _, p := range sorted {
		u, err := m.check(p.ImportPath, p.Dir, p.GoFiles, imp, opts)
		if err != nil {
			return nil, err
		}
		m.Base[p.ImportPath], bases[p.ImportPath] = u.Pkg, u
	}
	for _, p := range sorted {
		if err := m.checkTests(p, bases[p.ImportPath], imp, opts); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func isUnder(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && !strings.HasPrefix(rel, "..")
}

// order modülün paketlerini, her biri içe aktardıklarından sonra gelecek biçimde sıralar.
func order(pkgs []*Listed) []*Listed {
	byPath := map[string]*Listed{}
	for _, p := range pkgs {
		byPath[p.ImportPath] = p
	}
	done := map[string]bool{}
	var out []*Listed
	var visit func(p *Listed)
	visit = func(p *Listed) {
		if done[p.ImportPath] {
			return
		}
		done[p.ImportPath] = true
		for _, imp := range p.Imports {
			if q, ok := byPath[imp]; ok {
				visit(q)
			}
		}
		out = append(out, p)
	}
	for _, p := range pkgs {
		visit(p)
	}
	return out
}

// checkTests paketi iç testleriyle tek birim olarak, dış testleri ikinci birim olarak ekler. Dış
// testler paketi başkaları gibi içe aktarır; iç testlerin yardımcılarına dayanıyorlarsa testli
// değişkene ihtiyaç duyarlar.
func (m *Module) checkTests(p *Listed, base *Unit, imp types.Importer, opts Options) error {
	full := base
	if len(p.TestGoFiles) > 0 {
		var err error
		files := append(append([]string{}, p.GoFiles...), p.TestGoFiles...)
		if full, err = m.check(p.ImportPath, p.Dir, files, imp, opts); err != nil {
			return err
		}
	}
	m.Units = append(m.Units, full) // paketin dış test olmayan bütün dosyalarını kapsar
	if len(p.XTestGoFiles) == 0 {
		return nil
	}
	x, err := m.check(p.ImportPath+"_test", p.Dir, p.XTestGoFiles, imp, opts)
	if err != nil && full != base {
		withTests := importerFunc(func(path string) (*types.Package, error) {
			if path == p.ImportPath {
				return full.Pkg, nil
			}
			return imp.Import(path)
		})
		x, err = m.check(p.ImportPath+"_test", p.Dir, p.XTestGoFiles, withTests, opts)
	}
	if err != nil {
		return err
	}
	m.Units = append(m.Units, x)
	return nil
}

func (m *Module) check(path, dir string, names []string, imp types.Importer, opts Options) (*Unit, error) {
	u := &Unit{PkgPath: path, Info: &types.Info{
		Defs: map[*ast.Ident]types.Object{},
		Uses: map[*ast.Ident]types.Object{},
	}}
	if opts.Selections {
		u.Info.Selections = map[*ast.SelectorExpr]*types.Selection{}
	}
	if opts.Types {
		u.Info.Types = map[ast.Expr]types.TypeAndValue{}
	}
	var mode parser.Mode
	if opts.Comments {
		mode = parser.ParseComments
	}
	for _, n := range names {
		f, err := parser.ParseFile(m.Fset, filepath.Join(dir, n), nil, mode)
		if err != nil {
			return nil, err
		}
		u.Files = append(u.Files, f)
	}
	pkg, err := (&types.Config{Importer: imp}).Check(path, m.Fset, u.Files, u.Info)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	u.Pkg = pkg
	return u, nil
}

type importerFunc func(path string) (*types.Package, error)

func (f importerFunc) Import(path string) (*types.Package, error) { return f(path) }

// Rel dosya yolunu modül köküne göre, '/' ile verir; göreli yazılamayan yol olduğu gibi (eğik çizgili) kalır.
func (m *Module) Rel(file string) string {
	r, err := filepath.Rel(m.Root, file)
	if err != nil {
		return filepath.ToSlash(file)
	}
	return filepath.ToSlash(r)
}

// RecvName bir alıcı tipinin adını verir (gösterici atılır); adsız tipte "".
func RecvName(t types.Type) string {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := t.(*types.Named); ok {
		return n.Obj().Name()
	}
	return ""
}
