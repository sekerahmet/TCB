package main

import (
	"path"
	"slices"
	"strings"
	"testing"
)

// runLines cevabın `run:` satırlarının komutlarıdır.
func runLines(out string) []string {
	var cmds []string
	for _, l := range strings.Split(out, "\n") {
		if c, ok := strings.CutPrefix(l, "run: "); ok {
			cmds = append(cmds, c)
		}
	}
	return cmds
}

// Kural: tests-for'un varsayılan cevabında adı geçen her Go testi bir `run:` satırında koşar; satıra sığmayan ağır paket
// adları sayılır (28. bölümün G4'ü: docs/spec/28_ag.md'nin cevabı TestDocTRCodeBlocks'u listeliyor, koşu satırı onu
// koşmuyordu). Yalnız adlandırır: hangisinin commit'ten önce koşulacağına karar vermez.
func TestEveryListedTestIsNamedByARunLine(t *testing.T) {
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	ix.root = root
	nodes := runPlanFiles(g)
	if doc := ix.byID["doc:docs/spec/28_ag.md"]; doc != nil {
		nodes = append(nodes, doc)
	} else {
		t.Fatal("docs/spec/28_ag.md is not in the graph")
	}
	for _, n := range nodes {
		var out strings.Builder
		ix.printTestsFor(&out, n, defaultLimit)
		inRun := runCovers(runLines(out.String()))
		for _, gt := range listedIn(out.String()).goTests {
			if !inRun(gt) && !strings.Contains(out.String(), "did not fit the command line") {
				t.Errorf("%s: the answer lists %s %s, no run: line runs it", n.File, gt.dir, gt.name)
			}
		}
	}
}

// Kural: -run listesi commandLineLimit'i aşınca ağır olmayan paketler bütün koşar (ön denetimin kuralı), ağır paket
// (heavyTestPackages) -run'sız önerilmez: sığan ilk adlar yazılır, kalanı sayılır (DENETİM 358 Ş2).
func TestAHeavyPackageIsNeverRunWholeByTheRunLine(t *testing.T) {
	want := []string{"cmd/trlang/e2e", "internal/compute/gpu", "internal/detmath", "internal/lang/parser", "tools/codegraph"}
	if !slices.Equal(heavyTestPackages, want) {
		t.Errorf("heavy packages %q, want %q (the main session keeps this list equal to its hook's)", heavyTestPackages, want)
	}
	var names []string
	for i := 0; len(strings.Join(names, "|")) <= commandLineLimit; i++ {
		names = append(names, "TestAVeryLongNameThatKeepsTheListGrowing"+strings.Repeat("x", i%10)+string(rune('A'+i%26)))
	}
	p := runPlan{pkg: "a", others: []string{"b", "tools/codegraph"}, names: names, whole: true, heavy: []string{"tools/codegraph"}}
	p.fit, p.cut = p.fitNames(p.heavy, names)
	cmds := p.commands()
	if len(cmds) != 3 || cmds[1] != "go test -short ./b/" || !strings.HasPrefix(cmds[2], "go test -short ./tools/codegraph/ -run '^(") {
		t.Fatalf("the light package runs whole, the heavy one by name: %q", cmds)
	}
	if len(cmds[2]) > commandLineLimit || p.cut == 0 || len(p.fit)+p.cut != len(names) {
		t.Errorf("the heavy command fits the limit (%d chars), %d names fit, %d cut of %d", len(cmds[2]), len(p.fit), p.cut, len(names))
	}
	var out strings.Builder
	printRunPlan(&out, p)
	if !strings.Contains(out.String(), "more listed tests of tools/codegraph did not fit the command line") {
		t.Errorf("the answer counts the names that did not fit:\n%s", out.String())
	}
}

// Kural: ağır paketteki (heavyTestPackages) bir dosyanın koşu satırı paketini -run'sız önermez (D1 ipucu, 2026-11-06:
// kanca `go test -short ./tools/codegraph/`'ı durduruyordu): dosyanın kendi testleri ve cevapta adı geçen testleri
// adlandırılır, satır bunu söyler. Hiçbir `run:` komutu ağır bir paketi bütün olarak koşmaz.
func TestAFileOfAHeavyPackageNamesItsTestsInsteadOfRunningItWhole(t *testing.T) {
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	ix.root = root
	for _, c := range []struct{ file, test string }{
		{"tools/codegraph/query_run_line.go", "TestTheRunLineStaysWithinTheCommandLine"},
		{"internal/detmath/detmath.go", ""},
		{"internal/lang/parser/conditionals.go", ""},
	} {
		n := ix.byID[fileID(c.file)]
		if n == nil {
			t.Fatalf("%s is not in the graph", c.file)
		}
		var out strings.Builder
		ix.printTestsFor(&out, n, defaultLimit)
		cmds := runLines(out.String())
		for _, cmd := range cmds {
			args, _, hasRun := strings.Cut(strings.TrimPrefix(cmd, "go test -short "), " -run ")
			for _, a := range strings.Fields(args) {
				if d := strings.Trim(a, "./"); isHeavyPackage(d) && !hasRun {
					t.Errorf("%s: run: %s runs the heavy package %s whole", c.file, cmd, d)
				}
			}
		}
		if !strings.Contains(out.String(), "./"+path.Dir(c.file)+"/ is a heavy package: it does not run whole here") {
			t.Errorf("%s: the answer says the package is heavy:\n%s", c.file, out.String())
		}
		if c.test != "" && !runCovers(cmds)(goTest{path.Dir(c.file), c.test}) {
			t.Errorf("%s: the run line does not name the file's own test %s:\n%q", c.file, c.test, cmds)
		}
	}
}

// Kural: bir .tr modülünün cevabı TR testlerini listeliyorsa onları tek koşulabilir satır koşar:
// `go run ./cmd/trlang test <dosya1> <dosya2> ...` (gaps: lib/whisper.tr için dolaylı TR testleri listeleniyor, run:
// satırı yoktu; lib/audio.tr'nin doğrusu `trlang test lib/audio_test.tr`, S1 2026-11-05).
func TestATRModuleRunLineRunsItsListedTRTestFiles(t *testing.T) {
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	ix.root = root
	for _, c := range []struct{ file, want string }{
		{"lib/audio.tr", "lib/audio_test.tr"},
		{"lib/whisper.tr", "tests/whisper_model_test.tr"},
	} {
		var out strings.Builder
		ix.printTestsFor(&out, ix.byID[trFileID(c.file)], defaultLimit)
		var tr string
		for _, cmd := range runLines(out.String()) {
			if rest, ok := strings.CutPrefix(cmd, "go run ./cmd/trlang test "); ok {
				tr = " " + rest + " "
			}
		}
		if !strings.Contains(tr, " "+c.want+" ") {
			t.Errorf("%s: no run: line runs %s:\n%s", c.file, c.want, out.String())
		}
		for _, f := range listedIn(out.String()).trFiles {
			if !strings.Contains(tr, " "+f+" ") {
				t.Errorf("%s: the answer lists TR tests of %s, the trlang test line does not run it", c.file, f)
			}
		}
	}
}
