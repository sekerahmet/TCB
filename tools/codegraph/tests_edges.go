package main

import (
	"sort"
	"strings"
)

// addTestEdges `tests` kenarlarını kurar:
//   - test → kendi paketi (EXTRACTED: test dosyası o paketin klasöründedir);
//   - test → doğrudan çağırdığı test dışı işlev (EXTRACTED: çağrıdan);
//   - test → adından anlaşılan işlev ya da tip (INFERRED: TestParse → Parse, TestVM_Run → VM.Run).
func addTestEdges(b *builder, idx *goIndex) {
	byID := map[string]*testFunc{}
	for _, t := range idx.tests {
		byID[t.id] = t
		b.addEdge(t.id, pkgID(t.pkg), edgeTests, extracted, t.file, 0)
		for _, id := range nameTargets(t.name, idx.pkgSyms[t.pkg]) {
			b.addEdge(t.id, id, edgeTests, inferred, t.file, 0)
		}
	}
	// Çağrılar test dosyasındaki yardımcılardan geçerek de sayılır (TestX → run() → Parse).
	helpers := map[string][]*Edge{} // test dosyasındaki işlev -> çıkan EXTRACTED çağrıları
	for k, e := range b.edges {
		if k.kind == edgeCalls && e.Confidence == extracted && idx.inTestFile[k.from] {
			helpers[k.from] = append(helpers[k.from], e)
		}
	}
	for _, t := range idx.tests {
		for _, c := range reachableCalls(t.id, helpers, idx) {
			b.addEdge(t.id, c.To, edgeTests, extracted, c.File, c.Line)
		}
	}
}

// reachableCalls from işlevinden, yalnızca test dosyalarındaki işlevler üzerinden gidilerek
// varılan test dışı çağrıları sıralı verir.
func reachableCalls(from string, helpers map[string][]*Edge, idx *goIndex) []*Edge {
	seen := map[string]bool{from: true}
	queue := []string{from}
	var out []*Edge
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		list := append([]*Edge(nil), helpers[cur]...)
		sort.Slice(list, func(i, j int) bool { return list[i].To < list[j].To })
		for _, e := range list {
			switch {
			case seen[e.To]:
			case idx.inTestFile[e.To]:
				seen[e.To] = true
				queue = append(queue, e.To)
			default:
				seen[e.To] = true
				out = append(out, e)
			}
		}
	}
	return out
}

// nameTargets test adından sınanan simgeleri bulur. syms paketin "Ad" / "Tür.Metot" tablosudur.
func nameTargets(name string, syms map[string]string) []string {
	rest := ""
	for _, p := range []string{"Test", "Benchmark", "Fuzz", "Example"} {
		if strings.HasPrefix(name, p) {
			rest = strings.TrimPrefix(name[len(p):], "_")
			break
		}
	}
	if rest == "" {
		return nil
	}
	cands := []string{rest, strings.ReplaceAll(rest, "_", ".")}
	if i := strings.Index(rest, "_"); i > 0 {
		cands = append(cands, rest[:i])
	}
	for i := 1; i < len(rest); i++ {
		if rest[i] >= 'A' && rest[i] <= 'Z' {
			cands = append(cands, rest[:i]+"."+rest[i:])
		}
	}
	seen := map[string]bool{}
	var out []string
	for _, c := range cands {
		if id, ok := syms[c]; ok && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}
