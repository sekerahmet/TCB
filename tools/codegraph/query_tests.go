package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// maxTestDepth dolaylı testlerin izlendiği en çok çağrı adımıdır (hedef ← ... ← test).
const maxTestDepth = 4

// indirectTest hedefi çağrı zinciriyle dolaylı sınayan bir testtir.
type indirectTest struct {
	test  *Node
	steps int   // test ile hedef arasındaki çağrı adımı sayısı
	via   *Node // zincirde hedefe en yakın çağıran
}

// isTestNode test dosyasındaki bir TestX/BenchmarkX/FuzzX/ExampleX işlevi mi.
func isTestNode(n *Node) bool {
	if n.Kind != kindFunc || !strings.HasSuffix(n.File, "_test.go") {
		return false
	}
	name := n.Name[strings.LastIndex(n.Name, ".")+1:]
	return isTestName(name)
}

// indirectTests hedefi çağıranları geriye doğru, en çok maxTestDepth adım izler ve yolda
// bulduğu testleri en yakından uzağa verir. Zincir yalnızca EXTRACTED `calls` kenarlarından
// oluşur; bir testin ötesine geçilmez. Doğrudan `tests` kenarı olanlar listeye girmez.
func (ix *index) indirectTests(target *Node) []indirectTest {
	direct := map[string]bool{}
	for _, e := range ix.in[target.ID] {
		if e.Kind == edgeTests {
			direct[e.From] = true
		}
	}
	type item struct {
		id    string
		steps int
		via   string
	}
	seen := map[string]bool{target.ID: true}
	queue := []item{{target.ID, 0, ""}}
	var out []indirectTest
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.steps >= maxTestDepth {
			continue
		}
		for _, e := range ix.in[cur.id] {
			if e.Kind != edgeCalls || e.Confidence != extracted || seen[e.From] {
				continue
			}
			seen[e.From] = true
			via := cur.via
			if cur.steps == 0 {
				via = e.From
			}
			n := ix.byID[e.From]
			if isTestNode(n) {
				if !direct[n.ID] {
					out = append(out, indirectTest{n, cur.steps + 1, ix.byID[via]})
				}
				continue
			}
			queue = append(queue, item{e.From, cur.steps + 1, via})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].steps != out[j].steps {
			return out[i].steps < out[j].steps
		}
		return out[i].test.Name < out[j].test.Name
	})
	return out
}

// printTestsFor n'yi sınayan testleri yazar: önce doğrudan (EXTRACTED: çağrıdan ya da paketten),
// sonra adından çıkarılanlar, sonra çağrı zinciriyle dolaylı olanlar (INFERRED, kaç adım
// uzakta, en yakın önce); koruduğu kurallar en sonda.
func (ix *index) printTestsFor(w io.Writer, n *Node, limit int) {
	fmt.Fprintln(w, describe(n))
	var tests []*Edge
	for _, e := range ix.in[n.ID] {
		if e.Kind == edgeTests {
			tests = append(tests, e)
		}
	}
	walks := ix.walkEdges(n)
	indirect := ix.indirectTests(n)
	viaNames := ix.testsThroughNames(n)
	if len(tests) == 0 && len(indirect) == 0 && len(walks) == 0 && len(viaNames) == 0 {
		fmt.Fprintln(w, "  no tests found")
		return
	}
	ix.printGroups(w, "  <-", tests, fromOf, limit, true) // doğrudan testler önce: bu adı ya da dosyayı kullananlar
	ix.printWalks(w, walks, limit)
	ix.printIndirect(w, indirect, limit)
	ix.printViaNames(w, viaNames, limit)
	ix.printGuardedRules(w, tests, indirect, limit)
}

func (ix *index) printIndirect(w io.Writer, list []indirectTest, limit int) {
	if len(list) == 0 {
		return
	}
	fmt.Fprintf(w, "  <- tests, indirect (%d, through calls, up to %d steps):\n", len(list), maxTestDepth)
	forEachLimited(w, list, limit, "    ", moreHint, func(it indirectTest) string {
		return fmt.Sprintf("%s  %d steps via %s  [INFERRED]", describe(it.test), it.steps, it.via.Name)
	})
}

// printGuardedRules testlerin koruduğu kuralları (rule düğümleri) yazar.
func (ix *index) printGuardedRules(w io.Writer, tests []*Edge, indirect []indirectTest, limit int) {
	froms := map[string]bool{}
	for _, e := range tests {
		froms[e.From] = true
	}
	for _, it := range indirect {
		froms[it.test.ID] = true
	}
	rules := map[string]bool{}
	for id := range froms {
		for _, g := range ix.in[id] {
			if g.Kind == edgeGuardedBy {
				rules[g.From] = true
			}
		}
	}
	ids := make([]string, 0, len(rules))
	for id := range rules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return
	}
	fmt.Fprintf(w, "  rules guarded by these tests (%d):\n", len(ids))
	forEachLimited(w, ids, limit, "    ", moreHint, func(id string) string { return describe(ix.byID[id]) })
}
