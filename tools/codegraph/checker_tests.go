package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"path"
	"slices"
	"strings"
)

// Bu dosya TR programlarını tip denetleyicisinden (checker.Checker.CheckProgram) geçiren Go testlerini bağlar:
//   - CheckProgram'ı koşulsuz çağıran ortak test yardımcıları (ör. internal/testkit/trrun.MustExecChecked → ml.mustRun);
//   - CheckProgram'ı bir seçenek bayrağı altında çağıran giriş noktalarına (ör. source.Prepare içinde `if !s.TypeCheck { return }`)
//     bayrağı açık vererek çağrı yapan testler;
//   - Denetleyiciyi içeren ikiliyi derleyen (`builds`) alt süreç paketlerinde, denetleyiciye varan alt komutu
//     (`codes.Spellings`, ör. `TR_cmd_check`) `os/exec`'e geçiren testler (ör. TestDocTRCodeBlocks).

type paramGuard struct {
	param int
	field string
}

type callSite struct {
	file string
	line int
}

type optCall struct {
	callee, owner, file string
	line, param         int
	fields              []string
}

type checkerState struct {
	runners  map[string]callSite
	guarded  map[string]paramGuard
	optCalls []optCall
}

func newCheckerState() *checkerState {
	return &checkerState{runners: map[string]callSite{}, guarded: map[string]paramGuard{}}
}

const checkerRelPkg = "internal/lang/checker"

// noteCheckerFunc test dışı bir işlevin gövdesinde CheckProgram çağrısının koşulsuz mu yoksa `!param.Alan` erken dönüş
// koruması altında mı yapıldığını kaydeder.
func (x *goExtractor) noteCheckerFunc(fc *fileCtx, id string, obj *types.Func, body *ast.BlockStmt) {
	if fc.isTest || body == nil || fc.base == x.m.Path+"/"+checkerRelPkg {
		return
	}
	sig, ok := obj.Type().(*types.Signature)
	if !ok {
		return
	}
	params := map[*types.Var]int{}
	for i := 0; i < sig.Params().Len(); i++ {
		params[sig.Params().At(i)] = i
	}
	checkID := memberID(x.m.Path+"/"+checkerRelPkg, "Checker", "CheckProgram")
	var guards []paramGuard
	for _, st := range body.List {
		if line := x.findDirectCall(fc, st, checkID); line > 0 {
			switch len(guards) {
			case 0:
				x.idx.checker.runners[id] = callSite{fc.rel, line}
			case 1:
				x.idx.checker.guarded[id] = guards[0]
			}
			return
		}
		if g, ok := negFieldReturnGuard(fc, st, params); ok {
			guards = append(guards, g)
			continue
		}
		if _, isRet := st.(*ast.ReturnStmt); isRet {
			return
		}
	}
}

func (x *goExtractor) findDirectCall(fc *fileCtx, st ast.Stmt, target string) int {
	line := 0
	ast.Inspect(st, func(n ast.Node) bool {
		if line > 0 {
			return false
		}
		if _, lit := n.(*ast.FuncLit); lit {
			return false
		}
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if id := nameIdent(c.Fun); id != nil {
			if fn, ok := fc.u.Info.Uses[id].(*types.Func); ok && x.objectID(fn) == target {
				line = x.line(c.Pos())
				return false
			}
		}
		return true
	})
	return line
}

// negFieldReturnGuard `if !s.TypeCheck { return ... }` biçimindeki erken dönüş korumasını tanır.
func negFieldReturnGuard(fc *fileCtx, st ast.Stmt, params map[*types.Var]int) (paramGuard, bool) {
	ifs, ok := st.(*ast.IfStmt)
	if !ok || ifs.Else != nil || len(ifs.Body.List) == 0 {
		return paramGuard{}, false
	}
	if _, ret := ifs.Body.List[len(ifs.Body.List)-1].(*ast.ReturnStmt); !ret {
		return paramGuard{}, false
	}
	un, ok := ifs.Cond.(*ast.UnaryExpr)
	if !ok || un.Op != token.NOT {
		return paramGuard{}, false
	}
	sel, ok := un.X.(*ast.SelectorExpr)
	if !ok {
		return paramGuard{}, false
	}
	pid, ok := sel.X.(*ast.Ident)
	if !ok {
		return paramGuard{}, false
	}
	v, ok := fc.u.Info.Uses[pid].(*types.Var)
	idx, isParam := params[v]
	if !ok || !isParam {
		return paramGuard{}, false
	}
	return paramGuard{idx, sel.Sel.Name}, true
}

// noteOptCall yapı değişmeziyle anahtar-değer argümanı geçiren çağrıları kaydeder (`Options{TypeCheck: true}`).
func (x *goExtractor) noteOptCall(fc *fileCtx, owner string, c *ast.CallExpr) {
	id := nameIdent(c.Fun)
	if id == nil {
		return
	}
	fn, ok := fc.u.Info.Uses[id].(*types.Func)
	if !ok {
		return
	}
	ref := x.objectID(fn)
	if ref == "" {
		return
	}
	for i, arg := range c.Args {
		lit, ok := arg.(*ast.CompositeLit)
		if !ok {
			continue
		}
		var fields []string
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			k, ok := kv.Key.(*ast.Ident)
			if !ok || isFalseIdent(fc, kv.Value) {
				continue
			}
			fields = append(fields, k.Name)
		}
		if len(fields) > 0 {
			x.idx.checker.optCalls = append(x.idx.checker.optCalls, optCall{
				callee: ref, owner: owner, file: fc.rel, line: x.line(c.Pos()), param: i, fields: fields,
			})
		}
	}
}

func isFalseIdent(fc *fileCtx, e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	if !ok || id.Name != "false" {
		return false
	}
	_, isConst := fc.u.Info.Uses[id].(*types.Const)
	return isConst
}

// linkCheckerTests denetleyiciye varan ortak test yardımcılarından, seçenekli çağrılardan ve alt süreç komutlarından
// CheckProgram'a `tests` [INFERRED] kenarı çeker.
func linkCheckerTests(b *builder, idx *goIndex, reach *testReach) {
	checkID := memberID(b.module+"/"+checkerRelPkg, "Checker", "CheckProgram")
	if b.nodes[checkID] == nil {
		return
	}
	st := idx.checker
	checking := map[string]bool{checkID: true}
	for fn, site := range st.runners {
		checking[fn] = true
		for _, t := range reach.testsWithin(fn, testMaxSteps/2) {
			b.addEdge(t, checkID, edgeTests, inferred, site.file, site.line)
		}
	}
	for _, c := range st.optCalls {
		g, ok := st.guarded[c.callee]
		if !ok || c.param != g.param || !slices.Contains(c.fields, g.field) {
			continue
		}
		if idx.inTestFile[c.owner] {
			for _, t := range reach.testsWithin(c.owner, testMaxSteps/2) {
				b.addEdge(t, checkID, edgeTests, inferred, c.file, c.line)
			}
		} else {
			checking[c.owner] = true
		}
	}
	expandCheckingCallers(b, st.guarded, checking)
	linkCheckerSubprocessTests(b, idx, reach, checkID, checking)
}

func expandCheckingCallers(b *builder, guarded map[string]paramGuard, checking map[string]bool) {
	for changed := true; changed; {
		changed = false
		for k, e := range b.edges {
			if k.kind != edgeCalls || e.Confidence != extracted || !checking[k.to] || checking[k.from] {
				continue
			}
			if _, isGuarded := guarded[k.from]; isGuarded {
				continue
			}
			checking[k.from] = true
			changed = true
		}
	}
}

func linkCheckerSubprocessTests(b *builder, idx *goIndex, reach *testReach, checkID string, checking map[string]bool) {
	pkgCmds := checkerCommandsByPkg(b, idx, checking)
	dirCmds := map[string]map[string]bool{}
	for k := range b.edges {
		if k.kind != edgeBuilds || b.nodes[k.from] == nil {
			continue
		}
		cmds := pkgCmds[k.to]
		if len(cmds) == 0 {
			continue
		}
		dir := path.Dir(b.nodes[k.from].File)
		if dirCmds[dir] == nil {
			dirCmds[dir] = map[string]bool{}
		}
		for c := range cmds {
			dirCmds[dir][c] = true
		}
	}
	if len(dirCmds) == 0 {
		return
	}
	execSinks := map[paramKey]bool{
		{memberID("os/exec", "", "Command"), 1}:        true,
		{memberID("os/exec", "", "CommandContext"), 2}: true,
	}
	for changed := true; changed; {
		changed = false
		for _, f := range idx.snip.flows {
			if f.from.fn != "" && execSinks[f.callee] && !execSinks[f.from] {
				execSinks[f.from] = true
				changed = true
			}
		}
	}
	for _, f := range idx.snip.flows {
		cmds := dirCmds[path.Dir(f.file)]
		if f.from.fn != "" || !execSinks[f.callee] || len(cmds) == 0 {
			continue
		}
		if !slices.ContainsFunc(idx.snip.expandTexts(f.texts, 0), func(s string) bool { return cmds[s] }) {
			continue
		}
		for _, t := range reach.testsWithin(f.owner, testMaxSteps/2) {
			b.addEdge(t, checkID, edgeTests, inferred, f.file, f.line)
		}
	}
}

func checkerCommandsByPkg(b *builder, idx *goIndex, checking map[string]bool) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	note := func(pkg, cmd string) {
		if pkg == "" || cmd == "" {
			return
		}
		if out[pkg] == nil {
			out[pkg] = map[string]bool{}
		}
		out[pkg][cmd] = true
	}
	dirPkg := func(file string) string {
		if d := path.Dir(file); d != "." {
			return pkgID(b.module + "/" + d)
		}
		return pkgID(b.module)
	}
	for k, e := range b.edges {
		nn := b.nodes[e.Via]
		if k.kind != edgeCalls || !strings.HasPrefix(e.Via, "name:") || nn == nil || !checking[k.to] || b.nodes[k.from] == nil {
			continue
		}
		pkg := dirPkg(b.nodes[k.from].File)
		for _, sp := range nn.Aliases {
			note(pkg, sp)
		}
	}
	for _, cc := range idx.caseCalls {
		if slices.ContainsFunc(cc.callees, func(id string) bool { return checking[id] }) {
			pkg := dirPkg(cc.file)
			for _, v := range cc.values {
				note(pkg, v)
			}
		}
	}
	return out
}
