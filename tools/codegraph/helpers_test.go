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
	args = append([]string{args[0], "--graph", path}, args[1:]...)
	code = run(args, &so, &se)
	return code, so.String(), se.String()
}
