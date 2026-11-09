package main

import (
	"io"
	"slices"
	"testing"
)

// cacheGraph app'ta bir zincirdir: Run ve Setup → Register → (argüman olarak) leaf; TestRun Run'ı çağırır. leaf'in çağıranı
// callersOf'ta Register (argümanı izler), nameCallersOf'ta yok. asArg false, Register'ı leaf'e doğrudan bağlayan
// ikinci bir grafı verir (aynı kimlikler, başka kenar ve dir'le başka klasör).
func cacheGraph(asArg bool, dir string) *Graph {
	const m = "example.com/m"
	fn := func(name string) string { return "go:func:" + m + "/app." + name }
	leafEdge := Edge{From: fn("Register"), To: fn("leaf"), Kind: edgeCalls, Confidence: extracted}
	if asArg {
		leafEdge = Edge{From: fn("Register"), To: fn("leaf"), Kind: edgeCalls, Confidence: inferred, Via: viaArg}
	}
	return &Graph{Module: m, Nodes: []Node{
		{ID: pkgID(m + "/app"), Kind: kindPackage},
		{ID: fn("Run"), Kind: kindFunc, Name: "app.Run", File: dir + "/run.go"},
		{ID: fn("Setup"), Kind: kindFunc, Name: "app.Setup", File: dir + "/run.go"},
		{ID: fn("Register"), Kind: kindFunc, Name: "app.Register", File: dir + "/app.go"},
		{ID: fn("leaf"), Kind: kindFunc, Name: "app.leaf", File: dir + "/leaf.go"},
		{ID: fn("TestRun"), Kind: kindFunc, Name: "app.TestRun", File: dir + "/run_test.go"},
	}, Edges: []Edge{
		leafEdge,
		{From: fn("Run"), To: fn("Register"), Kind: edgeCalls, Confidence: extracted},
		{From: fn("Setup"), To: fn("Register"), Kind: edgeCalls, Confidence: extracted}, // Register'ın iki çağıranı: sıra görünür
		{From: fn("TestRun"), To: fn("Run"), Kind: edgeCalls, Confidence: extracted},
		{From: fn("TestRun"), To: fn("Run"), Kind: edgeTests, Confidence: extracted},
	}}
}

// Kural (hız işi, önbellek): önbellekten dönen çağıranlar her an önbelleksiz kurulanla aynıdır. Anahtar kimlik ve
// yürüyüştür (argümanı izleyen ve izlemeyen aynı düğümde farklıdır); tests-for'un yürüyüşleri dilimi değiştirmez.
func TestCachedCallersStayAsBuilt(t *testing.T) {
	g := cacheGraph(true, "app")
	ix := newIndex(g)
	leaf := "go:func:example.com/m/app.leaf"
	if got := ix.callersOf(leaf); len(got) != 1 {
		t.Fatalf("callersOf(leaf) follows the argument edge: %v", got)
	}
	if got := ix.nameCallersOf(leaf); len(got) != 0 {
		t.Errorf("nameCallersOf(leaf) after callersOf does not follow the argument edge: %v", got)
	}
	for i := range g.Nodes {
		ix.printTestsFor(io.Discard, &g.Nodes[i], 0) // yürüyüşler önbelleği kullanır
	}
	for k, got := range ix.callerMemo {
		if want := ix.buildCallers(k.id, k.throughArgs); !slices.Equal(got, want) {
			t.Errorf("cached callers of %s (throughArgs %v) changed: %v, built %v", k.id, k.throughArgs, got, want)
		}
		if cap(got) != len(got) {
			t.Errorf("cached callers of %s can be appended in place: len %d cap %d", k.id, len(got), cap(got))
		}
	}
}

// Kural (hız işi, önbellek): önbellek indeksin içindedir. Aynı süreçte aynı kimlikli iki graf kendi kenarlarını ve
// klasörlerini görür.
func TestTwoGraphsWithTheSameIDsHaveTheirOwnCallers(t *testing.T) {
	a, b := newIndex(cacheGraph(true, "app")), newIndex(cacheGraph(false, "other"))
	leaf := "go:func:example.com/m/app.leaf"
	if got := a.nameCallersOf(leaf); len(got) != 0 {
		t.Errorf("first graph: nameCallersOf(leaf) %v", got)
	}
	if got := b.nameCallersOf(leaf); len(got) != 1 {
		t.Errorf("second graph calls leaf directly: nameCallersOf(leaf) %v", got)
	}
	if pa, pb := a.pkgOf(leaf), b.pkgOf(leaf); pa != "app" || pb != "other" {
		t.Errorf("pkgOf: %q and %q, want app and other", pa, pb)
	}
}
