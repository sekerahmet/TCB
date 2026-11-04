package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
)

type paramPathHelper struct {
	obj      types.Object
	paramIdx int
	prefix   string
}

// resolveParamPaths gövdesinde `reporoot.Path(sabit + parametre)` kuran tek düzey Go yardımcılarını bulur ve onları
// sabit dize argümanıyla çağıran yerlerden kurulan kök yolunu kaydeder (`readExtensionJSON(t, "package.nls.json")`).
func (x *goExtractor) resolveParamPaths() {
	var funcs []scanFunc
	var helpers []paramPathHelper
	for obj, f := range x.idx.scans.allFuncs {
		if f.id != "" {
			funcs = append(funcs, f)
		}
		ast.Inspect(f.fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !x.rootPathCall(f.fc, call) || nameIdent(call.Fun).Name != "Path" || len(call.Args) != 1 {
				return true
			}
			bin, ok := ast.Unparen(call.Args[0]).(*ast.BinaryExpr)
			if !ok || bin.Op != token.ADD {
				return true
			}
			prefix := constString(f.fc, ast.Unparen(bin.X))
			if prefix == "" {
				return true
			}
			id, ok := ast.Unparen(bin.Y).(*ast.Ident)
			if !ok {
				return true
			}
			if idx := paramIndex(f.fc, f.fn, f.fc.u.Info.Uses[id]); idx >= 0 {
				helpers = append(helpers, paramPathHelper{obj: obj, paramIdx: idx, prefix: prefix})
			}
			return true
		})
	}
	if len(helpers) == 0 {
		return
	}
	sort.Slice(funcs, func(i, j int) bool { return funcs[i].id < funcs[j].id })
	byObj := map[types.Object][]paramPathHelper{}
	for _, h := range helpers {
		byObj[h.obj] = append(byObj[h.obj], h)
	}
	for _, caller := range funcs {
		ast.Inspect(caller.fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || call.Ellipsis.IsValid() {
				return true
			}
			hs := byObj[caller.fc.u.Info.Uses[nameIdent(call.Fun)]]
			for _, h := range hs {
				if h.paramIdx >= len(call.Args) {
					continue
				}
				if arg := constString(caller.fc, ast.Unparen(call.Args[h.paramIdx])); arg != "" {
					x.addRootRef(caller.fc, caller.id, call.Pos(), h.prefix+arg, nil)
				}
			}
			return true
		})
	}
}
