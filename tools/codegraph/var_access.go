package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// Paket düzeyi değişkenlerin okunma ve yazılma kuralı TEK yerdedir: writeTargets bir düğümün yazdığı ifadeleri
// verir (atama, işlemli atama, ++/--, range ataması, adresinin alınması, delete/clear, sync/atomic çağrısı).
// Hem grafik çıkarıcısı (`reads_var`/`writes_var`/`inits_var` kenarları) hem `package-state` denetimi bunu
// kullanır; iki yerde ayrı bir "yazma nedir" kuralı kalmaz. Yapı alanlarının kuralı (accessModes) aynı atama
// biçimlerini seçici düğümleri için işaretler; değişken tarafı kök tanımlayıcıya iner.

// writeTarget bir düğümün yazdığı bir ifadedir.
type writeTarget struct {
	expr      ast.Expr
	how       string // assign, inc/dec, address taken, delete/clear, atomic
	alsoReads bool   // x += 1, x++: önce okur, sonra yazar
	bare      bool   // hedef çıplak bir ad ya da paket nitelemeli ad (x, pkg.X); değilse öğe ya da alan yazımı
}

// writeTargets n düğümünün yazdığı ifadeleri verir. atomicAddr, sync/atomic işlevine verilen &x ifadelerini tutar
// (ebeveyn çağrı çocuk & ifadesinden önce gezildiği için o `address taken` sayılmaz).
func writeTargets(info *types.Info, n ast.Node, atomicAddr map[*ast.UnaryExpr]bool) []writeTarget {
	var out []writeTarget
	switch s := n.(type) {
	case *ast.AssignStmt:
		if s.Tok != token.DEFINE {
			for _, l := range s.Lhs {
				out = append(out, writeTarget{l, "assign", s.Tok != token.ASSIGN, false})
			}
		}
	case *ast.IncDecStmt:
		out = append(out, writeTarget{s.X, "inc/dec", true, false})
	case *ast.UnaryExpr:
		if s.Op == token.AND && !atomicAddr[s] {
			out = append(out, writeTarget{s.X, "address taken", false, false})
		}
	case *ast.RangeStmt:
		if s.Tok == token.ASSIGN {
			for _, e := range []ast.Expr{s.Key, s.Value} {
				if e != nil {
					out = append(out, writeTarget{e, "assign", false, false})
				}
			}
		}
	case *ast.CallExpr:
		out = append(out, callWrites(info, s, atomicAddr)...)
	}
	for i := range out {
		out[i].bare = isBareTarget(info, out[i].expr)
	}
	return out
}

// callWrites `atomic.AddInt64(&v, 1)` ve `delete(m, k)`/`clear(m)` çağrılarının yazdıklarıdır.
func callWrites(info *types.Info, c *ast.CallExpr, atomicAddr map[*ast.UnaryExpr]bool) []writeTarget {
	var out []writeTarget
	if sel, ok := c.Fun.(*ast.SelectorExpr); ok {
		if fn, ok := info.Uses[sel.Sel].(*types.Func); ok && fn.Pkg() != nil && fn.Pkg().Path() == "sync/atomic" {
			for _, arg := range c.Args {
				if un, ok := arg.(*ast.UnaryExpr); ok && un.Op == token.AND {
					atomicAddr[un] = true
					out = append(out, writeTarget{un.X, "atomic", false, false})
				}
			}
		}
	}
	if id, ok := c.Fun.(*ast.Ident); ok && len(c.Args) > 0 && (id.Name == "delete" || id.Name == "clear") {
		if _, isBuiltin := info.Uses[id].(*types.Builtin); isBuiltin {
			out = append(out, writeTarget{c.Args[0], "delete/clear", false, false})
		}
	}
	return out
}

// isBareTarget hedef çıplak bir ad mı (x = 1, pkg.X = 1); değilse yazım öğeye ya da alana gider (x[i] = 1, x.f = 1, *x = 1).
func isBareTarget(info *types.Info, e ast.Expr) bool {
	switch v := unparen(e).(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		if x, ok := v.X.(*ast.Ident); ok {
			_, isPkg := info.Uses[x].(*types.PkgName)
			return isPkg
		}
	}
	return false
}

// rootIdent bir yazım hedefinin kök tanımlayıcısıdır (`m[k].f` → m, `pkg.V[i]` → V); paket nitelemesi atılır.
func rootIdent(info *types.Info, e ast.Expr) *ast.Ident {
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
		case *ast.SliceExpr:
			e = v.X
		case *ast.SelectorExpr:
			if x, ok := v.X.(*ast.Ident); ok {
				if _, isPkg := info.Uses[x].(*types.PkgName); isPkg {
					return v.Sel
				}
			}
			e = v.X
		case *ast.Ident:
			return v
		default:
			return nil
		}
	}
}

// packageVar bir yazım hedefinin köküne inip paket düzeyi değişkeni verir; değilse nil.
func packageVar(info *types.Info, e ast.Expr) *types.Var {
	if id := rootIdent(info, e); id != nil {
		return pkgLevel(info.Uses[id])
	}
	return nil
}

// pkgLevel nesne bir paket düzeyi değişken ise onu verir.
func pkgLevel(o types.Object) *types.Var {
	v, ok := o.(*types.Var)
	if !ok || v.IsField() || v.Pkg() == nil || v.Parent() != v.Pkg().Scope() {
		return nil
	}
	return v
}

// varSiteMode bir yazımın kenar kipi (alanlardaki kiplerle aynı sözlük): çıplak atama write, işlemli atama
// read+write, öğe ya da alan yazımı ve delete/clear elem-write, adres alma addr, sync/atomic atomic.
func varSiteMode(w writeTarget) string {
	switch {
	case w.how == "address taken":
		return modeAddr
	case w.how == "atomic":
		return "atomic"
	case w.how == "delete/clear" || !w.bare:
		return modeElemWrite
	case w.alsoReads:
		return modeReadWrite
	}
	return modeWrite
}

// noteVarWrites n düğümünün yazdığı paket düzeyi değişkenlerin kök tanımlayıcılarını writes'a işler (tanımlayıcıya
// gelindiğinde ebeveyn çoktan gezildiği için kip hazırdır).
func (x *goExtractor) noteVarWrites(fc *fileCtx, writes map[*ast.Ident]writeTarget, atomicAddr map[*ast.UnaryExpr]bool, n ast.Node) {
	for _, w := range writeTargets(fc.u.Info, n, atomicAddr) {
		if id := rootIdent(fc.u.Info, w.expr); id != nil && pkgLevel(fc.u.Info.Uses[id]) != nil {
			writes[id] = w
		}
	}
}

// varUse paket düzeyi bir değişkenin kullanımını okuma ya da yazma olarak kaydeder: yazma `writes_var`, okuma
// `reads_var` (işlemli atama ve ++ ikisi birden), `init` işlevinin yazması `inits_var`. Erişim yeri kipiyle
// kenarda saklanır (alanlardaki gibi). Değişkenin bildirimdeki başlatıcısı ayrıca scanValueSpecs'te `inits_var` olur.
func (x *goExtractor) varUse(fc *fileCtx, owner, ref string, id *ast.Ident, writes map[*ast.Ident]writeTarget, closure string) {
	site := Site{File: fc.rel, Line: x.line(id.Pos()), Mode: modeRead, Closure: closure}
	w, written := writes[id]
	if !written {
		x.b.addSite(owner, ref, edgeReadsVar, site)
		return
	}
	site.Mode = varSiteMode(w)
	kind := edgeWritesVar
	if isInitOwner(owner) {
		kind = edgeInitsVar
	}
	x.b.addSite(owner, ref, kind, site)
	if w.alsoReads {
		read := site
		read.Mode = modeRead
		x.b.addSite(owner, ref, edgeReadsVar, read)
	}
}

// isInitOwner sahip bir paketin `init` işlevi mi (kimliği init#dosya:satır ile biter).
func isInitOwner(owner string) bool {
	i := strings.LastIndex(owner, ".init#")
	return i >= 0 && !strings.ContainsAny(owner[i+len(".init#"):], "/")
}
