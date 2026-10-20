package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func gitMust(t *testing.T, root string, args ...string) {
	t.Helper()
	if _, err := gitOut(root, args...); err != nil {
		t.Fatal(err)
	}
}

// branchFixture main'de fikstürü, ondan ayrılan üç claude/ dalını kurar:
// a ve b aynı satırı değiştirir (çakışır), c yalnızca belgeyi.
func branchFixture(t *testing.T) string {
	t.Helper()
	root := gitFixture(t)
	shape := filepath.Join(root, "shape", "shape.go")
	edit := func(branch string, apply func()) {
		gitMust(t, root, "checkout", "-q", "-b", branch, "main")
		apply()
		commitAll(t, root, branch)
		gitMust(t, root, "checkout", "-q", "main")
	}
	edit("claude/a", func() {
		replaceIn(t, shape, "_ = codes.A001", "_ = codes.A001 // from a")
		replaceIn(t, shape, "func Orphan() {}", "func Orphan() {}\n\n// Added makes a new function.\nfunc Added() {}")
		replaceIn(t, filepath.Join(root, "internal", "lang", "messages", "tr.json"),
			"\"H0002\": \"second message\"", "\"H0002\": \"second message\",\n\"H0003\": \"third\"")
	})
	edit("claude/b", func() { replaceIn(t, shape, "_ = codes.A001", "_ = codes.A001 // from b") })
	edit("claude/c", func() { replaceIn(t, filepath.Join(root, "docs", "guide.md"), "Second heading", "Another heading") })
	return root
}

func TestChangedMapsHunksTestsAndOtherBranches(t *testing.T) {
	root := branchFixture(t)
	graph := buildInto(t, root)
	code, out, errOut := commandIn("changed", root, graph, "claude/a")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	mustContain(t, out,
		"changed main...claude/a", "old-side lines",
		"shape.Describe [func]",                 // gövdesindeki satır değişti
		"H0003 (added or removed",               // grafikte olmayan yeni kod
		"shape.TestDescribe [func]", "  direct", // doğrudan test
		"shape.TestReport [func]", "  3 steps [INFERRED]", // Report → Twice → Describe
		"claude/b: 1 shared nodes (go:func:example.com/mini/shape.Describe)",
		"CONFLICT in shape/shape.go")
	if strings.Contains(out, "claude/c") {
		t.Errorf("claude/c touches other files only:\n%s", out)
	}
	if strings.Contains(out, "shape.Added [func]") {
		t.Errorf("a function that exists only on the branch has no graph node:\n%s", out)
	}
	// ... ama dalın kendi dosyasından adlandırılır.
	mustContain(t, out, "declarations on the branch that the graph does not have", "shape/shape.go: Added (func)")
}

func TestChangedOnCheckedOutBranchMapsNewSide(t *testing.T) {
	slowFixture(t)
	root := branchFixture(t)
	gitMust(t, root, "checkout", "-q", "claude/a")
	graph := buildInto(t, root)
	code, out, errOut := commandIn("changed", root, graph, "claude/a")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	mustContain(t, out, "new-side lines", "shape.Added [func]", "H0003 [message]")
}

func TestChangedAcceptsRangeAndRejectsUnknownRef(t *testing.T) {
	slowFixture(t)
	root := branchFixture(t)
	graph := buildInto(t, root)
	code, out, _ := commandIn("changed", root, graph, "main..claude/c")
	if code != exitOK || !strings.Contains(out, "doc sections") || !strings.Contains(out, "Usage [heading]") {
		t.Errorf("exit %d:\n%s", code, out)
	}
	if code, _, _ := commandIn("changed", root, graph, "claude/missing"); code != exitFail {
		t.Errorf("unknown ref: exit %d", code)
	}
}

func TestParseDiff(t *testing.T) {
	diff := "diff --git a/x.go b/x.go\nindex 1..2 100644\n--- a/x.go\n+++ b/x.go\n@@ -3,2 +3 @@ func f() {\n-a\n-b\n+c\n@@ -10,0 +10,2 @@\n+d\n+e\n"
	hs := parseDiff(diff)
	if len(hs) != 2 || hs[0].file != "x.go" || hs[0].oldStart != 3 || hs[0].oldCount != 2 || hs[0].newCount != 1 ||
		hs[1].oldCount != 0 || hs[1].newStart != 10 || hs[1].newCount != 2 || len(hs[0].changedLines) != 3 {
		t.Errorf("hunks: %+v", hs)
	}
}

// Eski bir sürümün kurduğu grafik (düğümlerde son satır yok) imzası aynı olsa da yeniden
// kurulur: aksi halde hunk'lar yalnızca bildirim satırına denk gelen düğümlere eşlenirdi.
func TestChangedRebuildsGraphFromOlderExtractor(t *testing.T) {
	slowFixture(t)
	root := branchFixture(t)
	graph := buildInto(t, root)
	g, err := readGraph(graph)
	if err != nil {
		t.Fatal(err)
	}
	g.Extractor = 0
	for i := range g.Nodes {
		g.Nodes[i].End = 0
	}
	if err := writeGraph(graph, g); err != nil {
		t.Fatal(err)
	}
	// --no-rebuild: eski grafik kullanılır, uyarı verilir, işlev eşlenmez.
	code, out, errOut := commandIn("changed", root, graph, "--no-rebuild", "claude/a")
	if code != exitOK || !strings.Contains(errOut, "older codegraph") || strings.Contains(out, "shape.Describe [func]") {
		t.Fatalf("stale format with --no-rebuild: exit %d\nout: %s\nerr: %s", code, out, errOut)
	}
	code, out, errOut = commandIn("changed", root, graph, "claude/a")
	if code != exitOK || !strings.HasPrefix(errOut, "graph rebuilt (") || !strings.Contains(out, "shape.Describe [func]") {
		t.Errorf("an older graph must be rebuilt: exit %d\nout: %s\nerr: %s", code, out, errOut)
	}
}
