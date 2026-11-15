package main

import (
	"path"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// runPlanFiles koşu satırı sınanan dosyalardır: olayın dosyası (2026-10-29) ve depodan sıralı bir örnek.
func runPlanFiles(g *Graph) []*Node {
	var out []*Node
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if n.Kind == kindFile && strings.HasSuffix(n.File, ".go") && !strings.Contains(n.File, "/testdata/") &&
			(n.File == "internal/lang/interp/compiler_assign.go" || n.File == "internal/lang/checker/unpack.go" || strings.HasPrefix(n.File, "internal/lang/interp/") && i%7 == 0) {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].File < out[j].File })
	return out
}

// Kural (codegraph 7, A1): tests-for'un dar koşu satırı yakın testleri koşar, uzakları koşmaz. İlk komut dosyanın paketini
// bütün olarak koşar; öbürleri dosyanın kendi testlerini, onu konu edinenleri (fikstür koşucularıyla) ve başka paketlerdeki
// doğrudan testleri (en çok narrowHubLimit). Dosyayı yalnız tarayan depo kuralları, denetleyici koşucuları ve dolaylı ya da
// zayıf zincirle bulunan testler satırda yoktur. Satırın koştuğu ile kalan sayısı birlikte `changed --go-tests`'in seçimidir
// (goTestSelection): kalanları commit öncesi denetim ve G4 koşar.
func TestTheNarrowRunLineHasItsNearTestsAndNoFarOnes(t *testing.T) {
	t.Parallel()
	_, _, g := repoGraph(t)
	ix := newIndex(g)
	files := runPlanFiles(g)
	if len(files) < 5 || files[0].File > "internal/lang/interp/compiler_assign.go" {
		t.Fatalf("only %d files sampled", len(files))
	}
	for _, rel := range []string{"internal/lang/checker/checker.go", "internal/lang/checker/conditions.go", "internal/lang/interp/library.go"} {
		files = append(files, ix.byID[fileID(rel)])
	}
	for _, file := range files {
		ts := ix.fileDecls(file)
		s := ix.fileTests(file, ts)
		p := ix.runPlanFor(file, ts, s)
		cmds := p.commands()
		if cmds[0] != "go test -short ./"+path.Dir(file.File)+"/" {
			t.Errorf("%s: first command %q, want the package whole", file.File, cmds[0])
		}
		inRun := runCovers(cmds)
		near := map[goTest]bool{}
		addNear := func(n *Node) {
			for _, r := range append([]*Node{n}, ix.fixtureRunners(n)...) {
				if gt, ok := goTestOf(r); ok {
					near[gt] = true
				}
			}
		}
		for _, n := range s.own {
			addNear(n)
		}
		for _, a := range s.about {
			addNear(a.test)
		}
		var direct []*Node
		for _, id := range s.others {
			if s.all[id] == 0 {
				direct = append(direct, ix.byID[id])
			}
		}
		far := 0
		for _, n := range direct {
			if gt, ok := goTestOf(n); ok && gt.dir != path.Dir(file.File) && !near[gt] {
				far++
			}
		}
		if far <= narrowHubLimit {
			for _, n := range direct {
				addNear(n)
			}
		} else if p.hub != far {
			t.Errorf("%s: %d direct tests in other packages, the plan counts %d", file.File, far, p.hub)
		}
		// codegraph'ın kendi testleri iki klasörün dışında koşmaz, sayı satırı sayar (codegraph dilim 6, D6-3).
		for gt := range near {
			if !inRun(gt) && !slices.Contains(p.selfTests, gt) {
				t.Errorf("%s: the narrow run line does not run its near test %s (%s)", file.File, gt.name, gt.dir)
			}
		}
		for _, h := range ix.scansCovering(file.File) {
			for _, n := range h.tests {
				if gt, ok := goTestOf(n); ok && gt.dir != path.Dir(file.File) && inRun(gt) && !near[gt] {
					t.Errorf("%s: the narrow run line runs the scan %s (%s); scans are left to the commit check", file.File, gt.name, gt.dir)
				}
			}
		}
		for _, id := range s.others {
			if gt, ok := goTestOf(ix.byID[id]); ok && s.all[id] > 0 && gt.dir != path.Dir(file.File) && inRun(gt) && !near[gt] {
				t.Errorf("%s: the narrow run line runs %s (%s), %d steps away", file.File, gt.name, gt.dir, s.all[id])
			}
		}
		var printed strings.Builder
		sel := newTouchSet()
		sel.nodes, sel.files = ts.nodes, map[string]bool{file.File: true}
		ix.printGoTests(&printed, sel, false) // changed --go-tests'in yazdığı
		rest := map[goTest]bool{}
		for _, gt := range p.rest {
			rest[gt] = true
		}
		for _, line := range nonEmptyLines(printed.String()) {
			dir, name, _ := strings.Cut(line, " ")
			gt := goTest{dir, name}
			if slices.Contains(p.selfTests, gt) {
				if inRun(gt) || rest[gt] {
					t.Errorf("%s: %s is left out as a codegraph self-test but is run or counted", file.File, line)
				}
				continue
			}
			if inRun(gt) == rest[gt] {
				t.Errorf("%s: %s is %s by the run line and %s in the remaining count", file.File, line, map[bool]string{true: "run", false: "not run"}[inRun(gt)], map[bool]string{true: "also", false: "not"}[rest[gt]])
			}
			delete(rest, gt)
		}
		for gt := range rest {
			t.Errorf("%s: %v is counted but changed --go-tests does not select it", file.File, gt)
		}
	}
}

// runCovers koşu satırlarının `go test` komutlarını metinden okur: her komutun paketlerinde -run listesindeki adlar,
// listesi yoksa paketlerin bütünü.
func runCovers(cmds []string) func(goTest) bool {
	type command struct{ pkgs, names map[string]bool }
	var all []command
	for _, c := range cmds {
		args, run, _ := strings.Cut(strings.TrimPrefix(c, "go test -short "), " -run ")
		if args == c {
			continue // go test olmayan satır (trlang test)
		}
		k := command{pkgs: map[string]bool{}}
		for _, a := range strings.Fields(args) {
			k.pkgs[strings.TrimSuffix(strings.TrimPrefix(a, "./"), "/")] = true
		}
		if run != "" {
			k.names = map[string]bool{}
			for _, n := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(run, "'^("), ")$'"), "|") {
				k.names[n] = true
			}
		}
		all = append(all, k)
	}
	return func(gt goTest) bool {
		return slices.ContainsFunc(all, func(k command) bool { return k.pkgs[gt.dir] && (k.names == nil || k.names[gt.name]) })
	}
}

// Kural: koşu satırı Windows komut satırını aşmaz: -run listesi commandLineLimit'i geçince ikinci komut paketleri
// bütün olarak koşar (ön denetimin kuralı, tools/full_check.sh) ve bunu söyler.
func TestTheRunLineStaysWithinTheCommandLine(t *testing.T) {
	t.Parallel()
	var names []string
	for i := 0; len(strings.Join(names, "|")) <= commandLineLimit; i++ {
		names = append(names, "TestAVeryLongNameThatKeepsTheListGrowing"+strings.Repeat("x", i%10))
	}
	p := runPlan{pkg: "a", others: []string{"b"}, names: names}
	p.whole = len(p.command(p.others, p.names)) > commandLineLimit
	cmds := p.commands()
	if !p.whole || len(cmds) != 2 || cmds[1] != "go test -short ./b/" {
		t.Fatalf("a list past the limit runs the packages whole: %q", cmds)
	}
	var out strings.Builder
	printRunPlan(&out, p)
	if !strings.Contains(out.String(), "runs its packages whole") {
		t.Errorf("the answer says why:\n%s", out.String())
	}
}

// Kural (codegraph 7, A3): dar satırın altında tek sayı satırı vardır: kalanlar (codegraph'ın kendi testleri dahil) ayrı
// listelenmez, gruplardadır; kısa cevapta --list'e gönderir. Kalan yoksa satır her testin koştuğunu söyler; ne komut ne kalan
// varsa satır yazılmaz. Eski iki satır ("not run by these commands", "tests: N listed") tests-for'da yoktur.
func TestTheRemainingCountSaysWhereTheTestsAre(t *testing.T) {
	t.Parallel()
	two := []goTest{{"b", "TestX"}, {"c", "TestZ"}}
	for _, c := range []struct {
		p     runPlan
		short bool
		want  string
	}{
		{runPlan{pkg: "a", rest: two}, false, "  narrow: these lines run the near tests; 2 more are left to the commit check and G4 (among the groups below)\n"},
		{runPlan{pkg: "a", rest: two[:1], selfTests: two[1:]}, true, "  narrow: these lines run the near tests; 2 more are left to the commit check and G4 (--list names them)\n"},
		{runPlan{pkg: "a"}, false, "  these commands run every Go test the pre-check selects for this file\n"},
		{runPlan{}, true, ""},
		{runPlan{rest: two}, true, "  narrow: these lines run the near tests; 2 more are left to the commit check and G4 (--list names them)\n"},
	} {
		var out strings.Builder
		printRunPlanText(&out, c.p, c.short)
		got := strings.Join(slices.DeleteFunc(strings.SplitAfter(out.String(), "\n"), func(l string) bool { return strings.HasPrefix(l, "run: ") }), "")
		if got != c.want || strings.Contains(out.String(), "not run by these commands") || strings.Contains(out.String(), " listed (") {
			t.Errorf("%+v: got %q, want %q", c.p, got, c.want)
		}
	}
}

// Kural (codegraph 7, A2): başka paketlerdeki doğrudan testler narrowHubLimit'i aşınca dar satıra girmez; satır sayılarını
// söyler (S3) ve onları commit öncesi denetime bırakır. `vm.go`'nun başka paketlerde yüzlerce doğrudan testi var; `library.go`'nun
// on tanesi satıra girer. Hub satırı sayı satırından sonra gelir ve onun içini söyler (B6, DENETİM 750 N1): önce ve ayrı bir
// sayı gibi yazılınca iki satır toplanabiliyordu.
func TestTheNarrowRunLineLeavesHubTestsToTheCommitCheck(t *testing.T) {
	t.Parallel()
	var out strings.Builder
	printRunPlan(&out, runPlan{pkg: "a", hub: 21, rest: make([]goTest, 30)})
	if want := "  narrow: these lines run the near tests; 30 more are left to the commit check and G4 (among the groups below)\n" +
		"  narrow: 21 of them are direct tests in other packages (more than 20 stay off the run line)\n"; !strings.Contains(out.String(), want) {
		t.Errorf("the count line, then the hub line inside it:\n%s", out.String())
	}
	_, _, g := repoGraph(t)
	ix := newIndex(g)
	for _, c := range []struct {
		file    string
		hub     bool
		minHere int
	}{{"internal/lang/interp/vm.go", true, 0}, {"internal/lang/interp/library.go", false, 5}} {
		file := ix.byID[fileID(c.file)]
		ts := ix.fileDecls(file)
		p := ix.runPlanFor(file, ts, ix.fileTests(file, ts))
		if (p.hub > narrowHubLimit) != c.hub || !c.hub && (p.hub != 0 || len(p.names) < c.minHere) || c.hub && len(p.names) > narrowHubLimit {
			t.Errorf("%s: hub %d, %d names in other packages (%q)", c.file, p.hub, len(p.names), p.commands())
		}
	}
}

// untrackedGitFixture geçici bir git deposunda izlenen (`shape/shape.go`) ve izlenmeyen (`shape/untracked.go`)
// iki Go dosyası kurar ve grafiğini derler.
func untrackedGitFixture(t *testing.T) (string, string) {
	t.Helper()
	root := gitFixture(t)
	replaceIn(t, path.Join(root, "shape/shape_test.go"), "check(t, Square{2})", "check(t, Square{Untracked()})")
	gitMust(t, root, "add", "shape/shape_test.go")
	testkit.WriteFile(t, root, "shape/untracked.go", "package shape\n\nfunc Untracked() float64 { return 2 }\n")
	return root, buildInto(t, root)
}

// Kural: git tarafından izlenmeyen bir dosyada tests-for (run: satırlarından önce) ve changed --go-tests dosyanın
// izlenmediğini, depo taramalarının yalnız izlenen dosyaları gördüğünü ve `git add` gerektiğini aynı uyarıyla söyler;
// izlenen dosyada (ya da dosya `git add` ile eklendikten sonra) bu uyarı çıkmaz.
func TestUntrackedFileWarningInTestsForAndChangedGoTests(t *testing.T) {
	t.Parallel()
	root, graph := untrackedGitFixture(t)
	wantWarn := "untracked: shape/untracked.go is not tracked by git; repository-wide checks do not see it (run `git add shape/untracked.go`)"

	// 1. İzlenmeyen dosyada tests-for: uyarı run: satırlarından önce çıkar.
	code, out, errOut := commandIn("tests-for", root, graph, "--list", "--no-rebuild", "shape/untracked.go")
	if code != exitOK {
		t.Fatalf("tests-for untracked: exit %d: %s", code, errOut)
	}
	wIdx := strings.Index(out, wantWarn)
	rIdx := strings.Index(out, "\nrun: go test -short ./shape/")
	if wIdx < 0 || rIdx < 0 || wIdx > rIdx {
		t.Errorf("tests-for untracked must print warning before run: lines:\n%s", out)
	}

	// 2. İzlenen dosyada tests-for: uyarı çıkmaz.
	code, out, errOut = commandIn("tests-for", root, graph, "--list", "--no-rebuild", "shape/shape.go")
	if code != exitOK {
		t.Fatalf("tests-for tracked: exit %d: %s", code, errOut)
	}
	if strings.Contains(out, "untracked:") || strings.Contains(out, "not tracked by git") {
		t.Errorf("tests-for tracked file must not print untracked warning:\n%s", out)
	}

	// 3. İzlenmeyen dosya varken changed --go-tests: aynı uyarıyı verir ve dosyanın testlerini seçer.
	code, out, errOut = commandIn("changed", root, graph, "--no-rebuild", "--go-tests", "HEAD")
	if code != exitOK {
		t.Fatalf("changed --go-tests with untracked: exit %d: %s", code, errOut)
	}
	if !strings.Contains(out, wantWarn) || !strings.Contains(out, "shape TestDescribe") {
		t.Errorf("changed --go-tests must warn about untracked file and select its tests:\n%s", out)
	}

	// 4. Dosya `git add` ile izlenmeye başlanınca iki komutta da uyarı kalkar.
	gitMust(t, root, "add", "shape/untracked.go")
	code, out, errOut = commandIn("tests-for", root, graph, "--list", "--no-rebuild", "shape/untracked.go")
	if code != exitOK || strings.Contains(out, "untracked:") {
		t.Errorf("after git add, tests-for must not warn: exit %d\n%s%s", code, out, errOut)
	}
	code, out, errOut = commandIn("changed", root, graph, "--no-rebuild", "--go-tests", "HEAD")
	if code != exitOK || strings.Contains(out, "untracked:") || !strings.Contains(out, "shape TestDescribe") {
		t.Errorf("after git add, changed --go-tests must not warn: exit %d\n%s%s", code, out, errOut)
	}
}

// Kural: belge dosyası (.md) için tests-for dosyayı adıyla okuyan testlerden bir `run:` satırı kurar
// (TestReviewedSectionsWriteIdentities); dosyayı yalnız tarayan altın dosya testi (TestDottedMemberSpellingsGolden) dar
// satırda yoktur (codegraph 7 A1). Altın dosya testi satırdaysa (definitions.json'ı gezen) `-update` notu yazılır; testsiz
// bir `.md` dosyasında `run:` satırı yoktur.
func TestDocFileRunLineIncludesReadersAndGoldenScans(t *testing.T) {
	t.Parallel()
	_, _, g := repoGraph(t)
	ix := newIndex(g)
	doc := ix.byID["doc:docs/status.md"]
	if doc == nil {
		t.Fatal("doc:docs/status.md not found")
	}
	var out strings.Builder
	ix.printTestsFor(&out, doc, 12)
	got := out.String()
	run := strings.Join(runLines(got), "\n")
	if !strings.Contains(run, "TestReviewedSectionsWriteIdentities") || strings.Contains(run, "TestDottedMemberSpellingsGolden") {
		t.Errorf("tests-for docs/status.md must run its reader TestReviewedSectionsWriteIdentities and leave the scan TestDottedMemberSpellingsGolden:\n%s", got)
	}
	out.Reset()
	ix.printTestsFor(&out, ix.byID["file:internal/lang/codes/definitions.json"], 12)
	if !strings.Contains(out.String(), "golden files (TestDottedMemberSpellingsGolden): if they change, rewrite with -update and read the diff") {
		t.Errorf("tests-for definitions.json must explain -update for golden tests:\n%s", out.String())
	}

	root := auditModule(t, map[string]string{
		"p/p.go":        "package p\n",
		"docs/empty.md": "# Başlık\n",
	})
	fg, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	fix := newIndex(fg)
	untestedDoc := fix.byID["doc:docs/empty.md"]
	if untestedDoc == nil {
		t.Fatal("doc:docs/empty.md not found")
	}
	out.Reset()
	fix.printTestsFor(&out, untestedDoc, 12)
	if strings.Contains(out.String(), "run:") {
		t.Errorf("untested .md file must not have a run: line:\n%s", out.String())
	}
}

// Kural: Go dışı dosyalar (betik, veri dosyası) için tests-for doğrudan okuyanları (en çok narrowHubLimit; fazlası sayılır)
// ve bütün listeyi gezenleri (ilk sayfa dışındakiler dahil) kapsayan dar `run:` satırı kurar; taramalar satırda yoktur,
// kalanlar sayılır (codegraph 7 A1–A3). Ağır paketleri -run'sız koşmaz; hiçbir testi ve taraması olmayan yalıtılmış
// betikte boş `run:` basılmaz.
func TestNonGoFileRunLineCoversDirectTestsWalkersAndScans(t *testing.T) {
	t.Parallel()
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	ix.root = root

	// 1. Kabuk ve Python betiği (Madde 4): tools/full_check.sh ve tools/bench/run.py.
	for _, rel := range []string{"tools/full_check.sh", "tools/bench/run.py"} {
		n := ix.byID["file:"+rel]
		if n == nil {
			t.Fatalf("%s node not found", rel)
		}
		var out strings.Builder
		ix.printTestsFor(&out, n, defaultLimit)
		cmds := runLines(out.String())
		if len(cmds) == 0 {
			t.Fatalf("%s: expected run: line, got:\n%s", rel, out.String())
		}
		inRun := runCovers(cmds)
		if rel == "tools/full_check.sh" {
			nt := ix.testsForNode(n)
			direct, _ := ix.splitSharedSites(nt.tests)
			if len(direct) < 10 {
				t.Fatalf("tools/full_check.sh: expected >=10 direct tests, got %d", len(direct))
			}
			self := ix.selfTestsOf(n)
			readers := map[goTest]bool{}
			for _, e := range direct {
				if gt, ok := goTestOf(ix.byID[e.From]); ok && isTestFile(e.File) {
					readers[gt] = true
				}
			}
			hub := strings.Contains(out.String(), "are direct tests in other packages (more than")
			for gt := range readers {
				if !hub && !inRun(gt) && !slices.Contains(self, gt) {
					t.Errorf("tools/full_check.sh: direct test %s (%s) not covered by run: line", gt.name, gt.dir)
				}
			}
			if hub != (len(readers) > narrowHubLimit) {
				t.Errorf("tools/full_check.sh: %d readers, the hub line is %v:\n%s", len(readers), hub, out.String())
			}
		}
	}

	// 2. Veri dosyası ve bütün listeyi gezenler (Madde 6): internal/lang/codes/definitions.json.
	defs := ix.byID["file:internal/lang/codes/definitions.json"]
	if defs == nil {
		t.Fatal("definitions.json node not found")
	}
	var out strings.Builder
	ix.printTestsFor(&out, defs, defaultLimit)
	defsOut := out.String()
	cmds := runLines(defsOut)
	inRun := runCovers(cmds)
	nt := ix.testsForNode(defs)
	if len(nt.walks) < 40 {
		t.Fatalf("definitions.json: expected >=40 whole-list walkers, got %d", len(nt.walks))
	}
	self := ix.selfTestsOf(defs) // codegraph'ın kendi testleri S3'te sayılır (codegraph dilim 6, D6-3)
	for _, e := range nt.walks {
		if gt, ok := goTestOf(ix.byID[e.From]); ok && !inRun(gt) && !slices.Contains(self, gt) {
			t.Errorf("definitions.json: whole-list walker %s (%s) not covered by run: line", gt.name, gt.dir)
		}
	}
	if !inRun(goTest{"cmd/trlang/e2e", "TestExamplesCoverNames"}) {
		t.Errorf("definitions.json: beyond-first-page walker e2e.TestExamplesCoverNames must be in run: line:\n%v", cmds)
	}
	for _, cmd := range cmds {
		args, _, hasRun := strings.Cut(strings.TrimPrefix(cmd, "go test -short "), " -run ")
		for _, a := range strings.Fields(args) {
			if d := strings.Trim(a, "./"); isHeavyPackage(d) && !hasRun {
				t.Errorf("definitions.json: heavy package %s must not run whole: %s", d, cmd)
			}
		}
	}
	if !strings.Contains(defsOut, "  narrow: these lines run the near tests; ") {
		t.Errorf("definitions.json: remaining embed users must be counted:\n%s", defsOut)
	}
	for _, h := range ix.scansCovering(defs.File) {
		for _, sc := range h.tests {
			isFrom := func(e *Edge) bool { return e.From == sc.ID }
			if gt, ok := goTestOf(sc); ok && inRun(gt) && !slices.ContainsFunc(nt.walks, isFrom) && !slices.ContainsFunc(nt.tests, isFrom) &&
				!slices.ContainsFunc(nt.own, func(o *Node) bool { return o.ID == sc.ID }) {
				t.Errorf("definitions.json: the scan %s (%s) is in the narrow run line", gt.name, gt.dir)
			}
		}
	}

	// 3. Olumsuz: hiçbir testi ve taraması olmayan yalıtılmış betik dosyasında boş `run:` basılmaz.
	fixRoot := auditModule(t, map[string]string{
		"p/p.go":           "package p\n",
		"scripts/empty.sh": "#!/bin/sh\necho ok\n",
	})
	fg, err := buildGraph(fixRoot)
	if err != nil {
		t.Fatal(err)
	}
	fix := newIndex(fg)
	emptyScript := fix.byID["file:scripts/empty.sh"]
	if emptyScript == nil {
		t.Fatal("file:scripts/empty.sh not found")
	}
	out.Reset()
	fix.printTestsFor(&out, emptyScript, defaultLimit)
	if strings.Contains(out.String(), "run:") {
		t.Errorf("untested script must not print a run: line:\n%s", out.String())
	}
}
