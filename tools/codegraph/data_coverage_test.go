package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
)

// Kural: depodaki her izlenen veri dosyası (.json, .tr, .md; testdata hariç) ya bir teste bağlıdır (grafikte kendisine
// gelen `tests`/`walks`/`cites` kenarı ya da, bir belgeyse, içindeki bir kuralın `Denetim:` testi) ya da gerekçeli bir
// istisna listesindedir (testdata/uncovered_data.txt). Yeni bir dosya sessizce kör kalmaz: `tests-for <dosya>`
// boş dönecek dosya ya testle bağlanır ya da gerekçesiyle o dosyaya yazılır. Liste yalnızca küçülür: bir istisna
// artık işe yaramıyorsa (dosya bağlandı ya da hiç yok) test düşer.

// maxUncoveredData istisna listesinin bugünkü satır sayısıdır (onlyShrinks).
const maxUncoveredData = 10

// onlyShrinks bir istisna listesinin yalnızca küçülmesini sağlar (bağımsız denetim S4: kural yalnızca yorumdaydı). Satır
// sayısı testte sabittir: artarsa test düşer, yeni boşluk listeye değil teste gider; bilerek eklemek sabiti de
// değiştirmeyi ister ve değişiklik incelemede görünür. Azalırsa sabit de düşürülür, kazanılan yer geri verilmez.
func onlyShrinks(t *testing.T, list string, n, limit int) {
	t.Helper()
	switch {
	case n > limit:
		t.Errorf("%s has %d entries, more than %d: the list only shrinks; link a test instead of adding an exception", list, n, limit)
	case n < limit:
		t.Errorf("%s has %d entries now: lower its limit from %d to %d so the list cannot grow back", list, n, limit, n)
	}
}

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
	root, _, g := repoGraph(t) // sınavla paylaşılan yükleme
	tracked, ok := trackedSet(root)
	if !ok {
		t.Skip("needs a git checkout")
	}
	ix := newIndex(g)
	hit := map[string]bool{}
	guardedDocs := map[string]bool{} // kuralı bir teste bağlı belgeler
	for _, e := range g.Edges {
		switch e.Kind {
		case edgeTests, edgeWalks, edgeCites: // anan test de tests-for'da görünür (cites grubu)
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
	onlyShrinks(t, "testdata/uncovered_data.txt", len(exceptions), maxUncoveredData)
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

// Kural: depoda izlenen her dosyanın yolu (betik, .gitignore, JS, YAML, testdata fikstürü dahil) tests-for'da o dosyanın
// düğümüne çözülür; ad aramasına düşüp belirsiz ya da başka bir düğüm olmaz. Kanca her ajanın her ilk düzenlemesinde
// tests-for <dosya> sorar; belirsiz cevap yanlış bir "yok" demektir (TCB 2026-10-25, 1. madde: tam denetim
// betiği ve `.gitignore` için tests-for belirsizdi). İstisna listesi yoktur.
func TestEveryTrackedFileIsANode(t *testing.T) {
	root, _, g := repoGraph(t)
	files, ok := codeindex.GitTracked(root)
	if !ok {
		t.Skip("needs a git checkout")
	}
	ix := newIndex(g)
	ix.root = root
	var wrong []string
	for _, f := range files {
		var out, errb strings.Builder
		n, code := ix.resolve(f, &queryOpts{stdout: &out, stderr: &errb})
		if code != exitOK || n.File != f {
			wrong = append(wrong, fmt.Sprintf("%s: exit %d %s%s", f, code, firstLineOf(out.String()), firstLineOf(errb.String())))
		}
	}
	if len(wrong) > 0 {
		t.Errorf("%d tracked file path(s) do not resolve to their file:\n  %s", len(wrong), strings.Join(wrong, "\n  "))
	}
}

func firstLineOf(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
