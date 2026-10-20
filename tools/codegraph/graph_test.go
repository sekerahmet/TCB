package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	describeID = fixturePkg + "Describe"
	twiceID    = fixturePkg + "Twice"
	testDescID = fixturePkg + "TestDescribe"
	testAreaID = fixturePkg + "TestSquare_Area"
	squareArea = fixturePkg + "Square.Area"
	shapeArea  = fixturePkg + "Shape.Area"
)

func TestGoNodes(t *testing.T) {
	g := fixture(t)
	for _, id := range []string{
		"go:pkg:example.com/mini/shape", "go:file:shape/shape.go", describeID, squareArea, shapeArea,
		"go:type:example.com/mini/shape.Square", "go:type:example.com/mini/shape.Shape",
	} {
		if !hasNode(g, id) {
			t.Errorf("missing node %s", id)
		}
	}
	for _, n := range g.Nodes {
		if n.ID == describeID && (n.File != "shape/shape.go" || n.Line != 21) {
			t.Errorf("Describe is at %s:%d, want shape/shape.go:21", n.File, n.Line)
		}
	}
}

func TestGoEdges(t *testing.T) {
	g := fixture(t)
	mustEdge(t, g, "go:pkg:example.com/mini/shape", "go:pkg:example.com/mini/internal/lang/messages", edgeImports, extracted)
	mustEdge(t, g, twiceID, describeID, edgeCalls, extracted)
	mustEdge(t, g, describeID, shapeArea, edgeCalls, extracted) // arayüz çağrısı arayüz metoduna gider
	mustEdge(t, g, "go:type:example.com/mini/shape.Square", "go:type:example.com/mini/shape.Shape", edgeImplements, extracted)
	if e := edge(g, twiceID, describeID, edgeCalls); e == nil || e.Count != 2 {
		t.Errorf("Twice calls Describe twice, edge: %+v", e)
	}
}

func TestCodeNodesAndUses(t *testing.T) {
	g := fixture(t)
	for _, n := range g.Nodes {
		if n.ID == "msg:H0001" && (n.Detail != "first message {name}" || len(n.Params) != 1 || n.Line != 2) {
			t.Errorf("H0001 node: %+v", n)
		}
	}
	mustEdge(t, g, describeID, "msg:H0001", edgeUsesMessage, extracted)
	mustEdge(t, g, describeID, "name:A001", edgeUsesName, extracted)
	if edge(g, "go:pkg:example.com/mini/internal/lang/messages", "msg:H0001", edgeUsesMessage) != nil {
		t.Error("the messages package's own definition must not count as a use")
	}
}

func TestTestEdges(t *testing.T) {
	g := fixture(t)
	mustEdge(t, g, testDescID, "go:pkg:example.com/mini/shape", edgeTests, extracted)
	mustEdge(t, g, testDescID, describeID, edgeTests, extracted) // check() yardımcısı üzerinden
	mustEdge(t, g, testAreaID, squareArea, edgeTests, extracted) // doğrudan çağrı
	if edge(g, testDescID, twiceID, edgeTests) != nil {
		t.Error("TestDescribe does not exercise Twice")
	}
}

func TestNameBasedTestEdgeIsInferred(t *testing.T) {
	syms := map[string]string{"Describe": "a", "Square.Area": "b", "Square": "c"}
	cases := map[string][]string{
		"TestDescribe":      {"a"},
		"TestSquare_Area":   {"b", "c"},
		"TestSquareArea":    {"b"},
		"BenchmarkDescribe": {"a"},
		"TestNothing":       nil,
	}
	for name, want := range cases {
		got := nameTargets(name, syms)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s -> %v, want %v", name, got, want)
		}
	}
}

func TestRules(t *testing.T) {
	g := fixture(t)
	mustEdge(t, g, "rule:README.md:7", testDescID, edgeGuardedBy, inferred) // kök belge, yol ipucu yok: depo geneli
	mustEdge(t, g, "rule:README.md:11", testAreaID, edgeGuardedBy, extracted)
	mustEdge(t, g, "rule:README.md:11", "go:file:shape/shape_test.go", edgeGuardedBy, extracted)
	for _, e := range g.Edges {
		if e.From == "rule:README.md:12" {
			t.Errorf("a table row without a test has no guard edge: %+v", e)
		}
	}
	found := false
	for _, w := range g.Warnings {
		found = found || strings.Contains(w, "TestMissing")
	}
	if !found {
		t.Errorf("a missing test must be a warning: %v", g.Warnings)
	}
}

func TestDocs(t *testing.T) {
	g := fixture(t)
	mustEdge(t, g, "heading:README.md#mini", "heading:README.md#rules", edgeContains, extracted)
	mustEdge(t, g, "heading:README.md#rules", "heading:docs/guide.md#usage", edgeLinksTo, extracted)
	mustEdge(t, g, "heading:README.md#rules", "go:pkg:example.com/mini/shape", edgeLinksTo, extracted)
	mustEdge(t, g, "heading:docs/guide.md#usage", "doc:README.md", edgeLinksTo, extracted)
	mustEdge(t, g, "heading:docs/guide.md#usage", "go:file:shape/shape.go", edgeLinksTo, extracted)
	if !hasNode(g, "heading:docs/guide.md#usage-1") {
		t.Error("a repeated heading gets a -1 suffix, as on GitHub")
	}
	broken := false
	for _, w := range g.Warnings {
		broken = broken || strings.Contains(w, "broken link: docs/guide.md:") && strings.Contains(w, "missing.md")
	}
	if !broken {
		t.Errorf("expected a broken link warning: %v", g.Warnings)
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Gönderim sırası: CPU ve GPU": "gönderim-sırası-cpu-ve-gpu",
		"Yeni kod (ad) isteği":        "yeni-kod-ad-isteği",
		"a_b-c":                       "a_b-c",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEdgesHaveConfidenceAndEndpoints(t *testing.T) {
	g := fixture(t)
	ids := map[string]bool{}
	for _, n := range g.Nodes {
		ids[n.ID] = true
	}
	for _, e := range g.Edges {
		if e.Confidence != extracted && e.Confidence != inferred {
			t.Errorf("edge %+v has no confidence label", e)
		}
		if !ids[e.From] || !ids[e.To] {
			t.Errorf("edge %+v has a dangling end", e)
		}
	}
}

func TestGraphIsDeterministic(t *testing.T) {
	g := fixture(t)
	if g.SchemaVersion != schemaVersion {
		t.Errorf("schema_version = %d", g.SchemaVersion)
	}
	abs, _ := filepath.Abs(filepath.Join("testdata", "mod"))
	again, err := buildGraph(abs)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.json"), filepath.Join(dir, "b.json")
	if writeGraph(a, g) != nil || writeGraph(b, again) != nil {
		t.Fatal("write failed")
	}
	da, _ := os.ReadFile(a)
	db, _ := os.ReadFile(b)
	if !bytes.Equal(da, db) {
		t.Error("two builds of the same tree differ")
	}
}

func TestReadGraphRejectsOtherSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "g.json")
	if err := os.WriteFile(path, []byte(`{"schema_version": 99}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readGraph(path); err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Errorf("err = %v", err)
	}
}

func TestConstAndVarNodes(t *testing.T) {
	g := fixture(t)
	const pkg = "example.com/mini/shape."
	for _, n := range g.Nodes {
		if n.ID == "go:const:"+pkg+"Sides" && (n.Kind != kindConst || n.File != "shape/shape.go" || !strings.Contains(n.Detail, "= 4")) {
			t.Errorf("Sides: %+v", n)
		}
	}
	for _, id := range []string{"go:const:" + pkg + "Sides", "go:var:" + pkg + "Default"} {
		if !hasNode(g, id) {
			t.Errorf("missing node %s", id)
		}
	}
	mustEdge(t, g, "go:func:"+pkg+"Corners", "go:const:"+pkg+"Sides", edgeUses, extracted)
	mustEdge(t, g, "go:func:"+pkg+"Corners", "go:var:"+pkg+"Default", edgeUses, extracted)
	mustEdge(t, g, "go:file:shape/shape.go", "go:const:"+pkg+"Sides", edgeContains, extracted)
	if hasNode(g, "go:var:"+pkg+"s") || hasNode(g, "go:var:"+pkg+"err") {
		t.Error("parameters and locals are not package-level variables")
	}
}

// Kod sabiti (messages.H0001, codes.A001) kodun düğümüne bağlanır; kullanımları hem
// uses_message/uses_name hem uses kenarıdır.
func TestCodeConstantsDenoteCodes(t *testing.T) {
	g := fixture(t)
	msgConst := "go:const:example.com/mini/internal/lang/messages.H0001"
	nameConst := "go:const:example.com/mini/internal/lang/codes.A001"
	mustEdge(t, g, msgConst, "msg:H0001", edgeDenotes, extracted)
	mustEdge(t, g, nameConst, "name:A001", edgeDenotes, extracted)
	mustEdge(t, g, describeID, msgConst, edgeUses, extracted)
	mustEdge(t, g, describeID, "msg:H0001", edgeUsesMessage, extracted)
	mustEdge(t, g, describeID, nameConst, edgeUses, extracted)
}
