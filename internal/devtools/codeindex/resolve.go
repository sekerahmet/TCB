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
	writes       map[*ast.Identifier]bool       // atamanın sol tarafındaki adlar
	writeMember  map[*ast.MemberExpression]bool // atamanın sol tarafındaki a.b
	params       map[string]string              // sahibin parametresi: ad -> parametre düğümünün kimliği
	bindCount    map[string]int                 // gövdede (parametre dışında) kaç kez bağlandığı
	assigned     map[string]bool                // sonradan atanan adlar
	localType    map[string]string              // türü çıkarılan tek bağlamalı yerel: ad -> yapı kimliği (infer.go)
	localBasis   map[string]Basis               // localType'ın dayanağı: yapıcı (local_type) ya da dönüş (inferred_type)
	passed       map[*ast.Identifier]bool       // çağrının argümanı olan çıplak adlar
	boundTo      map[*ast.Identifier]string     // `değişken g = ad` / `g = ad`: değer olan ad -> atandığı yerel
	calledNames  map[string]bool                // gövdede çağrılan adlar (`g(...)`)
}

// collectScope gövdede bağlanan adları toplar (parametreler, değişkenler, döngü ve yakala
// değişkenleri, liste üretecinin değişkeni): bunlar üst düzey tanımları ve yerleşikleri gölgeler.
func collectScope(root any) *scope {
	sc := &scope{locals: map[string]bool{}, bindings: map[*ast.Identifier]bool{}, handled: map[*ast.Identifier]bool{},
		called: map[*ast.Identifier]bool{}, calledMember: map[*ast.MemberExpression]bool{},
		writes: map[*ast.Identifier]bool{}, writeMember: map[*ast.MemberExpression]bool{},
		params: map[string]string{}, bindCount: map[string]int{}, assigned: map[string]bool{}, localType: map[string]string{}, localBasis: map[string]Basis{},
		passed: map[*ast.Identifier]bool{}, boundTo: map[*ast.Identifier]string{}, calledNames: map[string]bool{}}
	add := func(names ...string) {
		for _, n := range names {
			if n != "" {
				sc.locals[n] = true
			}
		}
	}
	bind := func(names ...string) {
		add(names...)
		for _, n := range names {
			if n != "" {
				sc.bindCount[n]++
			}
		}
	}
	ast.Walk(root, func(n any) bool {
		switch n := n.(type) {
		case *ast.FunctionLiteral:
			for _, p := range n.Parameters {
				if any(n) == root {
					add(p.Name) // sahibin kendi parametresi: yeniden bağlama sayılmaz
				} else {
					bind(p.Name)
				}
			}
		case *ast.VarStatement:
			sc.bindings[n.Name] = true
			bind(n.Name.Value)
			if v, ok := n.Value.(*ast.Identifier); ok {
				sc.boundTo[v] = n.Name.Value
			}
		case *ast.CallExpression:
			if fn, ok := n.Function.(*ast.Identifier); ok {
				sc.calledNames[fn.Value] = true
			}
			for _, a := range n.Arguments {
				if id, ok := a.(*ast.Identifier); ok {
					sc.passed[id] = true
				}
			}
			for _, a := range n.NamedArgs {
				if id, ok := a.Value.(*ast.Identifier); ok {
					sc.passed[id] = true
				}
			}
		case *ast.MultiVarStatement:
			for _, id := range n.Names {
				sc.bindings[id] = true
				bind(id.Value)
			}
		case *ast.AssignStatement:
			sc.markWrite(n.Target)
			if t, ok := n.Target.(*ast.Identifier); ok {
				if v, ok := n.Value.(*ast.Identifier); ok {
					sc.boundTo[v] = t.Value
				}
			}
		case *ast.MultiAssignStatement:
			for _, t := range n.Targets {
				sc.markWrite(t)
			}
		case *ast.ForInStatement:
			bind(n.KeyVar, n.ValueVar)
		case *ast.TryCatchStatement:
			bind(n.ErrorVar)
		case *ast.ListComprehension:
			bind(n.VarName)
		}
		return true
	})
	return sc
}

// markWrite atamanın hedefini işaretler (ad ya da a.b; indeksli hedefte değişen şey ad değil içeriktir).
func (sc *scope) markWrite(target ast.Expression) {
	switch t := target.(type) {
	case *ast.Identifier:
		sc.writes[t] = true
		sc.assigned[t.Value] = true
	case *ast.MemberExpression:
		sc.writeMember[t] = true
	}
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
	x.bindOwner(f, o, sc, root)
	ast.Walk(root, func(n any) bool {
		x.recordConstruct(f, o, sc, n)
		switch n := n.(type) {
		case *ast.CallExpression:
			x.call(f, o, sc, n)
			x.forwardArgs(f, o, sc, n)
		case *ast.AssignStatement:
			x.storeParam(f, o, sc, n)
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
		pos := tokenPos(f, a.Token.Line, a.Token.Column, a.Name)
		if id, conf, basis := x.namedArgTarget(f, o, sc, c, a.Name); id != "" {
			x.occurrenceBasis(id, RoleArgument, conf, pos, o.id, basis)
			continue
		}
		if k, ok := x.lang.Find(codes.NamespaceParam, a.Name); ok {
			if id, ok := x.builtin(k); ok {
				x.link(o, id, EdgeUses, Extracted, RoleRead, pos)
			}
		}
	}
	x.optionKeys(f, o, sc, c)
}

// ident bir adın kullanımını çözer: yerel ad bağ üretmez; üst düzey tanım (bu dosyada ya da
// takma adsız aktarılan modülde) `calls`/`uses`; aksi halde yerleşik ad.
func (x *indexer) ident(f *file, o owner, sc *scope, id *ast.Identifier) {
	pos := tokenPos(f, id.Token.Line, id.Token.Column, id.Value)
	if x.identParam(o, sc, id, pos) {
		return
	}
	if sc.bindings[id] || sc.handled[id] || sc.locals[id.Value] {
		return
	}
	if def, ok := x.lookupVisible(f, id.Value); ok {
		x.useDef(o, def, sc.called[id], sc.writes[id], pos)
		return
	}
	if k, ok := x.lang.Find(codes.NamespaceGlobal, id.Value); ok {
		if code, ok := x.builtin(k); ok {
			role, basis := RoleRead, Basis("")
			switch {
			case sc.called[id]:
				role = RoleCall
			case sc.passed[id] || sc.calledNames[sc.boundTo[id]]:
				basis = BasisCallableValue // çağrılabilir bir yere gidiyor: argüman ya da çağrılan değişken
			}
			x.linkBasis(o, code, EdgeUses, Extracted, role, pos, basis)
		}
		return
	}
	x.noteUnresolved(id.Value, pos)
}

// noteUnresolved hiçbir kapsamda ve dilin hiçbir adında bulunmayan adı kaydeder (dizin kalitesinin
// ölçüsü: `tr dizin --durum` sayısını yazar). Dilin öğesi olan adlar (anahtar sözcük, tip) sayılmaz.
func (x *indexer) noteUnresolved(name string, pos Pos) {
	for _, ns := range []codes.Namespace{codes.NamespaceGlobal, codes.NamespaceType, codes.NamespaceMember} {
		if _, ok := x.lang.Find(ns, name); ok {
			return
		}
	}
	x.unresolved = append(x.unresolved, Unresolved{Name: name, Pos: pos})
}

// useDef bir tanıma başvuruyu bağa çevirir: işlev ve yapı için `calls` (çağrıldıysa EXTRACTED,
// değer olarak INFERRED), sabit ve değişken için `uses`.
func (x *indexer) useDef(o owner, def string, called, write bool, pos Pos) {
	kind, conf, role := EdgeUses, Extracted, RoleRead
	if write {
		role = RoleWrite
	}
	if n := x.nodes[def]; n != nil && (n.Kind == KindFunction || n.Kind == KindStruct) {
		kind = EdgeCalls
		if called {
			role = RoleCall
		} else {
			conf = Inferred
		}
	}
	x.link(o, def, kind, conf, role, pos)
	if n := x.nodes[def]; n != nil && n.Kind == KindStruct && called {
		x.ctorCall(o, def, pos)
	}
}

// ctorCall `Yapı(...)` çağrısını yapının kurucusuna da bağlar: çağrı kurucuyu çalıştırır, kurucunun çağırdıkları da
// bu çağrının zincirindedir (bağımsız denetim Y12: ağırlık_başlat'ı yalnızca Doğrusal'ın kurucusu çağırıyordu ve iki
// araç da "sınayan yok" diyordu). Başvuru eklenmez: kurucunun adı çağrı yerinde yazılı değildir.
func (x *indexer) ctorCall(o owner, structID string, pos Pos) {
	ctor, kind := x.structMember(structID, x.lang.Name(codes.CtxInit))
	if kind != KindMethod {
		return
	}
	x.addEdge(o.id, ctor, EdgeCalls, Extracted, pos)
	if o.kind == KindTest {
		x.addEdge(o.id, ctor, EdgeTests, Extracted, pos)
	}
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
				x.useDef(o, def, true, sc.writeMember[m], pos)
			}
			return
		}
	}
	if x.bindMember(f, o, sc, m, pos) {
		return
	}
	role := RoleRead
	if sc.calledMember[m] {
		role = RoleCall
	}
	candidates := x.methodCandidates(f, m.Property)
	for _, mid := range candidates {
		x.linkBasis(o, mid, EdgeCalls, Inferred, RoleCall, pos, BasisNameMatch)
	}
	x.noteMemberGuess(o, sc, m, pos, len(candidates) == 0)
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

// methodCandidates aynı adlı, dosyada görünen TR metotlarıdır (en çok 4: fazlası tahmin değildir). Görünen dosyalar
// f'den başlayıp aktarmaları (takma adlı ve iç olanlar dahil) genişlikçe gezilerek bulunur; beşinci aday görülünce ya da
// projede o adda metot yoksa gezinti durur: zincir aktarımlı projede her erişim bütün kapanışı geziyordu.
func (x *indexer) methodCandidates(f *file, name string) []string {
	if x.methodNames == nil {
		x.methodNames = map[string]bool{}
		for _, g := range x.files {
			for n := range g.methods {
				x.methodNames[n] = true
			}
		}
	}
	if !x.methodNames[name] {
		return nil
	}
	var ids []string
	seen := map[string]bool{}
	queue, visited := []*file{f}, map[string]bool{f.rel: true}
	for i := 0; i < len(queue); i++ {
		for _, id := range queue[i].methods[name] {
			if !seen[id] {
				seen[id] = true
				if ids = append(ids, id); len(ids) > 4 {
					return nil
				}
			}
		}
		for _, imp := range queue[i].imports {
			if g := x.files[imp.file]; g != nil && !visited[imp.file] {
				visited[imp.file] = true
				queue = append(queue, g)
			}
		}
	}
	sort.Strings(ids)
	return ids
}

// builtin bir ad kodunun yerleşik düğümünü (gerekirse ekleyerek) verir: global yerleşik, üye (Y) ve adlı argüman
// (arg). Kullanıcının yazdığı kodun kullandığı adlar bunlardır; anahtar sözcük, ileti ve komut sayılmaz. Kitaplığın
// kodu yerleşik değildir: kitaplıktaki bildirimin kendisidir, ayrı düğümü yoktur (dil tanımı 17.3).
func (x *indexer) builtin(k codes.Code) (string, bool) {
	if k == codes.None || k.IsLibrary() || !(k.IsBuiltin() || k.Namespace() == codes.NamespaceMember || k.Namespace() == codes.NamespaceParam) {
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
	x.linkBasis(o, to, kind, conf, role, pos, "")
}

// linkBasis link gibidir; üye erişiminin dayanağını da kaydeder.
func (x *indexer) linkBasis(o owner, to string, kind EdgeKind, conf Confidence, role Role, pos Pos, basis Basis) {
	x.addEdge(o.id, to, kind, conf, pos)
	if o.kind == KindTest && (kind == EdgeCalls || x.exercisesBuiltin(to, role, basis)) {
		x.addEdge(o.id, to, EdgeTests, conf, pos)
		role = RoleTest
	}
	x.occurrenceBasis(to, role, conf, pos, o.id, basis)
}

// exercisesBuiltin test bloğunda bir yerleşiğin geçmesi onu sınamak mıdır. İşlev ve metot yerleşiği çağrılınca ya da
// çağrılabilir bir yere gidince (BasisCallableValue: argüman olarak verilir ya da atandığı değişken çağrılır) sınanır;
// yalnızca değer olarak anılması (`değişken f = zaman_ms`, f çağrılmaz) onu sınamaz (bağımsız denetim S3: böyle bir blok
// testsiz yerleşiği testli gösteriyordu). Sabit, özellik, tip ve adlı argüman okunarak kullanılır, okunması sınamaktır.
func (x *indexer) exercisesBuiltin(to string, role Role, basis Basis) bool {
	n := x.nodes[to]
	if n == nil || n.Kind != KindBuiltin {
		return false
	}
	if role == RoleCall || basis == BasisCallableValue {
		return true
	}
	k, err := codes.Parse(n.Name)
	if err != nil {
		return false
	}
	def, _ := codes.DefinitionOf(k)
	return def.Kind != codes.KindFunction && def.Kind != codes.KindMethod
}

func (x *indexer) occurrence(node string, role Role, conf Confidence, pos Pos, in string) {
	x.occurrenceBasis(node, role, conf, pos, in, "")
}

func (x *indexer) occurrenceBasis(node string, role Role, conf Confidence, pos Pos, in string, basis Basis) {
	x.occ = append(x.occ, Occurrence{Node: node, Role: role, Confidence: conf, Pos: pos, In: in, Basis: basis})
}

// tokenPos bir adın konumudur: sözcük birimi satırda ve sütunda başlar, ad diskteki yazımının (parsed.written) kod
// noktası kadar sürer.
func tokenPos(f *file, line, col int, name string) Pos {
	p := Pos{File: f.rel, Line: line, Col: col}
	if col > 0 {
		p.EndLine, p.EndCol = line, col+utf8.RuneCountInString(f.res.written(name))
	}
	return p
}
