package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"path"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/goload"
)

// goIndex Go çıkarımının, sonradan çalışan çıkarıcıların (test ve kural bağları) kullandığı
// ara tablolarıdır.
type goIndex struct {
	pkgSyms    map[string]map[string]string // paket yolu -> "Ad" ya da "Tür.Metot" -> kimlik (test dosyası dışı)
	inTestFile map[string]bool              // test dosyasında tanımlı işlevlerin kimlikleri
	tests      []*testFunc                  // TestX/BenchmarkX/FuzzX/ExampleX işlevleri
	byName     map[string][]*testFunc
	dirFiles   map[string]bool     // dosya düğümü olan göreli yollar
	pathRefs   []pathRef           // dosya yolu sabitleri ve gömme yönergeleri (test_paths.go)
	snip       *snippetState       // Go test dizelerindeki TR metinleri (tr_snippets.go)
	ranges     []rangeRef          // koleksiyon döndüren işlev/değişken üzerinde dönen range deyimleri (walks.go)
	envSets    map[string][]envSet // işlev -> koyduğu ortam değişkenleri (env_names.go)
	scans      *scanState          // depo genelini tarayan test çağrıları (scans.go)
}

// testFunc bir test işlevidir.
type testFunc struct {
	id, name, file, pkg string
}

type goExtractor struct {
	m         *module
	b         *builder
	idx       *goIndex
	fieldAt   map[string]string // alanın bildirim konumu -> alan düğümünün kimliği
	fieldUses []rawFieldUse     // çözülmeyi bekleyen alan erişimleri
	env       map[string]string // ortam değişkeni adı -> ortam değişkeninin ad kodu (env_names.go)
	codeKeys  map[string]string // ad kodu sabitinin adı -> dil dosyasındaki anahtarı (codeConstKeys)
	consumed  map[*ast.BasicLit]bool
	// rangeVals dize sabiti listesi üzerinde dönen döngü değişkenleri ve alabildikleri değerlerdir
	// (`for _, p := range []string{"docs/*.md", ...}`; test_paths.go: pathCall).
	rangeVals map[types.Object][]string
	// strConsts dışa açık dize sabitlerinin değerden kimliğe tablosu, textCompares değerin metinle anıldığı yerler
	// (value_compares.go).
	strConsts    map[string][]string
	strConstPkg  map[string]string // sabitin kimliği -> paketinin yolu
	textCompares []textCompare
	stdlibUses   []stdlibUse // standart kitaplığın arayüz ve yansımayla kullandığı değerler (stdlib_dispatch.go)
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

// funcRef bir işlevin kimliğini verir; adsız arayüzlerin metotları için "".
func funcRef(f *types.Func) string {
	sig, ok := f.Type().(*types.Signature)
	if !ok || f.Pkg() == nil {
		return ""
	}
	recv := ""
	if sig.Recv() != nil {
		if recv = goload.RecvName(sig.Recv().Type()); recv == "" {
			return ""
		}
	}
	return memberID(f.Pkg().Path(), recv, f.Name())
}

func (x *goExtractor) inModule(p string) bool {
	return p == x.m.Path || strings.HasPrefix(p, x.m.Path+"/")
}

// pkgName paketin sorguda yazılan adıdır: modül köküne göre yol.
func (x *goExtractor) pkgName(p string) string {
	if p == x.m.Path {
		return path.Base(p)
	}
	return strings.TrimPrefix(p, x.m.Path+"/")
}

// extractGo bütün modülün Go düğümlerini ve kenarlarını kurar.
func extractGo(m *module, b *builder) *goIndex {
	x := &goExtractor{m: m, b: b, idx: &goIndex{
		pkgSyms: map[string]map[string]string{}, inTestFile: map[string]bool{}, snip: newSnippetState(),
		byName: map[string][]*testFunc{}, dirFiles: map[string]bool{}, envSets: map[string][]envSet{},
		scans: newScanState(),
	}, fieldAt: map[string]string{}, env: envCodes(m.Root), codeKeys: codeConstKeys(m.Root), consumed: map[*ast.BasicLit]bool{}, rangeVals: map[types.Object][]string{},
		strConsts: map[string][]string{}, strConstPkg: map[string]string{}}
	for _, u := range m.Units {
		x.unit(u)
	}
	x.resolveFieldUses()
	x.resolveTextCompares()
	x.resolveStdlibUses()
	x.resolveScans()
	x.implements()
	x.testSkips()
	return x.idx
}

func (x *goExtractor) unit(u *unit) {
	base := u.PkgPath
	if _, ok := x.m.Base[base]; !ok {
		base = strings.TrimSuffix(base, "_test")
	}
	x.b.addNode(Node{ID: pkgID(base), Kind: kindPackage, Name: x.pkgName(base), Detail: base})
	for _, f := range u.Files {
		x.file(u, base, f)
	}
}

func (x *goExtractor) file(u *unit, base string, f *ast.File) {
	rel := x.m.Rel(x.m.Fset.Position(f.Pos()).Filename)
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
			x.b.addEdge(pkgID(base), pkgID(p), edgeImports, extracted, rel, x.m.Fset.Position(is.Pos()).Line)
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
	if isTest {
		x.citeRefs(fc, f)
		x.collectScans(fc, f)
	}
}

// fileCtx üzerinde çalışılan dosyanın bağlamıdır.
type fileCtx struct {
	u      *unit
	base   string // sınanan paketin yolu (dış testlerde _test eki yok)
	rel    string
	isTest bool
}

func (x *goExtractor) line(p token.Pos) int { return x.m.Fset.Position(p).Line }

func (x *goExtractor) addSym(base, key, id string) {
	if x.idx.pkgSyms[base] == nil {
		x.idx.pkgSyms[base] = map[string]string{}
	}
	x.idx.pkgSyms[base][key] = id
}

func (x *goExtractor) funcDecl(fc *fileCtx, d *ast.FuncDecl) {
	obj, _ := fc.u.Info.Defs[d.Name].(*types.Func)
	if obj == nil {
		return
	}
	pkg := obj.Pkg().Path()
	recv := ""
	kind := kindFunc
	if d.Recv != nil {
		kind = kindMethod
		recv = goload.RecvName(obj.Type().(*types.Signature).Recv().Type())
	}
	name := d.Name.Name
	id := memberID(pkg, recv, name)
	if name == "init" || name == "_" {
		id += "#" + path.Base(fc.rel) + ":" + strconv.Itoa(x.line(d.Pos())) // aynı satırdaki init'ler farklı dosyalarda çakışmasın
	}
	qual := path.Base(pkg)
	if recv != "" {
		qual += "." + recv
	}
	x.b.addNode(Node{ID: id, Kind: kind, Name: qual + "." + name, File: fc.rel, Line: x.line(d.Pos()), End: x.line(d.End()), Doc: docText(d.Doc),
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
	x.enterFunc(obj, id, strings.TrimPrefix(strings.TrimPrefix(pkg, x.m.Path), "/"), recv != "")
	x.scan(fc, id, d) // imza (alıcı, parametre, sonuç tipleri) ve gövde: tür kullanımı imzada da vardır
}

func (x *goExtractor) genDecl(fc *fileCtx, d *ast.GenDecl) {
	if d.Tok != token.TYPE {
		if d.Tok == token.CONST || d.Tok == token.VAR {
			x.valueDecl(fc, d)
		}
		x.embedRefs(fc, d)
		x.scanValueSpecs(fc, d)
		return
	}
	for _, spec := range d.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		obj, _ := fc.u.Info.Defs[ts.Name].(*types.TypeName)
		if obj == nil {
			continue
		}
		x.typeSpec(fc, d, ts, obj)
	}
}

func (x *goExtractor) typeSpec(fc *fileCtx, gen *ast.GenDecl, ts *ast.TypeSpec, obj *types.TypeName) {
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
	x.b.addNode(Node{ID: id, Kind: kindType, Name: path.Base(pkg) + "." + obj.Name(), File: fc.rel, Line: line, End: x.line(ts.End()), Detail: detail, Doc: docText(singleSpecDoc(gen, ts.Doc), ts.Comment)})
	x.b.addEdge(fileID(fc.rel), id, edgeContains, extracted, fc.rel, line)
	if !fc.isTest {
		x.addSym(fc.base, obj.Name(), id)
	}
	x.fieldNodes(fc, ts, id, pkg)
	x.scan(fc, id, ts) // alan tipleri, gömülü tipler, arayüz yöntem imzaları, tür kısıtları
	it, ok := obj.Type().Underlying().(*types.Interface)
	if !ok || obj.IsAlias() {
		return
	}
	docs := interfaceMethodDocs(ts)
	for i := 0; i < it.NumExplicitMethods(); i++ {
		mth := it.ExplicitMethod(i)
		mid := memberID(pkg, obj.Name(), mth.Name())
		ml := x.line(mth.Pos())
		x.b.addNode(Node{ID: mid, Kind: kindMethod, Name: path.Base(pkg) + "." + obj.Name() + "." + mth.Name(),
			File: fc.rel, Line: ml, End: ml, Doc: docs[mth.Name()], Detail: types.ObjectString(mth, func(p *types.Package) string { return p.Name() })})
		x.b.addEdge(id, mid, edgeContains, extracted, fc.rel, ml)
		if !fc.isTest {
			x.addSym(fc.base, obj.Name()+"."+mth.Name(), mid)
		}
	}
}

func valueID(kind, pkg, name string) string { return "go:" + kind + ":" + pkg + "." + name }

// valueDecl paket düzeyindeki const ve var bildirimlerini düğüm yapar. İleti kodu (messages.H2100)
// ve ad kodu (codes.CmdRun) sabitleri, kodun düğümüne `denotes` kenarıyla bağlanır.
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
			obj := fc.u.Info.Defs[name]
			if obj == nil || name.Name == "_" {
				continue
			}
			pkg := obj.Pkg().Path()
			id := valueID(kind, pkg, name.Name)
			detail := types.ObjectString(obj, func(p *types.Package) string { return p.Name() })
			if c, isConst := obj.(*types.Const); isConst {
				detail += " = " + truncate(c.Val().String(), 60)
				x.noteStringConst(fc, id, c)
			}
			line := x.line(name.Pos())
			x.b.addNode(Node{ID: id, Kind: kind, Name: path.Base(pkg) + "." + name.Name, File: fc.rel,
				Line: line, End: x.line(vs.End()), Detail: detail, Doc: docText(singleSpecDoc(d, vs.Doc), vs.Comment)})
			x.b.addEdge(fileID(fc.rel), id, edgeContains, extracted, fc.rel, line)
			x.denotes(fc, id, pkg, name.Name, line)
		}
	}
}

// denotes sabitin bir ileti ya da ad kodunun Go adı olduğunu bağlar.
func (x *goExtractor) denotes(fc *fileCtx, id, pkg, name string, line int) {
	switch {
	case fc.isTest:
	case pkg == x.m.Path+"/"+messagesDir && messageConst.MatchString(name):
		x.b.addEdge(id, "msg:"+name, edgeDenotes, extracted, fc.rel, line)
	case pkg == x.m.Path+"/"+codesDir && x.codeKeys[name] != "":
		x.b.addEdge(id, "name:"+x.codeKeys[name], edgeDenotes, extracted, fc.rel, line)
	}
}

// scanValueSpecs paket düzeyindeki const/var bildirimlerinin kullanımlarını bildirimin kendi düğümünden çıkarır
// (`var ValInt = TypeName(codes.TypeInt)`: ValInt TypeName'i ve TR_type_Int'ü kullanır). Hepsini paket düğümüne yazmak aynı
// hedefe giden farklı dosyalardaki kullanımları tek kenara indirirdi. Adı olmayan (`var _ = ...`) ya da çözülemeyen
// belirtim dosya düğümüne yazılır (dosyalar arasında aynı hedefe giden kullanımlar tek kenara inmesin).
func (x *goExtractor) scanValueSpecs(fc *fileCtx, d *ast.GenDecl) {
	for _, spec := range d.Specs {
		owner := fileID(fc.rel) // `var _ T = ...` gibi adsız belirtim dosyanın kendisinden kullanır
		if vs, ok := spec.(*ast.ValueSpec); ok {
			for _, name := range vs.Names {
				if obj := fc.u.Info.Defs[name]; obj != nil && name.Name != "_" && obj.Pkg() != nil {
					owner = valueID(map[token.Token]string{token.CONST: kindConst, token.VAR: kindVar}[d.Tok], obj.Pkg().Path(), name.Name)
					break
				}
			}
		}
		x.scan(fc, owner, spec)
		x.initializerSites(fc, d, spec)
	}
}

// initializerSites başlatıcılı bir paket değişkenini yazanın kendi bildirimi olarak işaretler: değişkenden kendine
// `inits_var` (kip "initializer"). `query codes.active` yazanları "başlatıcı" ve "init" ayrı grupta gösterir.
func (x *goExtractor) initializerSites(fc *fileCtx, d *ast.GenDecl, spec ast.Spec) {
	vs, ok := spec.(*ast.ValueSpec)
	if !ok || d.Tok != token.VAR || len(vs.Values) == 0 {
		return
	}
	for _, name := range vs.Names {
		if obj := fc.u.Info.Defs[name]; obj != nil && name.Name != "_" && obj.Pkg() != nil {
			id := valueID(kindVar, obj.Pkg().Path(), name.Name)
			x.b.addSite(id, id, edgeInitsVar, Site{File: fc.rel, Line: x.line(name.Pos()), Mode: "initializer"})
		}
	}
}
