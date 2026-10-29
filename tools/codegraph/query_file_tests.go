package main

import (
	"fmt"
	"io"
	"strings"
)

// Bu dosya dosya düzeyindeki tests-for'u kurar: bir Go dosyasını koruyan testler, dosyadaki bildirimlerin testlerinin
// birleşimidir (changed'in toplamasıyla aynı).

// testDistances n'yi sınayan testlerin en kısa uzaklığıdır (0: doğrudan ya da bütün listeyi gezen; >0: zincir ya da ad kodu
// üzerinden adım sayısı). tests-for'un gösterdiği testlerin aynısıdır (ölçümler hariç); `changed` bunu kullanır, böylece
// "bu değişikliği hangi testler koşar" ile "bu işlevi hangi testler sınar" aynı cevabı verir (bağımsız denetim Y3).
func (ix *index) testDistances(n *Node) map[string]int {
	best := map[string]int{}
	note := func(id string, d int) {
		if t := ix.byID[id]; t == nil || !isTestNode(t) || isMeasurement(t) {
			return
		}
		if old, ok := best[id]; !ok || d < old {
			best[id] = d
		}
	}
	for _, e := range ix.in[n.ID] {
		if e.Kind == edgeTests {
			note(e.From, 0)
		}
	}
	for _, e := range ix.walkEdges(n) {
		note(e.From, 0)
	}
	for _, it := range append(ix.valueTests(n), ix.indirectTests(n)...) {
		note(it.test.ID, it.steps)
	}
	for _, t := range ix.testsThroughNames(n) {
		note(t.test.ID, t.steps)
	}
	return best
}

// printGoFileTests bir Go dosyasını koruyan testlerdir: dosyadaki bildirimlerin (işlev, metot, tip, değer) testlerinin
// birleşimi, `changed`'in değişen düğümler için yaptığı toplamanın aynısı (testDistances; alt süreç testleri dahil).
// "Bir dosyaya dokunmadan önce tests-for <dosya>" kuralı Go dosyasında da cevap alır.
func (ix *index) printGoFileTests(w io.Writer, file *Node, limit int) {
	ts := ix.fileDecls(file)
	fmt.Fprintf(w, "  declarations in this file: %d\n", len(ts.nodes))
	if sibling := ix.byID[fileID(strings.TrimSuffix(file.File, ".go")+"_test.go")]; !isTestFile(file.File) && sibling != nil {
		// Adaş test dosyasının (aynı ya da dış `_test` paketteki x_test.go) testleri dosyanın testleridir: ilk grupta.
		for _, t := range ownTests(ix.fileDecls(sibling)) {
			ts.nodes[t.ID] = t
		}
	}
	ix.printTestsToRun(w, ts, limit, "the declarations in this file", "this file's tests")
	ix.printScans(w, file.File, limit)
	tests := map[string]bool{}
	for _, t := range ownTests(ts) {
		tests[t.ID] = true
	}
	for _, d := range ts.nodes {
		for id := range ix.testDistances(d) {
			tests[id] = true
		}
	}
	for _, h := range ix.scansCovering(file.File) {
		for _, t := range h.tests {
			tests[t.ID] = true
		}
	}
	ix.printRulesGuardedBy(w, tests, limit)
}

// fileDecls bir Go dosyasının bildirimleridir (işlev, metot, tip, değer).
func (ix *index) fileDecls(file *Node) *touchSet {
	ts := newTouchSet()
	for _, e := range ix.out[file.ID] {
		if d := ix.byID[e.To]; e.Kind == edgeContains && d != nil && strings.HasPrefix(d.ID, "go:") && d.Kind != kindFile {
			ts.nodes[d.ID] = d
		}
	}
	return ts
}
