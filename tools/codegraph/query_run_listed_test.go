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

// Kural: -run listesi commandLineLimit'i aşınca ağır olmayan paketler bütün koşar (ön denetimin kuralı), ağır paket
// (heavyTestPackages) -run'sız önerilmez: sığan ilk adlar yazılır, kalanı sayılır (DENETİM 358 Ş2).
func TestAHeavyPackageIsNeverRunWholeByTheRunLine(t *testing.T) {
	t.Parallel()
	want := []string{"cmd/trlang/e2e", "internal/compute/gpu", "internal/detmath", "internal/lang/parser",
		"internal/stdlib/httpserver", "internal/stdlib/process", "tools/codegraph"} // K2: G4'te 30 sn ya da dizüstünde 2 dakika
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
	if !strings.Contains(out.String(), "more near tests of tools/codegraph did not fit the command line") {
		t.Errorf("the answer counts the names that did not fit:\n%s", out.String())
	}
}

// Kural: ağır paketteki (heavyTestPackages) bir dosyanın koşu satırı paketini -run'sız önermez (D1 ipucu, 2026-11-06:
// kanca `go test -short ./tools/codegraph/`'ı durduruyordu): o paketteki yakın testler adlandırılır, satır bunu söyler.
// Hiçbir `run:` komutu ağır bir paketi bütün olarak koşmaz; K2 ile giren `httpserver` de.
func TestAFileOfAHeavyPackageNamesItsTestsInsteadOfRunningItWhole(t *testing.T) {
	t.Parallel()
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	ix.root = root
	for _, c := range []struct{ file, test string }{
		{"tools/codegraph/query_run_line.go", "TestTheRunLineStaysWithinTheCommandLine"},
		{"internal/detmath/detmath.go", ""},
		{"internal/lang/parser/conditionals.go", ""},
		{"internal/stdlib/httpserver/stream.go", ""},
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

// Kural: bir .tr modülünün dar satırı kendi TR test dosyasını ve modülü yükleyen TR test dosyasını tek koşulabilir satırla
// koşar: `go run ./cmd/trlang test <dosya1> <dosya2> ...` (gaps: lib/whisper.tr'nin run: satırı yoktu; lib/audio.tr'nin
// doğrusu `trlang test lib/audio_test.tr`, S1 2026-11-05). Dolaylı TR testleri satıra girmez (codegraph 7 A1).
func TestATRModuleRunLineRunsItsListedTRTestFiles(t *testing.T) {
	t.Parallel()
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
	}
}

// Kural: yakın TR testi olan her dosya ya da belge (Go dosyaları dahil, ör. stream.go) bu TR test dosyalarını tek
// `go run ./cmd/trlang test ...` satırıyla koşar; TR testi olmayan Go dosyasında (`root.go`) bu satır çıkmaz ve negatif
// fikstürler (`tests/negative/*.tr`) `trlang test` satırına girmez (Madde 5, DENETİM 510 N3).
func TestGoFileRunLineRunsListedTRTestsOnlyWhenPresent(t *testing.T) {
	t.Parallel()
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	ix.root = root

	stream := ix.byID[fileID("internal/stdlib/httpserver/stream.go")]
	if stream == nil {
		t.Fatal("stream.go not found")
	}
	var out strings.Builder
	ix.printTestsFor(&out, stream, defaultLimit)
	if !slices.Contains(runLines(out.String()), "go run ./cmd/trlang test tests/http_sunucusu_test.tr") {
		t.Errorf("stream.go must produce `run: go run ./cmd/trlang test tests/http_sunucusu_test.tr`:\n%s", out.String())
	}

	// N3 sıra kuralı: lib/audio.tr'de ilk hedef lib/audio_test.tr olmalı (planning_questions.txt:306).
	out.Reset()
	ix.printTestsFor(&out, ix.byID[trFileID("lib/audio.tr")], defaultLimit)
	if !strings.Contains(out.String(), "run: go run ./cmd/trlang test lib/audio_test.tr") {
		t.Errorf("lib/audio.tr must keep lib/audio_test.tr first in trlang test line:\n%s", out.String())
	}

	// Her örnek düğümde negatif fikstürler `trlang test` satırına girmez.
	nodes := append(runPlanFiles(g),
		stream,
		ix.byID[trFileID("lib/audio.tr")],
		ix.byID["file:tools/full_check.sh"],
		ix.byID["file:internal/lang/codes/definitions.json"],
		ix.byID["doc:docs/spec/28_ag.md"],
	)
	for _, n := range nodes {
		out.Reset()
		ix.printTestsFor(&out, n, defaultLimit)
		var tr string
		for _, cmd := range runLines(out.String()) {
			if rest, ok := strings.CutPrefix(cmd, "go run ./cmd/trlang test "); ok {
				tr = " " + rest + " "
			}
		}
		if strings.Contains(tr, "tests/negative/") {
			t.Errorf("%s: negative fixture leaked into trlang test line:%s", n.File, tr)
		}
	}

	// Olumsuz: TR testi olmayan Go dosyasında `trlang test` satırı üretilmez.
	out.Reset()
	ix.printTestsFor(&out, ix.byID[fileID("tools/codegraph/root.go")], defaultLimit)
	for _, cmd := range runLines(out.String()) {
		if strings.HasPrefix(cmd, "go run ./cmd/trlang test") {
			t.Errorf("root.go has no TR tests, unexpected trlang test line: %q", cmd)
		}
	}
}

// Kural: çıktıda listelenen `Benchmark*` ölçümleri (`definitions.json`, `36_karakter_modeli.tr`) hiçbir `run:`
// satırına sızmaz (Madde 6, DENETİM 510 Ş4).
func TestBenchmarksNeverLeakIntoRunLines(t *testing.T) {
	t.Parallel()
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	ix.root = root
	for _, c := range []struct {
		id       string
		wantBody string
	}{
		{"file:internal/lang/codes/definitions.json", "BenchmarkBuild"},
		{"file:internal/lang/codes/definitions.json", "BenchmarkLoad"},
		{"tr:file:examples/tr/36_karakter_modeli.tr", "BenchmarkAttentionCharModel"},
	} {
		var out strings.Builder
		ix.printTestsFor(&out, ix.byID[c.id], defaultLimit)
		if !strings.Contains(out.String(), c.wantBody) {
			t.Fatalf("%s: expected %s in body:\n%s", c.id, c.wantBody, out.String())
		}
		for _, cmd := range runLines(out.String()) {
			_, runArg, hasRun := strings.Cut(cmd, " -run ")
			if !hasRun {
				continue
			}
			for _, name := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(runArg, "'^("), ")$'"), "|") {
				if strings.HasPrefix(name, "Benchmark") || name == c.wantBody {
					t.Errorf("%s: benchmark %s leaked into run: line %q", c.id, name, cmd)
				}
			}
		}
	}
}

// Kural (codegraph dilim 6, D6-1, Karar 1; codegraph 7 A3): tests-for'un bayraksız cevabı dosya satırı, run: satırları,
// tek sayı satırı ve size: satırıdır; grupların ilk sayfası --list ile (--limit de açar), hepsi --all ile gelir. Kısa
// cevabın satırları --list cevabının başıdır ve koşu satırları aynıdır; gruplara gönderen satır --list'e gönderir (S8).
// Testi bulunmayan düğümün cevabı (dal satırı) kısa kipte de tamdır.
func TestTestsForShowsTheRunAndSizeLinesUnlessListed(t *testing.T) {
	t.Parallel()
	root := gitFixture(t)
	graph := buildInto(t, root)
	ask := func(args ...string) string {
		t.Helper()
		code, out, errOut := commandIn("tests-for", root, graph, append([]string{"--no-rebuild"}, args...)...)
		if code != exitOK {
			t.Fatalf("tests-for %v: exit %d: %s", args, code, errOut)
		}
		return out
	}
	short, list, all := ask("shape/shape.go"), ask("--list", "shape/shape.go"), ask("--all", "shape/shape.go")
	if strings.Contains(short, " listed (--list shows") || strings.Contains(short, "not run by these commands") {
		t.Errorf("the short answer has one count line, not the old two:\n%s", short)
	}
	if !strings.Contains(short, "\n  narrow: these lines run the near tests; ") && !strings.Contains(short, "\n  these commands run every Go test") {
		t.Errorf("the short answer must end its run: lines with the count line:\n%s", short)
	}
	head := strings.ReplaceAll(short, "(--list names them)", "(among the groups below)")
	// Fikstürde boyut testi yok, size: satırı da yok; depo dosyasındaki size: satırını kısa cevapta
	// TestSizeLineReadsFunctionsFromDiskWhenBuildIsBrokenAndReportsStaleOnSyntaxError sınar.
	if !strings.HasPrefix(list, head) || !strings.Contains(head, "\nrun: ") {
		t.Errorf("the short answer must be the file and run: lines that start the --list answer:\nshort:\n%s\nlist:\n%s", short, list)
	}
	for _, group := range []string{"  declarations in this file", "  this file's tests ("} {
		if strings.Contains(short, group) || !strings.Contains(list, group) || !strings.Contains(all, group) {
			t.Errorf("%q must be in --list and --all, not in the short answer:\nshort:\n%s", group, short)
		}
	}
	if limited := ask("--limit", "3", "shape/shape.go"); !strings.Contains(limited, "  declarations in this file") {
		t.Errorf("--limit must list the groups too:\n%s", limited)
	}
	if orphan := ask("shape.Orphan"); !strings.Contains(orphan, "no tests found") || orphan != ask("--list", "shape.Orphan") {
		t.Errorf("a node without tests keeps its whole answer:\n%s", orphan)
	}
}
