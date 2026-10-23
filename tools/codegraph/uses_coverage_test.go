package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path"
	"slices"
	"sort"
	"strings"
	"testing"
)

// Kural: who-uses kör nokta bırakmaz. go/types'ın Info.Uses kaydındaki her kullanım (tür, işlev, metot, sabit,
// değişken, alan; hangi tanımlayıcıda olursa olsun: imza, alan tipi, bileşik değer, tür kısıtı) grafikte kullanan
// bildirimden (işlev, onun içindeki kapanış, tür, paket düzeyi değer) o nesnenin düğümüne bir kenardır; çağrı
// EXTRACTED `calls` kenarıdır. Ters yönde de: grafikteki her Go `calls`/`uses` kenarının go/types'ta bir kullanımı
// vardır. Beklenen kimlikler çıkarıcının yardımcılarından değil go/types nesnesinden (paket yolu, alıcı, ad, nesne
// türü) bağımsızca kurulur; düğümü olmayan nesne atlanmaz, eksik sayılır (bağımsız denetim S1, S2).

// expectedUse go/types'ın gördüğü bir kullanımdır: hedef düğüm, kullanabilecek bildirimler, çağrı mı, yeri.
type expectedUse struct {
	target string
	owners []string // kenarın kaynağı bunlardan biri ya da onların kapanışı olmalı
	call   bool     // işlev çağrısı: EXTRACTED calls kenarı beklenir
	file   string
	line   int
}

// typesID nesnenin grafik kimliğini go/types'tan kurar; grafikte düğümü olmaması gereken nesnede (yerel, alan,
// modül dışı, adsız arayüz metodu) "".
func typesID(mod string, obj types.Object) string {
	if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != mod && !strings.HasPrefix(obj.Pkg().Path(), mod+"/") {
		return ""
	}
	pkg, top := obj.Pkg().Path(), obj.Parent() == obj.Pkg().Scope()
	switch o := obj.(type) {
	case *types.Func:
		sig := o.Origin().Type().(*types.Signature)
		if sig.Recv() == nil {
			return "go:func:" + pkg + "." + o.Name()
		}
		rt := sig.Recv().Type()
		if p, ok := rt.(*types.Pointer); ok {
			rt = p.Elem()
		}
		if named, ok := rt.(*types.Named); ok {
			return "go:func:" + pkg + "." + named.Obj().Name() + "." + o.Name()
		}
	case *types.TypeName:
		if top {
			return "go:type:" + pkg + "." + o.Name()
		}
	case *types.Const:
		if top {
			return "go:const:" + pkg + "." + o.Name()
		}
	case *types.Var:
		if top && !o.IsField() {
			return "go:var:" + pkg + "." + o.Name()
		}
	}
	return ""
}

// expectedUses modülün kullanımlarını bildirim bildirim toplar. fields alanın bildirim yerinden ("dosya:satır:ad")
// düğümüne giden tablodur (alan düğümünün kimliği konumdan bağımsız kurulamaz).
func expectedUses(m *module, include func(pkg string) bool, fields map[string]string) (uses []expectedUse, checked int) {
	named := namedStructFields(m)
	for _, u := range m.Units {
		if !include(strings.TrimSuffix(u.PkgPath, "_test")) {
			continue
		}
		for _, f := range u.Files {
			file := m.Rel(m.Fset.Position(f.Pos()).Filename)
			for _, decl := range f.Decls {
				owners := declOwners(m, u, file, decl)
				if owners == nil {
					continue
				}
				calls := map[*ast.Ident]bool{}
				ast.Inspect(decl, func(n ast.Node) bool {
					if c, ok := n.(*ast.CallExpr); ok {
						fn := ast.Unparen(c.Fun)
						if ix, ok := fn.(*ast.IndexExpr); ok { // genel işlev: F[int](x)
							fn = ix.X
						}
						switch fn := fn.(type) {
						case *ast.Ident:
							calls[fn] = true
						case *ast.SelectorExpr:
							calls[fn.Sel] = true
						}
					}
					id, ok := n.(*ast.Ident)
					if !ok || u.Info.Uses[id] == nil {
						return true
					}
					checked++
					obj := u.Info.Uses[id]
					target := typesID(m.Path, obj)
					if v, ok := obj.(*types.Var); ok && v.IsField() && named[v.Origin()] {
						d := m.Fset.Position(obj.Pos())
						where := fmt.Sprintf("%s:%d:%s", m.Rel(d.Filename), d.Line, obj.Name())
						if target = fields[where]; target == "" {
							target = "field " + where
						}
					}
					if target == "" {
						return true
					}
					_, isFunc := obj.(*types.Func)
					uses = append(uses, expectedUse{target, owners, isFunc && calls[id], file, m.Fset.Position(id.Pos()).Line})
					return true
				})
			}
		}
	}
	return uses, checked
}

// namedStructFields paket düzeyindeki adlı yapıların alanlarıdır (grafiğin alan düğümü kurduğu küme); yerel ve adsız
// yapıların alanları düğüm değildir.
func namedStructFields(m *module) map[types.Object]bool {
	out := map[types.Object]bool{}
	for _, u := range m.Units {
		scope := u.Pkg.Scope()
		for _, name := range scope.Names() {
			tn, ok := scope.Lookup(name).(*types.TypeName)
			if !ok {
				continue
			}
			if st, ok := tn.Type().Underlying().(*types.Struct); ok {
				for i := 0; i < st.NumFields(); i++ {
					out[st.Field(i)] = true
				}
			}
		}
	}
	return out
}

// declOwners bir üst düzey bildirimin içindeki kullanımların kaynağı olabilecek düğümlerdir: işlev ya da metot,
// tür, paket düzeyi değerler, dosya (adsız `var _ T`). İçe aktarma bildiriminde nil.
func declOwners(m *module, u *unit, file string, decl ast.Decl) []string {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		id := typesID(m.Path, u.Info.Defs[d.Name])
		if d.Name.Name == "init" || d.Name.Name == "_" {
			id += fmt.Sprintf("#%s:%d", path.Base(file), m.Fset.Position(d.Pos()).Line)
		}
		return []string{id}
	case *ast.GenDecl:
		if d.Tok == token.IMPORT {
			return nil
		}
		owners := []string{"go:file:" + file}
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				owners = append(owners, typesID(m.Path, u.Info.Defs[s.Name]))
			case *ast.ValueSpec:
				for _, n := range s.Names {
					owners = append(owners, typesID(m.Path, u.Info.Defs[n]))
				}
			}
		}
		return owners
	}
	return nil
}

// useCoverageMisses beklenen kullanımları grafikle karşılaştırır (compareUses).
func useCoverageMisses(t *testing.T, root string, include func(pkg string) bool) (misses []string, checked int) {
	t.Helper()
	m, err := load(root)
	if err != nil {
		t.Fatal(err)
	}
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	uses, checked := expectedUses(m, include, fieldsByDecl(g))
	return compareUses(g, uses, include), checked
}

// fieldsByDecl alan düğümlerini bildirim yerine göre verir ("dosya:satır:ad").
func fieldsByDecl(g *Graph) map[string]string {
	fields := map[string]string{}
	for i := range g.Nodes {
		if n := &g.Nodes[i]; n.Kind == kindField {
			fields[fmt.Sprintf("%s:%d:%s", n.File, n.Line, n.Name[strings.LastIndex(n.Name, ".")+1:])] = n.ID
		}
	}
	return fields
}

// ownerOf kenar kaynağının üst düzey bildirimidir: kapanış kendi işlevine indirgenir.
func ownerOf(from string) string {
	if rest, ok := strings.CutPrefix(from, "go:closure:"); ok {
		return "go:" + rest[:strings.Index(rest, "@")]
	}
	return from
}

// compareUses iki yönlü karşılaştırmadır: eksik ya da yanlış türdeki kenarlar ("hedef <- yer: neden") ve go/types'ta
// karşılığı olmayan Go calls/uses kenarları.
func compareUses(g *Graph, uses []expectedUse, include func(pkg string) bool) []string {
	ix := newIndex(g)
	type pair struct{ to, from string }
	kinds := map[pair]map[string]bool{} // (hedef, kaynak bildirim) -> "calls EXTRACTED", "any"
	for _, e := range g.Edges {
		if e.Kind == edgeContains || e.Kind == edgeDenotes || e.Kind == edgeImports {
			continue
		}
		k := pair{e.To, ownerOf(e.From)}
		if kinds[k] == nil {
			kinds[k] = map[string]bool{}
		}
		kinds[k][e.Kind+" "+e.Confidence] = true
		kinds[k]["any"] = true
	}
	seen := map[string]bool{}
	var misses []string
	miss := func(s string) {
		if !seen[s] {
			seen[s] = true
			misses = append(misses, s)
		}
	}
	expected := map[pair]bool{}
	for _, u := range uses {
		for _, o := range u.owners {
			expected[pair{u.target, o}] = true
		}
		n := ix.byID[u.target]
		if n == nil {
			miss(fmt.Sprintf("%s <- %s:%d: no node", u.target, u.file, u.line))
			continue
		}
		if inOwnDeclaration(n, u.file, token.Position{Line: u.line}) {
			continue
		}
		want := "any"
		if u.call {
			want = edgeCalls + " " + extracted
		}
		ok := false
		for _, o := range u.owners {
			ok = ok || kinds[pair{u.target, o}][want]
		}
		if !ok {
			miss(fmt.Sprintf("%s <- %s:%d: no %s edge from %s", u.target, u.file, u.line, want, strings.Join(u.owners, " or ")))
		}
	}
	for _, e := range g.Edges { // ters yön: grafikteki her Go çağrı ve kullanım kenarı gerçek bir kullanımdır
		if e.Kind != edgeCalls && e.Kind != edgeUses || !strings.HasPrefix(e.From, "go:") || !strings.HasPrefix(e.To, "go:") {
			continue
		}
		if f := ix.byID[fileID(e.File)]; f != nil && strings.HasPrefix(f.Detail, otherConfigDetail) {
			continue // bu yapılandırmada derlenmeyen dosyanın tipsiz okunan çağrısı: go/types onu görmez (other_os.go)
		}
		if from := ix.byID[e.From]; from != nil && include(goPkgOfFile(g, from.File)) && !expected[pair{e.To, ownerOf(e.From)}] {
			miss(fmt.Sprintf("%s -%s-> %s at %s:%d: go/types has no such use", e.From, e.Kind, e.To, e.File, e.Line))
		}
	}
	sort.Strings(misses)
	return misses
}

// goPkgOfFile dosyanın paket yoludur (modül yolu + klasör).
func goPkgOfFile(g *Graph, file string) string {
	if dir := path.Dir(file); dir != "." {
		return g.Module + "/" + dir
	}
	return g.Module
}

// inOwnDeclaration kullanım nesnenin kendi bildiriminin içinde mi (özyineli alan tipi, özyinelemeli çağrı):
// kendine başvuru başka birinin kullanımı değildir.
func inOwnDeclaration(n *Node, file string, pos token.Position) bool {
	return n.File == file && n.Line <= pos.Line && pos.Line <= max(n.End, n.Line)
}

var useCoverageFiles = map[string]string{
	"a/a.go": `package a

type Embedded struct{ G int }

type T struct {
	F    int
	Next *T
	Embedded
}

type Result struct{}

type Iface interface {
	M(x T) Result
}

type Alias = T

type Generic[X Constraint] struct{ v X }

type Constraint interface{ ~int | ~string }

const C = 1

var V = 2

func Make(t T) (Result, error) { return Result{}, nil }

func (t *T) Method(r Result) Result { return r }

func use() {
	var g Generic[int]
	_ = g.v
	_ = [C]int{}
	_ = T{F: V}
	f := (*T).Method
	_ = f
}
`,
	"b/b.go": `package b

import "example.com/audit/a"

type Holder struct {
	t   a.T
	res []a.Result
	m   map[string]a.Iface
}

func Take(h Holder) a.Alias { return h.t }

func Field(h *Holder) int { return h.t.F + h.t.G + a.C + a.V }

func Make() (a.Result, error) { return a.Make(a.T{}) }
`,
}

// Kural: küçük bir modülde (tür dönüş tipinde, alan tipinde, tür kısıtında, gömülü alanda, diğer pakette kullanım)
// grafik go/types'ın gördüğü her kullanımı örter.
func TestUseCoverageOnADiverseModule(t *testing.T) {
	root := auditModule(t, useCoverageFiles)
	misses, checked := useCoverageMisses(t, root, func(string) bool { return true })
	if checked < 30 {
		t.Fatalf("only %d uses seen; the module is not what the test expects", checked)
	}
	if len(misses) != 0 {
		t.Errorf("uses go/types sees but the graph lacks (%d):\n  %s", len(misses), strings.Join(misses, "\n  "))
	}
}

// Kural: aynı örtme tutarlılığı bu deponun kendisinde de geçerlidir (gpu.MemorySample örneği buradan çıktı). Modül ve
// grafik planlama sınavıyla paylaşılır (repoGraph); -short'ta, yani tam denetimde de koşar.
func TestUseCoverageOnTheRepository(t *testing.T) {
	_, m, g := repoGraph(t) // sınavla paylaşılan yükleme: -short'ta da koşar (S7)
	all := func(string) bool { return true }
	uses, checked := expectedUses(m, all, fieldsByDecl(g))
	if misses := compareUses(g, uses, all); len(misses) != 0 {
		limit := min(len(misses), 40)
		t.Errorf("%d of %d uses are missing from the graph, first %d:\n  %s", len(misses), checked, limit, strings.Join(misses[:limit], "\n  "))
	}
}

// Kural: kapsama karşılaştırması bozuk bir grafikte düşer (testin kendisinin testi). Dört bozma, her biri bir eski kör
// noktadır: paket düzeyi sabit ve değişken düğümleri hiç yok (S1: eski test düğümü olmayanı atlıyordu), bir işlevin
// bütün kenarları yok ama aynı dosyada başka işlev aynı hedefleri kullanıyor (S2: eski test dosya düzeyindeydi), çağrı
// `uses` diye yazılmış, gerçek olmayan bir kenar eklenmiş (ters yön).
func TestUseCoverageCatchesABrokenGraph(t *testing.T) {
	root := auditModule(t, useCoverageFiles)
	m, err := load(root)
	if err != nil {
		t.Fatal(err)
	}
	all := func(string) bool { return true }
	mutations := []struct {
		name, want string
		mutate     func(g *Graph)
	}{
		{"no const and var nodes", "go:const:example.com/audit/a.C <- b/b.go:13: no node", func(g *Graph) {
			g.Nodes = slices.DeleteFunc(g.Nodes, func(n Node) bool { return n.Kind == kindConst || n.Kind == kindVar })
		}},
		{"no edges from b.Field", "go:field:example.com/audit/b.Holder.t <- b/b.go:13: no any edge from go:func:example.com/audit/b.Field", func(g *Graph) {
			g.Edges = slices.DeleteFunc(g.Edges, func(e Edge) bool { return e.From == "go:func:example.com/audit/b.Field" })
		}},
		{"a call written as a use", "go:func:example.com/audit/a.Make <- b/b.go:15: no calls EXTRACTED edge", func(g *Graph) {
			for i := range g.Edges {
				if e := &g.Edges[i]; e.From == "go:func:example.com/audit/b.Make" && e.To == "go:func:example.com/audit/a.Make" {
					e.Kind = edgeUses
				}
			}
		}},
		{"an edge with no use behind it", "go:func:example.com/audit/b.Take -uses-> go:const:example.com/audit/a.C at b/b.go:10: go/types has no such use", func(g *Graph) {
			g.Edges = append(g.Edges, Edge{From: "go:func:example.com/audit/b.Take", To: "go:const:example.com/audit/a.C", Kind: edgeUses, Confidence: extracted, File: "b/b.go", Line: 10})
		}},
	}
	for _, mu := range mutations {
		g, err := buildGraph(root)
		if err != nil {
			t.Fatal(err)
		}
		uses, _ := expectedUses(m, all, fieldsByDecl(g))
		if misses := compareUses(g, uses, all); len(misses) != 0 {
			t.Fatalf("the unbroken graph already misses:\n  %s", strings.Join(misses, "\n  "))
		}
		mu.mutate(g)
		misses := compareUses(g, uses, all)
		if !slices.ContainsFunc(misses, func(s string) bool { return strings.HasPrefix(s, mu.want) }) {
			t.Errorf("%s: no miss starting with %q:\n  %s", mu.name, mu.want, strings.Join(misses, "\n  "))
		}
	}
}
