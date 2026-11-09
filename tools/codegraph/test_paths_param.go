package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"path"
	"sort"
	"strings"
)

// noteParamRootJoin test dışı kodda `filepath.Join(kök, ..., sabit)` çağrısının kökü işlevin parametresiyse (çağıran verir;
// testlerde geçici fikstür kökü) yol sabitini o köke göre okunmuş sayar: sabite ulaşan testler depodaki dosyayı okumaz
// (gaps, "`tests-for planning_questions.txt` sınav testini varsayılan çıktıda göstermiyor": parseExamWords(root) sınav
// dosyasını buildGraph'ın kökünden okuyor, buildGraph'a ulaşan 172 test okuyucu sayılıyordu).
func (x *goExtractor) noteParamRootJoin(fc *fileCtx, call *ast.CallExpr) {
	if fc.isTest || len(call.Args) < 2 || !isStdJoinCall(fc, call) {
		return
	}
	root, _ := ast.Unparen(call.Args[0]).(*ast.Ident)
	if v, ok := fc.u.Info.Uses[root].(*types.Var); root == nil || !ok || v.Kind() != types.ParamVar {
		return
	}
	for _, a := range call.Args[1:] {
		id, ok := unwrapDirExpr(a).(*ast.Ident)
		if !ok {
			continue
		}
		switch obj := fc.u.Info.Uses[id].(type) {
		case *types.Const:
			x.idx.rootedArg[x.objectID(obj)] = true
		case *types.Var: // paket düzeyi yol listesi üzerinde dönen değişken (`for _, rel := range identityMaps`)
			if over := x.rangeOver[obj]; over != nil {
				x.idx.rootedArg[x.objectID(over)] = true
			}
		}
	}
}

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

// resolveTestBareReads test dosyalarındaki (ve testing alan yardımcı işlevlerdeki) okuma çağrılarına verilen çıplak
// dosya adlarını kaydeder (`os.ReadFile("README.md")`); çözüm çağıran testin paket klasöründedir (linkTestPaths).
func (x *goExtractor) resolveTestBareReads() {
	var funcs []scanFunc
	for _, f := range x.idx.scans.allFuncs {
		if f.id != "" && (f.fc.isTest || takesTesting(f.fc, f.fn)) {
			funcs = append(funcs, f)
		}
	}
	sort.Slice(funcs, func(i, j int) bool { return funcs[i].id < funcs[j].id })
	for _, f := range funcs {
		ast.Inspect(f.fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if arg, ok := bareReadArg(f.fc, call); ok {
					for _, v := range x.concreteExprStrings(f.fc, f.fn, arg, 4) {
						if v != "" && v != "." && v != ".." && len(v) <= 200 && !strings.ContainsAny(v, "/\\*?[\t\n\r {}$%") {
							x.idx.pathRefs = append(x.idx.pathRefs, pathRef{owner: f.id, dir: path.Dir(f.fc.rel), file: f.fc.rel, line: x.line(call.Pos()), text: v, local: true})
						}
					}
				}
			}
			return true
		})
	}
}

// takesTesting işlevin parametrelerinden biri standart `testing` paketinden mi (`*testing.T`, `testing.TB`).
func takesTesting(fc *fileCtx, fn *ast.FuncDecl) bool {
	obj := fc.u.Info.Defs[fn.Name]
	if obj == nil {
		return false
	}
	sig, _ := obj.Type().(*types.Signature)
	for i := 0; sig != nil && i < sig.Params().Len(); i++ {
		s := sig.Params().At(i).Type().String()
		if strings.HasPrefix(s, "*testing.") || strings.HasPrefix(s, "testing.") {
			return true
		}
	}
	return false
}

// bareReadArg çalışma klasörüne göre dosya okuyan çağrının yol argümanını verir.
func bareReadArg(fc *fileCtx, call *ast.CallExpr) (ast.Expr, bool) {
	fn, ok := fc.u.Info.Uses[nameIdent(call.Fun)].(*types.Func)
	if !ok || fn.Pkg() == nil || len(call.Args) == 0 {
		return nil, false
	}
	switch pkg, name := fn.Pkg().Path(), fn.Name(); {
	case pkg == "os" && (name == "ReadFile" || name == "Open" || name == "OpenFile" || name == "Stat" || name == "Lstat" || name == "ReadDir"),
		pkg == "io/ioutil" && (name == "ReadFile" || name == "ReadDir"),
		strings.HasSuffix(pkg, "internal/reporoot") && name == "ReadDir":
		return call.Args[0], true
	case pkg == "go/parser" && name == "ParseDir" && len(call.Args) >= 2:
		return call.Args[1], true
	case pkg == "go/parser" && name == "ParseFile" && len(call.Args) >= 3:
		id, ok := ast.Unparen(call.Args[2]).(*ast.Ident)
		return call.Args[1], ok && id.Name == "nil"
	}
	return nil, false
}

// concreteExprStrings dize değişmezi, sabit, dize listesi döngüsü ya da tek atamalı yerel değişkenin dize değerleridir.
func (x *goExtractor) concreteExprStrings(fc *fileCtx, fn *ast.FuncDecl, e ast.Expr, depth int) []string {
	if depth < 0 {
		return nil
	}
	switch v := ast.Unparen(e).(type) {
	case *ast.BasicLit:
		if s, ok := stringLit(v); ok && v.Kind == token.STRING {
			return []string{s}
		}
	case *ast.Ident, *ast.SelectorExpr:
		if s := constString(fc, v); s != "" {
			return []string{s}
		}
		if id, ok := v.(*ast.Ident); ok {
			obj := fc.u.Info.Uses[id]
			if vals, ok := x.valuesOf(obj); ok {
				return vals
			}
			if rhs := localValue(fc, fn, obj); rhs != nil {
				return x.concreteExprStrings(fc, fn, rhs, depth-1)
			}
		}
	}
	return nil
}
