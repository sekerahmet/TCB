package main

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"strings"
)

// Bu dosya yalnızca belli bir ortamda koşan testleri işaretler (deneme 9 H2). Bir testin gövdesinin başındaki
// `if koşul { t.Skip(...) }` (ya da başta çağrılan bir test yardımcısının aynı biçimdeki satırı) koşulu, testin
// `Skip` alanına kaynak metniyle yazılır: `g.lt == nil`, `testing.Short()`, `runtime.GOOS != "linux"`. Koşul
// değerlendirilmez; okuyan "CI bunu koşuyor mu" sorusunu kendisi cevaplar. tests-for her testin yanına yazar ve
// listelenen testlerin hepsi böyle bir koşula bağlıysa bunu tek satırla söyler.

// skipLead gövdenin başında bakılan deyim sayısıdır: atlama genellikle ilk kurulumdan hemen sonra gelir.
const skipLead = 4

// testSkips modüldeki test dosyalarının işlevlerini tarar ve testlerin atlanma koşullarını düğümlerine yazar.
func (x *goExtractor) testSkips() {
	helpers := map[string]string{} // test dosyasındaki işlev kimliği -> başındaki atlama koşulu
	type testDecl struct {
		id   string
		decl *ast.FuncDecl
		info *types.Info
	}
	var tests []testDecl
	for _, u := range x.m.Units {
		for _, f := range u.Files {
			if !isTestFile(x.m.Rel(x.m.Fset.Position(f.Pos()).Filename)) {
				continue
			}
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				fn, _ := u.Info.Defs[fd.Name].(*types.Func)
				if fn == nil {
					continue
				}
				id := funcRef(fn)
				if cond := x.leadingSkip(fd); cond != "" {
					helpers[id] = cond
				}
				if fd.Recv == nil && isTestName(fd.Name.Name) {
					tests = append(tests, testDecl{id, fd, u.Info})
				}
			}
		}
	}
	for _, t := range tests {
		cond := helpers[t.id]
		if cond == "" {
			cond = x.helperSkip(t.decl, t.info, helpers)
		}
		if n := x.b.nodes[t.id]; n != nil && cond != "" {
			n.Skip = cond
		}
	}
}

// leadingSkip gövdenin başındaki `if koşul { ...Skip... }` deyiminin koşul metnidir; koşulsuz Skip "always" olur.
func (x *goExtractor) leadingSkip(fd *ast.FuncDecl) string {
	for i, s := range fd.Body.List {
		if i >= skipLead {
			break
		}
		switch s := s.(type) {
		case *ast.IfStmt:
			if s.Init == nil && callsSkip(s.Body) {
				return x.source(s.Cond)
			}
		case *ast.ExprStmt:
			if callsSkip(s) {
				return "always"
			}
		}
	}
	return ""
}

// helperSkip testin başında çağrılan bir test yardımcısının atlama koşuludur (bir düzey).
func (x *goExtractor) helperSkip(fd *ast.FuncDecl, info *types.Info, helpers map[string]string) string {
	for i, s := range fd.Body.List {
		if i >= skipLead {
			break
		}
		var call *ast.CallExpr
		switch s := s.(type) {
		case *ast.ExprStmt:
			call, _ = s.X.(*ast.CallExpr)
		case *ast.AssignStmt:
			if len(s.Rhs) == 1 {
				call, _ = s.Rhs[0].(*ast.CallExpr)
			}
		}
		if call == nil {
			continue
		}
		if id := nameIdent(call.Fun); id != nil {
			if fn, ok := info.Uses[id].(*types.Func); ok {
				if cond := helpers[funcRef(fn)]; cond != "" {
					return cond
				}
			}
		}
	}
	return ""
}

// callsSkip düğümün içinde t.Skip / t.Skipf / t.SkipNow (ya da b.Skip ...) çağrısı var mı.
func callsSkip(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if sel, ok := c.Fun.(*ast.SelectorExpr); ok && strings.HasPrefix(sel.Sel.Name, "Skip") {
				found = true
			}
		}
		return !found
	})
	return found
}

// source ifadenin kaynak metnidir (tek satır).
func (x *goExtractor) source(e ast.Expr) string {
	var b bytes.Buffer
	if err := printer.Fprint(&b, token.NewFileSet(), e); err != nil {
		return "?"
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
