package main

import (
	"strings"
	"testing"
)

var measurementFiles = map[string]string{
	"p/p.go": "package p\n\nfunc Work() int { return 1 }\n\nfunc Wrapped() int { return Work() }\n",
	"p/p_test.go": `package p

import "testing"

func TestWork(t *testing.T) { _ = Work() }

func BenchmarkWork(b *testing.B) { _ = Work() }

func BenchmarkWrapped(b *testing.B) { _ = Wrapped() }
`,
}

// Kural: `tests-for` ölçümleri (Benchmark* ve tools/bench) testlerin arasında değil, "measurements, not tests" başlığı
// altında verir; doğrudan ve dolaylı ölçüm ikisi de oraya gider, testler başlığın üstünde kalır.
func TestTestsForKeepsMeasurementsApartFromTests(t *testing.T) {
	root := auditModule(t, measurementFiles)
	var out, errb strings.Builder
	if code := run([]string{"build", "--root", root}, &out, &errb); code != exitOK {
		t.Fatalf("build: %d %s", code, errb.String())
	}
	out.Reset()
	if code := run([]string{"tests-for", "p.Work", "--root", root, "--no-rebuild"}, &out, &errb); code != exitOK {
		t.Fatalf("tests-for: %d %s", code, errb.String())
	}
	got := out.String()
	head, measured, found := strings.Cut(got, "measurements, not tests")
	if !found {
		t.Fatalf("no measurements section:\n%s", got)
	}
	if !strings.Contains(head, "p.TestWork") || strings.Contains(head, "Benchmark") {
		t.Errorf("tests above the heading, benchmarks never:\n%s", got)
	}
	if !strings.Contains(measured, "p.BenchmarkWork") || !strings.Contains(measured, "p.BenchmarkWrapped") || strings.Contains(measured, "TestWork") {
		t.Errorf("both benchmarks (direct and through Wrapped) under the heading:\n%s", got)
	}
}

// Kural: tests-for çıkarımla bulunan testleri kanıt gücüne göre verir: kısa zincir ve aynı paket önce; zincir sınırının
// yarısından uzun zincirle bulunanlar en sonda "weak evidence" başlığı altında. Ad kodu üzerinden bir adımda bulunan
// aynı paket testi, sekiz adımlık bir zincirle bulunan başka paket testinden önce gelir.
func TestTestsForOrdersInferredTestsByEvidence(t *testing.T) {
	g := &Graph{Nodes: []Node{
		{ID: "target", Kind: kindFunc, Name: "p.target", File: "p/x.go"},
		{ID: "name:Y010", Kind: kindName, Name: "Y010", Detail: "güncelle"},
		{ID: "near", Kind: kindFunc, Name: "p.TestNear", File: "p/x_test.go"},
		{ID: "far", Kind: kindFunc, Name: "q.TestFar", File: "q/y_test.go"},
	}}
	g.Edges = append(g.Edges,
		Edge{From: "target", To: "name:Y010", Kind: edgeUsesName, Confidence: extracted},
		Edge{From: "near", To: "name:Y010", Kind: edgeTests, Confidence: inferred})
	prev := "far"
	for i := 1; i < testMaxSteps; i++ {
		id := "q" + string(rune('a'+i))
		g.Nodes = append(g.Nodes, Node{ID: id, Kind: kindFunc, Name: "q." + id, File: "q/y.go"})
		g.Edges = append(g.Edges, Edge{From: prev, To: id, Kind: edgeCalls, Confidence: extracted})
		prev = id
	}
	g.Edges = append(g.Edges, Edge{From: prev, To: "target", Kind: edgeCalls, Confidence: extracted})
	ix := newIndex(g)
	var out strings.Builder
	ix.printTestsFor(&out, ix.byID["target"], 12)
	lines := strings.Split(out.String(), "\n")
	if len(lines) < 3 || !strings.Contains(lines[2], "p.TestNear") {
		t.Errorf("the one-step same-package test must be the first test line:\n%s", out.String())
	}
	_, weak, found := strings.Cut(out.String(), "weak evidence")
	if !found || !strings.Contains(weak, "q.TestFar") {
		t.Errorf("the eight-step chain must be under weak evidence, last:\n%s", out.String())
	}
}
