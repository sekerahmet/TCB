package main

import (
	"fmt"
	"go/token"
	"go/types"
	"sort"
	"strings"
	"testing"
)

// Kural: who-uses kör nokta bırakmaz. go/types'ın Info.Uses kaydındaki her kullanım (tür, işlev, metot, sabit,
// değişken, alan; hangi tanımlayıcıda olursa olsun: imza, alan tipi, bileşik değer, tür kısıtı) grafikte o
// nesnenin düğümüne gelen bir kenarın dosyasıyla örtülür. Kullanımların çıkarımı nesne türü başına ayrı bir kolla
// değil objectID üzerinden tek geçişte yapılır; bu test yeni bir kör noktayı (ör. dönüş tipinde geçen tür:
// gpu.MemorySample) sessiz bırakmaz.

// useCoverageMisses her kullanımın (nesne düğümü, dosya) çiftini grafikle karşılaştırır; grafikte karşılığı olmayanları
// "düğüm ← dosya:satır" olarak verir. include paket yoluna göre taranacak kullanım yerlerini seçer.
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
	ix := newIndex(g)
	x := &goExtractor{m: m}
	covered := map[string]map[string]bool{} // düğüm kimliği -> kullanım dosyaları
	for _, e := range g.Edges {
		if e.Kind == edgeContains || e.Kind == edgeDenotes || e.Kind == edgeImports {
			continue
		}
		if covered[e.To] == nil {
			covered[e.To] = map[string]bool{}
		}
		covered[e.To][e.File] = true
		for _, s := range e.Sites {
			covered[e.To][s.File] = true
		}
	}
	fields := map[string]string{} // "dosya:satır:ad" -> alan düğümü
	for i := range g.Nodes {
		if n := &g.Nodes[i]; n.Kind == kindField {
			fields[fmt.Sprintf("%s:%d:%s", n.File, n.Line, n.Name[strings.LastIndex(n.Name, ".")+1:])] = n.ID
		}
	}
	seen := map[string]bool{}
	for _, u := range m.Units {
		if !include(strings.TrimSuffix(u.PkgPath, "_test")) {
			continue
		}
		for id, obj := range u.Info.Uses {
			node := x.objectID(obj)
			if v, ok := obj.(*types.Var); ok && v.IsField() && obj.Pkg() != nil && x.inModule(obj.Pkg().Path()) {
				decl := m.Fset.Position(obj.Pos())
				node = fields[fmt.Sprintf("%s:%d:%s", m.Rel(decl.Filename), decl.Line, obj.Name())]
			}
			if node == "" || ix.byID[node] == nil {
				continue
			}
			pos := m.Fset.Position(id.Pos())
			file := m.Rel(pos.Filename)
			if inOwnDeclaration(ix.byID[node], file, pos) || covered[node][file] {
				continue
			}
			key := fmt.Sprintf("%s <- %s:%d", node, file, pos.Line)
			if !seen[key] {
				seen[key] = true
				misses = append(misses, key)
			}
		}
		checked += len(u.Info.Uses)
	}
	sort.Strings(misses)
	return misses, checked
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

// Kural: aynı örtme tutarlılığı bu deponun kendisinde de geçerlidir (gpu.MemorySample örneği buradan çıktı). Yavaştır
// (modülü iki kez yükler), -short ile atlanır; CI'da çalışır.
func TestUseCoverageOnTheRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("loads the whole module twice")
	}
	root, err := resolveRoot("")
	if err != nil {
		t.Fatal(err)
	}
	misses, checked := useCoverageMisses(t, root, func(string) bool { return true })
	if len(misses) != 0 {
		limit := min(len(misses), 40)
		t.Errorf("%d of %d uses are missing from the graph, first %d:\n  %s", len(misses), checked, limit, strings.Join(misses[:limit], "\n  "))
	}
}
