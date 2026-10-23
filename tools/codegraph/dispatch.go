package main

import (
	"go/ast"
	"go/types"
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
)

// Bu dosya TR dil yapılarını onları çalıştıran Go işlevlerine bağlar (deneme 9 H1, deneme 10; kod-grafigi-40 tasarımı,
// TCB onaylı). Gönderim tablosu elle yazılmaz, yorumlayıcının kodundan okunur: derleyicinin (interp.compiler
// metotları) `switch e := expr.(type) { case *ast.CallExpression: ... }` dallarında çağrılan işlevler (dalın içindeki
// kapanışlar dahil). codeindex her TR bildiriminin kullandığı yapıları kaydeder (Construct); her biri bu tablodaki
// işlevlere `implemented_by` kenarı (INFERRED) olur: metin yerleştirmesi değeri metne çeviren Value.Inspect'e, çağrı
// derleyicinin çağrı işlevine (oradan VM.callPrepared'a).

// dispatchTable dil yapısından Go uygulamalarına giden tablodur.
type dispatchTable struct {
	kinds map[string][]string // AST türü ("CallExpression") -> işlev kimlikleri
}

// readDispatch derleyicinin gönderim tablosunu modülden okur.
func readDispatch(m *module) *dispatchTable {
	t := &dispatchTable{kinds: map[string][]string{}}
	astPkg, interpPkg := m.Path+"/internal/lang/ast", m.Path+"/internal/lang/interp"
	forEachFile(m, func(u *unit, f *ast.File, rel string) {
		if isTestFile(rel) || strings.TrimSuffix(u.PkgPath, "_test") != interpPkg {
			return
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil || !strings.Contains(recvTypeName(fd), "compiler") {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if s, ok := n.(*ast.TypeSwitchStmt); ok {
					t.typeSwitch(u.Info, s, astPkg)
				}
				return true
			})
		}
	})
	return t
}

// typeSwitch derleyicinin AST türü dallarını tabloya yazar.
func (t *dispatchTable) typeSwitch(info *types.Info, s *ast.TypeSwitchStmt, astPkg string) {
	for _, cc := range s.Body.List {
		c := cc.(*ast.CaseClause)
		for _, e := range c.List {
			star, ok := e.(*ast.StarExpr)
			if !ok {
				continue
			}
			id := nameIdent(star.X)
			tn, ok := info.Uses[id].(*types.TypeName)
			if id == nil || !ok || tn.Pkg() == nil || tn.Pkg().Path() != astPkg {
				continue
			}
			for _, f := range clauseCallees(info, c.Body, astPkg) {
				if !contains(t.kinds[tn.Name()], f) {
					t.kinds[tn.Name()] = append(t.kinds[tn.Name()], f)
				}
			}
		}
	}
}

// clauseCallees dalın (iç kapanışları dahil) çağırdığı işlevlerdir; AST ve ileti yardımcıları hariç.
func clauseCallees(info *types.Info, body []ast.Stmt, astPkg string) []string {
	seen := map[string]bool{}
	var out []string
	for _, st := range body {
		ast.Inspect(st, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			fn, ok := info.Uses[nameIdent(c.Fun)].(*types.Func)
			if !ok || fn.Pkg() == nil || fn.Pkg().Path() == astPkg || strings.HasSuffix(fn.Pkg().Path(), "/internal/lang/messages") {
				return true
			}
			if id := funcRef(fn); id != "" && !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
			return true
		})
	}
	sort.Strings(out)
	return out
}

// recvTypeName metodun alıcı tipinin adıdır.
func recvTypeName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return ""
	}
	t := fd.Recv.List[0].Type
	if s, ok := t.(*ast.StarExpr); ok {
		t = s.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// linkConstructs TR bildirimlerinin dil yapılarını gönderim tablosundaki Go işlevlerine `implemented_by` ile bağlar.
func linkConstructs(b *builder, t *dispatchTable, idx *codeindex.Index, ids map[string]string) {
	for _, c := range idx.Constructs {
		if from := ids[c.In]; from != "" {
			for _, to := range t.kinds[c.Kind] {
				b.addEdge(from, to, edgeImplementedBy, inferred, c.Pos.File, c.Pos.Line)
			}
		}
	}
}
