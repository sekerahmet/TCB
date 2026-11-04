package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/devtools/goload"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// typeQueryExclusion tip yükleyicisine ulaştığı hâlde depo kaynaklarını sorgulamayan (ör. yalnız t.TempDir() yükleyen)
// bir birim testin gerekçeli dışlama kaydıdır.
type typeQueryExclusion struct {
	pkg, test, reason string
}

// repoTypeQueryExclusions tools/codegraph dışında goload.Load ya da (*types.Config).Check'e ulaşıp yalnızca geçici
// test modüllerini yükleyen birim testlerdir.
var repoTypeQueryExclusions = []typeQueryExclusion{
	{pkg: "internal/devtools/goload", test: "TestLoadChecksPackagesAndTests", reason: "yalnız t.TempDir() içindeki geçici test modülünü yükler"},
	{pkg: "internal/devtools/goload", test: "TestLoadRejectsAFolderWithoutGoPackages", reason: "yalnız boş t.TempDir() klasörünü yükler"},
	{pkg: "internal/devtools/goload", test: "TestRelAndRecvName", reason: "yalnız t.TempDir() içindeki geçici test modülünü yükler"},
	{pkg: "internal/devtools/goload", test: "TestLoadTypesOption", reason: "yalnız t.TempDir() içindeki geçici test modülünü yükler"},
	{pkg: "internal/devtools/goload", test: "TestSkipIgnoredLeavesOutPackagesGitIgnores", reason: "yalnız t.TempDir() içindeki geçici test modülünü yükler"},
}

// isTypeLoaderCall verilen çağrı hedefinin goload.Load ya da (*types.Config).Check olup olmadığını döner.
func isTypeLoaderCall(modPath string, fn *types.Func) bool {
	if fn == nil || fn.Pkg() == nil {
		return false
	}
	if fn.Name() == "Load" && fn.Pkg().Path() == modPath+"/internal/devtools/goload" {
		return true
	}
	if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
		return fn.Name() == "Check" && fn.Pkg().Path() == "go/types" && goload.RecvName(sig.Recv().Type()) == "Config"
	}
	return false
}

// calleeFuncIdent çağrı ifadesinin ad kimliğini çözer.
func calleeFuncIdent(e ast.Expr) *ast.Ident {
	switch x := e.(type) {
	case *ast.Ident:
		return x
	case *ast.SelectorExpr:
		return x.Sel
	case *ast.IndexExpr:
		return calleeFuncIdent(x.X)
	case *ast.IndexListExpr:
		return calleeFuncIdent(x.X)
	case *ast.ParenExpr:
		return calleeFuncIdent(x.X)
	}
	return nil
}

// typeLoaderSeeds modülde doğrudan goload.Load ya da (*types.Config).Check çağıran bildirim düğümü kimliklerini bulur.
func typeLoaderSeeds(m *module) []string {
	seen := map[string]bool{}
	add := func(id string) {
		if id != "" {
			seen[id] = true
		}
	}
	for _, u := range m.Units {
		for _, f := range u.Files {
			for _, decl := range f.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					fnObj, _ := u.Info.Defs[d.Name].(*types.Func)
					if fnObj == nil || d.Body == nil {
						continue
					}
					ast.Inspect(d.Body, func(n ast.Node) bool {
						if c, ok := n.(*ast.CallExpr); ok {
							if callee, _ := u.Info.Uses[calleeFuncIdent(c.Fun)].(*types.Func); isTypeLoaderCall(m.Path, callee) {
								add(funcRefIn(m.Fset, fnObj))
							}
						}
						return true
					})
				case *ast.GenDecl:
					for _, sp := range d.Specs {
						vs, ok := sp.(*ast.ValueSpec)
						if !ok {
							continue
						}
						hasCall := false
						ast.Inspect(vs, func(n ast.Node) bool {
							if c, ok := n.(*ast.CallExpr); ok {
								if callee, _ := u.Info.Uses[calleeFuncIdent(c.Fun)].(*types.Func); isTypeLoaderCall(m.Path, callee) {
									hasCall = true
								}
							}
							return true
						})
						if hasCall {
							for _, nm := range vs.Names {
								if obj := u.Info.Defs[nm]; obj != nil && obj.Pkg() != nil {
									add(valueID("var", obj.Pkg().Path(), nm.Name))
								}
							}
						}
					}
				}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// reachedTypeLoadingTests tools/codegraph dışında tip yükleyicisine çağrı ya da değişken okuma zinciriyle ulaşan
// bütün testleri ("klasör.TestAdı" biçiminde) bulur.
func reachedTypeLoadingTests(m *module, g *Graph) map[string]bool {
	byID := map[string]*Node{}
	for i := range g.Nodes {
		byID[g.Nodes[i].ID] = &g.Nodes[i]
	}
	rev := map[string][]string{}
	for _, e := range g.Edges {
		if e.Kind == edgeCalls || e.Kind == edgeReadsVar {
			rev[e.To] = append(rev[e.To], e.From)
		}
	}
	visited := map[string]bool{}
	queue := typeLoaderSeeds(m)
	for _, id := range queue {
		visited[id] = true
	}
	out := map[string]bool{}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if n := byID[cur]; isTestNode(n) {
			short := n.Name[strings.LastIndex(n.Name, ".")+1:]
			dir := path.Dir(n.File)
			if !strings.HasPrefix(short, "Benchmark") && dir != "tools/codegraph" {
				out[dir+"."+short] = true
			}
		}
		for _, caller := range rev[cur] {
			if !visited[caller] {
				visited[caller] = true
				queue = append(queue, caller)
			}
		}
	}
	return out
}

// checkTypeQueryCoverage tools/codegraph dışında goload.Load ya da (*types.Config).Check'e ulaşan her testin ya kapsam
// listesinde ya da gerekçeli dışlamada bulunduğunu ve listede bayat/gerekçesiz kayıt olmadığını doğrular.
func checkTypeQueryCoverage(m *module, g *Graph, rules []typeQueryRule, exclusions []typeQueryExclusion) []string {
	var errs []string
	listed := map[string]bool{}
	record := func(pkg, name, reason string) {
		key := pkg + "." + name
		if pkg == "" || name == "" || strings.TrimSpace(reason) == "" {
			errs = append(errs, fmt.Sprintf("missing pkg/test/reason on %q", key))
			return
		}
		if listed[key] {
			errs = append(errs, fmt.Sprintf("duplicate type-query entry %s", key))
			return
		}
		listed[key] = true
	}
	for _, r := range rules {
		if len(r.tests) == 0 {
			errs = append(errs, fmt.Sprintf("rule for %q has no tests", r.pkg))
		}
		for _, name := range r.tests {
			record(r.pkg, name, r.reason)
		}
	}
	for _, ex := range exclusions {
		record(ex.pkg, ex.test, ex.reason)
	}
	reached := reachedTypeLoadingTests(m, g)
	for key := range reached {
		if !listed[key] {
			errs = append(errs, fmt.Sprintf("unlisted type-loading test %s (add to repoTypeQueryRules or repoTypeQueryExclusions)", key))
		}
	}
	for key := range listed {
		if !reached[key] {
			errs = append(errs, fmt.Sprintf("stale type-query list entry %s (does not reach goload.Load or (*types.Config).Check)", key))
		}
	}
	sort.Strings(errs)
	return errs
}

// Kural: tools/codegraph dışında goload.Load ya da (*types.Config).Check'e ulaşan her test ya gerekçeli kapsam
// listesinde (repoTypeQueryRules) ya da gerekçeli dışlamadadır (repoTypeQueryExclusions); listeden bir satır silinince
// kapsam denetimi düşer; seçilen testler kapsamlarındaki her Go dosyasında (silinen dosya yolu dahil) seçilir.
func TestRepositoryTypeLoadedTestsAreSelected(t *testing.T) {
	root, m, g := repoGraph(t)
	if errs := checkTypeQueryCoverage(m, g, repoTypeQueryRules, repoTypeQueryExclusions); len(errs) > 0 {
		t.Fatalf("type-query coverage errors:\n  %s", strings.Join(errs, "\n  "))
	}

	// Listeden bir test ya da bir dışlama satırı silinince kapsam testi düşmelidir.
	trimmedRule := slices.Clone(repoTypeQueryRules)
	trimmedRule[0].tests = slices.Clone(trimmedRule[0].tests[1:])
	if errs := checkTypeQueryCoverage(m, g, trimmedRule, repoTypeQueryExclusions); len(errs) == 0 {
		t.Fatal("deleting one test from repoTypeQueryRules must fail checkTypeQueryCoverage")
	}
	if errs := checkTypeQueryCoverage(m, g, repoTypeQueryRules[:1], repoTypeQueryExclusions); len(errs) == 0 {
		t.Fatal("deleting the tensor gate rule from repoTypeQueryRules must fail checkTypeQueryCoverage")
	}
	if errs := checkTypeQueryCoverage(m, g, repoTypeQueryRules, repoTypeQueryExclusions[1:]); len(errs) == 0 {
		t.Fatal("deleting one entry from repoTypeQueryExclusions must fail checkTypeQueryCoverage")
	}

	graphPath := filepath.Join(t.TempDir(), "graph.json")
	if err := writeGraph(graphPath, g); err != nil {
		t.Fatal(err)
	}
	code, tfOut, errOut := commandIn("tests-for", root, graphPath, "--no-rebuild", "internal/stdlib/ml/members_freeze.go")
	if code != exitOK {
		t.Fatalf("tests-for members_freeze.go: exit %d: %s", code, errOut)
	}
	for _, want := range []string{
		"type-loaded checks that cover it",
		"impact.TestInterfaceImplementers",
		"impact.TestMembersAndPackages",
		"impact.TestNameUses",
		"impact.TestUnknownNames",
	} {
		testkit.MustContain(t, tfOut, want)
	}

	ix := newIndex(g)
	selectForFile := func(rel string) string {
		ts := newTouchSet()
		ts.files[rel] = true
		var b strings.Builder
		ix.printGoTests(&b, ts)
		return "\n" + b.String()
	}
	for _, rel := range []string{"internal/stdlib/ml/members_freeze.go", "internal/stdlib/ml/deleted_impl.go"} {
		sel := selectForFile(rel)
		for _, name := range []string{"TestInterfaceImplementers", "TestMembersAndPackages", "TestNameUses", "TestUnknownNames"} {
			testkit.MustContain(t, sel, "\ntools/impact "+name+"\n")
		}
		if strings.Contains(sel, "\ninternal/compute/tensor TestBackendDataCallsGoThroughTheGate\n") {
			t.Errorf("%s: gate test must stay limited to internal/compute/tensor scope:\n%s", rel, sel)
		}
	}
	tensorSel := selectForFile("internal/compute/tensor/tensor.go")
	testkit.MustContain(t, tensorSel, "\ninternal/compute/tensor TestBackendDataCallsGoThroughTheGate\n")
	if strings.Contains(selectForFile("internal/compute/tensor/gate_calls_test.go"), "\ninternal/compute/tensor TestBackendDataCallsGoThroughTheGate\n") {
		t.Errorf("gate_calls_test.go is a _test.go file excluded by the gate scope")
	}
	if edge(g, "go:func:github.com/sekerahmet/TCB/tools/codegraph.TestANameCodeAliasIsAUseOfTheCode", "go:file:internal/lang/interp/assign.go", edgeTests) != nil {
		t.Errorf("whole-graph extractor consistency test must not gain type-query edges to unrelated packages")
	}
}

// writeTypeQueryFixtureFiles goload.Load ile depoyu ve geçici modülü yükleyen fikstür deposunu yazar.
func writeTypeQueryFixtureFiles(t *testing.T, root string) {
	t.Helper()
	testkit.WriteFile(t, root, "internal/reporoot/root.go", "package reporoot\n\nfunc Root() string { return \".\" }\n")
	testkit.WriteFile(t, root, "internal/devtools/goload/goload.go", "package goload\n\nimport \"go/types\"\n\n"+
		"type Options struct{}\n\n"+
		"type Unit struct {\n\tPkg  *types.Package\n\tInfo *types.Info\n}\n\n"+
		"type Module struct{ Units []*Unit }\n\n"+
		"func Load(string, Options) (*Module, error) { return &Module{}, nil }\n")
	testkit.WriteFile(t, root, "api/api.go", "package api\n\ntype Runner interface{ Run() }\n\nconst TargetConst = 1\n")
	testkit.WriteFile(t, root, "sub/sub.go", "package sub\n\nfunc SubVal() int { return 1 }\n")
	testkit.WriteFile(t, root, "worker/worker.go", "package worker\n\ntype Worker struct{}\n\nfunc (*Worker) Run() {}\n")
	testkit.WriteFile(t, root, "client/client.go", "package client\n\nimport (\n\t\"example.com/mini/api\"\n\t\"example.com/mini/sub\"\n)\n\nfunc Use() int { return api.TargetConst + sub.SubVal() }\n")
	testkit.WriteFile(t, root, "other/other.go", "package other\n\nfunc Unrelated() int { return 0 }\n")
	testkit.WriteFile(t, root, "audit/audit_test.go", `package audit

import (
	"go/types"
	"testing"

	"example.com/mini/internal/devtools/goload"
	"example.com/mini/internal/reporoot"
)

func loadRepo(t *testing.T) *goload.Module {
	t.Helper()
	m, err := goload.Load(reporoot.Root(), goload.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestRunnerImplementers(t *testing.T) {
	m := loadRepo(t)
	var iface *types.Interface
	for _, u := range m.Units {
		if obj := u.Pkg.Scope().Lookup("Runner"); obj != nil {
			iface = obj.Type().Underlying().(*types.Interface)
		}
	}
	for _, u := range m.Units {
		for _, n := range u.Pkg.Scope().Names() {
			if obj := u.Pkg.Scope().Lookup(n); obj != nil && types.Implements(types.NewPointer(obj.Type()), iface) {
				_ = obj
			}
		}
	}
}

func TestSymbolUses(t *testing.T) {
	for _, u := range loadRepo(t).Units {
		for _, o := range u.Info.Uses {
			if o != nil && o.Name() == "TargetConst" {
				_ = o
			}
		}
	}
}

func TestPackageUsers(t *testing.T) {
	for _, u := range loadRepo(t).Units {
		for _, o := range u.Info.Uses {
			if o != nil && o.Pkg() != nil && o.Pkg().Path() == "example.com/mini/sub" {
				_ = o
			}
		}
	}
}

func TestTempModuleIgnored(t *testing.T) {
	m, _ := goload.Load(t.TempDir(), goload.Options{})
	for _, u := range m.Units {
		if obj := u.Pkg.Scope().Lookup("Runner"); obj != nil {
			_ = types.Implements(obj.Type(), obj.Type().Underlying().(*types.Interface))
		}
	}
}
`)
}

// buildFixtureGraphWithRules fikstür kökünü yükleyip verilen kapsam kurallarıyla grafiği yazar.
func buildFixtureGraphWithRules(t *testing.T, root string, rules []typeQueryRule) (*module, *Graph, string) {
	t.Helper()
	m, err := load(root)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	g := buildGraphFromWithRules(root, m, rules)
	graphPath := filepath.Join(t.TempDir(), "graph.json")
	if err := writeGraph(graphPath, g); err != nil {
		t.Fatal(err)
	}
	return m, g, graphPath
}

// Kural (DENETİM 101 Ş2 / 104): değişiklikten SONRA kurulan grafikte dahi (ön denetim ve edithook senaryosu) beş
// tablo satırının tamamı ve kapsam içindeki diğer Go değişiklikleri tip yüklemeli audit testlerini seçer; t.TempDir()
// yükleyen TestTempModuleIgnored seçilmez.
func TestTypeQueryScopeSelectsEveryGoChange(t *testing.T) {
	slowFixture(t)
	root := gitFixture(t)
	writeTypeQueryFixtureFiles(t, root)
	commitAll(t, root, "setup")

	rules := []typeQueryRule{{
		pkg:    "audit",
		tests:  []string{"TestRunnerImplementers", "TestSymbolUses", "TestPackageUsers"},
		scope:  scanScope{bases: []string{"."}, inc: []string{".go"}},
		reason: "goload.Load ile fikstür deposunu yükleyip arayüz, ad ve paket sorgular",
	}}
	exclusions := []typeQueryExclusion{{
		pkg: "audit", test: "TestTempModuleIgnored", reason: "yalnız t.TempDir() içindeki geçici modülü yükler",
	}}
	baseMod, baseGraph, _ := buildFixtureGraphWithRules(t, root, rules)
	if errs := checkTypeQueryCoverage(baseMod, baseGraph, rules, exclusions); len(errs) > 0 {
		t.Fatalf("fixture coverage errors: %v", errs)
	}
	if errs := checkTypeQueryCoverage(baseMod, baseGraph, rules, nil); len(errs) == 0 {
		t.Fatal("omitting TestTempModuleIgnored from exclusions must fail checkTypeQueryCoverage")
	}

	cases := []struct {
		branch string
		mutate func()
	}{
		{
			branch: "claude/body_changed",
			mutate: func() {
				testkit.WriteFile(t, root, "worker/worker.go", "package worker\n\ntype Worker struct{ v int }\n\nfunc (*Worker) Run() {}\n")
			},
		},
		{
			branch: "claude/method_renamed",
			mutate: func() {
				testkit.WriteFile(t, root, "worker/worker.go", "package worker\n\ntype Worker struct{}\n\nfunc (*Worker) Step() {}\n")
			},
		},
		{
			branch: "claude/impl_deleted",
			mutate: func() {
				if err := os.Remove(filepath.Join(root, "worker", "worker.go")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			branch: "claude/use_deleted",
			mutate: func() {
				testkit.WriteFile(t, root, "client/client.go", "package client\n\nfunc Use() int { return 0 }\n")
			},
		},
		{
			branch: "claude/bare_name_collision",
			mutate: func() {
				testkit.WriteFile(t, root, "extra/extra.go", "package extra\n\ntype Runner interface{ Run() }\n\nconst TargetConst = 2\n")
			},
		},
		{
			branch: "claude/unrelated",
			mutate: func() {
				testkit.WriteFile(t, root, "other/other.go", "package other\n\nfunc Unrelated() int { return 42 }\n")
			},
		},
	}

	want := []string{"audit TestRunnerImplementers", "audit TestSymbolUses", "audit TestPackageUsers"}
	for _, tc := range cases {
		gitMust(t, root, "checkout", "-q", "-b", tc.branch, "main")
		tc.mutate()
		commitAll(t, root, tc.branch)
		// Ön denetim ve edithook grafiği değişiklikten SONRA bu dalın çalışma ağacından kurar.
		_, _, postGraph := buildFixtureGraphWithRules(t, root, rules)
		code, out, errOut := commandIn("changed", root, postGraph, "--no-rebuild", "--go-tests", tc.branch)
		if code != exitOK {
			t.Fatalf("%s: exit %d: %s", tc.branch, code, errOut)
		}
		lines := "\n" + out
		for _, w := range want {
			if !strings.Contains(lines, "\n"+w+"\n") {
				t.Errorf("%s: missing %q in:\n%s", tc.branch, w, out)
			}
		}
		if strings.Contains(lines, "\naudit TestTempModuleIgnored\n") {
			t.Errorf("%s: TestTempModuleIgnored must remain unselected:\n%s", tc.branch, out)
		}
		gitMust(t, root, "checkout", "-q", "main")
	}
}
