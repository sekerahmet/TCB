package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// beyondModule p'de F0 ← F1 ← … ← F10 zinciridir: TestOwnDeep (F0'ın kendi test dosyasında) ve TestFar F10'u çağırır,
// F0'dan 11 adım; q'daki TestOther da F10'u çağırır ama paket sınırını geçer. F0, F1, F2'nin sınır içinde testi yok.
func beyondModule(t *testing.T) string {
	t.Helper()
	var chain strings.Builder
	chain.WriteString("package p\n")
	for i := 1; i <= 10; i++ {
		fmt.Fprintf(&chain, "\nfunc F%d() int { return F%d() }\n", i, i-1)
	}
	root := auditModule(t, map[string]string{
		"p/core.go":      "package p\n\nfunc F0() int { return 1 }\n",
		"p/chain.go":     chain.String(),
		"p/core_test.go": "package p\n\nimport \"testing\"\n\nfunc TestOwnDeep(t *testing.T) { F10() }\n",
		"p/far_test.go":  "package p\n\nimport \"testing\"\n\nfunc TestFar(t *testing.T) { F10() }\n",
		"q/q_test.go":    "package q\n\nimport (\n\t\"testing\"\n\n\t\"example.com/audit/p\"\n)\n\nfunc TestOther(t *testing.T) { p.F10() }\n",
	})
	if code, _, errOut := runCLI("build", "--root", root); code != exitOK {
		t.Fatalf("build: %d %s", code, errOut)
	}
	return root
}

// Kural (B1a, kademe 1 ve 2): klasör içi zincirle adım sınırının ötesinde ulaşan test, `untested`'da işlevi "testli"
// yapar; tests-for'da hedefin kendi test dosyasındaki adıyla (koşu satırına da girer), kalanı tek satırda girişiyle
// yazılır. Başka paketten gelen zincir sınırda kalır (TestOther yok).
func TestATestBeyondTheStepLimitMakesAFunctionTestedAndIsSummarized(t *testing.T) {
	root := beyondModule(t)
	code, out, errOut := runCLI("untested", "--root", root, "--no-rebuild")
	if code != exitOK || !strings.Contains(out, "11 non-test Go functions and methods: 11 tested") {
		t.Errorf("untested: %d %s\n%s", code, errOut, out)
	}
	code, out, errOut = runCLI("tests-for", "--root", root, "--no-rebuild", "p.F0")
	if code != exitOK {
		t.Fatalf("tests-for: %d %s", code, errOut)
	}
	for _, want := range []string{
		"  <- this file's tests beyond the 8-step limit (1; a chain inside its folder):\n    p.TestOwnDeep [func] p/core_test.go:5  11 steps via p.F1  [INFERRED]\n",
		"  beyond the 8-step limit, inside its folder: 1 more tests reach it through p.F3 (from 11 steps); `tests-for p.F3` lists them\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("tests-for p.F0 lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "TestOther") {
		t.Errorf("TestOther's chain crosses a package, it keeps the limit:\n%s", out)
	}
	if _, out, _ = runCLI("tests-for", "--root", root, "--no-rebuild", "p.F3"); !strings.Contains(out, "p.TestFar [func] p/far_test.go") {
		t.Errorf("tests-for of the entry lists TestFar within the limit:\n%s", out)
	}
}

// Kural ("değişmedi" denetimi, DENETİM 363 Ş1): listeler sınırda kalır. changed --go-tests'in seçimi (testDistances),
// tests-for'un zinciri (indirectTests) sınırın ötesindeki testi almaz; yalnız untested'ın kararı ve toplam satırı onu görür.
func TestTheListsKeepTheStepLimit(t *testing.T) {
	g, err := readGraph(filepath.Join(beyondModule(t), ".codegraph", "graph.json"))
	if err != nil {
		t.Fatal(err)
	}
	ix := newIndex(g)
	byName := func(name string) *Node {
		for i := range g.Nodes {
			if g.Nodes[i].Name == name {
				return &g.Nodes[i]
			}
		}
		t.Fatalf("no node %s", name)
		return nil
	}
	f0 := byName("p.F0")
	if d := ix.testDistances(f0); len(d) != 0 {
		t.Errorf("changed --go-tests selects no test of F0 beyond the limit: %v", d)
	}
	if got := ix.indirectTests(f0); len(got) != 0 {
		t.Errorf("tests-for's chain stays within the limit: %+v", got)
	}
	// Yürüyüşün öbür çağıranları (ad kodu yolu, alt süreç bağı, ad kodu testleri, testsWithin) kendi süzgeçlerine
	// güvenmez: sarmalayıcı sınırın ötesine hiç geçmez (DENETİM 363 Ş1).
	walkCallers(f0.ID, ix.callersOf, ix.pkgOf, func(r reached) bool {
		if r.Steps > testMaxSteps {
			t.Errorf("walkCallers went beyond the limit: %+v", r)
		}
		return true
	})
	if d := ix.testDistances(byName("p.F3")); d[byName("p.TestFar").ID] != testMaxSteps {
		t.Errorf("F3 is 8 steps from TestFar: %v", d)
	}
	if b := ix.beyondLimit(f0, nil); b.total() != 2 || len(b.own) != 1 || b.rest != 1 || b.entries[0].entry.Name != "p.F3" {
		t.Errorf("beyond the limit: %+v", b)
	}
}
