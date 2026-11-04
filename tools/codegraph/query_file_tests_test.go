package main

import (
	"slices"
	"strings"
	"testing"
)

// fileOrderFiles: p.Target'ı iki test 2 adımda sınar; biri başka paketten (adı alfabede önde), biri aynı paketten.
var fileOrderFiles = map[string]string{
	"p/x.go":         "package p\n\nfunc Target() int { return 1 }\n",
	"p/wrap.go":      "package p\n\nfunc Wrap() int { return Target() }\n",
	"p/wrap_test.go": "package p\n\nimport \"testing\"\n\nfunc TestLocal(t *testing.T) { _ = Wrap() }\n",
	"a/a_test.go":    "package a\n\nimport (\n\t\"testing\"\n\n\t\"example.com/audit/p\"\n)\n\nfunc TestAway(t *testing.T) { _ = p.Wrap() }\n",
}

// testsForItems tests-for'u fikstürde koşar ve heading'i içeren grubun öğelerinin adlarını verir.
func testsForItems(t *testing.T, root, target, heading string) []string {
	t.Helper()
	var out, errb strings.Builder
	if code := run([]string{"tests-for", target, "--root", root, "--no-rebuild"}, &out, &errb); code != exitOK {
		t.Fatalf("tests-for %s: %d %s", target, code, errb.String())
	}
	items, ok := groupItems(strings.Split(out.String(), "\n"), heading)
	if !ok {
		t.Fatalf("tests-for %s: no group %q:\n%s", target, heading, out.String())
	}
	var names []string
	for _, it := range items {
		names = append(names, strings.Fields(it)[0])
	}
	return names
}

// Kural: dosya düzeyindeki tests-for bildirim düzeyiyle aynı sıralama kuralını (testRank) kullanır: eşit uzaklıkta
// hedefle aynı klasördeki test önce, sonra ada göre (DENETİM 2026-10-29: `tests-for assign.go`'da düzenlenen paketin
// ilk testi 140. satırdaydı). Koşulan küme değişmez: dosyanın grupları changed'in seçimidir.
func TestAFileOrdersItsTestsLikeItsDeclarations(t *testing.T) {
	root := auditModule(t, fileOrderFiles)
	var out, errb strings.Builder
	if code := run([]string{"build", "--root", root}, &out, &errb); code != exitOK {
		t.Fatalf("build: %d %s", code, errb.String())
	}
	want := []string{"p.TestLocal", "a.TestAway"}
	if got := testsForItems(t, root, "p.Target", "<- tests, indirect ("); !slices.Equal(got, want) {
		t.Errorf("declaration: indirect tests %v, want %v (same folder first at equal steps)", got, want)
	}
	if got := testsForItems(t, root, "p/x.go", "<- tests, indirect ("); !slices.Equal(got, want) {
		t.Errorf("file: indirect tests %v, want %v (the declaration's order)", got, want)
	}
}

// Kural (DENETİM 104 N6): 0 adımlı da olsa [INFERRED] bir `tests` kenarı dosya düzeyindeki tests-for ve changed
// çıktısında `direct` diye yazılmaz (`[INFERRED]` olarak ve EXTRACTED `direct` testlerden sonra yazılır).
func TestInferredZeroStepEdgeIsNotWrittenAsDirect(t *testing.T) {
	root := auditModule(t, map[string]string{
		"p/dispatch.go": "package p\n\nfunc Run(cmd string) {\n\tswitch cmd {\n\tcase \"check\":\n\t\thandleCheck()\n\t}\n}\n",
		"p/cmd.go":      "package p\n\nfunc handleCheck() {}\n\nfunc DirectEntry() {}\n",
		"p/other_test.go": "package p\n\nimport \"testing\"\n\n" +
			"func TestZDirect(t *testing.T) { DirectEntry() }\n\n" +
			"func TestACase(t *testing.T) { Run(\"check\") }\n",
	})
	var buildOut, buildErr strings.Builder
	if code := run([]string{"build", "--root", root}, &buildOut, &buildErr); code != exitOK {
		t.Fatalf("build: %d %s", code, buildErr.String())
	}
	var out, errb strings.Builder
	if code := run([]string{"tests-for", "p/cmd.go", "--root", root, "--no-rebuild"}, &out, &errb); code != exitOK {
		t.Fatalf("tests-for p/cmd.go: %d %s", code, errb.String())
	}
	got := out.String()
	if strings.Contains(got, "p.TestACase [func] p/other_test.go:7  direct") {
		t.Errorf("INFERRED case test must not be labeled direct:\n%s", got)
	}
	wantDirect := "p.TestZDirect [func] p/other_test.go:5  direct"
	wantInferred := "p.TestACase [func] p/other_test.go:7  [INFERRED]"
	di, ii := strings.Index(got, wantDirect), strings.Index(got, wantInferred)
	if di < 0 || ii < 0 || di > ii {
		t.Errorf("expected EXTRACTED direct before INFERRED in tests-for p/cmd.go:\n%s", got)
	}
	ts := newTouchSet()
	g, err := readGraph(root + "/.codegraph/graph.json")
	if err != nil {
		t.Fatal(err)
	}
	ix := newIndex(g)
	for id, d := range ix.fileDecls(ix.byID[fileID("p/cmd.go")]).nodes {
		ts.nodes[id] = d
	}
	var chOut strings.Builder
	ix.printTestsToRun(&chOut, ts, 0, "the changed code", "changed tests")
	ch := chOut.String()
	if strings.Contains(ch, "p.TestACase [func] p/other_test.go:7  direct") || !strings.Contains(ch, wantInferred) {
		t.Errorf("changed output must label INFERRED zero-step edge [INFERRED], not direct:\n%s", ch)
	}
}
