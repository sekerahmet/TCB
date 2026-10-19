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

// listed is the part of `go list -json` output the tool uses.
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

// unit is one type-checked set of files: a package, its in-package tests, or its external
// tests. Uses of the same declaration from different units share the declaration's position.
type unit struct {
	pkgPath string // the package the files belong to (external tests: the tested package + "_test")
	files   []*ast.File
	info    *types.Info
	pkg     *types.Package
}

// module is the whole type-checked module.
type module struct {
	path  string
	root  string
	fset  *token.FileSet
	units []*unit
	base  map[string]*types.Package // import path -> package without tests
}

// load type-checks every package of the module in dir, with its tests, using only the
// standard library: `go list` gives files and export data of dependencies outside the module.
func load(dir string) (*module, error) {
	pkgs, err := goList(dir)
	if err != nil {
		return nil, err
	}
	exports := map[string]string{}
	var own []*listed
	modPath := ""
	for _, p := range pkgs {
		if strings.Contains(p.ImportPath, " ") || strings.HasSuffix(p.ImportPath, ".test") {
			continue // test variants and test mains; the plain entry has the file lists
		}
		if p.Module != nil && p.Dir != "" && isUnder(p.Dir, dir) {
			own = append(own, p)
			modPath = p.Module.Path
		} else if p.Export != "" {
			exports[p.ImportPath] = p.Export
		}
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
	// Packages first (in import order), then tests: a test may import a package that comes
	// later in the order.
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
	cmd := exec.Command("go", "list", "-e", "-json", "-export", "-deps", "-test", "./...")
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

// order sorts the module's packages so that every package comes after the ones it imports.
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

// checkTests adds p's files to the module as one unit: the package with its in-package tests
// (or the package alone when it has none), then its external tests as a second unit.
func (m *module) checkTests(p *listed, base *unit, imp types.Importer) error {
	full := base
	if len(p.TestGoFiles) > 0 {
		var err error
		files := append(append([]string{}, p.GoFiles...), p.TestGoFiles...)
		if full, err = m.check(p.ImportPath, p.Dir, files, imp); err != nil {
			return err
		}
	}
	m.units = append(m.units, full) // covers every non-external file of the package
	if len(p.XTestGoFiles) == 0 {
		return nil
	}
	// External tests import the package as other packages do. If they rely on helpers from
	// the in-package tests, they need the variant with tests instead.
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
		f, err := parser.ParseFile(m.fset, filepath.Join(dir, n), nil, 0)
		if err != nil {
			return nil, err
		}
		u.files = append(u.files, f)
	}
	conf := types.Config{Importer: imp}
	pkg, err := conf.Check(path, m.fset, u.files, u.info)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	u.pkg = pkg
	return u, nil
}

type importerFunc func(path string) (*types.Package, error)

func (f importerFunc) Import(path string) (*types.Package, error) { return f(path) }
