package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// Bu dosya modüldeki işlevlerin hangi tip sorgu yeteneklerini (`types.Implements`, `Info.Uses`, `Info.Selections`,
// `packages.Package.Imports`) taşıdığını ve hangi dize parametrelerinin ad/paket aramasına aktığını sabit noktaya kadar çözer.

func (x *goExtractor) findQueryCaps() {
	st := x.idx.typeQueries
	for _, f := range x.idx.scans.allFuncs {
		if f.id == "" || strings.HasSuffix(f.fc.base, "/goload") {
			continue
		}
		lhsSels := collectAssignedSelectors(f.fn.Body)
		var c queryCap
		ast.Inspect(f.fn.Body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				fn, _ := f.fc.u.Info.Uses[nameIdent(n.Fun)].(*types.Func)
				if fn != nil && fn.Pkg() != nil && fn.Pkg().Path() == "go/types" {
					switch fn.Name() {
					case "Implements", "AssignableTo", "AssertableTo", "MissingMethod":
						c.impl = true
					}
				}
			case *ast.SelectorExpr:
				if lhsSels[n] {
					return true
				}
				v, ok := f.fc.u.Info.Uses[n.Sel].(*types.Var)
				if !ok || !v.IsField() || v.Pkg() == nil {
					return true
				}
				pkg := v.Pkg().Path()
				switch {
				case pkg == "go/types" && v.Name() == "Uses":
					c.uses = true
				case pkg == "go/types" && v.Name() == "Selections":
					c.selections = true
				case (pkg == "golang.org/x/tools/go/packages" || strings.HasSuffix(pkg, "/go/packages")) && v.Name() == "Imports":
					c.uses = true
				}
			}
			return true
		})
		if c.any() {
			st.caps[f.id] = st.caps[f.id].merge(c)
		}
	}
}

func collectAssignedSelectors(body *ast.BlockStmt) map[*ast.SelectorExpr]bool {
	out := map[*ast.SelectorExpr]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		if as, ok := n.(*ast.AssignStmt); ok {
			for _, l := range as.Lhs {
				if sel, ok := l.(*ast.SelectorExpr); ok {
					out[sel] = true
				}
			}
		}
		return true
	})
	return out
}

func (x *goExtractor) findQueryParams() (map[paramKey]queryKind, map[string]bool) {
	resolvers := x.findPkgResolvers()
	params := map[paramKey]queryKind{}
	taints := map[*ast.FuncDecl]map[types.Object]uint64{}
	for _, f := range x.idx.scans.allFuncs {
		if f.id == "" {
			continue
		}
		t := x.paramTaints(f, resolvers)
		taints[f.fn] = t
		x.noteDirectQuerySinks(f, t, params)
	}
	for changed := true; changed; {
		changed = false
		for _, f := range x.idx.scans.allFuncs {
			if f.id == "" {
				continue
			}
			if x.propagateQueryCallArgs(f, taints[f.fn], params) {
				changed = true
			}
		}
	}
	return params, resolvers
}

func (x *goExtractor) findPkgResolvers() map[string]bool {
	out := map[string]bool{}
	for _, f := range x.idx.scans.allFuncs {
		obj, _ := f.fc.u.Info.Defs[f.fn.Name].(*types.Func)
		if obj == nil || f.id == "" {
			continue
		}
		sig, ok := obj.Type().(*types.Signature)
		if !ok || sig.Results().Len() == 0 || !isPlainString(sig.Results().At(0).Type()) {
			continue
		}
		t := x.paramTaints(f, nil)
		pkgKeys := collectPkgRangeKeys(f.fc, f.fn.Body)
		ast.Inspect(f.fn.Body, func(n ast.Node) bool {
			be, ok := n.(*ast.BinaryExpr)
			if !ok || (be.Op != token.EQL && be.Op != token.NEQ) {
				return true
			}
			if (isPkgKeyExpr(f.fc, be.X, pkgKeys) && exprTaint(f.fc, be.Y, t, nil, x) != 0) ||
				(isPkgKeyExpr(f.fc, be.Y, pkgKeys) && exprTaint(f.fc, be.X, t, nil, x) != 0) {
				out[f.id] = true
			}
			return true
		})
	}
	return out
}

func (x *goExtractor) paramTaints(f scanFunc, resolvers map[string]bool) map[types.Object]uint64 {
	t := map[types.Object]uint64{}
	idx := 0
	for _, field := range f.fn.Type.Params.List {
		for _, name := range field.Names {
			if obj := f.fc.u.Info.Defs[name]; obj != nil && idx < 64 && isStringType(obj.Type()) {
				t[obj] = 1 << idx
			}
			idx++
		}
		if len(field.Names) == 0 {
			idx++
		}
	}
	if len(t) == 0 {
		return t
	}
	for pass := 0; pass < 3; pass++ {
		ast.Inspect(f.fn.Body, func(n ast.Node) bool {
			switch s := n.(type) {
			case *ast.AssignStmt:
				x.taintAssign(f.fc, s, t, resolvers)
			case *ast.RangeStmt:
				if id, ok := s.Value.(*ast.Ident); ok && f.fc.u.Info.Defs[id] != nil {
					t[f.fc.u.Info.Defs[id]] |= exprTaint(f.fc, s.X, t, resolvers, x)
				}
			}
			return true
		})
	}
	return t
}

func (x *goExtractor) taintAssign(fc *fileCtx, s *ast.AssignStmt, t map[types.Object]uint64, resolvers map[string]bool) {
	setLhs := func(lhs ast.Expr, mask uint64) {
		if mask == 0 {
			return
		}
		id, ok := lhs.(*ast.Ident)
		if !ok {
			return
		}
		obj := fc.u.Info.Defs[id]
		if obj == nil {
			obj = fc.u.Info.Uses[id]
		}
		if obj != nil && isStringType(obj.Type()) {
			t[obj] |= mask
		}
	}
	if len(s.Rhs) == 1 {
		m := exprTaint(fc, s.Rhs[0], t, resolvers, x)
		for _, l := range asLhs(s) {
			setLhs(l, m)
		}
		return
	}
	for i := 0; i < len(s.Lhs) && i < len(s.Rhs); i++ {
		setLhs(s.Lhs[i], exprTaint(fc, s.Rhs[i], t, resolvers, x))
	}
}

func asLhs(s *ast.AssignStmt) []ast.Expr { return s.Lhs }

func exprTaint(fc *fileCtx, e ast.Expr, t map[types.Object]uint64, resolvers map[string]bool, x *goExtractor) uint64 {
	switch e := e.(type) {
	case *ast.ParenExpr:
		return exprTaint(fc, e.X, t, resolvers, x)
	case *ast.Ident:
		return t[fc.u.Info.Uses[e]]
	case *ast.SliceExpr:
		return exprTaint(fc, e.X, t, resolvers, x)
	case *ast.IndexExpr:
		return exprTaint(fc, e.X, t, resolvers, x)
	case *ast.CallExpr:
		fn, _ := fc.u.Info.Uses[nameIdent(e.Fun)].(*types.Func)
		if fn == nil || len(e.Args) == 0 {
			return 0
		}
		if resolvers != nil && resolvers[x.objectID(fn)] {
			return exprTaint(fc, e.Args[0], t, resolvers, x)
		}
		if fn.Pkg() == nil {
			return 0
		}
		switch fn.Pkg().Path() {
		case "strings":
			switch fn.Name() {
			case "Cut", "CutPrefix", "CutSuffix", "Trim", "TrimPrefix", "TrimSuffix", "TrimSpace", "TrimLeft", "TrimRight", "Split", "SplitN", "Fields":
				return exprTaint(fc, e.Args[0], t, resolvers, x)
			}
		case "path", "path/filepath":
			if fn.Name() == "Base" || fn.Name() == "Clean" || fn.Name() == "Dir" {
				return exprTaint(fc, e.Args[0], t, resolvers, x)
			}
		}
	}
	return 0
}

func (x *goExtractor) noteDirectQuerySinks(f scanFunc, t map[types.Object]uint64, params map[paramKey]queryKind) {
	if len(t) == 0 {
		return
	}
	impVars := collectImportVars(f.fc, f.fn.Body)
	addMask := func(mask uint64, k queryKind) {
		for i := 0; i < 64; i++ {
			if mask&(1<<i) != 0 {
				params[paramKey{f.id, i}] |= k
			}
		}
	}
	ast.Inspect(f.fn.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			fn, _ := f.fc.u.Info.Uses[nameIdent(n.Fun)].(*types.Func)
			if idx, k, ok := seedParamKind(fn); ok && idx < len(n.Args) {
				addMask(exprTaint(f.fc, n.Args[idx], t, nil, x), k)
			}
		case *ast.BinaryExpr:
			if n.Op != token.EQL && n.Op != token.NEQ {
				return true
			}
			switch {
			case isObjectNameCall(f.fc, n.X):
				addMask(exprTaint(f.fc, n.Y, t, nil, x), queryObj)
			case isObjectNameCall(f.fc, n.Y):
				addMask(exprTaint(f.fc, n.X, t, nil, x), queryObj)
			case isUsedPkgPathExpr(f.fc, n.X, impVars):
				addMask(exprTaint(f.fc, n.Y, t, nil, x), queryPkg)
			case isUsedPkgPathExpr(f.fc, n.Y, impVars):
				addMask(exprTaint(f.fc, n.X, t, nil, x), queryPkg)
			}
		case *ast.IndexExpr:
			if isPackagesImportsField(f.fc, n.X) {
				addMask(exprTaint(f.fc, n.Index, t, nil, x), queryPkg)
			}
		}
		return true
	})
}

func (x *goExtractor) propagateQueryCallArgs(f scanFunc, t map[types.Object]uint64, params map[paramKey]queryKind) bool {
	if len(t) == 0 {
		return false
	}
	changed := false
	ast.Inspect(f.fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fn, _ := f.fc.u.Info.Uses[nameIdent(call.Fun)].(*types.Func)
		if fn == nil {
			return true
		}
		sig, isSig := fn.Type().(*types.Signature)
		calleeID := x.objectID(fn)
		if !isSig || calleeID == "" || sig.Params().Len() == 0 {
			return true
		}
		for k, arg := range call.Args {
			idx := min(k, sig.Params().Len()-1)
			kind := params[paramKey{calleeID, idx}]
			if kind == 0 {
				continue
			}
			mask := exprTaint(f.fc, arg, t, nil, x)
			for i := 0; i < 64; i++ {
				if mask&(1<<i) != 0 && params[paramKey{f.id, i}]&kind != kind {
					params[paramKey{f.id, i}] |= kind
					changed = true
				}
			}
		}
		return true
	})
	return changed
}
