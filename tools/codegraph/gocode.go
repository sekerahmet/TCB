package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// goIndex Go çıkarımının, sonradan çalışan çıkarıcıların (test ve kural bağları) kullandığı
// ara tablolarıdır.
type goIndex struct {
	pkgSyms    map[string]map[string]string // paket yolu -> "Ad" ya da "Tür.Metot" -> kimlik (test dosyası dışı)
	inTestFile map[string]bool              // test dosyasında tanımlı işlevlerin kimlikleri
	tests      []*testFunc                  // TestX/BenchmarkX/FuzzX/ExampleX işlevleri
	byName     map[string][]*testFunc
	dirFiles   map[string]bool // dosya düğümü olan göreli yollar
}

// testFunc bir test işlevidir.
type testFunc struct {
	id, name, file, pkg string
}

type goExtractor struct {
	m   *module
	b   *builder
	idx *goIndex
}

func pkgID(p string) string    { return "go:pkg:" + p }
func fileID(rel string) string { return "go:file:" + rel }
func typeID(p, name string) string {
	return "go:type:" + p + "." + name
}

// memberID bir işlev ya da metodun kimliğidir; recv boşsa paket düzeyi işlevdir.
func memberID(p, recv, name string) string {
	if recv != "" {
		name = recv + "." + name
	}
	return "go:func:" + p + "." + name
}

// recvName bir alıcı tipinin adını verir (gösterici ve tip parametreleri atılır); adsız tipte "".
func recvName(t types.Type) string {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := t.(*types.Named); ok {
		return n.Obj().Name()
	}
	return ""
}

// funcRef bir işlevin kimliğini verir; adsız arayüzlerin metotları için "".
func funcRef(f *types.Func) string {
	sig, ok := f.Type().(*types.Signature)
	if !ok || f.Pkg() == nil {
		return ""
	}
	recv := ""
	if sig.Recv() != nil {
		if recv = recvName(sig.Recv().Type()); recv == "" {
			return ""
		}
	}
	return memberID(f.Pkg().Path(), recv, f.Name())
}

func (x *goExtractor) inModule(p string) bool {
	return p == x.m.path || strings.HasPrefix(p, x.m.path+"/")
}

func (x *goExtractor) rel(abs string) string {
	r, err := filepath.Rel(x.m.root, abs)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(r)
}

// pkgName paketin sorguda yazılan adıdır: modül köküne göre yol.
func (x *goExtractor) pkgName(p string) string {
	if p == x.m.path {
		return path.Base(p)
	}
	return strings.TrimPrefix(p, x.m.path+"/")
}

// extractGo bütün modülün Go düğümlerini ve kenarlarını kurar.
func extractGo(m *module, b *builder) *goIndex {
	x := &goExtractor{m: m, b: b, idx: &goIndex{
		pkgSyms: map[string]map[string]string{}, inTestFile: map[string]bool{},
		byName: map[string][]*testFunc{}, dirFiles: map[string]bool{},
	}}
	for _, u := range m.units {
		x.unit(u)
	}
	x.implements()
	return x.idx
}

func (x *goExtractor) unit(u *unit) {
	base := u.pkgPath
	if _, ok := x.m.base[base]; !ok {
		base = strings.TrimSuffix(base, "_test")
	}
	x.b.addNode(Node{ID: pkgID(base), Kind: kindPackage, Name: x.pkgName(base), Detail: base})
	for _, f := range u.files {
		x.file(u, base, f)
	}
}

func (x *goExtractor) file(u *unit, base string, f *ast.File) {
	rel := x.rel(x.m.fset.Position(f.Pos()).Filename)
	isTest := strings.HasSuffix(rel, "_test.go")
	detail := ""
	switch {
	case ast.IsGenerated(f):
		detail = "generated"
	case isTest:
		detail = "test"
	}
	x.b.addNode(Node{ID: fileID(rel), Kind: kindFile, Name: rel, File: rel, Line: 1, Detail: detail})
	x.idx.dirFiles[rel] = true
	x.b.addEdge(pkgID(base), fileID(rel), edgeContains, extracted, rel, 0)
	for _, is := range f.Imports {
		if p, err := strconv.Unquote(is.Path.Value); err == nil && x.inModule(p) {
			x.b.addEdge(pkgID(base), pkgID(p), edgeImports, extracted, rel, x.m.fset.Position(is.Pos()).Line)
		}
	}
	fc := &fileCtx{u: u, base: base, rel: rel, isTest: isTest}
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			x.funcDecl(fc, d)
		case *ast.GenDecl:
			x.genDecl(fc, d)
		}
	}
}

// fileCtx üzerinde çalışılan dosyanın bağlamıdır.
type fileCtx struct {
	u      *unit
	base   string // sınanan paketin yolu (dış testlerde _test eki yok)
	rel    string
	isTest bool
}

func (x *goExtractor) line(p token.Pos) int { return x.m.fset.Position(p).Line }

func (x *goExtractor) addSym(base, key, id string) {
	if x.idx.pkgSyms[base] == nil {
		x.idx.pkgSyms[base] = map[string]string{}
	}
	x.idx.pkgSyms[base][key] = id
}

func (x *goExtractor) funcDecl(fc *fileCtx, d *ast.FuncDecl) {
	obj, _ := fc.u.info.Defs[d.Name].(*types.Func)
	if obj == nil {
		return
	}
	pkg := obj.Pkg().Path()
	recv := ""
	kind := kindFunc
	if d.Recv != nil {
		kind = kindMethod
		recv = recvName(obj.Type().(*types.Signature).Recv().Type())
	}
	name := d.Name.Name
	id := memberID(pkg, recv, name)
	if name == "init" || name == "_" {
		id += "#" + strconv.Itoa(x.line(d.Pos()))
	}
	qual := path.Base(pkg)
	if recv != "" {
		qual += "." + recv
	}
	x.b.addNode(Node{ID: id, Kind: kind, Name: qual + "." + name, File: fc.rel, Line: x.line(d.Pos()), End: x.line(d.End()),
		Detail: types.ObjectString(obj, func(p *types.Package) string { return p.Name() })})
	x.b.addEdge(fileID(fc.rel), id, edgeContains, extracted, fc.rel, x.line(d.Pos()))
	if recv != "" {
		x.b.addEdge(typeID(pkg, recv), id, edgeContains, extracted, fc.rel, x.line(d.Pos()))
	}
	if fc.isTest {
		x.idx.inTestFile[id] = true
		if recv == "" && isTestName(name) {
			t := &testFunc{id: id, name: name, file: fc.rel, pkg: fc.base}
			x.idx.tests = append(x.idx.tests, t)
			x.idx.byName[name] = append(x.idx.byName[name], t)
		}
	} else {
		key := name
		if recv != "" {
			key = recv + "." + name
		}
		x.addSym(fc.base, key, id)
	}
	if d.Body != nil {
		x.scan(fc, id, d.Body)
	}
}

// isTestName go test'in tanıdığı işlev adlarını ayırır: önek, ardından küçük harf olmayan.
func isTestName(name string) bool {
	for _, p := range []string{"Test", "Benchmark", "Fuzz", "Example"} {
		if strings.HasPrefix(name, p) {
			rest := name[len(p):]
			return rest == "" || !(rest[0] >= 'a' && rest[0] <= 'z')
		}
	}
	return false
}

func (x *goExtractor) genDecl(fc *fileCtx, d *ast.GenDecl) {
	if d.Tok != token.TYPE {
		if d.Tok == token.CONST || d.Tok == token.VAR {
			x.valueDecl(fc, d)
		}
		x.scan(fc, pkgID(fc.base), d)
		return
	}
	for _, spec := range d.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		obj, _ := fc.u.info.Defs[ts.Name].(*types.TypeName)
		if obj == nil {
			continue
		}
		x.typeSpec(fc, ts, obj)
	}
}

func (x *goExtractor) typeSpec(fc *fileCtx, ts *ast.TypeSpec, obj *types.TypeName) {
	pkg := obj.Pkg().Path()
	id := typeID(pkg, obj.Name())
	detail := "alias"
	if !obj.IsAlias() {
		switch obj.Type().Underlying().(type) {
		case *types.Struct:
			detail = "struct"
		case *types.Interface:
			detail = "interface"
		default:
			detail = types.TypeString(obj.Type().Underlying(), func(p *types.Package) string { return p.Name() })
		}
	}
	line := x.line(ts.Pos())
	x.b.addNode(Node{ID: id, Kind: kindType, Name: path.Base(pkg) + "." + obj.Name(), File: fc.rel, Line: line, End: x.line(ts.End()), Detail: detail})
	x.b.addEdge(fileID(fc.rel), id, edgeContains, extracted, fc.rel, line)
	if !fc.isTest {
		x.addSym(fc.base, obj.Name(), id)
	}
	it, ok := obj.Type().Underlying().(*types.Interface)
	if !ok || obj.IsAlias() {
		return
	}
	for i := 0; i < it.NumExplicitMethods(); i++ {
		mth := it.ExplicitMethod(i)
		mid := memberID(pkg, obj.Name(), mth.Name())
		ml := x.line(mth.Pos())
		x.b.addNode(Node{ID: mid, Kind: kindMethod, Name: path.Base(pkg) + "." + obj.Name() + "." + mth.Name(),
			File: fc.rel, Line: ml, End: ml, Detail: types.ObjectString(mth, func(p *types.Package) string { return p.Name() })})
		x.b.addEdge(id, mid, edgeContains, extracted, fc.rel, ml)
		if !fc.isTest {
			x.addSym(fc.base, obj.Name()+"."+mth.Name(), mid)
		}
	}
}

func valueID(kind, pkg, name string) string { return "go:" + kind + ":" + pkg + "." + name }

// valueDecl paket düzeyindeki const ve var bildirimlerini düğüm yapar. İleti kodu (messages.H2100)
// ve ad kodu (codes.K003) sabitleri, kodun düğümüne `denotes` kenarıyla bağlanır.
func (x *goExtractor) valueDecl(fc *fileCtx, d *ast.GenDecl) {
	kind := kindConst
	if d.Tok == token.VAR {
		kind = kindVar
	}
	for _, spec := range d.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, name := range vs.Names {
			obj := fc.u.info.Defs[name]
			if obj == nil || name.Name == "_" {
				continue
			}
			pkg := obj.Pkg().Path()
			id := valueID(kind, pkg, name.Name)
			detail := types.ObjectString(obj, func(p *types.Package) string { return p.Name() })
			if c, isConst := obj.(*types.Const); isConst {
				detail += " = " + truncate(c.Val().String(), 60)
			}
			line := x.line(name.Pos())
			x.b.addNode(Node{ID: id, Kind: kind, Name: path.Base(pkg) + "." + name.Name, File: fc.rel,
				Line: line, End: x.line(vs.End()), Detail: detail})
			x.b.addEdge(fileID(fc.rel), id, edgeContains, extracted, fc.rel, line)
			x.denotes(fc, id, pkg, name.Name, line)
		}
	}
}

// denotes sabitin bir ileti ya da ad kodunun Go adı olduğunu bağlar.
func (x *goExtractor) denotes(fc *fileCtx, id, pkg, name string, line int) {
	switch {
	case fc.isTest:
	case pkg == x.m.path+"/"+messagesDir && messageConst.MatchString(name):
		x.b.addEdge(id, "msg:"+name, edgeDenotes, extracted, fc.rel, line)
	case pkg == x.m.path+"/"+codesDir && nameConst.MatchString(name):
		x.b.addEdge(id, "name:"+name, edgeDenotes, extracted, fc.rel, line)
	}
}
