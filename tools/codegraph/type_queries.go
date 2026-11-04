package main

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"path"
	"slices"
	"sort"
	"strings"
)

// Bu dosya depoyu (ya da bir paketi) tip bilgisiyle yükleyip (`goload.Load`, `packages.Load`, `(*types.Config).Check`)
// bir arayüzün uygulayıcılarını, bir adın kullanımlarını/seçimlerini ya da bir paketin kullanıcılarını denetleyen Go
// testlerini saptar. Sorgulanan adlar çağrı grafiğindeki arama parametrelerinden (`Scope.Lookup`, `LookupFieldOrMethod`,
// `MethodSet.Lookup`, `(types.Object).Name()`, `o.Pkg().Path()`) sabit noktaya kadar izlenir; eşleştirme ve kenar üretimi
// bütün çıkarıcılardan sonra `type_queries_link.go` içinde yapılır.

type queryKind uint8

const (
	queryObj queryKind = 1 << iota
	queryPkg
)

type queryCap struct {
	impl, uses, selections bool
}

func (c queryCap) any() bool { return c.impl || c.uses || c.selections }

func (c queryCap) merge(o queryCap) queryCap {
	return queryCap{c.impl || o.impl, c.uses || o.uses, c.selections || o.selections}
}

type loadScope struct {
	bases     []string
	incTests  bool
	singlePkg string
}

type typeLoader struct {
	id    string
	scope loadScope
}

type typeQuerySite struct {
	owner, callee, file, spec, pkgHint string
	line                               int
	kind                               queryKind
}

type typeQueryState struct {
	loaders []typeLoader
	caps    map[string]queryCap
	sites   []typeQuerySite
}

func newTypeQueryState() *typeQueryState {
	return &typeQueryState{caps: map[string]queryCap{}}
}

func (x *goExtractor) resolveTypeQueries() {
	callers := x.buildAllCallers()
	x.findTypeLoaders(callers)
	x.findQueryCaps()
	params, resolvers := x.findQueryParams()
	x.collectQuerySites(params, resolvers)
}

func (x *goExtractor) buildAllCallers() map[string][]scanFunc {
	out := map[string][]scanFunc{}
	seen := map[string]map[string]bool{}
	for _, f := range x.idx.scans.allFuncs {
		ast.Inspect(f.fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id := x.objectID(f.fc.u.Info.Uses[nameIdent(call.Fun)])
			if id == "" {
				return true
			}
			key := f.id + "@" + f.fc.u.PkgPath
			if seen[id] == nil {
				seen[id] = map[string]bool{}
			}
			if !seen[id][key] {
				seen[id][key] = true
				out[id] = append(out[id], f)
			}
			return true
		})
	}
	return out
}

func (x *goExtractor) addTypeLoader(id string, sc loadScope) {
	if id == "" || len(sc.bases) == 0 {
		return
	}
	bases := slices.Clone(sc.bases)
	sort.Strings(bases)
	sc.bases = compactStrings(bases)
	for _, cur := range x.idx.typeQueries.loaders {
		if cur.id == id && cur.scope.incTests == sc.incTests && cur.scope.singlePkg == sc.singlePkg && slices.Equal(cur.scope.bases, sc.bases) {
			return
		}
	}
	x.idx.typeQueries.loaders = append(x.idx.typeQueries.loaders, typeLoader{id: id, scope: sc})
}

func (x *goExtractor) findTypeLoaders(callers map[string][]scanFunc) {
	for _, f := range x.idx.scans.allFuncs {
		ast.Inspect(f.fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			fn, _ := f.fc.u.Info.Uses[nameIdent(call.Fun)].(*types.Func)
			if fn == nil || fn.Pkg() == nil || len(call.Args) == 0 {
				return true
			}
			pkg := fn.Pkg().Path()
			switch {
			case fn.Name() == "Load" && strings.HasSuffix(pkg, "/goload"):
				x.traceDirLoaders(f, call.Args[0], true, nil, callers, 5)
			case fn.Name() == "Load" && (pkg == "golang.org/x/tools/go/packages" || strings.HasSuffix(pkg, "/go/packages")):
				x.notePackagesLoad(f, call, callers)
			case fn.Name() == "Check" && pkg == "go/types" && len(call.Args) >= 3:
				x.noteTypesCheck(f, call)
			}
			return true
		})
	}
}

func (x *goExtractor) notePackagesLoad(f scanFunc, call *ast.CallExpr, callers map[string][]scanFunc) {
	pats := x.loadPatterns(f, call.Args[1:])
	if identName(call.Args[0]) == "nil" {
		if f.fc.isTest && !usesTempDir(f.fc, f.fn.Body) {
			x.addTypeLoader(f.id, loadScope{bases: applyLoadPatterns([]string{path.Dir(f.fc.rel)}, pats, x.m.Path)})
		}
		return
	}
	lit := compositeLitOf(f.fc, f.fn, call.Args[0])
	if lit == nil {
		return
	}
	incTests := boolField(lit, "Tests")
	if dir := fieldExpr(lit, "Dir"); dir != nil {
		x.traceDirLoaders(f, dir, incTests, pats, callers, 5)
		return
	}
	if f.fc.isTest && !usesTempDir(f.fc, f.fn.Body) {
		x.addTypeLoader(f.id, loadScope{bases: applyLoadPatterns([]string{path.Dir(f.fc.rel)}, pats, x.m.Path), incTests: incTests})
	}
}

func (x *goExtractor) noteTypesCheck(f scanFunc, call *ast.CallExpr) {
	if usesTempDir(f.fc, f.fn.Body) || !parsesDiskGoFiles(f.fc, f.fn.Body) {
		return
	}
	_, exc, _ := scanFilters(f.fn.Body, x.suffixEval(f.fc, f.fn, nil))
	incTests := !slices.Contains(exc, "_test.go")
	for _, raw := range x.concreteExprStrings(f.fc, f.fn, call.Args[0]) {
		p := raw
		if x.m.Base[p] == nil {
			p = x.m.Path + "/" + strings.TrimPrefix(raw, "./")
		}
		if x.m.Base[p] == nil {
			continue
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(p, x.m.Path), "/")
		if rel == "" {
			rel = "."
		}
		x.addTypeLoader(f.id, loadScope{bases: []string{rel}, incTests: incTests, singlePkg: p})
	}
}

func (x *goExtractor) traceDirLoaders(f scanFunc, dir ast.Expr, incTests bool, pats []string, callers map[string][]scanFunc, depth int) {
	if vals, rooted := x.evalPathWith(f.fc, f.fn, dir, blockedParams(f.fc, f.fn), 6); rooted && len(vals) > 0 {
		if !usesTempDir(f.fc, f.fn.Body) {
			x.addTypeLoader(f.id, loadScope{bases: applyLoadPatterns(vals, pats, x.m.Path), incTests: incTests})
		}
		return
	}
	if depth <= 0 {
		return
	}
	pIdx, sub, ok := x.dirParam(f.fc, f.fn, dir)
	if !ok {
		return
	}
	nextPats := pats
	if len(sub) > 0 {
		nextPats = applyLoadPatterns(sub, pats, x.m.Path)
	}
	for _, c := range callers[f.id] {
		ast.Inspect(c.fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || pIdx >= len(call.Args) || x.objectID(c.fc.u.Info.Uses[nameIdent(call.Fun)]) != f.id {
				return true
			}
			x.traceDirLoaders(c, call.Args[pIdx], incTests, nextPats, callers, depth-1)
			return true
		})
	}
}

func (x *goExtractor) dirParam(fc *fileCtx, fn *ast.FuncDecl, e ast.Expr) (int, []string, bool) {
	switch e := e.(type) {
	case *ast.ParenExpr:
		return x.dirParam(fc, fn, e.X)
	case *ast.Ident:
		obj := fc.u.Info.Uses[e]
		if i := paramIndex(fc, fn, obj); i >= 0 {
			return i, nil, true
		}
		if rhs := localValue(fc, fn, obj); rhs != nil {
			return x.dirParam(fc, fn, rhs)
		}
	case *ast.CallExpr:
		f, _ := fc.u.Info.Uses[nameIdent(e.Fun)].(*types.Func)
		if f != nil && f.Pkg() != nil && (f.Pkg().Path() == "path/filepath" || f.Pkg().Path() == "path") && f.Name() == "Join" && len(e.Args) > 1 {
			parts, ok := x.segments(fc, fn, e.Args[1:], blockedParams(fc, fn), 4)
			if !ok {
				return -1, nil, false
			}
			i, prev, ok := x.dirParam(fc, fn, e.Args[0])
			if !ok {
				return -1, nil, false
			}
			if len(prev) == 0 {
				prev = []string{"."}
			}
			return i, joinAll(prev, parts), true
		}
	}
	return -1, nil, false
}

func blockedParams(fc *fileCtx, fn *ast.FuncDecl) map[types.Object][]string {
	out := map[types.Object][]string{}
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			if obj := fc.u.Info.Defs[name]; obj != nil {
				out[obj] = nil
			}
		}
	}
	return out
}

func (x *goExtractor) loadPatterns(f scanFunc, args []ast.Expr) []string {
	var out []string
	for _, a := range args {
		out = append(out, x.concreteExprStrings(f.fc, f.fn, a)...)
	}
	return out
}

func applyLoadPatterns(bases, pats []string, modPath string) []string {
	if len(pats) == 0 {
		return bases
	}
	var out []string
	for _, pat := range pats {
		p := strings.TrimSuffix(strings.TrimSuffix(pat, "/..."), "...")
		switch {
		case p == "" || p == ".":
			out = append(out, bases...)
		case p == modPath:
			out = append(out, ".")
		case strings.HasPrefix(p, modPath+"/"):
			out = append(out, strings.TrimPrefix(p, modPath+"/"))
		default:
			sub := strings.TrimPrefix(p, "./")
			for _, b := range bases {
				out = append(out, path.Join(b, sub))
			}
		}
	}
	return out
}

func usesTempDir(fc *fileCtx, body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && !found {
			if fn, ok := fc.u.Info.Uses[nameIdent(call.Fun)].(*types.Func); ok && fn.Pkg() != nil {
				pkg := fn.Pkg().Path()
				if (pkg == "testing" && fn.Name() == "TempDir") || (pkg == "os" && fn.Name() == "MkdirTemp") {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

func parsesDiskGoFiles(fc *fileCtx, body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || found {
			return !found
		}
		fn, _ := fc.u.Info.Uses[nameIdent(call.Fun)].(*types.Func)
		if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != "go/parser" {
			return true
		}
		if fn.Name() == "ParseDir" || (fn.Name() == "ParseFile" && len(call.Args) == 4 && identName(call.Args[2]) == "nil") {
			found = true
		}
		return !found
	})
	return found
}

func compositeLitOf(fc *fileCtx, fn *ast.FuncDecl, e ast.Expr) *ast.CompositeLit {
	switch e := e.(type) {
	case *ast.ParenExpr:
		return compositeLitOf(fc, fn, e.X)
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return compositeLitOf(fc, fn, e.X)
		}
	case *ast.CompositeLit:
		return e
	case *ast.Ident:
		if rhs := localValue(fc, fn, fc.u.Info.Uses[e]); rhs != nil {
			return compositeLitOf(fc, fn, rhs)
		}
	}
	return nil
}

func fieldExpr(lit *ast.CompositeLit, name string) ast.Expr {
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok && identName(kv.Key) == name {
			return kv.Value
		}
	}
	return nil
}

func boolField(lit *ast.CompositeLit, name string) bool {
	return identName(fieldExpr(lit, name)) == "true"
}

func identName(e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func (x *goExtractor) concreteExprStrings(fc *fileCtx, fn *ast.FuncDecl, e ast.Expr) []string {
	switch v := e.(type) {
	case *ast.ParenExpr:
		return x.concreteExprStrings(fc, fn, v.X)
	case *ast.BasicLit:
		if s, ok := stringLit(v); ok {
			return []string{s}
		}
	case *ast.Ident:
		obj := fc.u.Info.Uses[v]
		if c, ok := obj.(*types.Const); ok && c.Val().Kind() == constant.String {
			return []string{constant.StringVal(c.Val())}
		}
		if vals, ok := x.valuesOf(obj); ok {
			return vals
		}
		if rhs := localValue(fc, fn, obj); rhs != nil {
			return x.concreteExprStrings(fc, fn, rhs)
		}
	}
	return nil
}
