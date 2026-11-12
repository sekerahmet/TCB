package main

import (
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
)

// Bu dosya tests-for cevabının paylaşımlı okuma yeri (splitSharedSites) ve ölçüm (isMeasurement) gruplarını kurar.

// splitSharedSites dosyaya gelen test kenarlarını ayırır: testin aynı yerden (dosya:satır) başka dosyalara da giden
// kenarı varsa bir kalıp (lib/*.tr) ya da gömülü dosya kümesi (lib/embedded.go'nun go:embed *.tr'si) bu dosyayı
// ötekilerle birlikte okur (bir yol listesi de: TestDocLinks README.md'yi docs/*.md ile okur); adıyla ya da aktarmayla okuyan testten (TestReadyModelMatchesTransformers) zayıftır ve
// sonra yazılır (DENETİM, parti K2: gömülü kitaplık üzerinden gelen dört test davranış testinin önündeydi).
func (ix *index) splitSharedSites(tests []*Edge) (own, shared []*Edge) {
	for _, e := range tests {
		other := slices.ContainsFunc(ix.out[e.From], func(o *Edge) bool {
			t := ix.byID[o.To]
			return o.Kind == edgeTests && o.To != e.To && o.File == e.File && o.Line == e.Line && t != nil && (t.Kind == kindFile || t.Kind == kindDoc)
		})
		if other {
			shared = append(shared, e)
		} else {
			own = append(own, e)
		}
	}
	return own, shared
}

// printSharedSites dosyayı ötekilerle birlikte okuyan testleri (splitSharedSites) kendi grubunda yazar.
func (ix *index) printSharedSites(w io.Writer, list []*Edge, limit int) {
	if len(list) == 0 {
		return
	}
	fmt.Fprintf(w, "  <- tests that read it among other files (%d; a list, a glob or an embedded file set: they read every file of it):\n", len(list))
	sort.SliceStable(list, func(i, j int) bool { return ix.byID[list[i].From].Name < ix.byID[list[j].From].Name })
	forEachLimited(w, list, limit, "    ", moreHint, func(e *Edge) string { return ix.edgeLine(e, e.From, true) })
}

// isTRMeasurement TR ölçüm programının bildirimi mi (tools/bench/tr/fib.tr): zincirde test gibi durak, ölçüm grubunda.
func isTRMeasurement(n *Node) bool {
	return n != nil && strings.HasPrefix(n.ID, "tr:") && isMeasurement(n)
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
