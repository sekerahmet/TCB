package main

import (
	"fmt"
	"go/types"
	"slices"
	"strings"
	"testing"
)

var dispatchFiles = map[string]string{
	"tensor/backend.go": `package tensor

// Backend tensör işlemlerinin geçtiği soyutlamadır.
type Backend interface {
	Softmax(x int) int
	Sum(x int) int
}

type cpuBackend struct{}

func (cpuBackend) Softmax(x int) int { return x }
func (cpuBackend) Sum(x int) int     { return x }

var active Backend = cpuBackend{}

// SoftmaxOp arayüz üzerinden çağırır.
func SoftmaxOp(x int) int { return active.Softmax(x) }
`,
	"gpu/backend.go": `package gpu

type backend struct{ inner }

type inner struct{}

func (inner) Sum(x int) int { return x }

func (backend) Softmax(x int) int { return x + 1 }
`,
	"gpu/register.go": `package gpu

import "example.com/audit/tensor"

var _ tensor.Backend = backend{}
`,
	"tensor/backend_test.go": `package tensor

import "testing"

// Softmax sonucu girdiyi korur.
func TestSoftmaxOp(t *testing.T) { _ = SoftmaxOp(1) }
`,
}

// Kural: bir arayüz yöntemi, onu uygulayan her yönteme INFERRED `dispatches_to` kenarı verir (gömülü alandan gelen
// uygulama dahil); `query` uygulamaları listeler, `callees` çağrının arayüzde kesilmediğini gösterir, `path` yönlü yolu
// uygulamaya kadar sürer ve `tests-for` uygulamanın testlerini arayüz çağrısının testlerinden bulur. Kural tek yerde
// (moduleImplementations) durur; dead-methods aynısını kullanır.
func TestInterfaceMethodsDispatchToTheirImplementations(t *testing.T) {
	root := auditModule(t, dispatchFiles)
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	const tens = "go:func:example.com/audit/tensor."
	mustEdge(t, g, tens+"Backend.Softmax", tens+"cpuBackend.Softmax", edgeDispatches, inferred)
	mustEdge(t, g, tens+"Backend.Softmax", "go:func:example.com/audit/gpu.backend.Softmax", edgeDispatches, inferred)
	mustEdge(t, g, tens+"Backend.Sum", "go:func:example.com/audit/gpu.inner.Sum", edgeDispatches, inferred) // gömülü alandan
	var out, errb strings.Builder
	if code := run([]string{"build", "--root", root}, &out, &errb); code != exitOK {
		t.Fatalf("build: %d %s", code, errb.String())
	}
	ask := func(args ...string) string {
		out.Reset()
		if code := run(append(args, "--root", root, "--no-rebuild"), &out, &errb); code != exitOK {
			t.Fatalf("%v: %d %s", args, code, errb.String())
		}
		return out.String()
	}
	if got := ask("query", "tensor.Backend.Softmax"); !strings.Contains(got, "dispatches_to (2)") {
		t.Errorf("query lists both implementations:\n%s", got)
	}
	if got := ask("callees", "tensor.SoftmaxOp"); !strings.Contains(got, "dispatches to: gpu.backend.Softmax, tensor.cpuBackend.Softmax") {
		t.Errorf("callees shows the dispatch:\n%s", got)
	}
	if got := ask("path", "tensor.SoftmaxOp", "tensor.cpuBackend.Softmax"); !strings.Contains(got, "(directed, 2 steps)") {
		t.Errorf("path follows the dispatch:\n%s", got)
	}
	if got := ask("tests-for", "tensor.cpuBackend.Softmax"); !strings.Contains(got, "tensor.TestSoftmaxOp") || !strings.Contains(got, "[INFERRED]") {
		t.Errorf("tests-for reaches the implementation through the interface call:\n%s", got)
	}
}

var promotedFiles = map[string]string{"p/p.go": `package p

type Storage interface {
	Hold()
	Len() int
}

// Counter depolamalara gömülür; Hold'u gömüldüğü tip üzerinden Storage'ı karşılar.
type Counter struct{}

func (*Counter) Hold() {}

type cpu struct{ Counter }

func (*cpu) Len() int { return 0 }

func Use(s Storage) int { s.Hold(); return s.Len() }

var _ = Use(&cpu{})
`}

// dispatchMisses go/types'ın kendi yöntem kümesine (types.NewMethodSet; codegraph'ın implementingMethods'u değil)
// karşı her (arayüz yöntemi, uygulayan yöntem) çiftinin grafikte `dispatches_to` kenarı olduğunu denetler; gömülü
// alandan yükseltilen uygulama dahil. Eksikleri verir.
func dispatchMisses(t *testing.T, m *module, g *Graph) (misses []string, promoted int) {
	t.Helper()
	have := map[[2]string]bool{}
	for _, e := range g.Edges {
		if e.Kind == edgeDispatches {
			have[[2]string{e.From, e.To}] = true
		}
	}
	var ifaces, concrete []*types.Named
	for _, pkg := range m.Base {
		for _, name := range pkg.Scope().Names() {
			tn, ok := pkg.Scope().Lookup(name).(*types.TypeName)
			if !ok || tn.IsAlias() {
				continue
			}
			if n, ok := tn.Type().(*types.Named); ok && n.TypeParams().Len() == 0 {
				if _, isIface := n.Underlying().(*types.Interface); isIface {
					ifaces = append(ifaces, n)
				} else {
					concrete = append(concrete, n)
				}
			}
		}
	}
	for _, c := range concrete {
		ptr := types.NewPointer(c)
		set := types.NewMethodSet(ptr)
		for _, i := range ifaces {
			it := i.Underlying().(*types.Interface)
			if !types.Implements(ptr, it) {
				continue
			}
			for k := 0; k < it.NumMethods(); k++ {
				im := it.Method(k)
				sel := set.Lookup(im.Pkg(), im.Name())
				impl := sel.Obj().(*types.Func)
				if len(sel.Index()) > 1 {
					promoted++
				}
				from := typesID(m.Path, im)
				to := typesID(m.Path, impl)
				if from != to && !have[[2]string{from, to}] {
					misses = append(misses, fmt.Sprintf("%s -> %s (through %s)", from, to, c.Obj().Name()))
				}
			}
		}
	}
	return misses, promoted
}

// Kural: arayüz gönderimi ve dead-methods yöntem kümesinin aynı kuralını kullanır (implementingMethods,
// types.NewMethodSet): gömülü alandan yükseltilen yöntem, gömüldüğü tipin sağladığı arayüzü karşılar. Grafik go/types'ın
// yöntem kümesine karşı tutarlıdır; yükseltilen kenarlar silinince tutarlılık testi düşer (mutasyon); dead-methods
// yükseltilen yöntemi "unreferenced" saymaz (TCB sağlık taraması P1: tensor.VersionCounter.Hold).
func TestPromotedMethodsSatisfyInterfacesInDispatchAndDeadMethods(t *testing.T) {
	root := auditModule(t, promotedFiles)
	m, err := load(root)
	if err != nil {
		t.Fatal(err)
	}
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	misses, promoted := dispatchMisses(t, m, g)
	if len(misses) != 0 || promoted == 0 {
		t.Fatalf("dispatches_to against go/types (promoted %d): %v", promoted, misses)
	}
	g.Edges = slices.DeleteFunc(g.Edges, func(e Edge) bool { return e.Kind == edgeDispatches && strings.Contains(e.To, ".Counter.") })
	if misses, _ := dispatchMisses(t, m, g); len(misses) == 0 {
		t.Error("removing the promoted dispatches_to edge must be caught")
	}
	res := auditJSON(t, "dead-methods", root)
	listed, _ := res["methods"].([]any) // aday yoksa nil
	for _, v := range listed {
		if name := v.(map[string]any)["method"].(string); strings.Contains(name, "Counter.Hold") {
			t.Errorf("Counter.Hold is called through Storage.Hold by embedding; listed as %v", v)
		}
	}
}

// Kural: deponun grafiği de go/types'ın yöntem kümesiyle tutarlıdır (yükseltilen yöntemler dahil). Modülü iki kez
// yükler, -short ile atlanır.
func TestDispatchesMatchMethodSetsOnTheRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("loads the whole module twice")
	}
	root, err := resolveRoot("")
	if err != nil {
		t.Fatal(err)
	}
	m, err := load(root)
	if err != nil {
		t.Fatal(err)
	}
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	if misses, promoted := dispatchMisses(t, m, g); len(misses) != 0 || promoted == 0 {
		t.Errorf("promoted %d; missing dispatches_to (%d): %v", promoted, len(misses), misses)
	}
}
