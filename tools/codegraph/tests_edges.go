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
	for _, t := range idx.tests {
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
	testOnly := testOnlySetters(b, idx)
	for _, t := range idx.tests {
		calls, visited := reachableCalls(t.id, helpers, idx)
		for _, c := range calls {
			b.addEdge(t.id, c.To, edgeTests, extracted, c.File, c.Line)
			if testOnly[c.To] {
				visited = append(visited, c.To)
			}
		}
		// Ortam değişkenini koyan test (ya da yardımcısı) onun ad kodunu sınar (env_names.go).
		for _, f := range visited {
			for _, s := range idx.envSets[f] {
				b.addEdge(t.id, "name:"+s.code, edgeTests, extracted, s.file, s.line)
			}
		}
	}
}

// testOnlySetters test dosyasında olmayan ama yalnızca test dosyalarından çağrılan (bütün kesin çağıranları test
// dosyasında) ve bir ortam değişkeni koyan işlevlerdir: ortak test yardımcısı `testkit.RunWithLanguageFile` alt süreci
// TR_LANG ile başlatır, onu çağıran test TR_LANG'i koymuş olur. Üretim kodunun koyduğu değişken teste kalkmaz.
func testOnlySetters(b *builder, idx *goIndex) map[string]bool {
	out := map[string]bool{}
	for k, e := range b.edges {
		if k.kind != edgeCalls || e.Confidence != extracted || len(idx.envSets[k.to]) == 0 || idx.inTestFile[k.to] {
			continue
		}
		if _, seen := out[k.to]; !seen {
			out[k.to] = true
		}
		out[k.to] = out[k.to] && idx.inTestFile[k.from]
	}
	return out
}

// reachableCalls from işlevinden, yalnızca test dosyalarındaki işlevler üzerinden gidilerek
// varılan test dışı çağrıları sıralı verir; visited geçilen test dosyası işlevleridir (from dahil).
func reachableCalls(from string, helpers map[string][]*Edge, idx *goIndex) (out []*Edge, visited []string) {
	seen := map[string]bool{from: true}
	queue := []string{from}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		visited = append(visited, cur)
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
	return out, visited
}

// nameTargets test adından sınanan simgeleri bulur. syms paketin "Ad" / "Tür.Metot" tablosudur.
func nameTargets(name string, syms map[string]string) []string {
	rest, ok := testNameRest(name)
	rest = strings.TrimPrefix(rest, "_")
	if !ok || rest == "" {
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
