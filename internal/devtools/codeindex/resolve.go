package codeindex

import (
	"sort"
	"strconv"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// owner bir başvurunun çıkış düğümüdür (başvuruyu içeren bildirim, test ya da modül).
type owner struct {
	id   string
	kind Kind
}

// scope bir sahibin gövdesindeki yerel adlar ve bağlama yerleridir.
type scope struct {
	locals       map[string]bool
	bindings     map[*ast.Identifier]bool // bağlama yerindeki adlar: kullanım değildir
	handled      map[*ast.Identifier]bool // MemberExpression'da ele alınmış (takma ad) adlar
	called       map[*ast.Identifier]bool // çağrılan adlar
	calledMember map[*ast.MemberExpression]bool
}

// collectScope gövdede bağlanan adları toplar (parametreler, değişkenler, döngü ve yakala
// değişkenleri, liste üretecinin değişkeni): bunlar üst düzey tanımları ve yerleşikleri gölgeler.
func collectScope(root any) *scope {
	sc := &scope{locals: map[string]bool{}, bindings: map[*ast.Identifier]bool{}, handled: map[*ast.Identifier]bool{},
		called: map[*ast.Identifier]bool{}, calledMember: map[*ast.MemberExpression]bool{}}
	add := func(names ...string) {
		for _, n := range names {
			if n != "" {
				sc.locals[n] = true
			}
		}
	}
	walkAST(root, func(n any) bool {
		switch n := n.(type) {
		case *ast.FunctionLiteral:
			for _, p := range n.Parameters {
				add(p.Name)
			}
		case *ast.VarStatement:
			sc.bindings[n.Name] = true
			add(n.Name.Value)
		case *ast.MultiVarStatement:
			for _, id := range n.Names {
				sc.bindings[id] = true
				add(id.Value)
			}
		case *ast.ForInStatement:
			add(n.KeyVar, n.ValueVar)
		case *ast.TryCatchStatement:
			add(n.ErrorVar)
		case *ast.ListComprehension:
			add(n.VarName)
		}
		return true
	})
	return sc
}

// resolve bir dosyadaki her bildirimin gövdesini tarar; bildirim dışı üst düzey ifadeler modül
// düğümünden çıkar.
func (x *indexer) resolve(f *file) {
	mod := owner{NodeID(KindModule, f.rel, ""), KindModule}
	for _, s := range f.res.program.Statements {
		switch s := s.(type) {
		case *ast.ExpressionStatement:
			if fl, ok := s.Expression.(*ast.FunctionLiteral); ok && fl.Name != "" {
				x.scan(f, owner{NodeID(KindFunction, f.rel, fl.Name), KindFunction}, fl)
				continue
			}
			x.scan(f, mod, s)
		case *ast.StructStatement:
			x.scanStruct(f, s)
		case *ast.TestStatement:
			x.scan(f, owner{NodeID(KindTest, f.rel, strconv.Itoa(s.Token.Line)), KindTest}, s.Body)
		case *ast.VarStatement:
			if s.Value != nil {
				kind := KindVariable
				if s.IsConst {
					kind = KindConstant
				}
				x.scan(f, owner{NodeID(kind, f.rel, s.Name.Value), kind}, s.Value)
			}
		case *ast.ImportStatement:
		default:
			x.scan(f, mod, s)
		}
	}
}

func (x *indexer) scanStruct(f *file, s *ast.StructStatement) {
	for _, fld := range s.Fields {
		if fld.DefaultValue != nil {
			x.scan(f, owner{NodeID(KindField, f.rel, s.Name+"."+fld.Name), KindField}, fld.DefaultValue)
		}
	}
	names := make([]string, 0, len(s.Methods))
	for n := range s.Methods {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		x.scan(f, owner{NodeID(KindMethod, f.rel, s.Name+"."+n), KindMethod}, s.Methods[n])
	}
}

// scan sahibin gövdesindeki adları çözer ve bağları ekler.
func (x *indexer) scan(f *file, o owner, root any) {
	sc := collectScope(root)
	walkAST(root, func(n any) bool {
		switch n := n.(type) {
		case *ast.CallExpression:
			x.call(f, o, sc, n)
		case *ast.MemberExpression:
			x.member(f, o, sc, n)
		case *ast.Identifier:
			x.ident(f, o, sc, n)
		}
		return true
	})
}

func (x *indexer) call(f *file, o owner, sc *scope, c *ast.CallExpression) {
	switch fn := c.Function.(type) {
	case *ast.Identifier:
		sc.called[fn] = true
	case *ast.MemberExpression:
		sc.calledMember[fn] = true
	}
	for _, a := range c.NamedArgs {
		if k, ok := x.lang.Find(codes.NamespaceParam, a.Name); ok {
			if id, ok := x.builtin(k); ok {
				x.link(o, id, EdgeUses, Extracted, RoleRead, tokenPos(f, a.Token.Line, a.Token.Column, a.Name))
			}
		}
	}
}

// ident bir adın kullanımını çözer: yerel ad bağ üretmez; üst düzey tanım (bu dosyada ya da
// takma adsız aktarılan modülde) `calls`/`uses`; aksi halde yerleşik ad.
func (x *indexer) ident(f *file, o owner, sc *scope, id *ast.Identifier) {
	if sc.bindings[id] || sc.handled[id] || sc.locals[id.Value] {
		return
	}
	pos := tokenPos(f, id.Token.Line, id.Token.Column, id.Value)
	if def, ok := f.visible[id.Value]; ok {
		x.useDef(o, def, sc.called[id], pos)
		return
	}
	if k, ok := x.lang.Find(codes.NamespaceGlobal, id.Value); ok {
		if code, ok := x.builtin(k); ok {
			role := RoleRead
			if sc.called[id] {
				role = RoleCall
			}
			x.link(o, code, EdgeUses, Extracted, role, pos)
		}
	}
}

// useDef bir tanıma başvuruyu bağa çevirir: işlev ve yapı için `calls` (çağrıldıysa EXTRACTED,
// değer olarak INFERRED), sabit ve değişken için `uses`.
func (x *indexer) useDef(o owner, def string, called bool, pos Pos) {
	kind, conf, role := EdgeUses, Extracted, RoleRead
	if n := x.nodes[def]; n != nil && (n.Kind == KindFunction || n.Kind == KindStruct) {
		kind = EdgeCalls
		if called {
			role = RoleCall
		} else {
			conf = Inferred
		}
	}
	x.link(o, def, kind, conf, role, pos)
}

// member `a.b` ifadesini çözer. a bir `olarak` takma adıysa b o modülün tanımıdır (EXTRACTED);
// değilse alıcının tipi bilinmez: aynı adlı görünür TR metotlarına INFERRED bağ ve, b bir
// yerleşik üye adıysa, ad koduna bağ (TR metodu da varsa INFERRED).
func (x *indexer) member(f *file, o owner, sc *scope, m *ast.MemberExpression) {
	line, col := m.NameToken.Line, m.NameToken.Column
	if line == 0 {
		line, col = m.Token.Line, 0
	}
	pos := tokenPos(f, line, col, m.Property)
	if obj, ok := m.Object.(*ast.Identifier); ok && !sc.locals[obj.Value] {
		if target := f.aliases[obj.Value]; target != nil {
			sc.handled[obj] = true
			if def, ok := target.defs[m.Property]; ok {
				x.useDef(o, def, true, pos)
			}
			return
		}
	}
	role := RoleRead
	if sc.calledMember[m] {
		role = RoleCall
	}
	candidates := x.methodCandidates(f, m.Property)
	for _, mid := range candidates {
		x.link(o, mid, EdgeCalls, Inferred, RoleCall, pos)
	}
	if k, ok := x.lang.Find(codes.NamespaceMember, m.Property); ok {
		if code, ok := x.builtin(k); ok {
			conf := Extracted
			if len(candidates) > 0 {
				conf = Inferred
			}
			x.link(o, code, EdgeUses, conf, role, pos)
		}
	}
}

// methodCandidates aynı adlı, dosyada görünen TR metotlarıdır (en çok 4: fazlası tahmin değildir).
func (x *indexer) methodCandidates(f *file, name string) []string {
	var ids []string
	seen := map[string]bool{}
	for _, g := range x.visibleFiles(f) {
		for _, id := range g.methods[name] {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	sort.Strings(ids)
	if len(ids) > 4 {
		return nil
	}
	return ids
}

// visibleFiles f ve onun (takma adlı olanlar dahil) aktardığı dosyalardır.
func (x *indexer) visibleFiles(f *file) []*file {
	out := []*file{f}
	seen := map[string]bool{f.rel: true}
	for i := 0; i < len(out); i++ {
		for _, imp := range out[i].imports {
			if g := x.files[imp.file]; g != nil && !seen[imp.file] {
				seen[imp.file] = true
				out = append(out, g)
			}
		}
	}
	return out
}

// builtin bir ad kodunun yerleşik düğümünü (gerekirse ekleyerek) verir; yerleşik olmayan kodlar
// (anahtar sözcük, ileti, komut) sayılmaz.
func (x *indexer) builtin(k codes.Code) (string, bool) {
	if k == codes.None || !k.IsBuiltin() {
		return "", false
	}
	id := NodeID(KindBuiltin, "", k.String())
	if x.nodes[id] == nil {
		x.nodes[id] = &Node{ID: id, Kind: KindBuiltin, Name: k.String()}
	}
	return id, true
}

// link bir bağ ve ona karşılık gelen başvuruyu ekler; test sahibinden çıkan çağrı aynı zamanda
// `tests` bağıdır ve başvurusu `test` rolündedir.
func (x *indexer) link(o owner, to string, kind EdgeKind, conf Confidence, role Role, pos Pos) {
	x.addEdge(o.id, to, kind, conf, pos)
	if o.kind == KindTest && kind == EdgeCalls {
		x.addEdge(o.id, to, EdgeTests, conf, pos)
		role = RoleTest
	}
	x.occurrence(to, role, conf, pos, o.id)
}

func (x *indexer) occurrence(node string, role Role, conf Confidence, pos Pos, in string) {
	x.occ = append(x.occ, Occurrence{Node: node, Role: role, Confidence: conf, Pos: pos, In: in})
}

// tokenPos bir adın konumudur: sözcük birimi satırda ve sütunda başlar, ad kod noktası kadar sürer.
func tokenPos(f *file, line, col int, name string) Pos {
	p := Pos{File: f.rel, Line: line, Col: col}
	if col > 0 {
		p.EndLine, p.EndCol = line, col+utf8.RuneCountInString(name)
	}
	return p
}
