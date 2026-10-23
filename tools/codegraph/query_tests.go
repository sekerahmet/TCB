package main

import (
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

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

// indirectTests hedefi çağıranlara doğru (walkCallers: tek zincir kuralı, test_reach.go) izler ve yolda bulduğu testleri
// en yakından uzağa verir; bir testin ötesine geçilmez. Doğrudan `tests` kenarı olanlar listeye girmez.
func (ix *index) indirectTests(target *Node) []indirectTest {
	direct := map[string]bool{}
	for _, e := range ix.in[target.ID] {
		if e.Kind == edgeTests {
			direct[e.From] = true
		}
	}
	var out []indirectTest
	walkCallers(target.ID, ix.callersOf, ix.pkgOf, func(r reached) bool {
		n := ix.byID[r.id]
		if !isTestNode(n) {
			return true
		}
		if !direct[n.ID] {
			out = append(out, indirectTest{n, r.steps, ix.byID[r.via]})
		}
		return false
	})
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].steps != out[j].steps {
			return out[i].steps < out[j].steps
		}
		return out[i].test.Name < out[j].test.Name
	})
	return out
}

// callersOf bir düğümü çağıran ya da kullananlardır (reachEdge kenarları), kimliğe göre sıralı.
func (ix *index) callersOf(id string) []string {
	var out []string
	for _, e := range ix.in[id] {
		if reachEdge(e.Kind, e.Confidence) {
			out = append(out, e.From)
		}
	}
	sort.Strings(out)
	return out
}

// pkgOf düğümün paket klasörüdür.
func (ix *index) pkgOf(id string) string {
	if n := ix.byID[id]; n != nil {
		return path.Dir(n.File)
	}
	return ""
}

// printTestsFor n'yi sınayan testleri yazar: önce doğrudan (EXTRACTED: çağrıdan ya da paketten),
// sonra adından çıkarılanlar, sonra çağrı zinciriyle dolaylı olanlar (INFERRED, kaç adım
// uzakta, en yakın önce); koruduğu kurallar en sonda.
func (ix *index) printTestsFor(w io.Writer, n *Node, limit int) {
	fmt.Fprintln(w, describe(n))
	var tests, measured []*Edge
	for _, e := range ix.in[n.ID] {
		switch {
		case e.Kind != edgeTests:
		case isMeasurement(ix.byID[e.From]):
			measured = append(measured, e)
		default:
			tests = append(tests, e)
		}
	}
	walks := ix.walkEdges(n)
	indirect, indirectMeasured := splitMeasurements(ix.indirectTests(n), func(t indirectTest) *Node { return t.test })
	viaNames, _ := splitMeasurements(ix.testsThroughNames(n), func(t nameTest) *Node { return t.test })
	if len(tests) == 0 && len(indirect) == 0 && len(walks) == 0 && len(viaNames) == 0 && len(measured) == 0 && len(indirectMeasured) == 0 {
		fmt.Fprintln(w, "  no tests found")
		return
	}
	if len(tests) == 0 && len(indirect) == 0 && len(walks) == 0 && len(viaNames) == 0 {
		fmt.Fprintln(w, "  no tests found (only measurements, below)")
	}
	ix.printGroups(w, "  <-", tests, fromOf, limit, true) // doğrudan testler önce: bu adı ya da dosyayı kullananlar
	ix.printWalks(w, walks, limit)
	ix.printIndirect(w, indirect, limit)
	ix.printViaNames(w, viaNames, limit)
	ix.printMeasurements(w, measured, indirectMeasured, limit)
	ix.printGuardedRules(w, tests, indirect, limit)
}

// isMeasurement düğüm bir test değil ölçüm mü: Benchmark* işlevi ya da tools/bench altındaki kod. Ölçüm sonuç
// doğrulamaz, süre ölçer; "önce ölç" kuralı için testlerden ayrı gösterilir.
func isMeasurement(n *Node) bool {
	if n == nil {
		return false
	}
	if strings.HasPrefix(n.File, "tools/bench/") {
		return true
	}
	return n.Kind == kindFunc && strings.HasPrefix(n.Name[strings.LastIndex(n.Name, ".")+1:], "Benchmark")
}

// splitMeasurements listeyi testler ve ölçümler olarak ikiye böler.
func splitMeasurements[T any](list []T, node func(T) *Node) (tests, measured []T) {
	for _, it := range list {
		if isMeasurement(node(it)) {
			measured = append(measured, it)
		} else {
			tests = append(tests, it)
		}
	}
	return tests, measured
}

// printMeasurements ölçümleri (benchmark'lar, tools/bench) testlerden ayrı başlık altında yazar.
func (ix *index) printMeasurements(w io.Writer, direct []*Edge, indirect []indirectTest, limit int) {
	total := len(direct) + len(indirect)
	if total == 0 {
		return
	}
	fmt.Fprintf(w, "  <- measurements, not tests (%d; benchmarks and tools/bench: they time it, they do not check it):\n", total)
	var lines []string
	for _, e := range direct {
		lines = append(lines, ix.edgeLine(e, e.From, true))
	}
	for _, it := range indirect {
		lines = append(lines, fmt.Sprintf("%s  %d steps via %s  [INFERRED]", describe(it.test), it.steps, it.via.Name))
	}
	printLimited(w, lines, limit, "    ")
}

func (ix *index) printIndirect(w io.Writer, list []indirectTest, limit int) {
	if len(list) == 0 {
		return
	}
	fmt.Fprintf(w, "  <- tests, indirect (%d, through calls, up to %d steps and %d package boundary):\n", len(list), testMaxSteps, testMaxHops)
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
