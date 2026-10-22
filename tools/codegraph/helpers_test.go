package main

import (
	"bytes"
	"path/filepath"
	"sync"
	"testing"
)

const fixturePkg = "go:func:example.com/mini/shape."

// Fixture grafiği bir kez kurulur (go list birkaç saniye sürer).
var (
	fixtureOnce sync.Once
	fixtureG    *Graph
	fixtureErr  error
)

func fixture(t *testing.T) *Graph {
	t.Helper()
	fixtureOnce.Do(func() {
		abs, err := filepath.Abs(filepath.Join("testdata", "mod"))
		if err != nil {
			fixtureErr = err
			return
		}
		fixtureG, fixtureErr = buildGraph(abs)
	})
	if fixtureErr != nil {
		t.Fatal(fixtureErr)
	}
	return fixtureG
}

// edge fixture grafiğinde bir kenar arar.
func edge(g *Graph, from, to, kind string) *Edge {
	for i := range g.Edges {
		e := &g.Edges[i]
		if e.From == from && e.To == to && e.Kind == kind {
			return e
		}
	}
	return nil
}

func hasNode(g *Graph, id string) bool {
	for _, n := range g.Nodes {
		if n.ID == id {
			return true
		}
	}
	return false
}

func mustEdge(t *testing.T, g *Graph, from, to, kind, conf string) {
	t.Helper()
	e := edge(g, from, to, kind)
	if e == nil {
		t.Fatalf("missing %s edge %s -> %s", kind, from, to)
	}
	if e.Confidence != conf {
		t.Errorf("%s edge %s -> %s is %s, want %s", kind, from, to, e.Confidence, conf)
	}
}

// cli komutu fixture grafiğiyle çalıştırır ve çıkış kodunu, stdout ve stderr'i verir.
func cli(t *testing.T, args ...string) (code int, out, errOut string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "graph.json")
	if err := writeGraph(path, fixture(t)); err != nil {
		t.Fatal(err)
	}
	var so, se bytes.Buffer
	args = append([]string{args[0], "--graph", path, "--no-rebuild"}, args[1:]...)
	code = run(args, &so, &se)
	return code, so.String(), se.String()
}

// slowFixture geçici git deposunda grafik kuran (her biri birkaç saniye süren) testleri
// `-short`'ta atlar. Tam denetim -short koşar; bu testler `--kip=uzun`da (CI) koşar.
func slowFixture(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("kısa kipte atlandı: geçici git deposunda grafik kuruyor (uzun kipte koşar)")
	}
}

// runCLI codegraph'ı verilen bağımsız değişkenlerle çalıştırır ve çıkış kodunu, standart çıktıyı ve standart
// hatayı verir.
func runCLI(args ...string) (code int, out, errOut string) {
	var so, se bytes.Buffer
	code = run(args, &so, &se)
	return code, so.String(), se.String()
}

// commandIn bir komutu fikstür deposunda ve grafiğiyle çalıştırır (yeniden kurma açık).
func commandIn(cmd, root, graph string, extra ...string) (code int, out, errOut string) {
	return runCLI(append([]string{cmd, "--root", root, "--graph", graph}, extra...)...)
}

// gitMust bir git komutunu fikstür deposunda çalıştırır (commit kimliği ve imzasız commit ayarları
// verilir); hata testi durdurur.
func gitMust(t *testing.T, root string, args ...string) {
	t.Helper()
	identity := []string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}
	if _, err := gitOut(root, append(identity, args...)...); err != nil {
		t.Fatal(err)
	}
}
