package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"regexp"
	"strings"
)

// Kod kayıtlarının paketleri (modül köküne göre) ve sabit adlarının kalıpları.
const (
	messagesDir = "internal/lang/messages"
	codesDir    = "internal/lang/codes"
)

var messageConst = regexp.MustCompile(`^H\d{4}$`)

// codeConstKeys modülün ad kodu paketindeki her sabitin dil dosyasındaki anahtarıdır (sabit adı → anahtar).
// Sabitin adı kimlikten türer, anahtarla aynı değildir (codes.FnPrint); üreteç (tools/gen/codeconsts) anahtarı
// sabitin satır yorumunun ilk sözcüğü olarak yazar (`FnPrint Code = ... // TR_fn_print: print · yazdır`). Yorumu böyle
// başlamayan sabit (aile tabanları, None) ad kodu değildir.
func codeConstKeys(root string) map[string]string {
	files, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(codesDir), "*.go"))
	keys := map[string]string{}
	for _, file := range files {
		if isTestFile(file) {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		for _, d := range f.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.CONST {
				continue
			}
			for _, spec := range g.Specs {
				vs := spec.(*ast.ValueSpec)
				if vs.Comment == nil || len(vs.Names) != 1 {
					continue
				}
				if key, _, ok := strings.Cut(vs.Comment.Text(), ":"); ok && key != "" && !strings.ContainsAny(key, " \t\n") {
					keys[vs.Names[0].Name] = key
				}
			}
		}
	}
	return keys
}

// viaArg bir çağrının argümanı olarak geçirilen paket düzeyi işlevin ya da somut alıcılı metot değerinin calls kenarındaki işaretidir (Edge.Via).
const viaArg = "arg"

func markCall(calls map[*ast.Ident]string, n *ast.CallExpr) {
	if id := nameIdent(n.Fun); id != nil {
		calls[id] = extracted
	}
	for _, a := range n.Args {
		if id := nameIdent(a); id != nil {
			calls[id] = viaArg
		}
	}
}

// scan root altındaki her tanımlayıcıyı çözer: işlev kullanımı `calls`, ileti kodu sabiti
// `uses_message`, ad kodu sabiti `uses_name` kenarı olur. Kenar owner'dan çıkar.
func (x *goExtractor) scan(fc *fileCtx, owner string, root ast.Node) {
	calls := map[*ast.Ident]string{}
	modes := map[ast.Expr]string{}
	closures := map[*ast.FuncLit]string{}
	writes := map[*ast.Ident]writeTarget{} // yazılan paket değişkenlerinin kök tanımlayıcıları (var_access.go)
	atomicAddr := map[*ast.UnaryExpr]bool{}
	var stack []ast.Node
	x.envStrings(fc, owner, root)
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		accessModes(modes, n)
		x.noteVarWrites(fc, writes, atomicAddr, n)
		x.noteSnippets(fc, owner, n)
		switch n := n.(type) {
		case *ast.CallExpr:
			markCall(calls, n)
			x.noteNameReads(fc, n)
			x.noteArgNameImpl(fc, n)
			x.envUse(fc, owner, n)
			x.pathCall(fc, owner, n)
			x.buildCall(fc, owner, n)
			x.stdlibCall(fc, owner, n)
			x.noteOptCall(fc, owner, n)
		case *ast.Ident:
			x.use(fc, owner, n, calls[n], writes, enclosingClosure(stack, closures), stack)
		case *ast.RangeStmt:
			x.rangeUse(fc, owner, root, n)
			x.noteRangeStrings(fc, n)
		case *ast.BasicLit:
			x.pathLit(fc, owner, n)
			x.noteTestText(fc, owner, n)
		case *ast.FuncLit:
			closures[n] = x.closureNode(fc, owner, n, stack, closures)
		case *ast.DeclStmt:
			if gd, ok := n.Decl.(*ast.GenDecl); ok && gd.Tok == token.TYPE {
				x.localTypeDecl(fc, owner, enclosingClosure(stack, closures), gd)
			}
		case *ast.SelectorExpr:
			x.fieldAccess(fc, owner, n, modes[n], enclosingClosure(stack, closures))
		case *ast.CompositeLit:
			x.literalFields(fc, owner, n, enclosingClosure(stack, closures))
			x.keyedImplementations(fc, owner, n)
			x.pathList(fc, owner, n)
			x.noteTextCompare(fc, owner, n)
		case *ast.CaseClause:
			x.noteTextCompare(fc, owner, n)
			x.noteCaseCalls(fc, owner, n)
		case *ast.BinaryExpr:
			x.noteTextCompare(fc, owner, n)
		}
		return true
	})
}

// enclosingClosure yığındaki en iç func literal'in düğüm kimliğidir (yoksa "").
func enclosingClosure(stack []ast.Node, closures map[*ast.FuncLit]string) string {
	for i := len(stack) - 1; i >= 0; i-- {
		if lit, ok := stack[i].(*ast.FuncLit); ok {
			return closures[lit]
		}
	}
	return ""
}

// fieldAccess alan seçicisini (x.f) tip bilgisiyle çözer ve ham erişim olarak biriktirir.
func (x *goExtractor) fieldAccess(fc *fileCtx, owner string, sel *ast.SelectorExpr, mode, closure string) {
	s := fc.u.Info.Selections[sel]
	if s == nil || s.Kind() != types.FieldVal {
		return
	}
	if mode == "" {
		mode = modeRead
	}
	x.fieldUses = append(x.fieldUses, rawFieldUse{posKey: x.posKey(s.Obj().Pos()), owner: owner,
		site: Site{File: fc.rel, Line: x.line(sel.Sel.Pos()), Mode: mode, Closure: closure}})
}

// literalFields T{f: v} bileşik değerindeki anahtarlı alanları yazma olarak biriktirir.
func (x *goExtractor) literalFields(fc *fileCtx, owner string, lit *ast.CompositeLit, closure string) {
	for _, e := range lit.Elts {
		kv, ok := e.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		if v, ok := fc.u.Info.Uses[key].(*types.Var); ok && v.IsField() {
			x.fieldUses = append(x.fieldUses, rawFieldUse{posKey: x.posKey(v.Pos()), owner: owner,
				site: Site{File: fc.rel, Line: x.line(key.Pos()), Mode: modeLiteral, Closure: closure}})
		}
	}
}

// keyedImplementations bileşik değerde ad kodunu anahtar, işlevi değer alan çiftin (`codes.CmdFormat: formatCommand`)
// işleve giden kenarına kodu yazar (Edge.Via): kodun uygulaması o çiftin değeridir, aynı satırdaki öbür çiftlerinki değil
// (implementationsAt). Kenar işlevin değer olarak kullanımıdır (calls, INFERRED; use ile aynı kenar).
func (x *goExtractor) keyedImplementations(fc *fileCtx, owner string, lit *ast.CompositeLit) {
	for _, e := range lit.Elts {
		kv, ok := e.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, value := nameIdent(kv.Key), nameIdent(kv.Value)
		if key == nil || value == nil {
			continue
		}
		c, isConst := fc.u.Info.Uses[key].(*types.Const)
		fn, isFunc := fc.u.Info.Uses[value].(*types.Func)
		if !isConst || !isFunc || c.Pkg() == nil || c.Pkg().Path() != x.m.Path+"/"+codesDir || x.codeKeys[c.Name()] == "" {
			continue
		}
		if ref := x.objectID(fn); ref != "" {
			x.b.addEdge(owner, ref, edgeCalls, inferred, fc.rel, x.line(value.Pos()))
			x.b.edges[edgeKey{owner, ref, edgeCalls}].Via = "name:" + x.codeKeys[c.Name()]
		}
	}
}

// nameIdent bir ifadenin adını verir: parantez, gösterici ve tip bağımsız değişkenleri atılır, `pkg.Ad`ta
// Ad alınır. Gömülü alanın tipi (*pkg.T, T[int]) ve çağrılan ifade (f, pkg.F, F[int], (f)) aynı kuralla
// çözülür.
func nameIdent(e ast.Expr) *ast.Ident {
	for {
		switch v := e.(type) {
		case *ast.ParenExpr:
			e = v.X
		case *ast.StarExpr:
			e = v.X
		case *ast.IndexExpr:
			e = v.X
		case *ast.IndexListExpr:
			e = v.X
		case *ast.SelectorExpr:
			return v.Sel
		case *ast.Ident:
			return v
		default:
			return nil
		}
	}
}

// objectID modülün paket düzeyindeki bir nesnesinin (işlev, metot, tür, sabit, değişken) düğüm kimliğidir;
// başka bir nesne (yerel, alan, standart kitaplık) için "". Kullanım kenarları ve tutarlılık testi aynı eşlemeyi
// kullanır: nesne türü başına ayrı bir kol yazılmaz, Info.Uses'taki her tanımlayıcı buradan geçer.
func (x *goExtractor) objectID(obj types.Object) string {
	if obj == nil || obj.Pkg() == nil || !x.inModule(obj.Pkg().Path()) {
		return ""
	}
	pkg := obj.Pkg()
	switch o := obj.(type) {
	case *types.Func:
		return funcRefIn(x.m.Fset, o.Origin())
	case *types.TypeName:
		return typeNameID(x.m.Fset, o)
	case *types.Const:
		if o.Parent() == pkg.Scope() {
			return valueID(kindConst, pkg.Path(), o.Name())
		}
	case *types.Var:
		if !o.IsField() && o.Parent() == pkg.Scope() {
			return valueID(kindVar, pkg.Path(), o.Name())
		}
	}
	return ""
}

// noteNameReads codes paketinin bir işlevine (ya da Language metoduna) argüman olarak verilen ad kodu sabitlerini
// işaretler: `codes.Spellings(codes.FnPrint)` kodun adını okur, yerleşiği uygulamaz (use: reads_name). Uygulayan
// işlev kodu kayıt tablosunda ya da bir switch'te anar; adın testi onun testidir, adını okuyanın değil (parti K2, C:
// kabuğun startsWithPrint'i yazdır'ın yazımlarını okuduğu için yazdır kullanan TR testleri cmd/trlang/main.go'nun testi
// görünüyordu).
func (x *goExtractor) noteNameReads(fc *fileCtx, c *ast.CallExpr) {
	fn, ok := fc.u.Info.Uses[nameIdent(c.Fun)].(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != x.m.Path+"/"+codesDir {
		return
	}
	for _, a := range c.Args {
		if id := nameIdent(a); id != nil {
			x.nameReads[id] = true
		}
	}
}

// nameUse bir ad kodunun kullanımıdır: codes paketinin bir işlevine argümansa adını okur (`reads_name`), değilse kodu
// kullanır (`uses_name`).
func (x *goExtractor) nameUse(owner, key string, id *ast.Ident, file string, line int) {
	kind := edgeUsesName
	if x.nameReads[id] {
		kind = edgeReadsName
	}
	x.b.addEdge(owner, "name:"+key, kind, extracted, file, line)
}

// codeAliasKeys modülde bir ad kodu sabitine eşitlenmiş sabitleri bulur: değeri yalnızca `codes.X` olan sabit bildirimi.
func (x *goExtractor) codeAliasKeys() map[string]string {
	out := map[string]string{}
	codesPkg := x.m.Path + "/" + codesDir
	for _, u := range x.m.Units {
		if u.PkgPath == codesPkg {
			continue
		}
		for _, f := range u.Files {
			for _, d := range f.Decls {
				gd, ok := d.(*ast.GenDecl)
				if !ok || gd.Tok != token.CONST {
					continue
				}
				for _, spec := range gd.Specs {
					vs := spec.(*ast.ValueSpec)
					for i, name := range vs.Names {
						if i >= len(vs.Values) {
							break
						}
						if sel, ok := vs.Values[i].(*ast.SelectorExpr); ok {
							if c, ok := u.Info.Uses[sel.Sel].(*types.Const); ok && c.Pkg().Path() == codesPkg && x.codeKeys[c.Name()] != "" {
								out[strings.TrimSuffix(u.PkgPath, "_test")+"."+name.Name] = x.codeKeys[c.Name()]
							}
						}
					}
				}
			}
		}
	}
	return out
}

func (x *goExtractor) use(fc *fileCtx, owner string, id *ast.Ident, callKind string, writes map[*ast.Ident]writeTarget, closure string, stack []ast.Node) {
	obj := fc.u.Info.Uses[id]
	ref := x.objectID(obj)
	if ref == "" {
		return
	}
	line := x.line(id.Pos())
	if _, isVar := obj.(*types.Var); isVar {
		x.varUse(fc, owner, ref, id, writes, closure)
		return
	}
	if fn, isFunc := obj.(*types.Func); isFunc {
		conf := inferred // işlev değeri olarak kullanım: çağrı sonradan olabilir
		if callKind == extracted {
			conf = extracted
		}
		x.b.addEdge(owner, ref, edgeCalls, conf, fc.rel, line)
		if callKind == viaArg && argCallable(fc, fn, stack[len(stack)-2]) {
			if e := x.b.edges[edgeKey{owner, ref, edgeCalls}]; e.Via == "" {
				e.Via = viaArg
			}
		}
		return
	}
	if c, isConst := obj.(*types.Const); isConst {
		if key := x.codeKey(fc, id); key != "" {
			x.nameUse(owner, key, id, fc.rel, line)
		} else if c.Pkg().Path() == x.m.Path+"/"+messagesDir && messageConst.MatchString(c.Name()) {
			x.b.addEdge(owner, "msg:"+c.Name(), edgeUsesMessage, extracted, fc.rel, line)
		}
	}
	if ref != owner && !selfTypeRef(fc, stack, obj) { // bir türün kendi adına başvurusu (özyineli alan) kullanım değildir
		x.b.addEdge(owner, ref, edgeUses, extracted, fc.rel, line)
	}
}

// codeKey tanımlayıcı bir ad kodu sabiti ya da onun takma sabitiyse dil dosyasındaki anahtarını verir (kendi paketindeki
// test dışı tanım tabloları hariç).
func (x *goExtractor) codeKey(fc *fileCtx, id *ast.Ident) string {
	c, ok := fc.u.Info.Uses[id].(*types.Const)
	if !ok || c.Pkg() == nil {
		return ""
	}
	if alias := x.codeAliases[c.Pkg().Path()+"."+c.Name()]; alias != "" {
		return alias
	}
	if (fc.isTest || c.Pkg().Path() != fc.base) && c.Pkg().Path() == x.m.Path+"/"+codesDir {
		return x.codeKeys[c.Name()]
	}
	return ""
}

type argNameImpl struct {
	fn, code, file string
	line           int
}

// noteArgNameImpl test dışı bir çağrının argümanlarında ve bir düzey iç çağrı argümanlarında tek ayrı bir ad kodu ile
// argüman olarak geçen tek bir işlev ya da somut alıcılı metot değeri varsa, işlevden koda INFERRED uses_name kenarı
// eklemek üzere kaydeder.
func (x *goExtractor) noteArgNameImpl(fc *fileCtx, c *ast.CallExpr) {
	if fc.isTest {
		return
	}
	var code, fnRef string
	var fnID *ast.Ident
	nCodes, nFuncs := 0, 0
	arg := func(a ast.Expr) {
		id := nameIdent(a)
		if id == nil {
			return
		}
		if !x.nameReads[id] {
			if k := x.codeKey(fc, id); k != "" && k != code {
				code = k
				nCodes++
			}
		}
		if fn, ok := fc.u.Info.Uses[id].(*types.Func); ok && argCallable(fc, fn, ast.Unparen(a)) {
			if ref := x.objectID(fn); ref != "" && ref != fnRef {
				fnRef, fnID = ref, id
				nFuncs++
			}
		}
	}
	for _, a := range c.Args {
		arg(a)
		if ic, ok := ast.Unparen(a).(*ast.CallExpr); ok {
			x.noteNameReads(fc, ic)
			for _, ia := range ic.Args {
				arg(ia)
			}
		}
	}
	if nCodes == 1 && nFuncs == 1 {
		x.argImpls = append(x.argImpls, argNameImpl{fn: fnRef, code: "name:" + code, file: fc.rel, line: x.line(fnID.Pos())})
	}
}

// argCallable çağrı argümanı olarak geçirilen işlevin ya da somut alıcılı metot değerinin/ifadesinin zincire girip girmediğidir.
func argCallable(fc *fileCtx, fn *types.Func, n ast.Node) bool {
	sig := fn.Type().(*types.Signature)
	if sig.Recv() == nil {
		return true
	}
	sel, ok := n.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	s := fc.u.Info.Selections[sel]
	return s != nil && (s.Kind() == types.MethodVal || s.Kind() == types.MethodExpr) && !types.IsInterface(s.Recv()) && !types.IsInterface(sig.Recv().Type())
}

func selfTypeRef(fc *fileCtx, stack []ast.Node, obj types.Object) bool {
	for i := len(stack) - 1; i >= 0; i-- {
		if ts, ok := stack[i].(*ast.TypeSpec); ok && fc.u.Info.Defs[ts.Name] == obj {
			return true
		}
	}
	return false
}

// implements modüldeki her somut tipten, onun uyduğu modül arayüzlerine `implements` kenarı
// çeker. Yalnızca test dışı tipler taranır (testsiz paket nesneleri tek tip evreni verir).
func (x *goExtractor) implements() {
	_, _, pairs := moduleImplementations(x.m)
	for _, p := range pairs {
		cfile, cline := x.declPos(p.concrete.Obj().Pos())
		x.b.addEdge(typeID(p.concrete.Obj().Pkg().Path(), p.concrete.Obj().Name()),
			typeID(p.iface.Obj().Pkg().Path(), p.iface.Obj().Name()), edgeImplements, extracted, cfile, cline)
		for _, pair := range implementingMethods(p) {
			from, to := funcRef(pair[0]), funcRef(pair[1].Origin())
			if from != "" && to != "" && from != to {
				file, line := x.declPos(pair[1].Pos())
				x.b.addEdge(from, to, edgeDispatches, inferred, file, line)
			}
		}
	}
}

// declPos bir bildirimin köke göre dosyası ve satırıdır.
func (x *goExtractor) declPos(p token.Pos) (string, int) {
	pos := x.m.Fset.Position(p)
	return x.m.Rel(pos.Filename), pos.Line
}
