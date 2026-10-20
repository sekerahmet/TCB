package main

import (
	"reflect"
	"sort"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

const astPkg = "github.com/sekerahmet/TCB/internal/lang/ast"

// walkAST bir TR sözdizimi ağacını (ast paketinin işaretçi düğümleri) derinlemesine gezer. Her
// ast düğümü için fn çağrılır, false dönerse çocuklarına inilmez. Düğüm türleri yansıma ile
// gezilir: ast'ye eklenen yeni bir düğüm türü elle eklenmeden de görülür.
func walkAST(root any, fn func(n any) bool) {
	var rec func(v reflect.Value)
	rec = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Interface:
			if !v.IsNil() {
				rec(v.Elem())
			}
		case reflect.Ptr:
			if v.IsNil() {
				return
			}
			if v.Type().Elem().PkgPath() == astPkg && !fn(v.Interface()) {
				return
			}
			rec(v.Elem())
		case reflect.Struct:
			t := v.Type()
			for i := 0; i < v.NumField(); i++ {
				if t.Field(i).IsExported() && t.Field(i).Type.Name() != "Token" {
					rec(v.Field(i))
				}
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < v.Len(); i++ {
				rec(v.Index(i))
			}
		case reflect.Map:
			keys := v.MapKeys()
			sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
			for _, k := range keys {
				rec(v.MapIndex(k))
			}
		}
	}
	rec(reflect.ValueOf(root))
}

// trScope bir sahibin gövdesindeki yerel adlar ve bağlama yerleridir.
type trScope struct {
	locals   map[string]bool
	bindings map[*ast.Identifier]bool // bağlama yerindeki adlar: kullanım değildir
	handled  map[*ast.Identifier]bool // MemberExpression'da ele alınmış (takma ad) adlar
	called   map[*ast.Identifier]bool // çağrılan adlar
}

// collectScope gövdede bağlanan adları toplar (parametreler, değişkenler, döngü ve yakala
// değişkenleri, liste üretecinin değişkeni): bunlar üst düzey tanımları ve yerleşikleri gölgeler.
func collectScope(root any) *trScope {
	sc := &trScope{locals: map[string]bool{}, bindings: map[*ast.Identifier]bool{}, handled: map[*ast.Identifier]bool{}, called: map[*ast.Identifier]bool{}}
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

// scan owner'ın gövdesindeki adları çözer ve kenarları ekler.
func (x *trExtractor) scan(f *trFile, owner trOwner, root any) {
	sc := collectScope(root)
	walkAST(root, func(n any) bool {
		switch n := n.(type) {
		case *ast.CallExpression:
			x.call(f, owner, sc, n)
		case *ast.MemberExpression:
			x.member(f, owner, sc, n)
		case *ast.Identifier:
			x.ident(f, owner, sc, n)
		}
		return true
	})
}

func (x *trExtractor) call(f *trFile, o trOwner, sc *trScope, c *ast.CallExpression) {
	switch fn := c.Function.(type) {
	case *ast.Identifier:
		sc.called[fn] = true
	}
	for _, a := range c.NamedArgs {
		if k, ok := x.lang.Find(codes.NamespaceParam, a.Name); ok {
			if id, ok := nameCode(k); ok {
				x.link(o, id, edgeUsesName, extracted, f.rel, a.Token.Line)
			}
		}
	}
}

// ident bir adın kullanımını çözer: yerel ad kenar üretmez; üst düzey tanım (bu dosyada ya da
// takma adsız aktarılan modülde) `calls`/`uses`; aksi halde yerleşik ad kodu `uses_name`.
func (x *trExtractor) ident(f *trFile, o trOwner, sc *trScope, id *ast.Identifier) {
	if sc.bindings[id] || sc.handled[id] || sc.locals[id.Value] {
		return
	}
	line := id.Line()
	if def, ok := f.visible[id.Value]; ok {
		x.useDef(f, o, def, sc.called[id], line)
		return
	}
	if k, ok := x.lang.Find(codes.NamespaceGlobal, id.Value); ok {
		if code, ok := nameCode(k); ok {
			x.link(o, code, edgeUsesName, extracted, f.rel, line)
		}
	}
}

// useDef bir tanıma başvuruyu kenara çevirir: işlev ve yapı için `calls` (çağrıldıysa
// EXTRACTED, değer olarak INFERRED), sabit ve değişken için `uses`.
func (x *trExtractor) useDef(f *trFile, o trOwner, def string, called bool, line int) {
	kind, conf := edgeUses, extracted
	if n := x.b.nodes[def]; n != nil && (n.Kind == kindTRFunc || n.Kind == kindTRStruct) {
		kind = edgeCalls
		if !called {
			conf = inferred
		}
	}
	x.link(o, def, kind, conf, f.rel, line)
}

// member `a.b` ifadesini çözer. a bir `olarak` takma adıysa b o modülün tanımıdır (EXTRACTED);
// değilse alıcının tipi bilinmez: aynı adlı görünür TR metotlarına INFERRED kenar ve, b bir
// yerleşik üye adıysa, ad koduna kenar (TR metodu da varsa INFERRED).
func (x *trExtractor) member(f *trFile, o trOwner, sc *trScope, m *ast.MemberExpression) {
	line := m.NameToken.Line
	if line == 0 {
		line = m.Line()
	}
	if obj, ok := m.Object.(*ast.Identifier); ok && !sc.locals[obj.Value] {
		if target := f.aliases[obj.Value]; target != nil {
			sc.handled[obj] = true
			if def, ok := target.defs[m.Property]; ok {
				x.useDef(f, o, def, true, line)
			}
			return
		}
	}
	candidates := x.methodCandidates(f, m.Property)
	for _, mid := range candidates {
		x.link(o, mid, edgeCalls, inferred, f.rel, line)
	}
	if k, ok := x.lang.Find(codes.NamespaceMember, m.Property); ok {
		if code, ok := nameCode(k); ok {
			conf := extracted
			if len(candidates) > 0 {
				conf = inferred
			}
			x.link(o, code, edgeUsesName, conf, f.rel, line)
		}
	}
}

// methodCandidates aynı adlı, dosyada görünen TR metotlarıdır (en çok 4: fazlası tahmin değildir).
func (x *trExtractor) methodCandidates(f *trFile, name string) []string {
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
func (x *trExtractor) visibleFiles(f *trFile) []*trFile {
	out := []*trFile{f}
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
