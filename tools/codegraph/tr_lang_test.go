package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// buildEnv alt süreçte fikstür grafiğini kurmak için kullanılan ortam değişkenidir.
const helperOut = "CODEGRAPH_TEST_GRAPH_OUT"

// TestHelperBuildFixture bir test değil, alt süreçtir: HELPER çıktı yolunu verirse fikstürün
// grafiğini oraya yazar.
func TestHelperBuildFixture(t *testing.T) {
	out := os.Getenv(helperOut)
	if out == "" {
		t.Skip("alt süreç yardımcısı")
	}
	abs, err := filepath.Abs(filepath.Join("testdata", "mod"))
	if err != nil {
		t.Fatal(err)
	}
	g, err := buildGraph(abs)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeGraph(out, g); err != nil {
		t.Fatal(err)
	}
}

// buildWithLanguage fikstür grafiğini TR_DIL=lang ortamında bir alt süreçte kurar.
func buildWithLanguage(t *testing.T, lang string) *Graph {
	t.Helper()
	out := filepath.Join(t.TempDir(), "g_"+lang+".json")
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperBuildFixture$")
	cmd.Env = append(os.Environ(), "TR_DIL="+lang, helperOut+"="+out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("TR_DIL=%s: %v\n%s", lang, err, b)
	}
	g, err := readGraph(out)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// trFacts grafikteki .tr düğümlerini ve .tr'den çıkan kenarları sıralı metin olarak verir.
func trFacts(g *Graph) []string {
	var out []string
	for _, n := range g.Nodes {
		if strings.HasPrefix(n.ID, "tr:") {
			out = append(out, "node "+n.ID+" "+n.Kind+" "+n.Name)
		}
	}
	for _, e := range g.Edges {
		if strings.HasPrefix(e.From, "tr:") {
			out = append(out, "edge "+e.From+" "+e.Kind+" "+e.To+" "+e.Confidence)
		}
	}
	sort.Strings(out)
	return out
}

// Grafik etkin dilden bağımsızdır: TR_DIL=en ve TR_DIL=tr ile kurulan grafiklerde .tr düğümleri ve
// kenarları (uses_name, calls, uses, imports, tests) aynıdır. Kodsuz dosya her zaman Türkçe, kodlu
// dosya Türkçe görünümüyle okunur.
func TestTRGraphIsIndependentOfTheActiveLanguage(t *testing.T) {
	tr, en := trFacts(buildWithLanguage(t, "tr")), trFacts(buildWithLanguage(t, "en"))
	if len(tr) < 20 {
		t.Fatalf("too few .tr facts to compare: %d", len(tr))
	}
	if strings.Join(tr, "\n") != strings.Join(en, "\n") {
		have := map[string]bool{}
		for _, f := range en {
			have[f] = true
		}
		var missing []string
		for _, f := range tr {
			if !have[f] {
				missing = append(missing, f)
			}
		}
		t.Errorf("TR_DIL=en changes the .tr graph (%d facts vs %d); only in tr:\n%s", len(tr), len(en), strings.Join(missing, "\n"))
	}
}
