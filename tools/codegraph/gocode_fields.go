package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"
)

// rawFieldUse tip bilgisiyle çözülmüş bir alan erişimidir; alanın düğümü bütün birimler
// gezildikten sonra bulunur (alan başka dosyada ya da birimde tanımlı olabilir).
type rawFieldUse struct {
	posKey string // alanın bildirim konumu: dosya:satır:sütun (birimler arasında aynıdır)
	owner  string
	site   Site
}

func (x *goExtractor) posKey(p token.Pos) string {
	pos := x.m.Fset.Position(p)
	return x.rel(pos.Filename) + ":" + strconv.Itoa(pos.Line) + ":" + strconv.Itoa(pos.Column)
}

// fieldNodes bir struct tipinin alanlarını düğüm yapar (type → field `contains`) ve alanın bildirim
// konumunu düğüme eşler. Erişimler konumla bulunur: gömülü (promoted) alanlar, tip parametreli
// örnekler ve anahtarlı bileşik değerler aynı alana çözülür.
func (x *goExtractor) fieldNodes(fc *fileCtx, ts *ast.TypeSpec, typeNode, pkg string) {
	st, ok := ts.Type.(*ast.StructType)
	if !ok || st.Fields == nil {
		return
	}
	qual := func(p *types.Package) string { return p.Name() }
	for _, f := range st.Fields.List {
		idents := f.Names
		if len(idents) == 0 { // gömülü alan: adı tipin adıdır
			if id := embeddedIdent(f.Type); id != nil {
				idents = []*ast.Ident{id}
			}
		}
		for _, id := range idents {
			if id.Name == "_" {
				continue
			}
			fid := "go:field:" + pkg + "." + ts.Name.Name + "." + id.Name
			line := x.line(id.Pos())
			detail := ""
			if obj := fc.u.Info.Defs[id]; obj != nil {
				detail = types.TypeString(obj.Type(), qual)
			}
			x.b.addNode(Node{ID: fid, Kind: kindField, Name: pathBase(pkg) + "." + ts.Name.Name + "." + id.Name,
				File: fc.rel, Line: line, End: x.line(f.End()), Detail: detail})
			x.b.addEdge(typeNode, fid, edgeContains, extracted, fc.rel, line)
			x.fieldAt[x.posKey(id.Pos())] = fid
		}
	}
}

// embeddedIdent gömülü alanın tip adını verir (*pkg.T, T[int] biçimleri dahil).
func embeddedIdent(e ast.Expr) *ast.Ident {
	for {
		switch v := e.(type) {
		case *ast.StarExpr:
			e = v.X
		case *ast.IndexExpr:
			e = v.X
		case *ast.IndexListExpr:
			e = v.X
		case *ast.SelectorExpr:
			return v.Sel
		case *ast.Ident:
			return v
		default:
			return nil
		}
	}
}

func pathBase(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

// resolveFieldUses biriken ham erişimleri alan düğümlerine bağlar: okuma `reads_field`, yazma ve
// adres alma `writes_field` (kipler Site.Mode'da); her erişim yeriyle kenarda saklanır.
func (x *goExtractor) resolveFieldUses() {
	for _, u := range x.fieldUses {
		fid, ok := x.fieldAt[u.posKey]
		if !ok {
			continue
		}
		if u.site.Mode == modeRead || u.site.Mode == modeReadWrite {
			x.b.addSite(u.owner, fid, edgeReadsField, u.site)
		}
		if u.site.Mode != modeRead {
			x.b.addSite(u.owner, fid, edgeWritesField, u.site)
		}
	}
}

// Erişim kipleri.
const (
	modeRead      = "read"
	modeWrite     = "write"
	modeReadWrite = "read+write"
	modeElemWrite = "elem-write" // x.f[i] = v, *x.f = v: alanın içeriği değişir, alan değil
	modeAddr      = "addr"       // &x.f
	modeLiteral   = "literal"    // T{f: v}
)

// markLHS bir atamanın sol tarafındaki seçiciyi işaretler; x.f[i] gibi indeksli ya da yıldızlı
// hedefte seçici elem-write olur.
func markLHS(modes map[ast.Expr]string, e ast.Expr, mode string) {
	e = unparen(e)
	if sel, ok := e.(*ast.SelectorExpr); ok {
		modes[sel] = mode
		return
	}
	for {
		switch v := e.(type) {
		case *ast.IndexExpr:
			e = unparen(v.X)
		case *ast.SliceExpr:
			e = unparen(v.X)
		case *ast.StarExpr:
			e = unparen(v.X)
		case *ast.SelectorExpr:
			modes[v] = modeElemWrite
			return
		default:
			return
		}
	}
}

func unparen(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}

func peelIndex(e ast.Expr) ast.Expr {
	for {
		switch v := unparen(e).(type) {
		case *ast.IndexExpr:
			e = v.X
		case *ast.SliceExpr:
			e = v.X
		default:
			return unparen(e)
		}
	}
}

// accessModes ebeveyn düğümden çocuk seçicilerin erişim kipini belirler (Inspect ebeveyni
// önce gezer, bu yüzden seçiciye gelindiğinde kip hazırdır).
func accessModes(modes map[ast.Expr]string, n ast.Node) {
	switch n := n.(type) {
	case *ast.AssignStmt:
		mode := modeWrite
		if n.Tok != token.ASSIGN && n.Tok != token.DEFINE {
			mode = modeReadWrite
		}
		for _, l := range n.Lhs {
			markLHS(modes, l, mode)
		}
	case *ast.IncDecStmt:
		markLHS(modes, n.X, modeReadWrite)
	case *ast.RangeStmt:
		if n.Tok == token.ASSIGN {
			for _, e := range []ast.Expr{n.Key, n.Value} {
				if e != nil {
					markLHS(modes, e, modeWrite)
				}
			}
		}
	case *ast.UnaryExpr:
		if n.Op == token.AND {
			if sel, ok := peelIndex(n.X).(*ast.SelectorExpr); ok {
				modes[sel] = modeAddr
			}
		}
	}
}

// captured func literal'in dış kapsamdan yakaladığı yerel değişkenlerdir ("ad tip" satırları,
// ilk kullanım sırasıyla; literal içinde atanıyorsa "(assigned)"). Paket düzeyi değişkenler ve
// alanlar yakalama değildir.
func (x *goExtractor) captured(info *types.Info, lit *ast.FuncLit) []string {
	assigned := map[*types.Var]bool{}
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		var targets []ast.Expr
		switch n := n.(type) {
		case *ast.AssignStmt:
			if n.Tok != token.DEFINE {
				targets = n.Lhs
			}
		case *ast.IncDecStmt:
			targets = []ast.Expr{n.X}
		case *ast.UnaryExpr:
			if n.Op == token.AND {
				targets = []ast.Expr{n.X}
			}
		}
		for _, t := range targets {
			if id, ok := peelIndex(t).(*ast.Ident); ok {
				if v, ok := info.Uses[id].(*types.Var); ok {
					assigned[v] = true
				}
			}
		}
		return true
	})
	seen := map[*types.Var]bool{}
	var out []string
	ast.Inspect(lit, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		v, ok := info.Uses[id].(*types.Var)
		if !ok || v.IsField() || v.Pkg() == nil || seen[v] || v.Parent() == v.Pkg().Scope() {
			return true
		}
		if v.Pos() >= lit.Pos() && v.Pos() < lit.End() {
			return true // literalin kendi parametresi ya da yereli
		}
		seen[v] = true
		entry := v.Name() + " " + types.TypeString(v.Type(), func(p *types.Package) string { return p.Name() })
		if assigned[v] {
			entry += " (assigned)"
		}
		out = append(out, entry)
		return true
	})
	return out
}

// closureNode func literal için düğüm ekler ve kapsayan işlev ya da literalden `contains`
// kenarı çeker; düğümün kimliğini verir. stack, literalin kendisi dahil ata düğümlerdir.
func (x *goExtractor) closureNode(fc *fileCtx, owner string, lit *ast.FuncLit, stack []ast.Node, closures map[*ast.FuncLit]string) string {
	parent, parentName := owner, ""
	if n := x.b.nodes[owner]; n != nil {
		parentName = n.Name
	}
	for i := len(stack) - 2; i >= 0; i-- {
		if outer, ok := stack[i].(*ast.FuncLit); ok {
			parent = closures[outer]
			parentName = x.b.nodes[parent].Name
			break
		}
	}
	pos := x.m.Fset.Position(lit.Pos())
	id := "go:closure:" + strings.TrimPrefix(owner, "go:") + "@" + strconv.Itoa(pos.Line) + ":" + strconv.Itoa(pos.Column)
	x.b.addNode(Node{ID: id, Kind: kindClosure, Name: fmt.Sprintf("%s > func literal@L%d", parentName, pos.Line),
		File: fc.rel, Line: pos.Line, End: x.line(lit.End()), Detail: "func literal", Captured: x.captured(fc.u.Info, lit)})
	x.b.addEdge(parent, id, edgeContains, extracted, fc.rel, pos.Line)
	return id
}

// sortSites erişim yerlerini dosya ve satıra göre sıralar.
func sortSites(sites []Site) {
	sort.SliceStable(sites, func(i, j int) bool {
		a, b := sites[i], sites[j]
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
}
