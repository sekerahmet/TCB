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

// Kural: tests-for <Go dosyası>'nın koşu satırı ön denetimin seçimini eksiltmez. İlk komut dosyanın paketini bütün
// olarak koşar; ikincisi dosyayı tarayan bütün depo kurallarını ve dosyanın bir bildirimini konu edinen Go testlerini
// (DENETİM 2026-10-29: compiler_assign.go'nun TestLayoutFuncSize'ı commit'ten sonra düştü). Seçimin geri kalanı
// `changed --go-tests`'in aynı fonksiyonundan (goTestSelection) sayılır: komutların koştuğu ile sayılan, seçimin tamamıdır.
func TestTheRunLineCoversThePackageTheScansAndTheTestsAboutIt(t *testing.T) {
	_, _, g := repoGraph(t)
	ix := newIndex(g)
	files := runPlanFiles(g)
	if len(files) < 5 || files[0].File > "internal/lang/interp/compiler_assign.go" {
		t.Fatalf("only %d files sampled", len(files))
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
		var want []*Node
		for _, h := range ix.scansCovering(file.File) {
			want = append(want, h.tests...)
		}
		for _, a := range s.about {
			want = append(want, a.test)
			want = append(want, ix.fixtureRunners(a.test)...)
		}
		for _, t := range ix.checkerRunnerTests(file, s.all) {
			want = append(want, t)
			want = append(want, ix.fixtureRunners(t)...)
		}
		for _, n := range want {
			if gt, ok := goTestOf(n); ok && !inRun(gt) {
				t.Errorf("%s: the run line does not run %s (%s)", file.File, n.Name, gt.dir)
			}
		}
		var printed strings.Builder
		sel := newTouchSet()
		sel.nodes, sel.files = ts.nodes, map[string]bool{file.File: true}
		ix.printGoTests(&printed, sel) // changed --go-tests'in yazdığı
		rest := map[goTest]bool{}
		for _, gt := range p.rest {
			rest[gt] = true
		}
		for _, line := range nonEmptyLines(printed.String()) {
			dir, name, _ := strings.Cut(line, " ")
			gt := goTest{dir, name}
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

// Kural: koşulmayanların satırı ayrı bir liste vaat etmez (onlar aşağıdaki gruplardadır) ve sayıya göre tekil ya da
// çoğul yazar (DENETİM, birleşme denetimi: "in 1 packages", "(listed below)").
func TestTheRemainingCountSaysWhereTheTestsAre(t *testing.T) {
	for _, c := range []struct {
		rest []goTest
		want string
	}{
		{[]goTest{{"b", "TestX"}}, "1 more Go test in 1 package (among the groups below)"},
		{[]goTest{{"b", "TestX"}, {"b", "TestY"}, {"c", "TestZ"}}, "3 more Go tests in 2 packages (among the groups below)"},
	} {
		var out strings.Builder
		printRunPlan(&out, runPlan{pkg: "a", rest: c.rest})
		if !strings.Contains(out.String(), "not run by these commands: "+c.want+"; the full check") {
			t.Errorf("want %q:\n%s", c.want, out.String())
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
	root, graph := untrackedGitFixture(t)
	wantWarn := "untracked: shape/untracked.go is not tracked by git; repository-wide checks do not see it (run `git add shape/untracked.go`)"

	// 1. İzlenmeyen dosyada tests-for: uyarı run: satırlarından önce çıkar.
	code, out, errOut := commandIn("tests-for", root, graph, "--no-rebuild", "shape/untracked.go")
	if code != exitOK {
		t.Fatalf("tests-for untracked: exit %d: %s", code, errOut)
	}
	wIdx := strings.Index(out, wantWarn)
	rIdx := strings.Index(out, "\nrun: go test -short ./shape/")
	if wIdx < 0 || rIdx < 0 || wIdx > rIdx {
		t.Errorf("tests-for untracked must print warning before run: lines:\n%s", out)
	}

	// 2. İzlenen dosyada tests-for: uyarı çıkmaz.
	code, out, errOut = commandIn("tests-for", root, graph, "--no-rebuild", "shape/shape.go")
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
	code, out, errOut = commandIn("tests-for", root, graph, "--no-rebuild", "shape/untracked.go")
	if code != exitOK || strings.Contains(out, "untracked:") {
		t.Errorf("after git add, tests-for must not warn: exit %d\n%s%s", code, out, errOut)
	}
	code, out, errOut = commandIn("changed", root, graph, "--no-rebuild", "--go-tests", "HEAD")
	if code != exitOK || strings.Contains(out, "untracked:") || !strings.Contains(out, "shape TestDescribe") {
		t.Errorf("after git add, changed --go-tests must not warn: exit %d\n%s%s", code, out, errOut)
	}
}

// Kural: belge dosyası (.md) için tests-for dosyayı adıyla okuyan ve tarayan testlerden bir `run:` satırı kurar
// (TestReviewedSectionsWriteIdentities ve TestDottedMemberSpellingsGolden dahil) ve altın dosya testi seçildiğinde
// `-update` notunu yazar; testsiz bir `.md` dosyasında `run:` satırı yoktur.
func TestDocFileRunLineIncludesReadersAndGoldenScans(t *testing.T) {
	_, _, g := repoGraph(t)
	ix := newIndex(g)
	doc := ix.byID["doc:docs/status.md"]
	if doc == nil {
		t.Fatal("doc:docs/status.md not found")
	}
	var out strings.Builder
	ix.printTestsFor(&out, doc, 12)
	got := out.String()
	if !strings.Contains(got, "\nrun: go test -short ") ||
		!strings.Contains(got, "TestReviewedSectionsWriteIdentities") ||
		!strings.Contains(got, "TestDottedMemberSpellingsGolden") {
		t.Errorf("tests-for docs/status.md must print a run: line with TestReviewedSectionsWriteIdentities and TestDottedMemberSpellingsGolden:\n%s", got)
	}
	if !strings.Contains(got, "golden files (TestDottedMemberSpellingsGolden): if they change, rewrite with -update and read the diff") {
		t.Errorf("tests-for docs/status.md must explain -update for golden tests:\n%s", got)
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
