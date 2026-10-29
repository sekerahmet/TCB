package main

import (
	"path"
	"sort"
	"strings"
	"testing"
)

// runPlanFiles koşu satırı sınanan dosyalardır: olayın dosyası (2026-10-29) ve depodan sıralı bir örnek.
func runPlanFiles(g *Graph) []*Node {
	var out []*Node
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if n.Kind == kindFile && strings.HasSuffix(n.File, ".go") && !strings.Contains(n.File, "/testdata/") &&
			(n.File == "internal/lang/interp/compiler_assign.go" || strings.HasPrefix(n.File, "internal/lang/interp/") && i%7 == 0) {
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
		p := ix.runPlanFor(file, ts, s.about)
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

// runCovers koşu satırının komutlarını metinden okur: ilk komutun paketi bütün olarak, ikincinin paketlerinde -run
// listesindeki adlar (listesi yoksa bütün paketler).
func runCovers(cmds []string) func(goTest) bool {
	pkgs, names := map[string]bool{}, map[string]bool{}
	whole := map[string]bool{strings.TrimSuffix(strings.TrimPrefix(cmds[0], "go test -short ./"), "/"): true}
	if len(cmds) > 1 {
		args, run, _ := strings.Cut(strings.TrimPrefix(cmds[1], "go test -short "), " -run ")
		for _, a := range strings.Fields(args) {
			pkgs[strings.TrimSuffix(strings.TrimPrefix(a, "./"), "/")] = true
		}
		for _, n := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(run, "'^("), ")$'"), "|") {
			names[n] = true
		}
		if run == "" {
			for d := range pkgs {
				whole[d] = true
			}
		}
	}
	return func(gt goTest) bool { return whole[gt.dir] || pkgs[gt.dir] && names[gt.name] }
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
		if !strings.Contains(out.String(), "not run by these commands: "+c.want+"; before committing") {
			t.Errorf("want %q:\n%s", c.want, out.String())
		}
	}
}
