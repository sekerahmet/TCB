package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// listed `go list -json` çıktısının aracın kullandığı kısmıdır.
type listed struct {
	ImportPath   string
	Dir          string
	GoFiles      []string
	TestGoFiles  []string
	XTestGoFiles []string
	Imports      []string
	Export       string
	Module       *struct{ Path string }
}

// unit birlikte tip denetimi yapılan dosya kümesidir: bir paket ile kendi içindeki testleri ya
// da dış testleri (pkgPath: sınanan paket + "_test").
type unit struct {
	pkgPath string
	files   []*ast.File
	info    *types.Info
	pkg     *types.Package
}

// module tip denetimi bitmiş bütün modüldür.
type module struct {
	path  string
	root  string
	fset  *token.FileSet
	units []*unit
	base  map[string]*types.Package // içe aktarma yolu -> testsiz paket
}

// load dir'deki modülün bütün paketlerini testleriyle birlikte yalnızca standart kitaplıkla tip
// denetiminden geçirir (tools/impact ile aynı yöntem): modül dışı bağımlılıkların dışa aktarım
// verisini `go list -export` verir.
func load(dir string) (*module, error) {
	pkgs, err := goList(dir)
	if err != nil {
		return nil, err
	}
	var own []*listed
	var outside []string
	modPath := ""
	for _, p := range pkgs {
		if strings.Contains(p.ImportPath, " ") || strings.HasSuffix(p.ImportPath, ".test") {
			continue
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
	// Dışa aktarım verisi (derleme) yalnızca modül dışı paketler için istenir: modülün kendi
	// paketleri kaynaktan denetlenir, onları derlemek soğuk önbellekte dakikalar tutar.
	exports, err := exportData(dir, outside)
	if err != nil {
		return nil, err
	}
	m := &module{path: modPath, root: dir, fset: token.NewFileSet(), base: map[string]*types.Package{}}
	ext := importer.ForCompiler(m.fset, "gc", func(path string) (io.ReadCloser, error) {
		if f, ok := exports[path]; ok {
			return os.Open(f)
		}
		return nil, fmt.Errorf("no export data for %s", path)
	})
	imp := importerFunc(func(path string) (*types.Package, error) {
		if b, ok := m.base[path]; ok {
			return b, nil
		}
		return ext.Import(path)
	})
	sorted := order(own)
	bases := map[string]*unit{}
	for _, p := range sorted {
		u, err := m.check(p.ImportPath, p.Dir, p.GoFiles, imp)
		if err != nil {
			return nil, err
		}
		m.base[p.ImportPath], bases[p.ImportPath] = u.pkg, u
	}
	for _, p := range sorted {
		if err := m.checkTests(p, bases[p.ImportPath], imp); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func goList(dir string) ([]*listed, error) {
	return runGoList(dir, "-e", "-json", "-deps", "-test", "./...")
}

// exportData modül dışı paketlerin derlenmiş dışa aktarım dosyalarını verir.
func exportData(dir string, paths []string) (map[string]string, error) {
	exports := map[string]string{}
	if len(paths) == 0 {
		return exports, nil
	}
	pkgs, err := runGoList(dir, append([]string{"-e", "-json", "-export"}, paths...)...)
	if err != nil {
		return nil, err
	}
	for _, p := range pkgs {
		if p.Export != "" {
			exports[p.ImportPath] = p.Export
		}
	}
	return exports, nil
}

func runGoList(dir string, args ...string) ([]*listed, error) {
	cmd := exec.Command("go", append([]string{"list"}, args...)...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go list: %v\n%s", err, errb.String())
	}
	var pkgs []*listed
	dec := json.NewDecoder(&out)
	for dec.More() {
		p := new(listed)
		if err := dec.Decode(p); err != nil {
			return nil, err
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}

func isUnder(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && !strings.HasPrefix(rel, "..")
}

// order modülün paketlerini, her biri içe aktardıklarından sonra gelecek biçimde sıralar.
func order(pkgs []*listed) []*listed {
	byPath := map[string]*listed{}
	for _, p := range pkgs {
		byPath[p.ImportPath] = p
	}
	done := map[string]bool{}
	var out []*listed
	var visit func(p *listed)
	visit = func(p *listed) {
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

// checkTests paketi iç testleriyle tek birim olarak, dış testleri ikinci birim olarak ekler.
func (m *module) checkTests(p *listed, base *unit, imp types.Importer) error {
	full := base
	if len(p.TestGoFiles) > 0 {
		var err error
		files := append(append([]string{}, p.GoFiles...), p.TestGoFiles...)
		if full, err = m.check(p.ImportPath, p.Dir, files, imp); err != nil {
			return err
		}
	}
	m.units = append(m.units, full)
	if len(p.XTestGoFiles) == 0 {
		return nil
	}
	x, err := m.check(p.ImportPath+"_test", p.Dir, p.XTestGoFiles, imp)
	if err != nil && full != base {
		withTests := importerFunc(func(path string) (*types.Package, error) {
			if path == p.ImportPath {
				return full.pkg, nil
			}
			return imp.Import(path)
		})
		x, err = m.check(p.ImportPath+"_test", p.Dir, p.XTestGoFiles, withTests)
	}
	if err != nil {
		return err
	}
	m.units = append(m.units, x)
	return nil
}

func (m *module) check(path, dir string, names []string, imp types.Importer) (*unit, error) {
	u := &unit{pkgPath: path, info: &types.Info{
		Defs: map[*ast.Ident]types.Object{},
		Uses: map[*ast.Ident]types.Object{},
	}}
	for _, n := range names {
		f, err := parser.ParseFile(m.fset, filepath.Join(dir, n), nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		u.files = append(u.files, f)
	}
	pkg, err := (&types.Config{Importer: imp}).Check(path, m.fset, u.files, u.info)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	u.pkg = pkg
	return u, nil
}

type importerFunc func(path string) (*types.Package, error)

func (f importerFunc) Import(path string) (*types.Package, error) { return f(path) }
