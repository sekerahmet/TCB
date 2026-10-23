package main

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
)

// Kural: depodaki her izlenen veri dosyası (.json, .tr, .md; testdata hariç) ya bir teste bağlıdır (grafikte kendisine
// gelen `tests`/`walks` kenarı ya da, bir belgeyse, içindeki bir kuralın `Denetim:` testi) ya da gerekçeli bir
// istisna listesindedir (testdata/uncovered_data.txt). Yeni bir dosya sessizce kör kalmaz: `tests-for <dosya>`
// boş dönecek dosya ya testle bağlanır ya da gerekçesiyle o dosyaya yazılır. Liste yalnızca küçülür: bir istisna
// artık işe yaramıyorsa (dosya bağlandı ya da hiç yok) test düşer.

// uncoveredData istisna kalıplarını ve gerekçelerini okur. Liste Go dizesi olarak değil dosya olarak durur: test
// kodunda yol gibi görünen her dize kendi dosyasını "sınanmış" gösterirdi, istisnanın kendisi kapsama sayılırdı.
func uncoveredData(t *testing.T) []struct{ pattern, reason string } {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "uncovered_data.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var out []struct{ pattern, reason string }
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pat, reason, ok := strings.Cut(line, "\t")
		if !ok || strings.TrimSpace(reason) == "" {
			t.Fatalf("testdata/uncovered_data.txt: %q: want a pattern, a tab and a reason", line)
		}
		out = append(out, struct{ pattern, reason string }{pat, reason})
	}
	return out
}

// trackedSet izlenen dosyaların kümesidir (codeindex.GitTracked'ın listesi); git yoksa ok=false.
func trackedSet(root string) (map[string]bool, bool) {
	files, ok := codeindex.GitTracked(root)
	set := map[string]bool{}
	for _, f := range files {
		set[f] = true
	}
	return set, ok
}

func TestEveryTrackedDataFileHasATestOrAReason(t *testing.T) {
	root, err := resolveRoot("")
	if err != nil {
		t.Fatal(err)
	}
	tracked, ok := trackedSet(root)
	if !ok {
		t.Skip("needs a git checkout")
	}
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	ix := newIndex(g)
	hit := map[string]bool{}
	guardedDocs := map[string]bool{} // kuralı bir teste bağlı belgeler
	for _, e := range g.Edges {
		switch e.Kind {
		case edgeTests, edgeWalks:
			hit[e.To] = true
		case edgeGuardedBy:
			guardedDocs[ix.byID[e.From].File] = true
		}
	}
	importedByTests := trImportedByTestFiles(g)
	var uncovered []string
	for _, n := range g.Nodes {
		ext := path.Ext(n.File)
		if (n.Kind != kindFile && n.Kind != kindDoc) || !tracked[n.File] || (ext != ".json" && ext != ".tr" && ext != ".md") ||
			underAnyName(n.File, "testdata") || hit[n.ID] || guardedDocs[n.File] || importedByTests[n.ID] {
			continue
		}
		uncovered = append(uncovered, n.File)
	}
	sort.Strings(uncovered)
	exceptions := uncoveredData(t)
	used := map[int]bool{}
	var blind []string
	for _, f := range uncovered {
		matched := false
		for i, ex := range exceptions {
			if matchGlob(ex.pattern, f) {
				used[i] = true
				matched = true
				break
			}
		}
		if !matched {
			blind = append(blind, f)
		}
	}
	if len(blind) > 0 {
		t.Errorf("%d tracked data file(s) no test is linked to (tests-for would be empty): link a test (a path constant a test or its helper reads; a go:embed read by a loader a test reaches) or add the file to testdata/uncovered_data.txt with a reason:\n  %s",
			len(blind), strings.Join(blind, "\n  "))
	}
	for i, ex := range exceptions {
		if !used[i] {
			t.Errorf("testdata/uncovered_data.txt entry %q matches no uncovered file any more: remove it", ex.pattern)
		}
	}
}

// trImportedByTestFiles test bloğu taşıyan bir .tr dosyasının (doğrudan ya da dolaylı) `aktar`dığı .tr dosyalarının
// düğümleridir: testin yardımcı modülü, o testin bir parçası olarak çalışır.
func trImportedByTestFiles(g *Graph) map[string]bool {
	imports := map[string][]string{}
	for _, e := range g.Edges {
		if e.Kind == edgeImports && strings.HasPrefix(e.From, "tr:file:") {
			imports[e.From] = append(imports[e.From], e.To)
		}
	}
	reached := map[string]bool{}
	var queue []string
	for _, n := range g.Nodes {
		if n.Kind == kindTRTest {
			queue = append(queue, trFileID(n.File))
		}
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, to := range imports[id] {
			if !reached[to] {
				reached[to] = true
				queue = append(queue, to)
			}
		}
	}
	return reached
}

// underAnyName yolun klasör parçalarından biri name mi.
func underAnyName(file, name string) bool {
	for _, part := range strings.Split(file, "/") {
		if part == name {
			return true
		}
	}
	return false
}
