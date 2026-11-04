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
// reporoot.Path(...), repodir.Resolve(""), kökten Join); değilse değerler yalnızca birer parçadır. depth parametreden
// ya da yardımcı çağrısından kaç adım gidileceğidir.
func (x *goExtractor) evalPath(fc *fileCtx, fn *ast.FuncDecl, e ast.Expr, depth int) ([]string, bool) {
	return x.evalPathWith(fc, fn, e, nil, depth)
}

func (x *goExtractor) evalPathWith(fc *fileCtx, fn *ast.FuncDecl, e ast.Expr, bind map[types.Object][]string, depth int) ([]string, bool) {
	if depth < 0 {
		return nil, false
	}
	switch e := e.(type) {
	case *ast.ParenExpr:
		return x.evalPathWith(fc, fn, e.X, bind, depth)
	case *ast.BasicLit:
		if s, err := strconv.Unquote(e.Value); err == nil && e.Kind == token.STRING {
			return []string{s}, false
		}
	case *ast.CallExpr:
		if vals, rooted := x.evalPathCall(fc, fn, e, bind, depth); rooted || len(vals) > 0 {
			return vals, rooted
		}
		if depth > 0 {
			return x.evalCallReturn(fc, fn, e, 0, bind, depth-1)
		}
	case *ast.SelectorExpr:
		if obj := fc.u.Info.Uses[e.Sel]; obj != nil && depth > 0 {
			if refs := x.idx.scans.assigns[obj]; len(refs) == 1 {
				return x.evalAssignWith(refs[0], nil, depth-1)
			}
		}
	case *ast.Ident:
		obj := fc.u.Info.Uses[e]
		if obj == nil {
			return nil, false
		}
		if vals, ok := bind[obj]; ok {
			return vals, false
		}
		if vals, ok := x.valuesOf(obj); ok {
			return vals, false
		}
		if ref, ok := localAssign(fc, fn, obj); ok && depth > 0 {
			return x.evalAssignWith(ref, bind, depth-1)
		}
		if refs := x.idx.scans.assigns[obj]; len(refs) == 1 && depth > 0 {
			return x.evalAssignWith(refs[0], nil, depth-1)
		}
		if i := paramIndex(fc, fn, obj); i >= 0 && depth > 0 {
			return x.evalParam(fn, fc, i, depth-1)
		}
	}
	return nil, false
}

func (x *goExtractor) evalAssignWith(ref assignRef, bind map[types.Object][]string, depth int) ([]string, bool) {
	if ref.idx < 0 {
		return x.evalPathWith(ref.fc, ref.fn, ref.rhs, bind, depth)
	}
	call, ok := ref.rhs.(*ast.CallExpr)
	if !ok || depth <= 0 {
		return nil, false
	}
	if ref.idx == 0 {
		if vals, rooted := x.evalPathCall(ref.fc, ref.fn, call, bind, depth); rooted {
			return vals, true
		}
	}
	return x.evalCallReturn(ref.fc, ref.fn, call, ref.idx, bind, depth-1)
}

// evalPathCall reporoot.Root, reporoot.Path, repodir.Resolve("") ve filepath/path.Join çağrılarını çözer.
func (x *goExtractor) evalPathCall(fc *fileCtx, fn *ast.FuncDecl, call *ast.CallExpr, bind map[types.Object][]string, depth int) ([]string, bool) {
	f, _ := fc.u.Info.Uses[nameIdent(call.Fun)].(*types.Func)
	if f == nil || f.Pkg() == nil {
		return nil, false
	}
	pkg := f.Pkg().Path()
	root := strings.HasSuffix(pkg, "internal/reporoot")
	switch {
	case root && f.Name() == "Root" && len(call.Args) == 0:
		return []string{"."}, true
	case root && f.Name() == "Path":
		parts, ok := x.segments(fc, fn, call.Args, bind, depth)
		return joinAll([]string{"."}, parts), ok
	case strings.HasSuffix(pkg, "tools/internal/repodir") && f.Name() == "Resolve" && len(call.Args) == 1:
		vals, rooted := x.evalPathWith(fc, fn, call.Args[0], bind, depth)
		if !rooted && len(vals) == 1 && vals[0] == "" {
			return []string{"."}, true
		}
	case (pkg == "path/filepath" || pkg == "path") && f.Name() == "Join" && len(call.Args) > 0:
		first, rooted := x.evalPathWith(fc, fn, call.Args[0], bind, depth)
		if !rooted {
			return nil, false
		}
		parts, ok := x.segments(fc, fn, call.Args[1:], bind, depth)
		return joinAll(first, parts), ok
	}
	return nil, false
}

// evalCallReturn modül içindeki bir yardımcı işlevin idx'inci dönüş değerini çözer (`root, _, g := repoGraph(t)`).
func (x *goExtractor) evalCallReturn(fc *fileCtx, fn *ast.FuncDecl, call *ast.CallExpr, idx int, bind map[types.Object][]string, depth int) ([]string, bool) {
	fnObj, _ := fc.u.Info.Uses[nameIdent(call.Fun)].(*types.Func)
	if fnObj == nil {
		return nil, false
	}
	if sig, ok := fnObj.Type().(*types.Signature); !ok || idx >= sig.Results().Len() || sig.Results().At(idx).Type().String() != "string" {
		return nil, false
	}
	callee, ok := x.idx.scans.allFuncs[fnObj]
	if !ok || callee.fn.Body == nil {
		return nil, false
	}
	calleeBind := map[types.Object][]string{}
	for i, a := range call.Args {
		if p := paramAt(callee.fc, callee.fn, i); p != nil {
			if vals, rooted := x.evalPathWith(fc, fn, a, bind, depth); !rooted && len(vals) > 0 {
				calleeBind[p] = vals
			}
		}
	}
	var out []string
	ast.Inspect(callee.fn.Body, func(n ast.Node) bool {
		if _, isLit := n.(*ast.FuncLit); isLit {
			return false
		}
		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		switch {
		case idx < len(ret.Results):
			if vals, rooted := x.evalPathWith(callee.fc, callee.fn, ret.Results[idx], calleeBind, depth); rooted {
				out = append(out, vals...)
			}
		case len(ret.Results) == 1:
			if inner, ok := ret.Results[0].(*ast.CallExpr); ok {
				if vals, rooted := x.evalAssignWith(assignRef{callee.fc, callee.fn, inner, idx}, calleeBind, depth); rooted {
					out = append(out, vals...)
				}
			}
		}
		return true
	})
	return out, len(out) > 0
}

// segments yol parçalarının her birinin alabildiği değerlerdir; çözülmeyen parça bütün yolu çözülmez yapar.
func (x *goExtractor) segments(fc *fileCtx, fn *ast.FuncDecl, args []ast.Expr, bind map[types.Object][]string, depth int) ([][]string, bool) {
	var out [][]string
	for _, a := range args {
		vals, rooted := x.evalPathWith(fc, fn, a, bind, depth)
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

// localAssign işlevde bir kez atanan yerel değişkenin sağ tarafını verir (çoklu dönüş dahil: `root, _, g := repoGraph(t)`).
func localAssign(fc *fileCtx, fn *ast.FuncDecl, obj types.Object) (assignRef, bool) {
	rhs, idx := singleLocalAssign(fc, fn.Body, obj)
	if rhs == nil {
		return assignRef{}, false
	}
	return assignRef{fc, fn, rhs, idx}, true
}

// singleLocalAssign root içinde bir kez atanan yerel değişkenin sağ taraf ifadesini ve çoklu dönüş indisini (-1: tekli) verir.
func singleLocalAssign(fc *fileCtx, root ast.Node, obj types.Object) (ast.Expr, int) {
	var rhs ast.Expr
	idx := -1
	count := 0
	ast.Inspect(root, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, l := range as.Lhs {
			if id, ok := l.(*ast.Ident); ok && (fc.u.Info.Defs[id] == obj || fc.u.Info.Uses[id] == obj) {
				count++
				if as.Tok != token.DEFINE && as.Tok != token.ASSIGN {
					continue
				}
				switch {
				case len(as.Lhs) == len(as.Rhs):
					rhs, idx = as.Rhs[i], -1
				case len(as.Rhs) == 1 && len(as.Lhs) > 1:
					rhs, idx = as.Rhs[0], i
				}
			}
		}
		return true
	})
	if count != 1 || rhs == nil || usesObj(fc, rhs, obj) {
		return nil, -1
	}
	return rhs, idx
}

func usesObj(fc *fileCtx, n ast.Node, obj types.Object) bool {
	found := false
	ast.Inspect(n, func(cur ast.Node) bool {
		if id, ok := cur.(*ast.Ident); ok && fc.u.Info.Uses[id] == obj {
			found = true
		}
		return !found
	})
	return found
}

// localValue işlevde bir kez tekli atanan yerel değişkenin değeridir (`ext := ".tr"`).
func localValue(fc *fileCtx, fn *ast.FuncDecl, obj types.Object) ast.Expr {
	if ref, ok := localAssign(fc, fn, obj); ok && ref.idx < 0 {
		return ref.rhs
	}
	return nil
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

// paramAt fn'nin idx'inci parametresinin nesnesidir (adsız ya da taşarsa nil).
func paramAt(fc *fileCtx, fn *ast.FuncDecl, idx int) types.Object {
	i := 0
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			if i == idx {
				return fc.u.Info.Defs[name]
			}
			i++
		}
		if len(field.Names) == 0 {
			if i == idx {
				return nil
			}
			i++
		}
	}
	return nil
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
	var ev func(ast.Expr, int) []string
	ev = func(e ast.Expr, depth int) []string {
		if depth > 4 {
			return nil
		}
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
				return ev(rhs, depth+1)
			}
		}
		return nil
	}
	return func(e ast.Expr) []string { return ev(e, 0) }
}

// scanFilters bir gövdedeki dosya adı süzgecidir: atlama koşulundaki (`return`/`continue` tek deyimli `if`) olumsuz ek
// dahil, olumlu ek hariç; boolean dönüş (`return strings.HasSuffix(rel, ".tr")`), başka koşullardaki olumlu ek,
// `switch filepath.Ext(...)` durumları ve etiketsiz switch'in dolu durumları dahil, boş durumu hariç.
func scanFilters(body ast.Node, ev func(ast.Expr) []string) (inc, exc []string, dirs bool) {
	extVars := collectExtVars(body)
	isExt := func(e ast.Expr) bool {
		if isExtCall(e) {
			return true
		}
		id, ok := e.(*ast.Ident)
		return ok && extVars[id.Name]
	}
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
			suffixAtoms(s.Cond, skip, isExt, ev, add)
			dirs = dirs || skip && skipsFiles(s.Cond)
		case *ast.ReturnStmt:
			if len(s.Results) == 1 {
				suffixAtoms(s.Results[0], false, isExt, ev, add)
			}
		case *ast.SwitchStmt:
			for _, c := range s.Body.List {
				cc := c.(*ast.CaseClause)
				for _, e := range cc.List {
					switch {
					case isExt(s.Tag):
						for _, v := range ev(e) {
							add(v, true)
						}
					case s.Tag == nil:
						suffixAtoms(e, len(cc.Body) == 0, isExt, ev, add)
					}
				}
			}
		}
		return true
	})
	return inc, exc, dirs
}

// collectExtVars gövdede bir kez `filepath.Ext(...)` ya da `path.Ext(...)` ile atanan yerel adlardır.
func collectExtVars(body ast.Node) map[string]bool {
	counts := map[string]int{}
	ext := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, l := range as.Lhs {
			id, ok := l.(*ast.Ident)
			if !ok {
				continue
			}
			counts[id.Name]++
			if len(as.Lhs) == len(as.Rhs) && isExtCall(as.Rhs[i]) {
				ext[id.Name] = true
			}
		}
		return true
	})
	for k, n := range counts {
		if n != 1 {
			delete(ext, k)
		}
	}
	return ext
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

// isSkip bloğun tek deyimi bir atlama dönüşü ya da döngü atlaması mı (`return true` atlama değildir).
func isSkip(b *ast.BlockStmt) bool {
	if len(b.List) != 1 {
		return false
	}
	switch s := b.List[0].(type) {
	case *ast.ReturnStmt:
		if len(s.Results) == 1 {
			if id, ok := s.Results[0].(*ast.Ident); ok && id.Name == "true" {
				return false
			}
		}
		return true
	case *ast.BranchStmt:
		return true
	}
	return false
}

// suffixAtoms koşuldaki ek sınamalarını (strings.HasSuffix(x, "ek"), filepath.Ext(x) ==/!= "ek") olumluluklarıyla verir;
// neg koşulun tersinin istendiğidir (atlama koşulu).
func suffixAtoms(e ast.Expr, neg bool, isExt func(ast.Expr) bool, ev func(ast.Expr) []string, add func(string, bool)) {
	switch e := e.(type) {
	case *ast.ParenExpr:
		suffixAtoms(e.X, neg, isExt, ev, add)
	case *ast.UnaryExpr:
		if e.Op == token.NOT {
			suffixAtoms(e.X, !neg, isExt, ev, add)
		}
	case *ast.BinaryExpr:
		switch e.Op {
		case token.LAND, token.LOR:
			suffixAtoms(e.X, neg, isExt, ev, add)
			suffixAtoms(e.Y, neg, isExt, ev, add)
		case token.EQL, token.NEQ:
			if !isExt(e.X) && !isNameCall(e.X) {
				return
			}
			for _, v := range ev(e.Y) {
				if isExt(e.X) || strings.Contains(strings.TrimPrefix(v, "."), ".") { // ".git", "testdata": klasör adı
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
