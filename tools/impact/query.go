package main

import (
	"fmt"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"
)

// declKey names a declaration by its file and offset: the same declaration seen from different
// type-checked units (a package and its test variant) has the same key.
type declKey struct {
	file string
	off  int
}

func (m *module) key(obj types.Object) declKey {
	p := m.fset.Position(obj.Pos())
	return declKey{p.Filename, p.Offset}
}

// rel shows a path relative to the module root, with forward slashes.
func (m *module) rel(file string) string {
	if r, err := filepath.Rel(m.root, file); err == nil {
		return filepath.ToSlash(r)
	}
	return file
}

// shortPath drops the module prefix from an import path: internal/lang/codes.
func (m *module) shortPath(path string) string {
	return strings.TrimPrefix(strings.TrimPrefix(path, m.path), "/")
}

// findPackage resolves a package given by its import path, its path inside the module or, if
// unique, its last element ("codes").
func (m *module) findPackage(spec string) (string, error) {
	var hits []string
	for path := range m.base {
		if path == spec || m.shortPath(path) == spec || pathBase(path) == spec {
			hits = append(hits, path)
		}
	}
	sort.Strings(hits)
	switch len(hits) {
	case 0:
		return "", fmt.Errorf("no package %q in the module", spec)
	case 1:
		return hits[0], nil
	}
	return "", fmt.Errorf("%q is ambiguous: %s", spec, strings.Join(hits, ", "))
}

func pathBase(p string) string { return p[strings.LastIndex(p, "/")+1:] }

// findObject resolves "pkg.Name" or "pkg.Type.Member" (fields and methods). pkg is anything
// findPackage accepts.
func (m *module) findObject(spec string) (types.Object, error) {
	// The package part may contain dots (github.com/...): try every split point.
	for i := len(spec) - 1; i > 0; i-- {
		if spec[i] != '.' {
			continue
		}
		path, err := m.findPackage(spec[:i])
		if err != nil {
			continue
		}
		return lookup(m.base[path], spec[i+1:])
	}
	return nil, fmt.Errorf("cannot resolve %q; write it as package.Name or package.Type.Member", spec)
}

func lookup(pkg *types.Package, names string) (types.Object, error) {
	name, member, hasMember := strings.Cut(names, ".")
	obj := pkg.Scope().Lookup(name)
	if obj == nil {
		return nil, fmt.Errorf("%s has no %s", pkg.Path(), name)
	}
	if !hasMember {
		return obj, nil
	}
	o, _, _ := types.LookupFieldOrMethod(obj.Type(), true, pkg, member)
	if o == nil {
		return nil, fmt.Errorf("%s.%s has no field or method %s", pkg.Name(), name, member)
	}
	return o, nil
}

// use is one place where a declaration is used.
type use struct {
	pkg  string // short import path of the using package (external tests end in _test)
	file string
	line int
}

// usesOf returns every use of obj in the module, sorted by package, file and line.
func (m *module) usesOf(obj types.Object) []use {
	want := m.key(obj)
	seen := map[token.Position]bool{}
	var out []use
	for _, u := range m.units {
		for id, o := range u.info.Uses {
			if o == nil || o.Pkg() == nil || m.key(o) != want {
				continue
			}
			p := m.fset.Position(id.Pos())
			if seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, use{m.shortPath(u.pkgPath), m.rel(p.Filename), p.Line})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.pkg != b.pkg {
			return a.pkg < b.pkg
		}
		if a.file != b.file {
			return a.file < b.file
		}
		return a.line < b.line
	})
	return out
}

// implementers lists the module's named types that implement iface (directly or through a
// pointer), with the file and line of their declaration.
func (m *module) implementers(iface *types.Interface) []string {
	var out []string
	for path, pkg := range m.base {
		for _, name := range pkg.Scope().Names() {
			tn, ok := pkg.Scope().Lookup(name).(*types.TypeName)
			if !ok || types.IsInterface(tn.Type()) {
				continue
			}
			if types.Implements(tn.Type(), iface) || types.Implements(types.NewPointer(tn.Type()), iface) {
				p := m.fset.Position(tn.Pos())
				out = append(out, fmt.Sprintf("%s.%s  %s:%d", pathBase(path), name, m.rel(p.Filename), p.Line))
			}
		}
	}
	sort.Strings(out)
	return out
}

// interfaceOf returns the interface obj declares or belongs to (an interface type, or a method
// of one), or nil.
func interfaceOf(obj types.Object) *types.Interface {
	if tn, ok := obj.(*types.TypeName); ok {
		if it, ok := tn.Type().Underlying().(*types.Interface); ok {
			return it
		}
	}
	if f, ok := obj.(*types.Func); ok {
		if recv := f.Type().(*types.Signature).Recv(); recv != nil {
			if it, ok := recv.Type().Underlying().(*types.Interface); ok {
				return it
			}
		}
	}
	return nil
}

// packageUsers maps each package that uses target (an import path) to the names it uses from
// it, with counts. In-package tests of target itself are left out.
func (m *module) packageUsers(target string) map[string]map[string]int {
	out := map[string]map[string]int{}
	for _, u := range m.units {
		if u.pkgPath == target {
			continue
		}
		for _, o := range u.info.Uses {
			if o == nil || o.Pkg() == nil || o.Pkg().Path() != target {
				continue
			}
			who := m.shortPath(u.pkgPath)
			if out[who] == nil {
				out[who] = map[string]int{}
			}
			out[who][objName(o)]++
		}
	}
	return out
}

// objName writes a package-level object as Name, a field or method as Type.Name.
func objName(o types.Object) string {
	switch o := o.(type) {
	case *types.Func:
		if recv := o.Type().(*types.Signature).Recv(); recv != nil {
			t := recv.Type()
			if p, ok := t.(*types.Pointer); ok {
				t = p.Elem()
			}
			if n, ok := t.(*types.Named); ok {
				return n.Obj().Name() + "." + o.Name()
			}
		}
	case *types.Var:
		if o.IsField() {
			return "(field)." + o.Name()
		}
	}
	return o.Name()
}
