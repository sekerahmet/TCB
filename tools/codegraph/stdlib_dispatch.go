package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"strings"
)

// Bu dosya standart kitaplığın arayüz ve yansıma üzerinden yaptığı çağrılar: bir değer `fmt.Sprintf("%s", k)`'ye,
// `json.MarshalIndent(snap)`'e, `sort.Sort(byLen(x))`'e verildiğinde k.String(), snap'in alanları ve byLen.Less çağrılır
// ama kaynakta çağrı yoktur; who-uses onları göstermez, liste eksiksiz görünür (eksikler listesi, kritik 3: adım 0'da
// codes.Code.String'in çıktısı değişiyor, profiler.Snapshot.Start yalnız JSON ile yazılıyor). Değerin kullanım rolü
// kenarın türüdür ("kullanım yerinin rolü" isteğinin görüntü rolü buradan çıkar):
//   - formats: fmt, log, testing'in biçimleyicileri ve modülün ileti paketi `any` argümanın String, Error, Format,
//     GoString metodunu çağırır;
//   - encodes: encoding/json `any` argümanın dışa açık alanlarını (iç içe yapılar dahil, `json:"-"` hariç) ve
//     MarshalJSON, UnmarshalJSON, MarshalText, UnmarshalText metotlarını kullanır;
//   - stdlib_calls: boş olmayan bir standart kitaplık arayüzü tipli parametreye (io.Writer, sort.Interface) verilen
//     değerin o arayüzü karşılayan metotları.
// Kenarlar INFERRED'dır (çağrı çalışma anında olur, hangi yolda olduğu bilinmez). Argümanın tipi tip bilgisi
// yüklenmeden (Info.Types yok, grafik kurulumu %17 uzardı) tanımlayıcı, seçici, bileşik değer, dönüşüm ve çağrı
// sonucundan çözülür; çözülemeyen argüman atlanır. Kapsamı tam go/types ile kuran test korur
// (TestEveryStandardLibraryDispatchHasAnEdge). Aynı metot adları dead-methods'un dış sözleşme listesine girer.

// formatMethods ve encodeMethods standart kitaplığın `any` değerde dinamik çağırdığı metotlardır.
var (
	formatMethods = []string{"String", "Error", "Format", "GoString"}
	encodeMethods = []string{"MarshalJSON", "UnmarshalJSON", "MarshalText", "UnmarshalText"}
)

// formattingCallee çağrılanın `any` argümanını biçimleyen bir işlev olup olmadığıdır: fmt ve log'un her işlevi,
// testing'in Log/Error/Fatal/Skip aileleri ve modülün ileti paketi (iletinin argümanları metne biçimlenir).
func (x *goExtractor) formattingCallee(fn *types.Func) bool {
	switch p := fn.Pkg().Path(); {
	case p == "fmt", p == "log":
		return true
	case p == "testing":
		for _, pre := range []string{"Log", "Error", "Fatal", "Skip"} {
			if strings.HasPrefix(fn.Name(), pre) {
				return true
			}
		}
	case p == x.m.Path+"/"+messagesDir:
		return true
	}
	return false
}

// encodingCallee encoding/json'un değeri yansımayla okuyan ya da yazan işlevleridir.
func encodingCallee(fn *types.Func) bool {
	if fn.Pkg().Path() != "encoding/json" {
		return false
	}
	switch fn.Name() {
	case "Marshal", "MarshalIndent", "Unmarshal", "Encode", "Decode":
		return true
	}
	return false
}

// stdlibUse çözülmeyi bekleyen bir rol kullanımıdır (alan düğümleri bütün paketler okununca bilinir).
type stdlibUse struct {
	owner, kind, file string
	line              int
	target            types.Type    // değerin tipi (encodes: alanları ve metotları)
	methods           []*types.Func // formats ve stdlib_calls: çağrılan metotlar
}

// stdlibCall bir çağrının standart kitaplığa ya da ileti paketine verdiği modül değerlerini rolleriyle kaydeder.
func (x *goExtractor) stdlibCall(fc *fileCtx, owner string, call *ast.CallExpr) {
	id := nameIdent(call.Fun)
	fn, ok := fc.u.Info.Uses[id].(*types.Func)
	if !ok || fn.Pkg() == nil || x.inModule(fn.Pkg().Path()) && !x.formattingCallee(fn) {
		return
	}
	sig := fn.Type().(*types.Signature)
	for i, arg := range call.Args {
		pt := paramType(sig, i, call.Ellipsis.IsValid())
		if pt == nil {
			continue
		}
		it, isIface := pt.Underlying().(*types.Interface)
		if !isIface {
			continue
		}
		t := x.argType(fc, arg)
		if t == nil || !x.moduleNamed(t) {
			continue
		}
		use := stdlibUse{owner: owner, file: fc.rel, line: x.line(arg.Pos()), target: t}
		switch {
		case it.Empty() && x.formattingCallee(fn):
			use.kind, use.methods = edgeFormats, namedMethods(t, formatMethods)
		case it.Empty() && encodingCallee(fn):
			use.kind, use.methods = edgeEncodes, namedMethods(t, encodeMethods)
		case !it.Empty() && !x.inModule(fn.Pkg().Path()):
			use.kind = edgeStdlibCalls
			for k := 0; k < it.NumMethods(); k++ {
				use.methods = append(use.methods, namedMethods(t, []string{it.Method(k).Name()})...)
			}
		default:
			continue
		}
		x.stdlibUses = append(x.stdlibUses, use)
	}
}

// paramType i. argümanın parametre tipidir (değişken sayılı parametrede öğe tipi; `args...` yayılımında yok).
func paramType(sig *types.Signature, i int, spread bool) types.Type {
	n := sig.Params().Len()
	switch {
	case n == 0:
		return nil
	case sig.Variadic() && i >= n-1:
		if spread {
			return nil
		}
		return sig.Params().At(n - 1).Type().(*types.Slice).Elem()
	case i < n:
		return sig.Params().At(i).Type()
	}
	return nil
}

// moduleNamed t (ya da gösterdiği) modülde bildirilmiş adlı bir tip mi.
func (x *goExtractor) moduleNamed(t types.Type) bool {
	n, ok := types.Unalias(derefType(t)).(*types.Named)
	return ok && n.Obj().Pkg() != nil && x.inModule(n.Obj().Pkg().Path())
}

func derefType(t types.Type) types.Type {
	if p, ok := t.(*types.Pointer); ok {
		return p.Elem()
	}
	return t
}

// namedMethods t'nin (değer ya da gösterici alıcılı) adı names'te olan metotlarıdır; gömülü alandan yükselen dahil.
func namedMethods(t types.Type, names []string) []*types.Func {
	var out []*types.Func
	ms := types.NewMethodSet(types.NewPointer(derefType(t)))
	for _, name := range names {
		if sel := ms.Lookup(nil, name); sel != nil {
			out = append(out, sel.Obj().(*types.Func))
		}
	}
	return out
}

// resolveStdlibUses rol kullanımlarını kenar yapar: metotlara ve (encodes) dışa açık alanlara.
func (x *goExtractor) resolveStdlibUses() {
	for _, u := range x.stdlibUses {
		for _, m := range u.methods {
			if id := funcRef(m.Origin()); id != "" && id != u.owner {
				x.b.addEdge(u.owner, id, u.kind, inferred, u.file, u.line)
			}
		}
		if u.kind == edgeEncodes {
			x.encodedFields(u, derefType(u.target), map[types.Type]bool{})
		}
	}
}

// encodedFields JSON'un okuduğu ve yazdığı alanlardır: dışa açık, `json:"-"` olmayan; alanın tipi modülde bir yapıysa
// (gösterici, dilim, dizi, harita öğesi dahil) onunkiler de, MarshalJSON gibi metotları da.
func (x *goExtractor) encodedFields(u stdlibUse, t types.Type, seen map[types.Type]bool) {
	for {
		switch c := t.(type) {
		case *types.Pointer:
			t = c.Elem()
			continue
		case *types.Slice:
			t = c.Elem()
			continue
		case *types.Array:
			t = c.Elem()
			continue
		case *types.Map:
			t = c.Elem()
			continue
		}
		break
	}
	if seen[t] || !x.moduleNamed(t) {
		return
	}
	seen[t] = true
	for _, m := range namedMethods(t, encodeMethods) {
		if id := funcRef(m.Origin()); id != "" {
			x.b.addEdge(u.owner, id, edgeEncodes, inferred, u.file, u.line)
		}
	}
	st, ok := t.Underlying().(*types.Struct)
	if !ok {
		return
	}
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if !f.Exported() && !f.Embedded() || strings.HasPrefix(reflect.StructTag(st.Tag(i)).Get("json"), "-") {
			continue
		}
		if fid, ok := x.fieldAt[x.posKey(f.Pos())]; ok && f.Exported() {
			x.b.addEdge(u.owner, fid, edgeEncodes, inferred, u.file, u.line)
		}
		x.encodedFields(u, f.Type(), seen)
	}
}

// argType bir argümanın tipidir, tip bilgisi (Info.Types) olmadan: tanımlayıcı ve seçici nesnesinden, bileşik değerin
// yazılı tipinden, &x, *x, x[i], x.(T), T(x) ve f(...)'in (tek ya da ilk) sonucundan. Çözülemezse nil.
func (x *goExtractor) argType(fc *fileCtx, e ast.Expr) types.Type {
	info := fc.u.Info
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		if o := info.Uses[e]; o != nil {
			if _, isType := o.(*types.TypeName); !isType {
				return o.Type()
			}
		}
	case *ast.SelectorExpr:
		if s := info.Selections[e]; s != nil {
			return s.Type()
		}
		return x.argType(fc, e.Sel)
	case *ast.CompositeLit:
		return typeExprType(info, e.Type)
	case *ast.UnaryExpr:
		if t := x.argType(fc, e.X); t != nil && e.Op == token.AND {
			return types.NewPointer(t)
		}
	case *ast.StarExpr:
		if p, ok := x.argType(fc, e.X).(*types.Pointer); ok {
			return p.Elem()
		}
	case *ast.IndexExpr:
		t := x.argType(fc, e.X)
		if t == nil {
			return nil
		}
		switch c := derefType(t).Underlying().(type) {
		case *types.Slice:
			return c.Elem()
		case *types.Array:
			return c.Elem()
		case *types.Map:
			return c.Elem()
		}
	case *ast.TypeAssertExpr:
		return typeExprType(info, e.Type)
	case *ast.CallExpr:
		if t := typeExprType(info, e.Fun); t != nil {
			return t // dönüşüm: T(x)
		}
		var sig *types.Signature
		if s, ok := e.Fun.(*ast.SelectorExpr); ok && info.Selections[s] != nil {
			sig, _ = info.Selections[s].Type().(*types.Signature)
		} else if id := nameIdent(e.Fun); id != nil && info.Uses[id] != nil {
			sig, _ = info.Uses[id].Type().(*types.Signature)
		}
		if sig != nil && sig.Results().Len() > 0 && sig.TypeParams().Len() == 0 {
			return sig.Results().At(0).Type()
		}
	}
	return nil
}

// typeExprType bir tip ifadesinin (T, pkg.T, *T, []T) tipidir; tip değilse nil.
func typeExprType(info *types.Info, e ast.Expr) types.Type {
	switch e := ast.Unparen(e).(type) {
	case *ast.StarExpr:
		if t := typeExprType(info, e.X); t != nil {
			return types.NewPointer(t)
		}
	case *ast.ArrayType:
		if t := typeExprType(info, e.Elt); t != nil && e.Len == nil {
			return types.NewSlice(t)
		}
	case *ast.Ident, *ast.SelectorExpr:
		if tn, ok := info.Uses[nameIdent(e)].(*types.TypeName); ok {
			return tn.Type()
		}
	}
	return nil
}
