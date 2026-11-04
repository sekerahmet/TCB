package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"path"
	"slices"
	"sort"
	"strings"
)

// dirSpec kökten kurulan `<kök>/<klasör>/<dosya>` çağrısındaki dinamik klasör parçasının kapsamıdır.
type dirSpec struct {
	exact     []string // sabit klasör listesi
	prefixes  []string // strings.HasPrefix ile daraltılan önekler
	scanBases []string // Go dosyası taramasının taban klasörleri
	goPkg     bool     // klasör filepath.Dir ile Go dosyalarından türetiliyor
	testPkg   bool     // klasörde *_test.go bulunması şart koşuluyor
	anySub    bool     // özel kısıtı olmayan alt klasör değişkeni
}

// resolvePerDirPaths testlerin paket ya da alt klasör başına okuduğu belgeleri (`filepath.Join(reporoot.Root(), dir, "README.md")`,
// `reporoot.Path(dir, "README.md")` ya da `WalkDir` geri çağırmasındaki `filepath.Join(p, "README.md")`) ilgili dosya/belge
// düğümlerine kök başvurusu olarak kaydeder.
func (x *goExtractor) resolvePerDirPaths() {
	var funcs []scanFunc
	for _, f := range x.idx.scans.funcs {
		if f.id != "" && f.fn.Body != nil {
			funcs = append(funcs, f)
		}
	}
	sort.Slice(funcs, func(i, j int) bool { return funcs[i].id < funcs[j].id })
	for _, f := range funcs {
		x.walkDirJoinPaths(f)
		ast.Inspect(f.fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || call.Ellipsis.IsValid() {
				return true
			}
			bases, dirExpr, tail, ok := x.perDirJoinArgs(f.fc, f.fn, call)
			if !ok {
				return true
			}
			spec := x.resolveDirSpec(f.fc, f.fn, dirExpr, 4)
			x.emitDirSpecRefs(f.fc, f.id, call.Pos(), bases, spec, tail)
			return true
		})
	}
}

// walkDirJoinPaths `reporoot.WalkDir(kök, func(p string, ...) { ... filepath.Join(p, "README.md") ... })` çağrılarını çözer.
func (x *goExtractor) walkDirJoinPaths(f scanFunc) {
	ast.Inspect(f.fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 2 || !x.rootPathCall(f.fc, call) || nameIdent(call.Fun).Name != "WalkDir" {
			return true
		}
		lit, ok := call.Args[1].(*ast.FuncLit)
		if !ok || lit.Body == nil || lit.Type.Params == nil || len(lit.Type.Params.List) == 0 || len(lit.Type.Params.List[0].Names) == 0 {
			return true
		}
		pObj := f.fc.u.Info.Defs[lit.Type.Params.List[0].Names[0]]
		if pObj == nil {
			return true
		}
		bases, rooted := x.evalPath(f.fc, f.fn, call.Args[0], 4)
		if !rooted || len(bases) == 0 {
			return true
		}
		ast.Inspect(lit.Body, func(m ast.Node) bool {
			inner, ok := m.(*ast.CallExpr)
			if !ok || !isStdJoinCall(f.fc, inner) || len(inner.Args) < 2 {
				return true
			}
			id, ok := unwrapDirExpr(inner.Args[0]).(*ast.Ident)
			if !ok || f.fc.u.Info.Uses[id] != pObj {
				return true
			}
			tailParts, start := trailingStringLits(inner.Args)
			if start != 1 || len(tailParts) == 0 {
				return true
			}
			tail := path.Join(tailParts...)
			if !isDocOrDataTail(tail) {
				return true
			}
			for _, b := range bases {
				x.addRootRef(f.fc, f.id, inner.Pos(), path.Join(b, "**", tail), nil)
			}
			return true
		})
		return true
	})
}

func (x *goExtractor) perDirJoinArgs(fc *fileCtx, fn *ast.FuncDecl, call *ast.CallExpr) ([]string, ast.Expr, string, bool) {
	tailParts, start := trailingStringLits(call.Args)
	if len(tailParts) == 0 || start == 0 {
		return nil, nil, "", false
	}
	tail := path.Join(tailParts...)
	if !isDocOrDataTail(tail) {
		return nil, nil, "", false
	}
	if x.rootPathCall(fc, call) && nameIdent(call.Fun).Name == "Path" {
		headParts, headEnd := leadingStringLits(call.Args[:start])
		mid := call.Args[headEnd:start]
		if len(mid) != 1 {
			return nil, nil, "", false
		}
		base := "."
		if len(headParts) > 0 {
			base = path.Join(headParts...)
		}
		return []string{base}, mid[0], tail, true
	}
	if isStdJoinCall(fc, call) && start >= 2 {
		vals, rooted := x.evalPath(fc, fn, call.Args[0], 4)
		if !rooted || len(vals) == 0 {
			return nil, nil, "", false
		}
		headParts, headEnd := leadingStringLits(call.Args[1:start])
		mid := call.Args[1+headEnd : start]
		if len(mid) != 1 {
			return nil, nil, "", false
		}
		prefix := path.Join(headParts...)
		bases := make([]string, 0, len(vals))
		for _, v := range vals {
			bases = append(bases, path.Join(v, prefix))
		}
		return bases, mid[0], tail, true
	}
	return nil, nil, "", false
}

func (x *goExtractor) resolveDirSpec(fc *fileCtx, fn *ast.FuncDecl, expr ast.Expr, depth int) dirSpec {
	expr = unwrapDirExpr(expr)
	if depth < 0 || expr == nil {
		return dirSpec{}
	}
	if _, isLit := expr.(*ast.BasicLit); isLit {
		return dirSpec{}
	}
	if vals, rooted := x.evalPath(fc, fn, expr, 3); !rooted && len(vals) > 0 {
		return dirSpec{exact: vals}
	}
	if isDirOfCall(fc, expr) {
		return x.specFromCollector(fc, fn)
	}
	id, ok := expr.(*ast.Ident)
	if !ok {
		return dirSpec{anySub: true}
	}
	obj := fc.u.Info.Uses[id]
	if obj == nil {
		return dirSpec{}
	}
	if idx := paramIndex(fc, fn, obj); idx >= 0 {
		return x.resolveDirParam(fc, fn, idx, depth-1)
	}
	if rhs := localValue(fc, fn, obj); rhs != nil {
		return x.resolveDirSpec(fc, fn, rhs, depth-1)
	}
	if over := findRangeExpr(fc, fn.Body, obj); over != nil {
		if call, ok := ast.Unparen(over).(*ast.CallExpr); ok {
			if callee, ok := x.idx.scans.funcs[fc.u.Info.Uses[nameIdent(call.Fun)]]; ok {
				return x.specFromCollector(callee.fc, callee.fn)
			}
		}
		return x.specFromCollector(fc, fn)
	}
	if b, ok := obj.Type().(*types.Basic); ok && b.Info()&types.IsString != 0 {
		return dirSpec{anySub: true}
	}
	return dirSpec{}
}

func (x *goExtractor) resolveDirParam(fc *fileCtx, fn *ast.FuncDecl, idx, depth int) dirSpec {
	fnObj := fc.u.Info.Defs[fn.Name]
	if fnObj == nil {
		return dirSpec{}
	}
	var merged dirSpec
	for _, c := range x.directCallers(x.objectID(fnObj)) {
		ast.Inspect(c.fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || c.fc.u.Info.Uses[nameIdent(call.Fun)] != fnObj || idx >= len(call.Args) {
				return true
			}
			merged = mergeDirSpec(merged, x.resolveDirSpec(c.fc, c.fn, call.Args[idx], depth))
			return true
		})
	}
	return merged
}

func mergeDirSpec(a, b dirSpec) dirSpec {
	a.exact = append(a.exact, b.exact...)
	if b.goPkg || len(b.prefixes) > 0 {
		switch {
		case !a.goPkg && len(a.prefixes) == 0:
			a.goPkg, a.testPkg = b.goPkg, b.testPkg
			a.prefixes = append(a.prefixes, b.prefixes...)
			a.scanBases = append(a.scanBases, b.scanBases...)
		default:
			a.goPkg = a.goPkg || b.goPkg
			a.testPkg = a.testPkg && b.testPkg
			a.prefixes = append(a.prefixes, b.prefixes...)
			a.scanBases = append(a.scanBases, b.scanBases...)
		}
	}
	a.anySub = a.anySub || b.anySub
	return a
}

// specFromCollector dosyaları gezip `filepath.Dir` ile klasör toplayan bir gövdeden Go paketi ve önek süzgeçlerini çıkarır.
func (x *goExtractor) specFromCollector(fc *fileCtx, fn *ast.FuncDecl) dirSpec {
	if fn == nil || fn.Body == nil {
		return dirSpec{anySub: true}
	}
	var spec dirSpec
	bases, hasGoScan := x.collectorGoScan(fc, fn)
	if hasGoScan {
		spec.goPkg = true
		spec.scanBases = bases
	}
	if hasDirOfCall(fc, fn.Body) && bodyRequiresTestFile(fc, fn.Body) {
		spec.goPkg = true
		spec.testPkg = true
	}
	spec.prefixes = x.positiveHasPrefixes(fc, fn.Body, 1)
	if !spec.goPkg && len(spec.prefixes) == 0 {
		spec.anySub = true
	}
	return spec
}

func (x *goExtractor) collectorGoScan(fc *fileCtx, fn *ast.FuncDecl) ([]string, bool) {
	selfID := x.objectID(fc.u.Info.Defs[fn.Name])
	targets := map[string]bool{selfID: true}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if obj := fc.u.Info.Uses[nameIdent(call.Fun)]; obj != nil {
				targets[x.objectID(obj)] = true
			}
		}
		return true
	})
	var bases []string
	hit := false
	for _, s := range x.idx.scans.done {
		if targets[s.from] && (slices.Contains(s.scope.inc, ".go") || slices.Contains(s.scope.inc, "_test.go")) {
			hit = true
			bases = append(bases, s.scope.bases...)
		}
	}
	return compactStrings(bases), hit
}

// positiveHasPrefixes gövdedeki olumlu `strings.HasPrefix(x, önek)` süzgeçlerini (1 adım bool yardımcı işlevler dahil) verir.
func (x *goExtractor) positiveHasPrefixes(fc *fileCtx, body *ast.BlockStmt, depth int) []string {
	var out []string
	ast.Inspect(body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.IfStmt:
			out = append(out, x.condHasPrefixes(fc, s.Cond, isSkip(s.Body), depth)...)
		case *ast.ReturnStmt:
			if len(s.Results) == 1 {
				out = append(out, x.condHasPrefixes(fc, s.Results[0], false, depth)...)
			}
		}
		return true
	})
	sort.Strings(out)
	return compactStrings(out)
}

func (x *goExtractor) condHasPrefixes(fc *fileCtx, e ast.Expr, neg bool, depth int) []string {
	switch e := ast.Unparen(e).(type) {
	case *ast.UnaryExpr:
		if e.Op == token.NOT {
			return x.condHasPrefixes(fc, e.X, !neg, depth)
		}
	case *ast.BinaryExpr:
		if e.Op == token.LAND || e.Op == token.LOR {
			return append(x.condHasPrefixes(fc, e.X, neg, depth), x.condHasPrefixes(fc, e.Y, neg, depth)...)
		}
	case *ast.CallExpr:
		if neg {
			return nil
		}
		fn, _ := fc.u.Info.Uses[nameIdent(e.Fun)].(*types.Func)
		if fn == nil {
			return nil
		}
		if fn.Pkg() != nil && fn.Pkg().Path() == "strings" && fn.Name() == "HasPrefix" && len(e.Args) == 2 {
			if s := constString(fc, ast.Unparen(e.Args[1])); s != "" {
				return []string{s}
			}
			if id, ok := ast.Unparen(e.Args[1]).(*ast.Ident); ok {
				if vals, ok := x.valuesOf(fc.u.Info.Uses[id]); ok {
					return vals
				}
			}
			return nil
		}
		if depth > 0 {
			if callee, ok := x.idx.scans.allFuncs[fn]; ok && callee.fn.Body != nil {
				return x.positiveHasPrefixes(callee.fc, callee.fn.Body, depth-1)
			}
		}
	}
	return nil
}

func (x *goExtractor) emitDirSpecRefs(fc *fileCtx, owner string, pos token.Pos, bases []string, spec dirSpec, tail string) {
	seen := map[string]bool{}
	emit := func(p string) {
		p = path.Clean(p)
		if !seen[p] {
			seen[p] = true
			x.addRootRef(fc, owner, pos, p, nil)
		}
	}
	for _, d := range spec.exact {
		for _, b := range bases {
			emit(path.Join(b, d, tail))
		}
	}
	switch {
	case spec.goPkg:
		for _, d := range x.matchingGoDirs(spec) {
			for _, b := range bases {
				emit(path.Join(b, d, tail))
			}
		}
	case len(spec.prefixes) > 0:
		for _, p := range spec.prefixes {
			clean := strings.TrimSuffix(p, "/")
			for _, b := range bases {
				emit(path.Join(b, clean, tail))
				emit(path.Join(b, clean, "**", tail))
			}
		}
	}
	if spec.anySub {
		for _, b := range bases {
			if b == "." {
				emit("*/**/" + tail)
			} else {
				emit(path.Join(b, "**", tail))
			}
		}
	}
}

func (x *goExtractor) matchingGoDirs(spec dirSpec) []string {
	set := map[string]bool{}
	for rel := range x.idx.dirFiles {
		if spec.testPkg && !strings.HasSuffix(rel, "_test.go") {
			continue
		}
		d := path.Dir(rel)
		if d == "." {
			continue
		}
		if len(spec.scanBases) > 0 && !underAnyBase(d, spec.scanBases) {
			continue
		}
		if len(spec.prefixes) > 0 && !matchesAnyPrefix(d, spec.prefixes) {
			continue
		}
		set[d] = true
	}
	out := make([]string, 0, len(set))
	for d := range set {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

func underAnyBase(dir string, bases []string) bool {
	for _, b := range bases {
		if b == "." || dir == b || strings.HasPrefix(dir, strings.TrimSuffix(b, "/")+"/") {
			return true
		}
	}
	return false
}

func matchesAnyPrefix(dir string, prefixes []string) bool {
	withSlash := dir + "/"
	for _, p := range prefixes {
		if strings.HasPrefix(withSlash, p) || (!strings.HasSuffix(p, "/") && (dir == p || strings.HasPrefix(withSlash, p+"/"))) {
			return true
		}
	}
	return false
}

func trailingStringLits(args []ast.Expr) ([]string, int) {
	start := len(args)
	for start > 0 {
		lit, ok := args[start-1].(*ast.BasicLit)
		if !ok {
			break
		}
		if _, ok := stringLit(lit); !ok {
			break
		}
		start--
	}
	parts := make([]string, 0, len(args)-start)
	for _, a := range args[start:] {
		s, _ := stringLit(a.(*ast.BasicLit))
		parts = append(parts, s)
	}
	return parts, start
}

func leadingStringLits(args []ast.Expr) ([]string, int) {
	var parts []string
	for i, a := range args {
		lit, ok := a.(*ast.BasicLit)
		if !ok {
			return parts, i
		}
		s, ok := stringLit(lit)
		if !ok {
			return parts, i
		}
		parts = append(parts, s)
	}
	return parts, len(args)
}

func isDocOrDataTail(tail string) bool {
	return strings.Contains(path.Base(tail), ".") && !strings.ContainsAny(tail, " \t\n\r{}$%")
}

func isStdJoinCall(fc *fileCtx, call *ast.CallExpr) bool {
	fn, _ := fc.u.Info.Uses[nameIdent(call.Fun)].(*types.Func)
	return fn != nil && fn.Pkg() != nil && (fn.Pkg().Path() == "path/filepath" || fn.Pkg().Path() == "path") && fn.Name() == "Join"
}

func unwrapDirExpr(e ast.Expr) ast.Expr {
	for e != nil {
		e = ast.Unparen(e)
		call, ok := e.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return e
		}
		fn, _ := call.Fun.(*ast.SelectorExpr)
		if fn == nil || (fn.Sel.Name != "FromSlash" && fn.Sel.Name != "ToSlash" && fn.Sel.Name != "Clean") {
			return e
		}
		e = call.Args[0]
	}
	return nil
}

func isDirOfCall(fc *fileCtx, e ast.Expr) bool {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	fn, _ := fc.u.Info.Uses[nameIdent(call.Fun)].(*types.Func)
	return fn != nil && fn.Pkg() != nil && (fn.Pkg().Path() == "path/filepath" || fn.Pkg().Path() == "path") && fn.Name() == "Dir"
}

func hasDirOfCall(fc *fileCtx, body ast.Node) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if e, ok := n.(ast.Expr); ok && isDirOfCall(fc, e) {
			found = true
		}
		return !found
	})
	return found
}

func bodyRequiresTestFile(fc *fileCtx, body ast.Node) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.SelectorExpr:
			if s.Sel.Name == "test" {
				if v, ok := fc.u.Info.Uses[s.Sel].(*types.Var); ok {
					if b, ok := v.Type().(*types.Basic); ok && b.Info()&types.IsBoolean != 0 {
						found = true
					}
				}
			}
		case *ast.BasicLit:
			if str, ok := stringLit(s); ok && str == "_test.go" {
				found = true
			}
		}
		return !found
	})
	return found
}

func findRangeExpr(fc *fileCtx, body *ast.BlockStmt, obj types.Object) ast.Expr {
	var found ast.Expr
	ast.Inspect(body, func(n ast.Node) bool {
		rs, ok := n.(*ast.RangeStmt)
		if !ok {
			return found == nil
		}
		if id, ok := rs.Value.(*ast.Ident); ok && fc.u.Info.Defs[id] == obj {
			found = rs.X
		} else if rs.Value == nil {
			if id, ok := rs.Key.(*ast.Ident); ok && fc.u.Info.Defs[id] == obj {
				found = rs.X
			}
		}
		return found == nil
	})
	return found
}
