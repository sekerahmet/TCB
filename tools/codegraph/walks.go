package main

import (
	"go/ast"
	"go/types"

	"github.com/sekerahmet/TCB/internal/devtools/goload"
)

// Bir veri dosyasındaki bütün girdileri gezen test (`for _, k := range codes.Codes()`, `d := Source(); for _, k := range d.Codes()`)
// ya da bir dize koleksiyonunu (istisna çizelgesi, belgedeki sözcükler) sözlük aramasıyla denetleyen test her girdiyi korur;
// tek bir girdiyi arayan test (`codes.Name(k)`) etmez. Bu dosya test dosyalarındaki `range` deyimlerini ve sözlük türlerinin
// işlevlerini toplar, test_paths.go bunları gömülen dosyalara bağlar (`walks` kenarı: test → dosya).

// rangeRef bir range deyiminin gezdiği koleksiyonu üreten ya da gövdesinde sözlük araması yapan nesnelerdir.
type rangeRef struct {
	owner     string   // deyimi içeren işlevin kimliği
	ids       []string // gezilen koleksiyonu üreten işlev/değişken düğümleri
	lookupIDs []string // dize koleksiyonu döngüsünün gövdesinde çağrılan işlevler
	file      string
	line      int
}

// walkState liste gezintisi çözümünün paket/tür/döngü kayıtlarıdır.
type walkState struct {
	ranges      []rangeRef
	genVars     map[string]bool     // üretilmiş (`*_generated.go`) koleksiyon değişkenleri
	multiEmbed  map[string]bool     // aynı haritayı ilklendiren çoklu `//go:embed` değişkenleri (dil dosyaları)
	dictLookups map[string]bool     // dizeden paket türüne çeviren (Kod, bool|error) kütüphane işlevleri
	returnTypes map[string][]string // işlev kimliği -> aynı pakette döndürdüğü adlı tip kimlikleri
	recvType    map[string]string   // metot kimliği -> alıcı adlı tip kimliği
}

func newWalkState() *walkState {
	return &walkState{
		genVars:     map[string]bool{},
		multiEmbed:  map[string]bool{},
		dictLookups: map[string]bool{},
		returnTypes: map[string][]string{},
		recvType:    map[string]string{},
	}
}

// noteWalkFunc test dışı işlevin alıcı tipini, döndürdüğü aynı paket tiplerini ve sözlük araması imzasını kaydeder.
func (x *goExtractor) noteWalkFunc(id string, obj *types.Func) {
	sig, ok := obj.Type().(*types.Signature)
	if !ok || obj.Pkg() == nil {
		return
	}
	pkg := obj.Pkg().Path()
	w := x.idx.walks
	if sig.Recv() != nil {
		if r := goload.RecvName(sig.Recv().Type()); r != "" {
			w.recvType[id] = typeID(pkg, r)
		}
	}
	res := sig.Results()
	for i := 0; i < res.Len(); i++ {
		rt := res.At(i).Type()
		if p, isPtr := rt.(*types.Pointer); isPtr {
			rt = p.Elem()
		}
		if nt, isNamed := rt.(*types.Named); isNamed && nt.Obj().Pkg() != nil && nt.Obj().Pkg().Path() == pkg {
			w.returnTypes[id] = append(w.returnTypes[id], typeID(pkg, nt.Obj().Name()))
		}
	}
	if isDictLookupSig(sig, pkg) {
		w.dictLookups[id] = true
	}
}

// isDictLookupSig işlev dize alıp aynı paketteki bir adlı tipi `(T, bool)` ya da `(T, error)` olarak döndürüyor mu.
func isDictLookupSig(sig *types.Signature, pkg string) bool {
	if sig.Results().Len() != 2 {
		return false
	}
	r1 := sig.Results().At(1).Type().String()
	if r1 != "bool" && r1 != "error" {
		return false
	}
	nt, ok := sig.Results().At(0).Type().(*types.Named)
	if !ok || nt.Obj().Pkg() == nil || nt.Obj().Pkg().Path() != pkg {
		return false
	}
	for i := 0; i < sig.Params().Len(); i++ {
		if b, isBasic := sig.Params().At(i).Type().(*types.Basic); isBasic && b.Kind() == types.String {
			return true
		}
	}
	return false
}

// rangeUse range deyiminin gezdiği koleksiyonu üreten modül nesnelerini ve dize döngüsündeki sözlük aramalarını kaydeder.
func (x *goExtractor) rangeUse(fc *fileCtx, owner string, root ast.Node, rs *ast.RangeStmt) {
	if !fc.isTest { // kütüphanenin kendi döngüleri değil, testin döngüsü bütün listeyi gezmektir
		return
	}
	var ids, lookupIDs []string
	t := rangeExprType(fc, rs.X)
	if t != nil && isCollection(t) {
		x.collectRangeProducers(fc, root, rs.X, 3, &ids)
		if _, isLit := rs.X.(*ast.CompositeLit); !isLit {
			lookupIDs = x.rangeStringLookups(fc, rs)
		}
	}
	if len(ids) > 0 || len(lookupIDs) > 0 {
		x.idx.walks.ranges = append(x.idx.walks.ranges, rangeRef{
			owner: owner, ids: ids, lookupIDs: lookupIDs, file: fc.rel, line: x.line(rs.Pos()),
		})
	}
}

// rangeStringLookups dize döngüsü değişkenini doğrudan argüman olarak alan gövde çağrılarını toplar.
func (x *goExtractor) rangeStringLookups(fc *fileCtx, rs *ast.RangeStmt) []string {
	if rs.Body == nil {
		return nil
	}
	var loopVars []types.Object
	for _, lv := range []ast.Expr{rs.Key, rs.Value} {
		id, ok := lv.(*ast.Ident)
		if !ok || id.Name == "_" {
			continue
		}
		if obj := fc.u.Info.ObjectOf(id); obj != nil && isStringBasic(obj.Type()) {
			loopVars = append(loopVars, obj)
		}
	}
	if len(loopVars) == 0 {
		return nil
	}
	var out []string
	for _, stmt := range rs.Body.List {
		ast.Inspect(stmt, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.RangeStmt, *ast.FuncLit:
				return false
			case *ast.CallExpr:
				if id := nameIdent(n.Fun); id != nil && argsUseAny(fc, n.Args, loopVars) {
					if ref := x.objectID(fc.u.Info.Uses[id]); ref != "" {
						out = append(out, ref)
					}
				}
			}
			return true
		})
	}
	return out
}

func argsUseAny(fc *fileCtx, args []ast.Expr, objs []types.Object) bool {
	for _, a := range args {
		for _, o := range objs {
			if usesObj(fc, a, o) {
				return true
			}
		}
	}
	return false
}

// rangeExprType goload Types haritası kapalıyken bile range ifadesinin tipini nesne ve seçim bilgisinden çıkarır.
func rangeExprType(fc *fileCtx, e ast.Expr) types.Type {
	if e == nil {
		return nil
	}
	if fc.u.Info.Types != nil {
		if tv, ok := fc.u.Info.Types[e]; ok && tv.Type != nil {
			return tv.Type
		}
	}
	switch e := e.(type) {
	case *ast.ParenExpr:
		return rangeExprType(fc, e.X)
	case *ast.SliceExpr:
		return rangeExprType(fc, e.X)
	case *ast.StarExpr:
		if t := rangeExprType(fc, e.X); t != nil {
			if p, ok := t.Underlying().(*types.Pointer); ok {
				return p.Elem()
			}
		}
	case *ast.Ident:
		if obj := fc.u.Info.ObjectOf(e); obj != nil {
			return obj.Type()
		}
	case *ast.SelectorExpr:
		if sel := fc.u.Info.Selections[e]; sel != nil {
			return sel.Type()
		}
		if obj := fc.u.Info.Uses[e.Sel]; obj != nil {
			return obj.Type()
		}
	case *ast.IndexExpr:
		if t := rangeExprType(fc, e.X); t != nil {
			switch u := t.Underlying().(type) {
			case *types.Slice:
				return u.Elem()
			case *types.Array:
				return u.Elem()
			case *types.Map:
				return u.Elem()
			}
		}
	case *ast.CallExpr:
		if id := nameIdent(e.Fun); id != nil {
			if b, isB := fc.u.Info.Uses[id].(*types.Builtin); isB && b.Name() == "append" && len(e.Args) > 0 {
				return rangeExprType(fc, e.Args[0])
			}
		}
		if ft := rangeExprType(fc, e.Fun); ft != nil {
			if sig, ok := ft.Underlying().(*types.Signature); ok && sig.Results().Len() == 1 {
				return sig.Results().At(0).Type()
			}
		}
	case *ast.CompositeLit:
		return rangeExprType(fc, e.Type)
	case *ast.ArrayType:
		if elt := rangeExprType(fc, e.Elt); elt != nil {
			return types.NewSlice(elt)
		}
	case *ast.MapType:
		if k := rangeExprType(fc, e.Key); k != nil {
			return types.NewMap(k, types.Typ[types.Invalid])
		}
	}
	return nil
}

// collectRangeProducers gezilen koleksiyonu üreten zincirdeki modül işlevlerini ve değişkenlerini toplar.
func (x *goExtractor) collectRangeProducers(fc *fileCtx, root ast.Node, e ast.Expr, depth int, out *[]string) {
	if e == nil || depth < 0 {
		return
	}
	switch e := e.(type) {
	case *ast.ParenExpr:
		x.collectRangeProducers(fc, root, e.X, depth, out)
	case *ast.SliceExpr:
		x.collectRangeProducers(fc, root, e.X, depth, out)
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			if ref := x.objectID(fc.u.Info.Uses[sel.Sel]); ref != "" {
				*out = append(*out, ref)
				x.collectRangeProducers(fc, root, sel.X, depth, out)
			} else {
				x.collectRangeProducers(fc, root, sel.X, depth, out)
				for _, a := range e.Args {
					x.collectRangeProducers(fc, root, a, depth, out)
				}
			}
			return
		}
		if id := nameIdent(e.Fun); id != nil {
			obj := fc.u.Info.Uses[id]
			if ref := x.objectID(obj); ref != "" {
				*out = append(*out, ref)
			} else {
				for _, a := range e.Args {
					x.collectRangeProducers(fc, root, a, depth, out)
				}
			}
		}
	case *ast.SelectorExpr:
		if ref := x.objectID(fc.u.Info.Uses[e.Sel]); ref != "" {
			*out = append(*out, ref)
		}
		x.collectRangeProducers(fc, root, e.X, depth, out)
	case *ast.Ident:
		obj := fc.u.Info.Uses[e]
		if obj == nil {
			return
		}
		if ref := x.objectID(obj); ref != "" {
			if yieldsCollection(obj) || depth < 3 {
				*out = append(*out, ref)
			}
			return
		}
		if rhs, _ := singleLocalAssign(fc, root, obj); rhs != nil {
			x.collectRangeProducers(fc, root, rhs, depth-1, out)
		}
	}
}

// yieldsCollection nesne bir koleksiyon mu (değişken: tipi; işlev: sonuçlarından biri).
func yieldsCollection(obj types.Object) bool {
	switch o := obj.(type) {
	case *types.Var:
		return isCollection(o.Type())
	case *types.Func:
		res := o.Type().(*types.Signature).Results()
		for i := 0; i < res.Len(); i++ {
			if isCollection(res.At(i).Type()) {
				return true
			}
		}
	}
	return false
}

// isCollection tip dilim, dizi, dizi göstergesi, harita ya da yineleyici işlev mi.
func isCollection(t types.Type) bool {
	switch u := t.Underlying().(type) {
	case *types.Slice, *types.Array, *types.Map:
		return true
	case *types.Pointer:
		_, isArr := u.Elem().Underlying().(*types.Array)
		return isArr
	case *types.Signature:
		return u.Params().Len() == 1 && u.Results().Len() == 0 // iter.Seq benzeri: func(yield func(...) bool)
	}
	return false
}

func isStringBasic(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Kind() == types.String
}

// markMultiEmbed aynı paket değişkenini (ör. dil haritası `map[string][]byte{"a": aJSON, "b": bJSON}`)
// birlikte ilklendiren birden çok `//go:embed` değişkenini işaretler: üretilmiş kod tablosu (`*_generated.go`)
// tekil tanım dosyasına bağlanır, dil çeviri dosyalarına değil.
func (w *walkState) markMultiEmbed(refs []pathRef, reach *testReach, b *builder) {
	embeds := map[string]bool{}
	for _, r := range refs {
		if r.embed && r.prod && !reach.isTest[r.owner] {
			embeds[r.owner] = true
		}
	}
	byTarget := map[string][]string{}
	for owner := range embeds {
		for _, c := range reach.callers[owner] {
			if n := b.nodes[c]; n != nil && n.Kind == kindVar && !isTestFile(n.File) {
				byTarget[c] = append(byTarget[c], owner)
			}
		}
	}
	for _, owners := range byTarget {
		if len(owners) >= 2 {
			for _, owner := range owners {
				w.multiEmbed[owner] = true
			}
		}
	}
}

// walkersOf owner'dan paketin içinde yukarı doğru çıkan işlev/değişkenlerin (ve paketin üretilmiş koleksiyonlarının)
// ürettiği koleksiyonu gezen ya da dize koleksiyonunu sözlük aramasıyla sınayan range deyimleridir.
func walkersOf(owner string, w *walkState, reach *testReach, b *builder) []rangeRef {
	pkg := reach.pkgOf(owner)
	closure := map[string]bool{owner: true}
	queue := []string{owner}
	if !w.multiEmbed[owner] {
		for id := range w.genVars {
			if reach.pkgOf(id) == pkg && !closure[id] {
				closure[id] = true
				queue = append(queue, id)
			}
		}
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, c := range reach.callers[id] {
			if n := b.nodes[c]; closure[c] || n == nil || reach.pkgOf(c) != pkg || isTestFile(n.File) {
				continue
			}
			closure[c] = true
			queue = append(queue, c)
		}
	}
	retTypes := map[string]bool{}
	for id := range closure {
		for _, tid := range w.returnTypes[id] {
			retTypes[tid] = true
		}
	}
	for mid, tid := range w.recvType {
		if retTypes[tid] {
			if n := b.nodes[mid]; n != nil && !isTestFile(n.File) {
				closure[mid] = true
			}
		}
	}
	lookupSet := testLookupHelpers(closure, w.dictLookups, reach, b)
	var out []rangeRef
	for _, r := range w.ranges {
		if matchesAny(r.ids, closure) || matchesAny(r.lookupIDs, lookupSet) {
			out = append(out, r)
		}
	}
	return out
}

// testLookupHelpers kapanıştaki sözlük arama işlevlerini ve onları çağıran test yardımcısı işlevlerini verir.
func testLookupHelpers(closure, dictLookups map[string]bool, reach *testReach, b *builder) map[string]bool {
	set := map[string]bool{}
	var level []string
	for id := range closure {
		if dictLookups[id] {
			set[id] = true
			level = append(level, id)
		}
	}
	for depth := 0; depth < 3 && len(level) > 0; depth++ {
		var next []string
		for _, id := range level {
			for _, c := range reach.callers[id] {
				n := b.nodes[c]
				if set[c] || n == nil || !isTestFile(n.File) || reach.isTest[c] {
					continue
				}
				set[c] = true
				next = append(next, c)
			}
		}
		level = next
	}
	return set
}

func matchesAny(ids []string, set map[string]bool) bool {
	for _, id := range ids {
		if set[id] {
			return true
		}
	}
	return false
}
