package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// Bu dosya test dosyalarındaki somut tip sorgusu çağrı/karşılaştırma yerlerini (`typeQuerySite`) ve tür/AST yardımcıları toplar.

func (x *goExtractor) collectQuerySites(params map[paramKey]queryKind, resolvers map[string]bool) {
	st := x.idx.typeQueries
	for _, f := range x.idx.scans.funcs {
		hint := x.funcPkgHint(f, resolvers)
		impVars := collectImportVars(f.fc, f.fn.Body)
		skipConds := collectSkipConds(f.fn.Body)
		ast.Inspect(f.fn.Body, func(n ast.Node) bool {
			if expr, isExpr := n.(ast.Expr); isExpr && skipConds[expr] {
				return false
			}
			switch n := n.(type) {
			case *ast.CallExpr:
				x.noteQueryCallSite(f, n, hint, params)
			case *ast.BinaryExpr:
				if n.Op == token.EQL {
					x.noteQueryCompareSite(f, n, hint, impVars)
				}
			case *ast.IndexExpr:
				if isPackagesImportsField(f.fc, n.X) {
					for _, s := range x.concreteArgStrings(f.fc, f.fn, n.Index) {
						st.sites = append(st.sites, typeQuerySite{owner: f.id, file: f.fc.rel, line: x.line(n.Pos()), spec: s, kind: queryPkg})
					}
				}
			}
			return true
		})
	}
}

func (x *goExtractor) noteQueryCallSite(f scanFunc, call *ast.CallExpr, hint string, params map[paramKey]queryKind) {
	fn, _ := f.fc.u.Info.Uses[nameIdent(call.Fun)].(*types.Func)
	if fn == nil {
		return
	}
	sig, isSig := fn.Type().(*types.Signature)
	if !isSig || sig.Params().Len() == 0 {
		return
	}
	calleeID := x.objectID(fn)
	seedIdx, seedKind, hasSeed := seedParamKind(fn)
	callHint := x.receiverBaseKey(f.fc, f.fn, call.Fun)
	if callHint == "" {
		callHint = hint
	}
	for k, arg := range call.Args {
		idx := min(k, sig.Params().Len()-1)
		kind := params[paramKey{calleeID, idx}]
		if hasSeed && idx == seedIdx {
			kind |= seedKind
		}
		if kind == 0 {
			continue
		}
		for _, s := range x.concreteArgStrings(f.fc, f.fn, arg) {
			if s != "" && s != "_" {
				x.idx.typeQueries.sites = append(x.idx.typeQueries.sites, typeQuerySite{
					owner: f.id, callee: calleeID, file: f.fc.rel, line: x.line(arg.Pos()), spec: s, pkgHint: callHint, kind: kind,
				})
			}
		}
	}
}

func (x *goExtractor) noteQueryCompareSite(f scanFunc, be *ast.BinaryExpr, hint string, impVars map[types.Object]bool) {
	check := func(lhs, rhs ast.Expr) {
		switch {
		case isObjectNameCall(f.fc, lhs):
			for _, s := range x.concreteArgStrings(f.fc, f.fn, rhs) {
				if s != "" && s != "_" {
					x.idx.typeQueries.sites = append(x.idx.typeQueries.sites, typeQuerySite{
						owner: f.id, file: f.fc.rel, line: x.line(be.Pos()), spec: s, pkgHint: hint, kind: queryObj,
					})
				}
			}
		case isUsedPkgPathExpr(f.fc, lhs, impVars):
			for _, s := range x.concreteArgStrings(f.fc, f.fn, rhs) {
				if s != "" {
					x.idx.typeQueries.sites = append(x.idx.typeQueries.sites, typeQuerySite{
						owner: f.id, file: f.fc.rel, line: x.line(be.Pos()), spec: s, kind: queryPkg,
					})
				}
			}
		}
	}
	check(be.X, be.Y)
	check(be.Y, be.X)
}

func (x *goExtractor) concreteArgStrings(fc *fileCtx, fn *ast.FuncDecl, e ast.Expr) []string {
	if vals := x.concreteExprStrings(fc, fn, e); len(vals) > 0 {
		return vals
	}
	var out []string
	for _, ts := range x.textSources(fc, e) {
		if ts.format || len(ts.parts) > 0 {
			continue
		}
		for _, s := range x.idx.snip.expandTexts([]textSrc{ts}, 0) {
			if s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func isPlainString(t types.Type) bool {
	if t == nil {
		return false
	}
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Kind() == types.String
}

func exprType(fc *fileCtx, e ast.Expr) types.Type {
	switch e := e.(type) {
	case *ast.ParenExpr:
		return exprType(fc, e.X)
	case *ast.Ident:
		if o := fc.u.Info.Uses[e]; o != nil {
			return o.Type()
		}
		if o := fc.u.Info.Defs[e]; o != nil {
			return o.Type()
		}
	case *ast.SelectorExpr:
		if s := fc.u.Info.Selections[e]; s != nil {
			return s.Type()
		}
		if o := fc.u.Info.Uses[e.Sel]; o != nil {
			return o.Type()
		}
	case *ast.CallExpr:
		if fn, ok := fc.u.Info.Uses[nameIdent(e.Fun)].(*types.Func); ok {
			if sig, ok := fn.Type().(*types.Signature); ok && sig.Results().Len() == 1 {
				return sig.Results().At(0).Type()
			}
		}
	case *ast.IndexExpr:
		if t := exprType(fc, e.X); t != nil {
			switch u := t.Underlying().(type) {
			case *types.Map:
				return u.Elem()
			case *types.Slice:
				return u.Elem()
			}
		}
	}
	return nil
}

func isNamedType(t types.Type, pkgSuffix, name string) bool {
	if t == nil {
		return false
	}
	n, ok := derefType(t).(*types.Named)
	if !ok || n.Obj().Pkg() == nil || n.Obj().Name() != name {
		return false
	}
	p := n.Obj().Pkg().Path()
	return p == pkgSuffix || strings.HasSuffix(p, "/"+pkgSuffix)
}

func collectPkgRangeKeys(fc *fileCtx, body *ast.BlockStmt) map[types.Object]bool {
	out := map[types.Object]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		rs, ok := n.(*ast.RangeStmt)
		if !ok {
			return true
		}
		key, ok := rs.Key.(*ast.Ident)
		if !ok || fc.u.Info.Defs[key] == nil {
			return true
		}
		if xt := exprType(fc, rs.X); xt != nil {
			if m, ok := xt.Underlying().(*types.Map); ok && (isNamedType(m.Elem(), "go/types", "Package") || isNamedType(m.Elem(), "go/packages", "Package")) {
				out[fc.u.Info.Defs[key]] = true
			}
		}
		return true
	})
	return out
}

func isPkgKeyExpr(fc *fileCtx, e ast.Expr, pkgKeys map[types.Object]bool) bool {
	switch e := e.(type) {
	case *ast.Ident:
		return pkgKeys != nil && pkgKeys[fc.u.Info.Uses[e]]
	case *ast.CallExpr:
		fn, _ := fc.u.Info.Uses[nameIdent(e.Fun)].(*types.Func)
		if fn == nil || fn.Pkg() == nil || len(e.Args) != 0 || fn.Pkg().Path() != "go/types" {
			return false
		}
		sig, ok := fn.Type().(*types.Signature)
		return ok && sig.Recv() != nil && (fn.Name() == "Path" || fn.Name() == "Name") && isNamedType(sig.Recv().Type(), "go/types", "Package")
	case *ast.SelectorExpr:
		v, ok := fc.u.Info.Uses[e.Sel].(*types.Var)
		return ok && v.IsField() && (v.Name() == "PkgPath" || v.Name() == "ID" || v.Name() == "Name") && isNamedType(exprType(fc, e.X), "go/packages", "Package")
	}
	return false
}

func seedParamKind(fn *types.Func) (int, queryKind, bool) {
	if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != "go/types" {
		return 0, 0, false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return 0, 0, false
	}
	if sig.Recv() == nil && fn.Name() == "LookupFieldOrMethod" {
		return 3, queryObj, true
	}
	if sig.Recv() != nil && fn.Name() == "Lookup" {
		switch {
		case isNamedType(sig.Recv().Type(), "go/types", "Scope"):
			return 0, queryObj, true
		case isNamedType(sig.Recv().Type(), "go/types", "MethodSet"):
			return 1, queryObj, true
		}
	}
	return 0, 0, false
}

func isObjectNameCall(fc *fileCtx, e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	fn, _ := fc.u.Info.Uses[sel.Sel].(*types.Func)
	if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != "go/types" || fn.Name() != "Name" {
		return false
	}
	xt := exprType(fc, sel.X)
	return !isNamedType(xt, "go/types", "Package") && !isNamedType(xt, "go/types", "PkgName")
}

func collectImportVars(fc *fileCtx, body *ast.BlockStmt) map[types.Object]bool {
	out := map[types.Object]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		rs, ok := n.(*ast.RangeStmt)
		if !ok || (!isPackagesImportsField(fc, rs.X) && !isTypesImportsCall(fc, rs.X)) {
			return true
		}
		for _, idExpr := range []ast.Expr{rs.Key, rs.Value} {
			if id, ok := idExpr.(*ast.Ident); ok && fc.u.Info.Defs[id] != nil {
				out[fc.u.Info.Defs[id]] = true
			}
		}
		return true
	})
	return out
}

func isPackagesImportsField(fc *fileCtx, e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	v, ok := fc.u.Info.Uses[sel.Sel].(*types.Var)
	return ok && v.IsField() && v.Name() == "Imports" && isNamedType(exprType(fc, sel.X), "go/packages", "Package")
}

func isTypesImportsCall(fc *fileCtx, e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	fn, _ := fc.u.Info.Uses[nameIdent(call.Fun)].(*types.Func)
	if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != "go/types" || fn.Name() != "Imports" {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	return ok && sig.Recv() != nil && isNamedType(sig.Recv().Type(), "go/types", "Package")
}

func isUsedPkgPathExpr(fc *fileCtx, e ast.Expr, impVars map[types.Object]bool) bool {
	switch e := e.(type) {
	case *ast.Ident:
		obj := fc.u.Info.Uses[e]
		return obj != nil && impVars[obj] && isPlainString(obj.Type())
	case *ast.SelectorExpr:
		id, ok := e.X.(*ast.Ident)
		return ok && impVars[fc.u.Info.Uses[id]] && (e.Sel.Name == "PkgPath" || e.Sel.Name == "ID")
	case *ast.CallExpr:
		sel, ok := e.Fun.(*ast.SelectorExpr)
		if !ok || len(e.Args) != 0 || !isPkgKeyExpr(fc, e, nil) {
			return false
		}
		if id, ok := sel.X.(*ast.Ident); ok && impVars[fc.u.Info.Uses[id]] {
			return true
		}
		inner, ok := sel.X.(*ast.CallExpr)
		if !ok || len(inner.Args) != 0 {
			return false
		}
		fn, _ := fc.u.Info.Uses[nameIdent(inner.Fun)].(*types.Func)
		return fn != nil && fn.Pkg() != nil && fn.Pkg().Path() == "go/types" && fn.Name() == "Pkg"
	}
	return false
}

func collectSkipConds(body *ast.BlockStmt) map[ast.Expr]bool {
	out := map[ast.Expr]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		if ifs, ok := n.(*ast.IfStmt); ok && isSkip(ifs.Body) {
			out[ifs.Cond] = true
		}
		return true
	})
	return out
}

func (x *goExtractor) funcPkgHint(f scanFunc, resolvers map[string]bool) string {
	skip := collectSkipConds(f.fn.Body)
	var hits []string
	ast.Inspect(f.fn.Body, func(n ast.Node) bool {
		if expr, isExpr := n.(ast.Expr); isExpr && skip[expr] {
			return false
		}
		switch n := n.(type) {
		case *ast.BinaryExpr:
			if n.Op == token.EQL {
				if isPkgKeyExpr(f.fc, n.X, nil) {
					hits = append(hits, x.concreteArgStrings(f.fc, f.fn, n.Y)...)
				} else if isPkgKeyExpr(f.fc, n.Y, nil) {
					hits = append(hits, x.concreteArgStrings(f.fc, f.fn, n.X)...)
				}
			}
		case *ast.CallExpr:
			fn, _ := f.fc.u.Info.Uses[nameIdent(n.Fun)].(*types.Func)
			if fn != nil && resolvers[x.objectID(fn)] && len(n.Args) > 0 {
				hits = append(hits, x.concreteArgStrings(f.fc, f.fn, n.Args[0])...)
			}
		}
		return true
	})
	if len(hits) == 0 {
		return ""
	}
	for _, h := range hits[1:] {
		if h != hits[0] {
			return ""
		}
	}
	return hits[0]
}

func (x *goExtractor) receiverBaseKey(fc *fileCtx, fn *ast.FuncDecl, fun ast.Expr) string {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	scopeCall, ok := sel.X.(*ast.CallExpr)
	if !ok {
		return ""
	}
	scopeSel, ok := scopeCall.Fun.(*ast.SelectorExpr)
	if !ok || scopeSel.Sel.Name != "Scope" {
		return ""
	}
	idx, ok := scopeSel.X.(*ast.IndexExpr)
	if !ok {
		return ""
	}
	vals := x.concreteExprStrings(fc, fn, idx.Index)
	if len(vals) == 1 {
		return vals[0]
	}
	return ""
}
