package interp

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kancalar: yorumlayıcıya bir arayüzle bağlanan her tip derleme anı denetimi taşır
// (`var _ interp.X = (*T)(nil)`). Yorumlayıcı kancaları tip denetimiyle bulur; adı değişen bir
// metot bu denetim olmadan kancayı sessizce koparır. Test, interp'in dışa açık kanca
// arayüzlerini ve depodaki tiplerin metot imzalarını Go sözdizimi ağacından okur; bir arayüzün
// bütün metotlarını (ad ve imza) taşıyan her tip için aynı paketinde bir denetim arar.

// notHooks kanca olmayan arayüzlerdir: her değer tipi uygular, kancayla bağlanmaz.
var notHooks = map[string]bool{"Value": true}

// hookMethod bir arayüz metodunun adı ve imzası (parametre adları olmadan).
type hookMethod struct{ name, sig string }

// hookTypeInfo bir tipin yöntem kümesi (alıcı türünden bağımsız) ve bulunduğu paket klasörü.
type hookTypeInfo struct {
	dir     string
	methods map[string]string // ad → imza
}

func TestHooksHaveCompileTimeChecks(t *testing.T) {
	root := reporoot.Root()
	hooks := hookInterfaces(t, filepath.Join(root, "internal", "lang", "interp"))
	if len(hooks) < 10 {
		t.Fatalf("kanca arayüzü az bulundu: %d", len(hooks))
	}
	types, checked := scanHookTypes(t, root)
	var missing []string
	for tname, ti := range types {
		for hname, ms := range hooks {
			if hookImplements(ti, ms) && !checked[ti.dir+"|"+tname.name+"|"+hname] {
				missing = append(missing, tname.dir+": "+tname.name+" "+hname+" arayüzünü uyguluyor ama `var _ interp."+hname+" = (*"+tname.name+")(nil)` yok")
			}
		}
	}
	sort.Strings(missing)
	for _, m := range missing {
		t.Error(m)
	}
}

// hookTypeKey bir tipin paket klasörü ve adıdır.
type hookTypeKey struct{ dir, name string }

func hookImplements(ti hookTypeInfo, ms []hookMethod) bool {
	for _, m := range ms {
		if got, ok := ti.methods[m.name]; !ok || got != m.sig {
			return false
		}
	}
	return len(ms) > 0
}

// hookInterfaces interp paketinin dışa açık arayüzlerini (kancaları) ve metotlarını okur.
func hookInterfaces(t *testing.T, dir string) map[string][]hookMethod {
	t.Helper()
	out := map[string][]hookMethod{}
	for _, f := range parseNonTestDir(t, dir) {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, s := range gd.Specs {
				ts, ok := s.(*ast.TypeSpec)
				if !ok || !ts.Name.IsExported() || notHooks[ts.Name.Name] {
					continue
				}
				it, ok := ts.Type.(*ast.InterfaceType)
				if !ok {
					continue
				}
				for _, m := range it.Methods.List {
					if ft, ok := m.Type.(*ast.FuncType); ok && len(m.Names) == 1 {
						out[ts.Name.Name] = append(out[ts.Name.Name], hookMethod{m.Names[0].Name, hookSignature(ft)})
					}
				}
			}
		}
	}
	return out
}

// scanPackages depodaki test dışı Go dosyalarından tiplerin metotlarını ve var _ denetimlerini
// ("klasör|tip|arayüz" anahtarlı) toplar.
func scanHookTypes(t *testing.T, root string) (map[hookTypeKey]hookTypeInfo, map[string]bool) {
	t.Helper()
	types := map[hookTypeKey]hookTypeInfo{}
	checked := map[string]bool{}
	for _, top := range []string{"internal", "cmd"} {
		err := reporoot.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return err
			}
			for _, f := range parseNonTestDir(t, path) {
				collectTypes(f, path, types, checked)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return types, checked
}

func collectTypes(f *ast.File, dir string, types map[hookTypeKey]hookTypeInfo, checked map[string]bool) {
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil || len(d.Recv.List) == 0 || !d.Name.IsExported() {
				continue
			}
			name := hookTypeName(d.Recv.List[0].Type)
			k := hookTypeKey{dir, name}
			ti, ok := types[k]
			if !ok {
				ti = hookTypeInfo{dir: dir, methods: map[string]string{}}
			}
			ti.methods[d.Name.Name] = hookSignature(d.Type)
			types[k] = ti
		case *ast.GenDecl:
			for _, s := range d.Specs {
				vs, ok := s.(*ast.ValueSpec)
				if !ok || len(vs.Names) != 1 || vs.Names[0].Name != "_" || vs.Type == nil || len(vs.Values) != 1 {
					continue
				}
				if sel, ok := vs.Type.(*ast.SelectorExpr); ok {
					checked[dir+"|"+hookValueTypeName(vs.Values[0])+"|"+sel.Sel.Name] = true
				} else if id, ok := vs.Type.(*ast.Ident); ok {
					checked[dir+"|"+hookValueTypeName(vs.Values[0])+"|"+id.Name] = true // interp'in kendi paketinde
				}
			}
		}
	}
}

// valueTypeName `(*T)(nil)` ya da `T{}` ifadesindeki T'yi verir.
func hookValueTypeName(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.CallExpr:
		if p, ok := v.Fun.(*ast.ParenExpr); ok {
			return hookTypeName(p.X)
		}
	case *ast.CompositeLit:
		return hookTypeName(v.Type)
	}
	return ""
}

func hookTypeName(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.StarExpr:
		return hookTypeName(v.X)
	case *ast.Ident:
		return v.Name
	case *ast.IndexExpr: // üretken tip
		return hookTypeName(v.X)
	}
	return ""
}

// signature bir fonksiyon tipinin parametre ve sonuç tiplerini adsız, "interp." öneki atılmış
// olarak yazar; yorumlayıcının içindeki ve dışındaki yazım aynı imzayı verir.
func hookSignature(ft *ast.FuncType) string {
	list := func(fl *ast.FieldList) string {
		if fl == nil {
			return ""
		}
		var parts []string
		for _, f := range fl.List {
			var b strings.Builder
			_ = printer.Fprint(&b, token.NewFileSet(), f.Type)
			n := len(f.Names)
			if n == 0 {
				n = 1
			}
			for i := 0; i < n; i++ {
				parts = append(parts, strings.ReplaceAll(b.String(), "interp.", ""))
			}
		}
		return strings.Join(parts, ",")
	}
	return "(" + list(ft.Params) + ")(" + list(ft.Results) + ")"
}

// parseDir klasördeki test dışı Go dosyalarını ayrıştırır (alt klasörlere inmez).
func parseNonTestDir(t *testing.T, dir string) []*ast.File {
	t.Helper()
	entries, err := reporoot.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, p := range entries {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	return files
}
