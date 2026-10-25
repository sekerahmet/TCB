package main

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"path"
	"strconv"
	"strings"
)

// evalPath bir tarama tabanını depo köküne göre yollara çözer. rooted: yol depo kökünden kuruldu (reporoot.Root(),
// reporoot.Path(...), kökten Join); değilse değerler yalnızca birer parçadır. depth parametreden çağırana kaç adım
// gidileceğidir.
func (x *goExtractor) evalPath(fc *fileCtx, fn *ast.FuncDecl, e ast.Expr, depth int) ([]string, bool) {
	switch e := e.(type) {
	case *ast.ParenExpr:
		return x.evalPath(fc, fn, e.X, depth)
	case *ast.BasicLit:
		if s, err := strconv.Unquote(e.Value); err == nil && e.Kind == token.STRING {
			return []string{s}, false
		}
	case *ast.CallExpr:
		return x.evalPathCall(fc, fn, e, depth)
	case *ast.Ident:
		obj := fc.u.Info.Uses[e]
		if obj == nil {
			return nil, false
		}
		if vals, ok := x.valuesOf(obj); ok {
			return vals, false
		}
		if rhs := localValue(fc, fn, obj); rhs != nil {
			return x.evalPath(fc, fn, rhs, depth)
		}
		if i := paramIndex(fc, fn, obj); i >= 0 && depth > 0 {
			return x.evalParam(fn, fc, i, depth-1)
		}
	}
	return nil, false
}

// evalPathCall reporoot.Root, reporoot.Path ve filepath/path.Join çağrılarını çözer.
func (x *goExtractor) evalPathCall(fc *fileCtx, fn *ast.FuncDecl, call *ast.CallExpr, depth int) ([]string, bool) {
	f, _ := fc.u.Info.Uses[nameIdent(call.Fun)].(*types.Func)
	if f == nil || f.Pkg() == nil {
		return nil, false
	}
	root := strings.HasSuffix(f.Pkg().Path(), "internal/reporoot")
	switch {
	case root && f.Name() == "Root" && len(call.Args) == 0:
		return []string{"."}, true
	case root && f.Name() == "Path":
		parts, ok := x.segments(fc, fn, call.Args, depth)
		return joinAll([]string{"."}, parts), ok
	case (f.Pkg().Path() == "path/filepath" || f.Pkg().Path() == "path") && f.Name() == "Join" && len(call.Args) > 0:
		first, rooted := x.evalPath(fc, fn, call.Args[0], depth)
		if !rooted {
			return nil, false
		}
		parts, ok := x.segments(fc, fn, call.Args[1:], depth)
		return joinAll(first, parts), ok
	}
	return nil, false
}

// segments yol parçalarının her birinin alabildiği değerlerdir; çözülmeyen parça bütün yolu çözülmez yapar.
func (x *goExtractor) segments(fc *fileCtx, fn *ast.FuncDecl, args []ast.Expr, depth int) ([][]string, bool) {
	var out [][]string
	for _, a := range args {
		vals, rooted := x.evalPath(fc, fn, a, depth)
		if rooted || len(vals) == 0 {
			return nil, false
		}
		out = append(out, vals)
	}
	return out, true
}

// joinAll tabanların ve parça değerlerinin bütün birleşimleridir.
func joinAll(bases []string, parts [][]string) []string {
	out := bases
	for _, p := range parts {
		var next []string
		for _, b := range out {
			for _, v := range p {
				next = append(next, path.Join(b, v))
			}
		}
		out = next
	}
	return out
}

// valuesOf bir döngü değişkeninin alabildiği değerlerdir: sabit liste ya da paket düzeyi liste üzerinde dönüyorsa.
func (x *goExtractor) valuesOf(obj types.Object) ([]string, bool) {
	st := x.idx.scans
	over, ok := st.ranges[obj]
	if !ok {
		return nil, false
	}
	if vals, ok := stringList(over.x); ok {
		return vals, true
	}
	if id, ok := over.x.(*ast.Ident); ok {
		vals, ok := st.lists[over.fc.u.Info.Uses[id]]
		return vals, ok
	}
	return nil, false
}

// localValue işlevde bir kez atanan yerel değişkenin değeridir (`root := reporoot.Root()`); birden çok atama ya da
// çoklu dönüş (`root, _, g := repoGraph(t)`) çözülmez.
func localValue(fc *fileCtx, fn *ast.FuncDecl, obj types.Object) ast.Expr {
	var value ast.Expr
	count := 0
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, l := range as.Lhs {
			if id, ok := l.(*ast.Ident); ok && (fc.u.Info.Defs[id] == obj || fc.u.Info.Uses[id] == obj) {
				count++
				if len(as.Lhs) == len(as.Rhs) {
					value = as.Rhs[i]
				}
			}
		}
		return true
	})
	if count != 1 {
		return nil
	}
	return value
}

// paramIndex obj fn'nin kaçıncı parametresi (değilse -1).
func paramIndex(fc *fileCtx, fn *ast.FuncDecl, obj types.Object) int {
	i := 0
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			if fc.u.Info.Defs[name] == obj {
				return i
			}
			i++
		}
		if len(field.Names) == 0 {
			i++
		}
	}
	return -1
}

// evalParam bir parametrenin değerini doğrudan çağıranların verdiği bağımsız değişkenlerden çözer; kökten çözülmeyen
// bir çağıran atlanır.
func (x *goExtractor) evalParam(fn *ast.FuncDecl, fc *fileCtx, i, depth int) ([]string, bool) {
	obj := fc.u.Info.Defs[fn.Name]
	if obj == nil {
		return nil, false
	}
	var out []string
	for _, c := range x.directCallers(x.objectID(obj)) {
		ast.Inspect(c.fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || c.fc.u.Info.Uses[nameIdent(call.Fun)] != obj || i >= len(call.Args) {
				return true
			}
			if vals, rooted := x.evalPath(c.fc, c.fn, call.Args[i], depth); rooted {
				out = append(out, vals...)
			}
			return true
		})
	}
	return out, len(out) > 0
}

// suffixEval bir süzgeç ifadesinin dize değerleridir: sabit, adlı sabit, bir kez atanan yerel ya da çağırana bağlanmış
// parametre (bind).
func (x *goExtractor) suffixEval(fc *fileCtx, fn *ast.FuncDecl, bind map[types.Object][]string) func(ast.Expr) []string {
	var ev func(ast.Expr) []string
	ev = func(e ast.Expr) []string {
		switch e := e.(type) {
		case *ast.BasicLit:
			if s, err := strconv.Unquote(e.Value); err == nil && e.Kind == token.STRING {
				return []string{s}
			}
		case *ast.Ident:
			obj := fc.u.Info.Uses[e]
			if vals, ok := bind[obj]; ok {
				return vals
			}
			if c, ok := obj.(*types.Const); ok && c.Val().Kind() == constant.String {
				return []string{constant.StringVal(c.Val())}
			}
			if rhs := localValue(fc, fn, obj); rhs != nil {
				return ev(rhs)
			}
		}
		return nil
	}
	return ev
}

// scanFilters bir gövdedeki dosya adı süzgecidir: atlama koşulundaki (`return`/`continue` tek deyimli `if`) olumsuz ek
// dahil, olumlu ek hariç; başka koşullardaki olumlu ek, `switch filepath.Ext(...)` durumları ve etiketsiz switch'in dolu
// durumları dahil, boş durumu hariç. Ek `strings.HasSuffix(x, ek)`, `filepath.Ext(x) ==/!= ek`, tam ad `d.Name() ==/!=
// ad` ya da `filepath.Base(x) ==/!= ad` sınamasıdır. dirs: atlama koşulu dosyaları atlıyor (`!d.IsDir()`), tarama
// yalnızca klasörleri gezer.
func scanFilters(body ast.Node, ev func(ast.Expr) []string) (inc, exc []string, dirs bool) {
	add := func(suffix string, positive bool) {
		if !strings.Contains(suffix, ".") || len(suffix) < 2 || strings.ContainsAny(suffix, "/ ") {
			return
		}
		if positive {
			inc = append(inc, suffix)
		} else {
			exc = append(exc, suffix)
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.IfStmt:
			skip := isSkip(s.Body)
			suffixAtoms(s.Cond, skip, ev, add)
			dirs = dirs || skip && skipsFiles(s.Cond)
		case *ast.SwitchStmt:
			for _, c := range s.Body.List {
				cc := c.(*ast.CaseClause)
				for _, e := range cc.List {
					switch {
					case isExtCall(s.Tag):
						for _, v := range ev(e) {
							add(v, true)
						}
					case s.Tag == nil:
						suffixAtoms(e, len(cc.Body) == 0, ev, add)
					}
				}
			}
		}
		return true
	})
	return inc, exc, dirs
}

// skipsFiles atlama koşulunun bir terimi `!x.IsDir()` mi (dosyalar atlanır).
func skipsFiles(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.ParenExpr:
		return skipsFiles(e.X)
	case *ast.BinaryExpr:
		return e.Op == token.LOR && (skipsFiles(e.X) || skipsFiles(e.Y))
	case *ast.UnaryExpr:
		call, ok := e.X.(*ast.CallExpr)
		if !ok || e.Op != token.NOT {
			return false
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		return ok && sel.Sel.Name == "IsDir"
	}
	return false
}

// isSkip bloğun tek deyimi bir dönüş ya da döngü atlaması mı (atlama koşulu).
func isSkip(b *ast.BlockStmt) bool {
	if len(b.List) != 1 {
		return false
	}
	switch b.List[0].(type) {
	case *ast.ReturnStmt, *ast.BranchStmt:
		return true
	}
	return false
}

// suffixAtoms koşuldaki ek sınamalarını (strings.HasSuffix(x, "ek"), filepath.Ext(x) ==/!= "ek") olumluluklarıyla verir;
// neg koşulun tersinin istendiğidir (atlama koşulu).
func suffixAtoms(e ast.Expr, neg bool, ev func(ast.Expr) []string, add func(string, bool)) {
	switch e := e.(type) {
	case *ast.ParenExpr:
		suffixAtoms(e.X, neg, ev, add)
	case *ast.UnaryExpr:
		if e.Op == token.NOT {
			suffixAtoms(e.X, !neg, ev, add)
		}
	case *ast.BinaryExpr:
		switch e.Op {
		case token.LAND, token.LOR:
			suffixAtoms(e.X, neg, ev, add)
			suffixAtoms(e.Y, neg, ev, add)
		case token.EQL, token.NEQ:
			if !isExtCall(e.X) && !isNameCall(e.X) {
				return
			}
			for _, v := range ev(e.Y) {
				if isExtCall(e.X) || strings.Contains(strings.TrimPrefix(v, "."), ".") { // ".git", "testdata": klasör adı
					add(v, (e.Op == token.EQL) != neg)
				}
			}
		}
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "HasSuffix" && len(e.Args) == 2 {
			for _, v := range ev(e.Args[1]) {
				add(v, !neg)
			}
		}
	}
}

// isNameCall dosyanın adını veren çağrı mı: d.Name() ya da filepath.Base(x).
func isNameCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && (sel.Sel.Name == "Name" && len(call.Args) == 0 || sel.Sel.Name == "Base" && len(call.Args) == 1)
}

// isExtCall filepath.Ext(x) ya da path.Ext(x) mi.
func isExtCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "Ext"
}
