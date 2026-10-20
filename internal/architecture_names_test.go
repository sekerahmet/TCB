package internal_test

import (
	"go/ast"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// goNameInDoc is a backticked Go name in the architecture document: pkg.Name, pkg.Type.Member
// or Type.Member, with an optional "()".
var goNameInDoc = regexp.MustCompile("`(" + `[A-Za-z_]\w*(?:\.[A-Za-z_]\w*){1,2}` + `)(?:\(\))?` + "`")

// fileExtensions are the dotted endings that make a backticked word a file name, not a Go name.
var fileExtensions = set("go md json sh tr yml py js s txt so dll npy trt")

// declIndex is every declared Go name of the repository: packages → top-level names, and
// types → members (methods, struct fields, interface methods).
type declIndex struct {
	pkgNames map[string]map[string]bool // package name → top-level names
	members  map[string]map[string]bool // type name → members
	embedded map[string]map[string]bool // type name → embedded type names (members are promoted)
}

func collectDecls(t *testing.T) declIndex {
	idx := declIndex{map[string]map[string]bool{}, map[string]map[string]bool{}, map[string]map[string]bool{}}
	add := func(m map[string]map[string]bool, k, v string) {
		if m[k] == nil {
			m[k] = map[string]bool{}
		}
		m[k][v] = true
	}
	for _, d := range goFiles(t) {
		if d.test {
			continue
		}
		pkg := d.file.Name.Name
		add(idx.pkgNames, pkg, "")
		for _, decl := range d.file.Decls {
			switch dd := decl.(type) {
			case *ast.FuncDecl:
				if dd.Recv == nil {
					add(idx.pkgNames, pkg, dd.Name.Name)
				} else if name, _, _ := strings.Cut(funcName(dd), "."); name != "" {
					add(idx.members, name, dd.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range dd.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						add(idx.pkgNames, pkg, s.Name.Name)
						collectMembers(s, func(m string) { add(idx.members, s.Name.Name, m) },
							func(e string) { add(idx.embedded, s.Name.Name, e) })
					case *ast.ValueSpec:
						for _, n := range s.Names {
							add(idx.pkgNames, pkg, n.Name)
						}
					}
				}
			}
		}
	}
	return idx
}

// collectMembers calls add for each field of a struct type and each method of an interface, and embed for each embedded type.
func collectMembers(s *ast.TypeSpec, add, embed func(string)) {
	var fields *ast.FieldList
	switch ty := s.Type.(type) {
	case *ast.StructType:
		fields = ty.Fields
	case *ast.InterfaceType:
		fields = ty.Methods
	}
	if fields == nil {
		return
	}
	for _, f := range fields.List {
		if len(f.Names) == 0 {
			embed(embeddedTypeName(f.Type))
		}
		for _, n := range f.Names {
			add(n.Name)
		}
	}
}

// Every Go name the architecture document writes in backticks (pkg.Name, pkg.Type.Method,
// Type.Method) exists in the repository: a renamed or deleted name must be fixed in the
// document, so it never describes code that is gone.
func TestArchitectureGoNamesExist(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(reporoot.Root(), "docs", "architecture.md"))
	if err != nil {
		t.Fatal(err)
	}
	idx := collectDecls(t)
	missing := map[string]bool{}
	checked := 0
	for _, m := range goNameInDoc.FindAllStringSubmatch(string(data), -1) {
		parts := strings.Split(m[1], ".")
		if fileExtensions[parts[len(parts)-1]] {
			continue
		}
		var typeName, member string
		switch {
		case idx.pkgNames[parts[0]] != nil && len(parts) == 2:
			checked++
			if !idx.pkgNames[parts[0]][parts[1]] {
				missing[m[1]] = true
			}
			continue
		case idx.pkgNames[parts[0]] != nil:
			if !idx.pkgNames[parts[0]][parts[1]] {
				checked++
				missing[m[1]] = true
				continue
			}
			typeName, member = parts[1], parts[2]
		case len(parts) == 2 && parts[0][0] >= 'A' && parts[0][0] <= 'Z':
			typeName, member = parts[0], parts[1]
		default:
			continue // not a Go name (cp.async, a file, ...)
		}
		checked++
		if !idx.hasMember(typeName, member, map[string]bool{}) {
			missing[m[1]] = true
		}
	}
	if checked < 50 {
		t.Errorf("only %d Go names checked; the document or the name pattern changed", checked)
	}
	var list []string
	for k := range missing {
		list = append(list, k)
	}
	sort.Strings(list)
	for _, k := range list {
		t.Errorf("docs/architecture.md: `%s` does not exist in the code; fix or remove the name", k)
	}
}

// embeddedTypeName is the type name of an embedded field (T, *T or pkg.T).
func embeddedTypeName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.StarExpr:
		return embeddedTypeName(x.X)
	case *ast.SelectorExpr:
		return x.Sel.Name
	case *ast.Ident:
		return x.Name
	}
	return ""
}

// hasMember reports whether the type has the member, directly or promoted from an embedded type.
func (idx declIndex) hasMember(typeName, member string, seen map[string]bool) bool {
	if seen[typeName] {
		return false
	}
	seen[typeName] = true
	if idx.members[typeName][member] {
		return true
	}
	for e := range idx.embedded[typeName] {
		if idx.hasMember(e, member, seen) {
			return true
		}
	}
	return false
}
