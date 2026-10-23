package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
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
	path := filepath.Join(t.TempDir(), "g_"+lang+".json")
	if err := os.WriteFile(path, testkit.RunWithLanguageFile(t, "TestHelperBuildFixture", helperOut, lang), 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := readGraph(path)
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
			out = append(out, fmt.Sprintf("node %s %s %s @%s:%d-%d %q", n.ID, n.Kind, n.Name, n.File, n.Line, n.End, n.Detail))
		}
	}
	for _, e := range g.Edges {
		if strings.HasPrefix(e.From, "tr:") {
			out = append(out, fmt.Sprintf("edge %s %s %s %s @%s:%d x%d", e.From, e.Kind, e.To, e.Confidence, e.File, e.Line, e.Count))
		}
	}
	sort.Strings(out)
	return out
}

// Grafik etkin dilden bağımsızdır: TR_DIL=en ve TR_DIL=tr ile kurulan grafiklerde .tr düğümleri ve
// kenarları (uses_name, calls, uses, imports, tests) aynıdır. Kodsuz dosya her zaman Türkçe, kodlu
// dosya Türkçe görünümüyle okunur.
func TestTRGraphIsIndependentOfTheActiveLanguage(t *testing.T) {
	testkit.AssertSameFactSet(t, "the .tr graph", 20, trFacts(buildWithLanguage(t, "tr")), trFacts(buildWithLanguage(t, "en")))
}

// Altın olgu dosyası: fikstürdeki .tr düğümleri ve kenarları (kimlik, tür, ad, güven) tek tek
// kayıtlıdır. TR çıkarımı başka pakete taşınırken değişmemesi gereken sözleşmedir; -update ile
// yeniden yazılır.
func TestTRFactsGolden(t *testing.T) {
	got := strings.Join(trFacts(fixture(t)), "\n") + "\n"
	path := filepath.Join("testdata", "tr_facts.golden")
	if *updateGolden {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ReplaceAll(string(want), "\r\n", "\n") != got {
		t.Errorf(".tr facts changed (go test -run TRFactsGolden -update to rewrite):\n got:\n%s\nwant:\n%s", got, want)
	}
}

var updateGolden = flag.Bool("update", false, "altın dosyaları yeniden yaz")
